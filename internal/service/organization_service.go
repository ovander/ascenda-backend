package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ascenda/internal/model"
	"ascenda/internal/repo"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
)

// ============================================================================
// DTOs
// ============================================================================

// OrgDTO is the admin-facing representation of an Organization.
type OrgDTO struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Plan         string    `json:"plan"`
	MaxUsers     int       `json:"maxUsers"`
	BillingEmail string    `json:"billingEmail,omitempty"`
	Domain       string    `json:"domain,omitempty"`
	IsActive     bool      `json:"isActive"`
	TenantCount  int       `json:"tenantCount"`
	UserCount    int64     `json:"userCount"`
	CreatedAt    time.Time `json:"createdAt"`
}

// OrgListResponse wraps a paginated list of organizations.
type OrgListResponse struct {
	Organizations []OrgDTO `json:"organizations"`
	TotalCount    int64    `json:"totalCount"`
	Page          int      `json:"page"`
	PageSize      int      `json:"pageSize"`
}

// OrgTenantDTO is the view of a tenant as seen from an org context.
type OrgTenantDTO struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Plan      string    `json:"plan"`
	IsActive  bool      `json:"isActive"`
	UserCount int64     `json:"userCount"`
	CreatedAt time.Time `json:"createdAt"`
}

// ============================================================================
// Request types
// ============================================================================

// CreateOrganizationRequest is the request to provision a new enterprise org.
type CreateOrganizationRequest struct {
	Name         string `json:"name"         validate:"required"`
	Slug         string `json:"slug"         validate:"required"`
	Plan         string `json:"plan"`     // default: enterprise
	MaxUsers     int    `json:"maxUsers"` // 0 = unlimited
	BillingEmail string `json:"billingEmail"`
	Domain       string `json:"domain"`

	// Optional: first department tenant created alongside the org.
	FirstDepartment     string `json:"firstDepartment"`     // name for the first tenant; defaults to org Name
	FirstDepartmentSlug string `json:"firstDepartmentSlug"` // slug for the first tenant; defaults to org Slug

	// Optional: invite the first owner of the first tenant.
	OwnerEmail string `json:"ownerEmail"`
	OwnerName  string `json:"ownerName"`
}

// UpdateOrganizationRequest is the request to update an existing org.
type UpdateOrganizationRequest struct {
	Name         *string `json:"name,omitempty"`
	Plan         *string `json:"plan,omitempty"`
	MaxUsers     *int    `json:"maxUsers,omitempty"`
	BillingEmail *string `json:"billingEmail,omitempty"`
	Domain       *string `json:"domain,omitempty"`
	IsActive     *bool   `json:"isActive,omitempty"`
}

// AddOrgTenantRequest is the request to add a new department tenant to an org.
type AddOrgTenantRequest struct {
	Name       string `json:"name" validate:"required"`
	Slug       string `json:"slug" validate:"required"`
	OwnerEmail string `json:"ownerEmail"`
	OwnerName  string `json:"ownerName"`
}

// ============================================================================
// Service
// ============================================================================

// OrganizationService handles enterprise organization lifecycle.
type OrganizationService struct {
	orgRepo    repo.OrganizationRepository
	tenantRepo repo.AdminTenantRepository
	userRepo   repo.UserRepository
	inviter    SocrateInviter // optional; nil if Socrate not configured
	logger     *logrus.Entry
}

// NewOrganizationService creates a new OrganizationService.
func NewOrganizationService(
	orgRepo repo.OrganizationRepository,
	tenantRepo repo.AdminTenantRepository,
	userRepo repo.UserRepository,
	inviter SocrateInviter,
	logger *logrus.Entry,
) *OrganizationService {
	return &OrganizationService{
		orgRepo:    orgRepo,
		tenantRepo: tenantRepo,
		userRepo:   userRepo,
		inviter:    inviter,
		logger:     logger,
	}
}

// ── List / Get ────────────────────────────────────────────────────────────────

