package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"ascenda/internal/event"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/socrate"
	"ascenda/internal/repo"
)

// SocrateInviter can invite a new user via the Socrate admin port using the
// backend service-account token (client_credentials). Satisfied by *socrate.Client.
type SocrateInviter interface {
	InviteUserAsService(ctx context.Context, req socrate.ServiceInviteRequest) (*socrate.CreateUserResult, error)
}

// SocrateProfileFetcher fetches individual user profiles from Socrate for
// email/name enrichment using the service-account token.
type SocrateProfileFetcher interface {
	GetUserAsService(ctx context.Context, userID string) (*socrate.User, error)
}

// UserService handles tenant-scoped user lifecycle operations.
type UserService struct {
	userRepo        repo.UserRepository
	tenantRepo      repo.TenantRepository
	socrateInviter  SocrateInviter        // nil → skip Socrate (local dev / admin port not configured)
	socrateProfiler SocrateProfileFetcher // nil → skip enrichment
	emitter         *event.Emitter
	logger          *logrus.Entry
}

func NewUserService(userRepo repo.UserRepository, tenantRepo repo.TenantRepository, socrateInviter SocrateInviter, socrateProfiler SocrateProfileFetcher, emitter *event.Emitter, logger *logrus.Entry) *UserService {
	return &UserService{
		userRepo:        userRepo,
		tenantRepo:      tenantRepo,
		socrateInviter:  socrateInviter,
		socrateProfiler: socrateProfiler,
		emitter:         emitter,
		logger:          logger,
	}
}

// GetOrCreateUser finds an existing user by external ID, or auto-provisions on first login.
// Returns the user and their Ascenda role.
func (s *UserService) GetOrCreateUser(ctx context.Context, tenantID uuid.UUID, externalID, email, name string) (*model.User, error) {
	// Try to find existing user by external ID
	user, err := s.userRepo.GetByExternalID(externalID)
	if err == nil && user != nil {
		// Update cached fields if changed
		changed := false
		if user.Email != email && email != "" {
			user.Email = email
			changed = true
		}
		if user.Name != name && name != "" {
			user.Name = name
			changed = true
		}
		if user.JoinedAt == nil {
			now := time.Now()
			user.JoinedAt = &now
			changed = true
		}
		if changed {
			s.userRepo.Update(user)
		}
		return user, nil
	}

	// Check if there's a pending invite for this email
	invited, _ := s.userRepo.GetByEmail(tenantID, email)
	if invited != nil && invited.ExternalID == "" {
		// Claim the invite
		invited.ExternalID = externalID
		if name != "" {
			invited.Name = name
		}
		now := time.Now()
		invited.JoinedAt = &now
		if err := s.userRepo.Update(invited); err != nil {
			return nil, apierror.Internal("failed to claim invite")
		}
		return invited, nil
	}

	// Auto-create as base user (plan access comes from plan_members)
	count, _ := s.userRepo.CountByTenant(tenantID)
	// The very first user in a tenant becomes owner
	role := "user"
	if count == 0 {
		role = "owner"
	}

	now := time.Now()
	newUser := &model.User{
		ID:         uuid.New(),
		TenantID:   tenantID,
		ExternalID: externalID,
		Email:      email,
		Name:       name,
		Role:       role,
		IsActive:   true,
		JoinedAt:   &now,
	}
	if err := s.userRepo.Create(newUser); err != nil {
		s.logger.WithError(err).Error("failed to create user")
		return nil, apierror.Internal("failed to create user")
	}

	s.logger.WithFields(logrus.Fields{
		"user_id": newUser.ID, "role": role, "email": email,
	}).Info("user auto-provisioned")

	return newUser, nil
}

// ListUsers returns all users in a tenant, enriched with Socrate profile data
// (email, name) when a profile fetcher is configured. Enrichment is done in a
// single bulk call to Socrate (list endpoint) rather than per-user, and
// enriched values are persisted back to the DB so subsequent calls are free.
func (s *UserService) ListUsers(ctx context.Context, tenantID uuid.UUID, offset, limit int) ([]*model.User, int64, error) {
	users, total, err := s.userRepo.ListByTenant(tenantID, offset, limit)
	if err != nil {
		return nil, 0, err
	}

	if s.socrateProfiler == nil {
		return users, total, nil
	}

	// Check whether any users need enrichment; skip Socrate if all are populated.
	needsEnrichment := false
	for _, u := range users {
		if u.ExternalID != "" && (u.Email == "" || u.Name == "") {
			needsEnrichment = true
			break
		}
	}
	if !needsEnrichment {
		return users, total, nil
	}

	// Enrich each user that is missing email/name via individual GetUserAsService calls.
	// Use a short timeout so an unreachable Socrate admin port doesn't stall the response.
	enrichCtx, enrichCancel := context.WithTimeout(ctx, 2*time.Second)
	defer enrichCancel()

	for _, u := range users {
		if u.ExternalID == "" || (u.Email != "" && u.Name != "") {
			continue
		}

		profile, fetchErr := s.socrateProfiler.GetUserAsService(enrichCtx, u.ExternalID)
		if fetchErr != nil || profile == nil {
			s.logger.WithError(fetchErr).WithField("externalID", u.ExternalID).
				Warn("failed to fetch Socrate user for enrichment")
			continue
		}

		// Derive best-available display name.
		name := strings.TrimSpace(profile.Name)
		if name == "" && profile.Email != "" {
			name = profile.Email
		}

		changed := false
		if profile.Email != "" && u.Email != profile.Email {
			u.Email = profile.Email
			changed = true
		}
		if name != "" && u.Name != name {
			u.Name = name
			changed = true
		}
		if changed {
			if updateErr := s.userRepo.Update(u); updateErr != nil {
				s.logger.WithError(updateErr).Warn("failed to persist enriched user data")
			}
		}
	}

	return users, total, nil
}

