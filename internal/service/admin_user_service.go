package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"ascenda/internal/model"
	"ascenda/internal/pkg/apierror"
	"ascenda/internal/pkg/socrate"
	"ascenda/internal/repo"
)

// SocrateUserManager is the interface that must be satisfied by the Socrate client.
// Socrate recognises exactly two roles per application: "admin" and "user".
// Ascenda's own tenant roles (owner, user, …) are a separate, local concern.
type SocrateUserManager interface {
	ListUsers(ctx context.Context, search string, page, pageSize int) (*socrate.UserListResponse, error)
	GetUser(ctx context.Context, userID string) (*socrate.User, error)
	CreateUser(ctx context.Context, req socrate.CreateUserRequest) (*socrate.User, error)
	UpdateUser(ctx context.Context, userID string, req socrate.UpdateUserRequest) (*socrate.User, error)
	DeleteUser(ctx context.Context, userID string) error
	ResendVerification(ctx context.Context, userID string) error
	ResetPassword(ctx context.Context, userID string) error
}

// socrateRoles are the only roles recognised by the Socrate identity provider.
var socrateRoles = map[string]bool{"admin": true, "user": true}

// AdminUserService handles platform-wide user and tenant management via Socrate proxy.
type AdminUserService struct {
	socrateClient SocrateUserManager    // nil if Socrate not configured
	userRepo      repo.UserRepository
	tenantRepo    repo.AdminTenantRepository
	logger        *logrus.Entry
}

// NewAdminUserService creates a new AdminUserService.
// socrateClient may be nil if Socrate is not configured (endpoints will return 503).
func NewAdminUserService(socrateClient SocrateUserManager, userRepo repo.UserRepository, tenantRepo repo.AdminTenantRepository, logger *logrus.Entry) *AdminUserService {
	return &AdminUserService{
		socrateClient: socrateClient,
		userRepo:      userRepo,
		tenantRepo:    tenantRepo,
		logger:        logger,
	}
}

// requireSocrate returns an error if Socrate client is not configured.
func (s *AdminUserService) requireSocrate() error {
	if s.socrateClient == nil {
		return apierror.ServiceUnavailable("Socrate identity provider is not configured")
	}
	return nil
}

// ============================================================================
// User Management (via Socrate proxy)
// ============================================================================