func (s *OrganizationService) ListOrganizations(_ context.Context, page, pageSize int) (*OrgListResponse, error) {
	offset := (page - 1) * pageSize
	orgs, err := s.orgRepo.ListAll(offset, pageSize)
	if err != nil {
		return nil, apierror.Internal("failed to list organizations")
	}
	total, err := s.orgRepo.CountAll()
	if err != nil {
		return nil, apierror.Internal("failed to count organizations")
	}

	dtos := make([]OrgDTO, 0, len(orgs))
	for _, org := range orgs {
		dto, buildErr := s.buildOrgDTO(org)
		if buildErr != nil {
			s.logger.WithError(buildErr).WithField("org_id", org.ID).Warn("failed to build org DTO — skipping stats")
		}
		dtos = append(dtos, dto)
	}

	return &OrgListResponse{
		Organizations: dtos,
		TotalCount:    total,
		Page:          page,
		PageSize:      pageSize,
	}, nil
}

func (s *OrganizationService) GetOrganization(_ context.Context, id uuid.UUID) (*OrgDTO, error) {
	org, err := s.orgRepo.GetByID(id)
	if err != nil {
		return nil, apierror.NotFound("organization", id.String())
	}
	dto, _ := s.buildOrgDTO(org)
	return &dto, nil
}

// ── Create ───────────────────────────────────────────────────────────────────

// CreateOrganization provisions an enterprise org, creates the first department
// tenant under it, and optionally invites the first tenant owner.
func (s *OrganizationService) CreateOrganization(ctx context.Context, req CreateOrganizationRequest) (*OrgDTO, error) {
	// Defaults
	if req.Plan == "" {
		req.Plan = "enterprise"
	}
	if !validPlans[req.Plan] {
		return nil, apierror.BadRequest("invalid plan: must be 'freemium', 'pro', or 'enterprise'")
	}

	// 1. Create the org record.
	org := &model.Organization{
		ID:           uuid.New(),
		Name:         req.Name,
		Slug:         req.Slug,
		Plan:         req.Plan,
		MaxUsers:     req.MaxUsers,
		BillingEmail: req.BillingEmail,
		Domain:       req.Domain,
		IsActive:     true,
	}
	if err := s.orgRepo.Create(org); err != nil {
		s.logger.WithError(err).Error("failed to create organization")
		if strings.Contains(err.Error(), "unique") {
			return nil, apierror.Conflict("an organization with this slug already exists")
		}
		return nil, apierror.Internal("failed to create organization")
	}
	s.logger.WithField("org_id", org.ID).WithField("name", org.Name).Info("organization created")

	// 2. Create the first department tenant under the org.
	deptName := req.FirstDepartment
	if deptName == "" {
		deptName = req.Name
	}
	deptSlug := req.FirstDepartmentSlug
	if deptSlug == "" {
		deptSlug = req.Slug
	}
	tenant, tenantErr := s.provisionTenant(ctx, org, deptName, deptSlug, req.OwnerEmail, req.OwnerName)
	if tenantErr != nil {
		// Org was created; log but don't fail the whole call — admin can add tenants separately.
		s.logger.WithError(tenantErr).WithField("org_id", org.ID).Warn("org created but first department tenant failed")
	} else {
		s.logger.WithField("org_id", org.ID).WithField("tenant_id", tenant.ID).Info("first department tenant provisioned")
	}

	dto, _ := s.buildOrgDTO(org)
	return &dto, nil
}

// ── Add department tenant ─────────────────────────────────────────────────────

// AddTenant provisions a new department tenant under an existing org.
func (s *OrganizationService) AddTenant(ctx context.Context, orgID uuid.UUID, req AddOrgTenantRequest) (*OrgTenantDTO, error) {
	org, err := s.orgRepo.GetByID(orgID)
	if err != nil {
		return nil, apierror.NotFound("organization", orgID.String())
	}
	if !org.IsActive {
		return nil, apierror.BadRequest("cannot add a tenant to an inactive organization")
	}

	tenant, err := s.provisionTenant(ctx, org, req.Name, req.Slug, req.OwnerEmail, req.OwnerName)
	if err != nil {
		return nil, err
	}

	dto := s.buildTenantDTO(tenant, 0)
	return &dto, nil
}

// ListTenants returns all tenants belonging to an org.
func (s *OrganizationService) ListTenants(_ context.Context, orgID uuid.UUID) ([]OrgTenantDTO, error) {
	if _, err := s.orgRepo.GetByID(orgID); err != nil {
		return nil, apierror.NotFound("organization", orgID.String())
	}

	tenants, err := s.orgRepo.ListTenants(orgID)
	if err != nil {
		return nil, apierror.Internal("failed to list tenants for organization")
	}

	dtos := make([]OrgTenantDTO, 0, len(tenants))
	for _, t := range tenants {
		dtos = append(dtos, s.buildTenantDTO(t, 0))
	}
	return dtos, nil
}