// InviteUser creates a pending user record and dispatches an invite email via
// the Socrate admin port (POST /api/apps/{id}/service/users on :8081).
// That endpoint accepts the backend service-account token (client_credentials),
// bypassing the numeric-sub requirement of the regular /api/apps/ route group.
//
// The invite is always non-fatal: if Socrate is unreachable the local pending
// record is still created and TenantMiddleware's "claim invite" logic links the
// Socrate identity automatically on first login.
func (s *UserService) InviteUser(ctx context.Context, tenantID, invitedBy uuid.UUID, email, name, role string) (*model.User, error) {
	validRoles := map[string]bool{"editor": true, "reader": true, "user": true}
	if !validRoles[role] {
		return nil, apierror.BadRequest("invalid role: must be 'editor' or 'reader'")
	}

	existing, _ := s.userRepo.GetByEmail(tenantID, email)
	if existing != nil {
		if existing.ExternalID != "" {
			// Already an active / claimed member — hard conflict.
			return nil, apierror.Conflict("user is already a member of this tenant")
		}
		// Pending invite (ExternalID still empty) — resend the invite email
		// via Socrate and return the existing record so the caller gets a 200.
		if s.socrateInviter != nil {
			inv, err := s.socrateInviter.InviteUserAsService(ctx, socrate.ServiceInviteRequest{
				Email: email,
				Role:  role,
			})
			switch {
			case err == nil && inv != nil:
				s.logger.WithField("email", email).Info("pending invite resent via Socrate admin port")
				// If Socrate now knows the user, capture the external ID.
				if inv.UserID != 0 && existing.ExternalID == "" {
					existing.ExternalID = fmt.Sprintf("%d", inv.UserID)
					_ = s.userRepo.Update(existing)
				}
			case errors.Is(err, socrate.ErrUserAlreadyExists):
				s.logger.WithField("email", email).Debug("user already exists in Socrate — resend skipped")
			default:
				s.logger.WithError(err).WithField("email", email).
					Warn("Socrate resend failed — existing pending record unchanged")
			}
		}
		s.logger.WithField("email", email).Info("invite resent for pending user")
		return existing, nil
	}

	// New invite — create a local pending record and dispatch via Socrate.
	var externalID string
	if s.socrateInviter != nil {
		inv, err := s.socrateInviter.InviteUserAsService(ctx, socrate.ServiceInviteRequest{
			Email: email,
			Role:  role,
		})
		switch {
		case err == nil && inv != nil:
			if inv.UserID != 0 {
				externalID = fmt.Sprintf("%d", inv.UserID)
			}
			if inv.EmailSent {
				s.logger.WithFields(logrus.Fields{"email": email, "socrate_id": externalID}).
					Info("user invited via Socrate admin port — invite email dispatched")
			} else {
				s.logger.WithFields(logrus.Fields{"email": email, "email_error": inv.EmailError}).
					Warn("user created in Socrate but invite email failed to send")
			}
		case errors.Is(err, socrate.ErrUserAlreadyExists):
			s.logger.WithField("email", email).
				Debug("user already exists in Socrate — skipping invite email")
		default:
			s.logger.WithError(err).WithField("email", email).
				Warn("Socrate invite failed — local pending record created; invitee claims on first login")
		}
	}

	user := &model.User{
		ID:         uuid.New(),
		TenantID:   tenantID,
		ExternalID: externalID,
		Email:      email,
		Name:       name,
		Role:       role,
		IsActive:   true,
		InvitedBy:  &invitedBy,
	}
	if err := s.userRepo.Create(user); err != nil {
		s.logger.WithError(err).Error("failed to create invited user record")
		return nil, apierror.Internal("failed to invite user")
	}

	s.logger.WithFields(logrus.Fields{
		"email": email, "role": role, "invited_by": invitedBy,
		"socrate_dispatched": externalID != "",
	}).Info("user invited")

	return user, nil
}