// AdminUserDTO enriches a Socrate user with Ascenda metadata.
type AdminUserDTO struct {
	SocrateID   uint       `json:"socrateId"`
	Email       string     `json:"email"`
	Name        string     `json:"name"`
	Role        string     `json:"role"`       // Socrate role
	Status      string     `json:"status"`
	IsVerified  bool       `json:"isVerified"`
	AscendaID   *string    `json:"ascendaId,omitempty"`
	TenantID    *string    `json:"tenantId,omitempty"`
	TenantName  *string    `json:"tenantName,omitempty"`
	AscendaRole *string    `json:"ascendaRole,omitempty"` // owner|user (Ascenda tenant role)
	IsActive    bool       `json:"isActive"`
	LastLogin   *time.Time `json:"lastLogin,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// AdminUserListResponse is the response from listing users.
type AdminUserListResponse struct {
	Users      []AdminUserDTO `json:"users"`
	TotalCount int64          `json:"totalCount"`
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
}

// ListUsers lists all users from Socrate, enriched with Ascenda data.
func (s *AdminUserService) ListUsers(ctx context.Context, search string, page, pageSize int) (*AdminUserListResponse, error) {
	if err := s.requireSocrate(); err != nil {
		return nil, err
	}

	result, err := s.socrateClient.ListUsers(ctx, search, page, pageSize)
	if err != nil {
		s.logger.WithError(err).Error("failed to list users from Socrate")
		return nil, apierror.Internal("failed to list users from identity provider")
	}

	// Enrich each user with Ascenda data
	dtos := make([]AdminUserDTO, 0, len(result.Users))
	for _, u := range result.Users {
		dto := s.enrichUser(u)
		dtos = append(dtos, dto)
	}

	return &AdminUserListResponse{
		Users:      dtos,
		TotalCount: result.TotalCount,
		Page:       result.Page,
		PageSize:   result.PageSize,
	}, nil
}

// GetUser retrieves a single user from Socrate by Socrate ID, enriched with Ascenda data.
func (s *AdminUserService) GetUser(ctx context.Context, socrateID string) (*AdminUserDTO, error) {
	if err := s.requireSocrate(); err != nil {
		return nil, err
	}

	u, err := s.socrateClient.GetUser(ctx, socrateID)
	if err != nil {
		s.logger.WithError(err).Error("failed to get user from Socrate")
		return nil, apierror.Internal("failed to get user from identity provider")
	}
	if u == nil || u.ID == 0 {
		return nil, apierror.NotFound("user", socrateID)
	}

	dto := s.enrichUser(*u)
	return &dto, nil
}

// CreateUserRequest is the request to create a user via the platform admin API.
type CreateUserRequest struct {
	Email       string `json:"email" validate:"required,email"`
	FullName    string `json:"fullName" validate:"required"`
	Role        string `json:"role"`
}

// CreateUser creates a new user in Socrate (invite flow — no password required).
// Role must be one of Socrate's two application roles: "admin" or "user".
func (s *AdminUserService) CreateUser(ctx context.Context, req CreateUserRequest) (*AdminUserDTO, error) {
	if err := s.requireSocrate(); err != nil {
		return nil, err
	}

	// Default to "user" if not specified; validate against Socrate's two roles.
	if req.Role == "" {
		req.Role = "user"
	}
	if !socrateRoles[req.Role] {
		return nil, apierror.BadRequest("invalid Socrate role: must be 'admin' or 'user'")
	}

	u, err := s.socrateClient.CreateUser(ctx, socrate.CreateUserRequest{
		Email:    req.Email,
		FullName: req.FullName,
		Role:     req.Role,
	})
	if err != nil {
		if errors.Is(err, socrate.ErrUserAlreadyExists) {
			return nil, apierror.Conflict("user with this email already exists in the identity provider")
		}
		s.logger.WithError(err).Error("failed to create user in Socrate")
		return nil, apierror.Internal("failed to create user in identity provider")
	}

	s.logger.WithFields(logrus.Fields{
		"email":      req.Email,
		"socrateID":  u.ID,
	}).Info("user created in Socrate")

	dto := s.enrichUser(*u)
	return &dto, nil
}

// UpdateUserRequest is the request to update a user via the platform admin API.
type UpdateUserRequest struct {
	FullName    *string `json:"fullName,omitempty"`
	Role        *string `json:"role,omitempty"`
}

// UpdateUser updates a user in Socrate.
// If Role is provided it must be one of Socrate's two roles: "admin" or "user".
func (s *AdminUserService) UpdateUser(ctx context.Context, socrateID string, req UpdateUserRequest) (*AdminUserDTO, error) {
	if err := s.requireSocrate(); err != nil {
		return nil, err
	}

	if req.Role != nil && !socrateRoles[*req.Role] {
		return nil, apierror.BadRequest("invalid Socrate role: must be 'admin' or 'user'")
	}

	u, err := s.socrateClient.UpdateUser(ctx, socrateID, socrate.UpdateUserRequest{
		FullName: req.FullName,
		Role:     req.Role,
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to update user in Socrate")
		return nil, apierror.Internal("failed to update user in identity provider")
	}

	s.logger.WithField("socrateID", socrateID).Info("user updated in Socrate")

	dto := s.enrichUser(*u)
	return &dto, nil
}

// DeleteUser removes a user from Socrate and deactivates them in Ascenda.
func (s *AdminUserService) DeleteUser(ctx context.Context, socrateID string) error {
	if err := s.requireSocrate(); err != nil {
		return err
	}

	if err := s.socrateClient.DeleteUser(ctx, socrateID); err != nil {
		s.logger.WithError(err).Error("failed to delete user in Socrate")
		return apierror.Internal("failed to delete user from identity provider")
	}

	// Deactivate in Ascenda DB if they have a record
	ascendaUser, err := s.userRepo.GetByExternalID(socrateID)
	if err == nil && ascendaUser != nil {
		ascendaUser.IsActive = false
		if updateErr := s.userRepo.Update(ascendaUser); updateErr != nil {
			// Log but don't fail — Socrate is the source of truth
			s.logger.WithError(updateErr).Warn("failed to deactivate user in Ascenda after Socrate deletion")
		}
	}

	s.logger.WithField("socrateID", socrateID).Info("user deleted from Socrate")
	return nil
}

// ResendVerification resends the verification email for a user.
func (s *AdminUserService) ResendVerification(ctx context.Context, socrateID string) error {
	if err := s.requireSocrate(); err != nil {
		return err
	}

	if err := s.socrateClient.ResendVerification(ctx, socrateID); err != nil {
		s.logger.WithError(err).Error("failed to resend verification")
		return apierror.Internal("failed to resend verification email")
	}

	s.logger.WithField("socrateID", socrateID).Info("verification email resent")
	return nil
}

// ResetPassword triggers a password reset for a user.
func (s *AdminUserService) ResetPassword(ctx context.Context, socrateID string) error {
	if err := s.requireSocrate(); err != nil {
		return err
	}

	if err := s.socrateClient.ResetPassword(ctx, socrateID); err != nil {
		s.logger.WithError(err).Error("failed to reset password")
		return apierror.Internal("failed to trigger password reset")
	}

	s.logger.WithField("socrateID", socrateID).Info("password reset triggered")
	return nil
}

// enrichUser adds Ascenda metadata to a Socrate user.
func (s *AdminUserService) enrichUser(u socrate.User) AdminUserDTO {
	dto := AdminUserDTO{
		SocrateID:  u.ID,
		Email:      u.Email,
		Name:       u.Name,
		Role:       u.Role,
		Status:     u.Status,
		IsVerified: u.IsVerified,
		IsActive:   u.Status == "active",
		LastLogin:  u.LastLogin,
		CreatedAt:  u.CreatedAt,
	}

	// Look up Ascenda user record via external_id
	externalID := fmt.Sprintf("%d", u.ID)
	ascendaUser, err := s.userRepo.GetByExternalID(externalID)
	if err == nil && ascendaUser != nil {
		idStr := ascendaUser.ID.String()
		tenantIDStr := ascendaUser.TenantID.String()
		dto.AscendaID = &idStr
		dto.TenantID = &tenantIDStr
		dto.AscendaRole = &ascendaUser.Role
		dto.IsActive = ascendaUser.IsActive

		// Enrich with tenant name
		tenant, err := s.tenantRepo.GetByID(ascendaUser.TenantID)
		if err == nil && tenant != nil {
			dto.TenantName = &tenant.Name
		}
	}

	return dto
}

// ============================================================================
// Tenant Management (Ascenda DB)
// ============================================================================

// AdminTenantDTO is the admin view of a tenant with aggregate counts.
type AdminTenantDTO struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Tier      string    `json:"tier"`
	IsActive  bool      `json:"isActive"`
	MaxUsers  int       `json:"maxUsers"`
	MaxPlans  int       `json:"maxPlans"`
	CreatedAt time.Time `json:"createdAt"`
}

// AdminTenantListResponse is the response from listing tenants.
type AdminTenantListResponse struct {
	Tenants    []AdminTenantDTO `json:"tenants"`
	TotalCount int64            `json:"totalCount"`
	Page       int              `json:"page"`
	PageSize   int              `json:"pageSize"`
}

// ListTenants returns all tenants with pagination.
func (s *AdminUserService) ListTenants(_ context.Context, page, pageSize int) (*AdminTenantListResponse, error) {
	offset := (page - 1) * pageSize
	tenants, err := s.tenantRepo.ListAll(offset, pageSize)
	if err != nil {
		return nil, apierror.Internal("failed to list tenants")
	}

	total, err := s.tenantRepo.CountAll()
	if err != nil {
		return nil, apierror.Internal("failed to count tenants")
	}

	dtos := make([]AdminTenantDTO, 0, len(tenants))
	for _, t := range tenants {
		dtos = append(dtos, AdminTenantDTO{
			ID:        t.ID.String(),
			Name:      t.Name,
			Slug:      t.Slug,
			Tier:      t.Tier,
			IsActive:  t.IsActive,
			MaxUsers:  t.MaxUsers,
			MaxPlans:  t.MaxPlans,
			CreatedAt: t.CreatedAt,
		})
	}

	return &AdminTenantListResponse{
		Tenants:    dtos,
		TotalCount: total,
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

// GetTenant returns a single tenant by ID.
func (s *AdminUserService) GetTenant(_ context.Context, id uuid.UUID) (*AdminTenantDTO, error) {
	t, err := s.tenantRepo.GetByID(id)
	if err != nil {
		return nil, apierror.NotFound("tenant", id.String())
	}

	dto := AdminTenantDTO{
		ID:        t.ID.String(),
		Name:      t.Name,
		Slug:      t.Slug,
		Tier:      t.Tier,
		IsActive:  t.IsActive,
		MaxUsers:  t.MaxUsers,
		MaxPlans:  t.MaxPlans,
		CreatedAt: t.CreatedAt,
	}
	return &dto, nil
}

// CreateTenantRequest is the request to create a new tenant.
type CreateTenantRequest struct {
	Name     string `json:"name" validate:"required"`
	Slug     string `json:"slug" validate:"required"`
	Tier     string `json:"tier"`
	MaxUsers int    `json:"maxUsers"`
	MaxPlans int    `json:"maxPlans"`
}

// CreateTenant creates a new tenant.
func (s *AdminUserService) CreateTenant(_ context.Context, req CreateTenantRequest) (*AdminTenantDTO, error) {
	tier := req.Tier
	if tier == "" {
		tier = "free"
	}
	maxUsers := req.MaxUsers
	if maxUsers == 0 {
		maxUsers = 5
	}
	maxPlans := req.MaxPlans
	if maxPlans == 0 {
		maxPlans = 3
	}

	tenant := &model.Tenant{
		ID:       uuid.New(),
		Name:     req.Name,
		Slug:     req.Slug,
		Tier:     tier,
		IsActive: true,
		MaxUsers: maxUsers,
		MaxPlans: maxPlans,
	}

	if err := s.tenantRepo.Create(tenant); err != nil {
		s.logger.WithError(err).Error("failed to create tenant")
		return nil, apierror.Internal("failed to create tenant")
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenant.ID,
		"name":      tenant.Name,
	}).Info("tenant created")

	dto := AdminTenantDTO{
		ID:        tenant.ID.String(),
		Name:      tenant.Name,
		Slug:      tenant.Slug,
		Tier:      tenant.Tier,
		IsActive:  tenant.IsActive,
		MaxUsers:  tenant.MaxUsers,
		MaxPlans:  tenant.MaxPlans,
		CreatedAt: tenant.CreatedAt,
	}
	return &dto, nil
}

// UpdateTenantRequest is the request to update a tenant.
type UpdateTenantRequest struct {
	Name     *string `json:"name,omitempty"`
	Tier     *string `json:"tier,omitempty"`
	IsActive *bool   `json:"isActive,omitempty"`
	MaxUsers *int    `json:"maxUsers,omitempty"`
	MaxPlans *int    `json:"maxPlans,omitempty"`
}

// UpdateTenant updates a tenant.
func (s *AdminUserService) UpdateTenant(_ context.Context, id uuid.UUID, req UpdateTenantRequest) (*AdminTenantDTO, error) {
	tenant, err := s.tenantRepo.GetByID(id)
	if err != nil {
		return nil, apierror.NotFound("tenant", id.String())
	}

	if req.Name != nil {
		tenant.Name = *req.Name
	}
	if req.Tier != nil {
		tenant.Tier = *req.Tier
	}
	if req.IsActive != nil {
		tenant.IsActive = *req.IsActive
	}
	if req.MaxUsers != nil {
		tenant.MaxUsers = *req.MaxUsers
	}
	if req.MaxPlans != nil {
		tenant.MaxPlans = *req.MaxPlans
	}

	if err := s.tenantRepo.Update(tenant); err != nil {
		return nil, apierror.Internal("failed to update tenant")
	}

	dto := AdminTenantDTO{
		ID:        tenant.ID.String(),
		Name:      tenant.Name,
		Slug:      tenant.Slug,
		Tier:      tenant.Tier,
		IsActive:  tenant.IsActive,
		MaxUsers:  tenant.MaxUsers,
		MaxPlans:  tenant.MaxPlans,
		CreatedAt: tenant.CreatedAt,
	}
	return &dto, nil
}