// ── Update ───────────────────────────────────────────────────────────────────

// UpdateOrganization updates mutable org fields. When Plan changes, all child
// tenants are updated to reflect the new effective plan.
func (s *OrganizationService) UpdateOrganization(ctx context.Context, id uuid.UUID, req UpdateOrganizationRequest) (*OrgDTO, error) {
	org, err := s.orgRepo.GetByID(id)
	if err != nil {
		return nil, apierror.NotFound("organization", id.String())
	}

	planChanged := false
	if req.Name != nil {
		org.Name = *req.Name
	}
	if req.Plan != nil {
		if !validPlans[*req.Plan] {
			return nil, apierror.BadRequest("invalid plan: must be 'freemium', 'pro', or 'enterprise'")
		}
		if org.Plan != *req.Plan {
			planChanged = true
		}
		org.Plan = *req.Plan
	}
	if req.MaxUsers != nil {
		org.MaxUsers = *req.MaxUsers
	}
	if req.BillingEmail != nil {
		org.BillingEmail = *req.BillingEmail
	}
	if req.Domain != nil {
		org.Domain = *req.Domain
	}
	if req.IsActive != nil {
		org.IsActive = *req.IsActive
	}

	if err := s.orgRepo.Update(org); err != nil {
		return nil, apierror.Internal("failed to update organization")
	}

	// Cascade plan change to all child tenants so tier gating stays in sync.
	if planChanged {
		s.cascadePlanToTenants(ctx, org)
	}

	dto, _ := s.buildOrgDTO(org)
	return &dto, nil
}

// ── Delete ───────────────────────────────────────────────────────────────────

func (s *OrganizationService) DeleteOrganization(_ context.Context, id uuid.UUID) error {
	if _, err := s.orgRepo.GetByID(id); err != nil {
		return apierror.NotFound("organization", id.String())
	}
	if err := s.orgRepo.Delete(id); err != nil {
		return apierror.Internal("failed to delete organization")
	}
	s.logger.WithField("org_id", id).Info("organization soft-deleted")
	return nil
}

// ── License enforcement ───────────────────────────────────────────────────────

// CheckLicenseCapacity returns an error if the org has reached its user limit.
// MaxUsers == 0 means unlimited.
func (s *OrganizationService) CheckLicenseCapacity(_ context.Context, orgID uuid.UUID) error {
	org, err := s.orgRepo.GetByID(orgID)
	if err != nil {
		return apierror.NotFound("organization", orgID.String())
	}
	if org.MaxUsers == 0 {
		return nil // unlimited
	}
	count, err := s.orgRepo.CountActiveUsers(orgID)
	if err != nil {
		return apierror.Internal("failed to count organization users")
	}
	if count >= int64(org.MaxUsers) {
		return apierror.Forbidden(fmt.Sprintf(
			"organization user limit reached (%d / %d) — contact your Ascenda admin to increase capacity",
			count, org.MaxUsers,
		))
	}
	return nil
}

// ============================================================================
// Internal helpers
// ============================================================================

// provisionTenant creates an enterprise tenant under org and optionally invites
// the first owner via Socrate.
func (s *OrganizationService) provisionTenant(
	ctx context.Context,
	org *model.Organization,
	name, slug, ownerEmail, ownerName string,
) (*model.Tenant, error) {
	tenant := &model.Tenant{
		ID:             uuid.New(),
		OrganizationID: &org.ID,
		Type:           model.TenantTypeEnterprise,
		Plan:           org.Plan,
		Name:           name,
		Slug:           slug,
		IsActive:       true,
	}
	if err := s.tenantRepo.Create(tenant); err != nil {
		s.logger.WithError(err).Error("failed to create enterprise tenant")
		if strings.Contains(err.Error(), "unique") {
			return nil, apierror.Conflict("a tenant with this slug already exists")
		}
		return nil, apierror.Internal("failed to create tenant")
	}

	// Optionally invite the first owner.
	if ownerEmail != "" {
		if inviteErr := s.inviteOwner(ctx, tenant, ownerEmail, ownerName); inviteErr != nil {
			// Non-fatal: tenant is created; admin can invite manually later.
			s.logger.WithError(inviteErr).
				WithField("tenant_id", tenant.ID).
				WithField("owner_email", ownerEmail).
				Warn("tenant created but owner invite failed")
		}
	}

	return tenant, nil
}

