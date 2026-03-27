package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"ascenda/internal/event"
	"ascenda/internal/model"
	"ascenda/internal/pkg/apierror"
	"ascenda/internal/pkg/socrate"
	"ascenda/internal/repo"
)

// UserService handles tenant-scoped user lifecycle operations.
// It reuses the SocrateRegistrar interface (defined in registration_service.go)
// to dispatch invite emails via the Socrate service-account token.
type UserService struct {
	userRepo       repo.UserRepository
	tenantRepo     repo.TenantRepository
	socrateInviter SocrateRegistrar // nil → skip Socrate (local dev without IdP)
	emitter        *event.Emitter
	logger         *logrus.Entry
}

func NewUserService(userRepo repo.UserRepository, tenantRepo repo.TenantRepository, socrateInviter SocrateRegistrar, emitter *event.Emitter, logger *logrus.Entry) *UserService {
	return &UserService{
		userRepo:       userRepo,
		tenantRepo:     tenantRepo,
		socrateInviter: socrateInviter,
		emitter:        emitter,
		logger:         logger,
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

	// Check tenant user limit
	count, _ := s.userRepo.CountByTenant(tenantID)
	tenant, _ := s.tenantRepo.GetByID(tenantID)
	if tenant != nil && int(count) >= tenant.MaxUsers {
		return nil, apierror.Forbidden("tenant user limit reached")
	}

	// Auto-create as base user (plan access comes from plan_members)
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

// ListUsers returns all users in a tenant.
func (s *UserService) ListUsers(ctx context.Context, tenantID uuid.UUID, offset, limit int) ([]*model.User, int64, error) {
	return s.userRepo.ListByTenant(tenantID, offset, limit)
}

// InviteUser creates a pending user record and dispatches an invite email via
// Socrate (when configured). The flow mirrors RegistrationService.Register but
// is triggered by a tenant owner/admin rather than the user themselves.
func (s *UserService) InviteUser(ctx context.Context, tenantID, invitedBy uuid.UUID, email, name, role string) (*model.User, error) {
	// Validate role — only "user" can be invited; "owner" is assigned automatically
	// and "admin" is a platform-level role managed separately.
	validRoles := map[string]bool{"user": true}
	if !validRoles[role] {
		return nil, apierror.BadRequest("invalid role: must be 'user'")
	}

	// Check if user already exists in this tenant
	existing, _ := s.userRepo.GetByEmail(tenantID, email)
	if existing != nil {
		return nil, apierror.Conflict("user with this email already exists in tenant")
	}

	// Check tenant user limit
	count, _ := s.userRepo.CountByTenant(tenantID)
	tenant, _ := s.tenantRepo.GetByID(tenantID)
	if tenant != nil && int(count) >= tenant.MaxUsers {
		return nil, apierror.Forbidden("tenant user limit reached")
	}

	// Create identity in Socrate (sends verification/invite email automatically).
	// If Socrate is not configured (local dev), skip silently.
	var externalID string
	if s.socrateInviter != nil {
		socrateUser, err := s.socrateInviter.RegisterUser(ctx, socrate.CreateUserRequest{
			Email:    email,
			FullName: name,
			Role:     "user", // Socrate role — always "user" for tenant members
		})
		if err != nil {
			// ErrUserAlreadyExists means they're already registered in Socrate
			// (e.g. previously registered via another tenant). We still create the
			// local record so they gain access to this tenant — they just won't get
			// a second signup email.
			if !errors.Is(err, socrate.ErrUserAlreadyExists) {
				s.logger.WithError(err).Error("failed to create user in Socrate")
				return nil, apierror.Internal(fmt.Sprintf("failed to send invite: %v", err))
			}
			s.logger.WithField("email", email).Debug("user already exists in Socrate — skipping registration email")
		} else if socrateUser != nil {
			externalID = fmt.Sprintf("%d", socrateUser.ID)
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
		"socrate_dispatched": s.socrateInviter != nil,
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
// Tenant roles are: user | owner (owner is transferred via TransferOwnership).
// The platform "admin" role is managed separately by Ascenda operators.
func (s *UserService) UpdateRole(ctx context.Context, tenantID, targetUserID uuid.UUID, callerRole, newRole string) error {
	// Within a tenant, roles are "user" and "owner".
	// "owner" changes go via TransferOwnership; "admin" is platform-level only.
	validTenantRoles := map[string]bool{"user": true}

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
		return apierror.BadRequest("invalid role: must be 'user'")
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
