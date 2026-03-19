package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"kerplan/internal/event"
	"kerplan/internal/model"
	"kerplan/internal/pkg/apierror"
	"kerplan/internal/repo"
)

type UserService struct {
	userRepo   repo.UserRepository
	tenantRepo repo.TenantRepository
	emitter    *event.Emitter
	logger     *logrus.Entry
}

func NewUserService(userRepo repo.UserRepository, tenantRepo repo.TenantRepository, emitter *event.Emitter, logger *logrus.Entry) *UserService {
	return &UserService{
		userRepo:   userRepo,
		tenantRepo: tenantRepo,
		emitter:    emitter,
		logger:     logger,
	}
}

// GetOrCreateUser finds an existing user by external ID, or auto-provisions on first login.
// Returns the user and their KerPlan role.
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

// InviteUser creates a pending user record.
func (s *UserService) InviteUser(ctx context.Context, tenantID, invitedBy uuid.UUID, email, role string) (*model.User, error) {
	// Validate role
	validRoles := map[string]bool{"viewer": true, "editor": true, "admin": true}
	if !validRoles[role] {
		return nil, apierror.BadRequest("invalid role: must be viewer, editor, or admin")
	}

	// Check if user already exists
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

	user := &model.User{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Email:     email,
		Role:      role,
		IsActive:  true,
		InvitedBy: &invitedBy,
	}
	if err := s.userRepo.Create(user); err != nil {
		s.logger.WithError(err).Error("failed to invite user")
		return nil, apierror.Internal("failed to invite user")
	}

	s.logger.WithFields(logrus.Fields{
		"email": email, "role": role, "invited_by": invitedBy,
	}).Info("user invited")

	return user, nil
}

// UpdateRole changes a user's role with business rules.
func (s *UserService) UpdateRole(ctx context.Context, tenantID, targetUserID uuid.UUID, callerRole, newRole string) error {
	validRoles := map[string]bool{"viewer": true, "editor": true, "admin": true}

	target, err := s.userRepo.GetByID(tenantID, targetUserID)
	if err != nil || target == nil {
		return apierror.NotFound("user", targetUserID.String())
	}

	// Cannot change owner role
	if target.Role == "owner" {
		return apierror.Forbidden("cannot change owner role — use transfer ownership instead")
	}

	// Cannot promote to owner
	if newRole == "owner" {
		return apierror.Forbidden("cannot promote to owner — use transfer ownership instead")
	}

	// Admins can only set viewer or editor
	if callerRole == "admin" && !validRoles[newRole] {
		return apierror.Forbidden("admins can only assign viewer, editor, or admin roles")
	}

	// Only owner can promote to admin
	if newRole == "admin" && callerRole != "owner" {
		return apierror.Forbidden("only owner can promote to admin")
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

	if target.Role != "admin" {
		return apierror.Forbidden("can only transfer ownership to an admin")
	}

	// Swap roles
	current.Role = "admin"
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