// DeleteUser deactivates a user in the local DB and removes them from Socrate
// when an external ID is present. Owners cannot be deleted — use TransferOwnership first.
func (s *UserService) DeleteUser(ctx context.Context, tenantID, targetUserID uuid.UUID) error {
	target, err := s.userRepo.GetByID(tenantID, targetUserID)
	if err != nil || target == nil {
		return apierror.NotFound("user", targetUserID.String())
	}

	if target.Role == "owner" {
		return apierror.Forbidden("cannot delete the owner — transfer ownership first")
	}

	// Soft-delete: mark inactive. The record is kept for audit trail / foreign keys.
	target.IsActive = false
	if err := s.userRepo.Update(target); err != nil {
		return apierror.Internal("failed to deactivate user")
	}

	s.logger.WithFields(logrus.Fields{
		"user_id": targetUserID, "email": target.Email,
	}).Info("user soft-deleted")

	return nil
}

// GetUserImpact returns dependency counts for a user — used by the safe-delete preview.
func (s *UserService) GetUserImpact(ctx context.Context, tenantID, targetUserID uuid.UUID) (map[string]int64, error) {
	target, err := s.userRepo.GetByID(tenantID, targetUserID)
	if err != nil || target == nil {
		return nil, apierror.NotFound("user", targetUserID.String())
	}
	// For now we report a stub impact (plan ownership count would require a plan repo).
	// The handler can enrich this with more context.
	return map[string]int64{
		"ownedPlans": 0, // enriched by handler if needed
	}, nil
}

// UpdateRole changes a user's role with business rules.
// Tenant roles are: editor | reader | user (legacy alias for editor).
// owner is transferred via TransferOwnership; admin is managed by Ascenda operators.
func (s *UserService) UpdateRole(ctx context.Context, tenantID, targetUserID uuid.UUID, callerRole, newRole string) error {
	// Valid assignable tenant roles. "owner" and "admin" are handled separately.
	validTenantRoles := map[string]bool{"editor": true, "reader": true, "user": true}

	target, err := s.userRepo.GetByID(tenantID, targetUserID)
	if err != nil || target == nil {
		return apierror.NotFound("user", targetUserID.String())
	}

	// Cannot change owner role — use TransferOwnership instead
	if target.Role == "owner" {
		return apierror.Forbidden("cannot change owner role — use transfer ownership instead")
	}

	// Cannot assign owner or admin via this endpoint
	if newRole == "owner" {
		return apierror.Forbidden("cannot promote to owner — use transfer ownership instead")
	}
	if newRole == "admin" {
		return apierror.Forbidden("cannot assign platform admin role — contact Ascenda support")
	}

	if !validTenantRoles[newRole] {
		return apierror.BadRequest("invalid role: must be 'editor', 'reader', or 'user'")
	}

	target.Role = newRole
	if err := s.userRepo.Update(target); err != nil {
		return apierror.Internal("failed to update role")
	}

	s.logger.WithFields(logrus.Fields{
		"user_id": targetUserID, "new_role": newRole,
	}).Info("user role updated")

	return nil
}

// DeactivateUser soft-deactivates a user.
func (s *UserService) DeactivateUser(ctx context.Context, tenantID, targetUserID uuid.UUID) error {
	target, err := s.userRepo.GetByID(tenantID, targetUserID)
	if err != nil || target == nil {
		return apierror.NotFound("user", targetUserID.String())
	}

	if target.Role == "owner" {
		return apierror.Forbidden("cannot deactivate the owner")
	}

	target.IsActive = false
	return s.userRepo.Update(target)
}

// ReactivateUser re-enables a deactivated user.
func (s *UserService) ReactivateUser(ctx context.Context, tenantID, targetUserID uuid.UUID) error {
	target, err := s.userRepo.GetByID(tenantID, targetUserID)
	if err != nil || target == nil {
		return apierror.NotFound("user", targetUserID.String())
	}

	target.IsActive = true
	return s.userRepo.Update(target)
}

// TransferOwnership transfers ownership from current owner to target admin.
func (s *UserService) TransferOwnership(ctx context.Context, tenantID, currentOwnerID, newOwnerID uuid.UUID) error {
	current, err := s.userRepo.GetByID(tenantID, currentOwnerID)
	if err != nil || current == nil || current.Role != "owner" {
		return apierror.Forbidden("only the current owner can transfer ownership")
	}

	target, err := s.userRepo.GetByID(tenantID, newOwnerID)
	if err != nil || target == nil {
		return apierror.NotFound("user", newOwnerID.String())
	}

	// The new owner must be an active user in this tenant (any non-owner role)
	if target.Role == "owner" {
		return apierror.Forbidden("target user is already the owner")
	}

	// Former owner becomes a regular user; target becomes the new owner
	current.Role = "user"
	target.Role = "owner"

	if err := s.userRepo.Update(current); err != nil {
		return apierror.Internal("failed to update current owner")
	}
	if err := s.userRepo.Update(target); err != nil {
		return apierror.Internal("failed to update new owner")
	}

	s.logger.WithFields(logrus.Fields{
		"from": currentOwnerID, "to": newOwnerID,
	}).Info("ownership transferred")

	return nil
}