// inviteOwner creates a pending local user record and sends a Socrate invite.
func (s *OrganizationService) inviteOwner(_ context.Context, tenant *model.Tenant, email, name string) error {
	// Create local pending user (role=owner, plan mirrors tenant).
	user := &model.User{
		ID:       uuid.New(),
		TenantID: tenant.ID,
		Email:    email,
		Name:     name,
		Role:     "owner",
		Plan:     tenant.Plan,
		IsActive: true,
	}
	if createErr := s.userRepo.Create(user); createErr != nil {
		return fmt.Errorf("create local pending user: %w", createErr)
	}

	// Send Socrate invite if configured.
	if s.inviter == nil {
		s.logger.Warn("Socrate inviter not configured — skipping invite email for owner")
		return nil
	}

	// Use an independent context so the Socrate HTTP call is not bounded by the
	// short-lived CRUD request deadline (typically 5 s). Socrate may take several
	// seconds to create the user and send the email.
	inviteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	inv, err := s.inviter.InviteUserAsService(inviteCtx, socrate.ServiceInviteRequest{
		Email: email,
		Role:  "user", // Socrate platform role; tenant role (owner) is local only
	})
	if err != nil {
		return fmt.Errorf("socrate invite: %w", err)
	}

	// Link external Socrate ID if returned.
	if inv != nil && inv.UserID != 0 {
		user.ExternalID = fmt.Sprintf("%d", inv.UserID)
		if updateErr := s.userRepo.Update(user); updateErr != nil {
			s.logger.WithError(updateErr).Warn("failed to persist external_id on invited owner")
		}
	}

	if inv != nil && inv.EmailSent {
		s.logger.WithField("email", email).Info("invite email sent to org tenant owner")
	} else if inv != nil && inv.EmailError != "" {
		s.logger.WithField("email", email).WithField("email_error", inv.EmailError).
			Warn("socrate created user but invite email failed")
	}

	return nil
}

// cascadePlanToTenants propagates an org plan change to all its child tenants.
func (s *OrganizationService) cascadePlanToTenants(_ context.Context, org *model.Organization) {
	tenants, err := s.orgRepo.ListTenants(org.ID)
	if err != nil {
		s.logger.WithError(err).WithField("org_id", org.ID).Warn("failed to list tenants for plan cascade")
		return
	}
	for _, t := range tenants {
		t.Plan = org.Plan
		if updateErr := s.tenantRepo.Update(t); updateErr != nil {
			s.logger.WithError(updateErr).WithField("tenant_id", t.ID).Warn("failed to cascade plan to tenant")
		}
	}
	s.logger.WithField("org_id", org.ID).
		WithField("plan", org.Plan).
		WithField("tenants_updated", len(tenants)).
		Info("org plan cascaded to all department tenants")
}

func (s *OrganizationService) buildOrgDTO(org *model.Organization) (OrgDTO, error) {
	tenants, err := s.orgRepo.ListTenants(org.ID)
	userCount, _ := s.orgRepo.CountActiveUsers(org.ID)

	dto := OrgDTO{
		ID:           org.ID.String(),
		Name:         org.Name,
		Slug:         org.Slug,
		Plan:         org.Plan,
		MaxUsers:     org.MaxUsers,
		BillingEmail: org.BillingEmail,
		Domain:       org.Domain,
		IsActive:     org.IsActive,
		TenantCount:  len(tenants),
		UserCount:    userCount,
		CreatedAt:    org.CreatedAt,
	}
	return dto, err
}

func (s *OrganizationService) buildTenantDTO(t *model.Tenant, userCount int64) OrgTenantDTO {
	return OrgTenantDTO{
		ID:        t.ID.String(),
		Name:      t.Name,
		Slug:      t.Slug,
		Plan:      t.Plan,
		IsActive:  t.IsActive,
		UserCount: userCount,
		CreatedAt: t.CreatedAt,
	}
}

// Compile-time check.
var _ = (*OrganizationService)(nil)
