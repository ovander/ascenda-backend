package service

import (
	"context"

	"ascenda/internal/model"
	"ascenda/internal/repo"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/sirupsen/logrus"
)

// TenantServicer is the interface that TenantHandler depends on.
// Separating the interface from the concrete service keeps the handler
// independently testable and prevents direct repository access from handlers.
type TenantServicer interface {
	GetTenant(ctx context.Context, tenantID uuid.UUID) (*model.Tenant, error)
	UpdateTenant(ctx context.Context, tenantID uuid.UUID, name string) error
}

// TenantService orchestrates tenant read/write operations.
type TenantService struct {
	tenantRepo repo.TenantRepository
	logger     *logrus.Entry
}

// NewTenantService creates a new TenantService.
func NewTenantService(tenantRepo repo.TenantRepository, logger *logrus.Entry) *TenantService {
	return &TenantService{
		tenantRepo: tenantRepo,
		logger:     logger,
	}
}

// GetTenant retrieves a tenant by ID.
func (s *TenantService) GetTenant(ctx context.Context, tenantID uuid.UUID) (*model.Tenant, error) {
	tenant, err := s.tenantRepo.GetByID(tenantID)
	if err != nil {
		s.logger.WithError(err).WithField("tenant_id", tenantID).Warn("tenant not found")
		return nil, apierror.NotFound("tenant", tenantID.String())
	}
	return tenant, nil
}

// UpdateTenant updates mutable tenant fields (currently only Name).
func (s *TenantService) UpdateTenant(ctx context.Context, tenantID uuid.UUID, name string) error {
	if name == "" {
		return apierror.ValidationError("name is required", nil)
	}

	tenant, err := s.tenantRepo.GetByID(tenantID)
	if err != nil {
		return apierror.NotFound("tenant", tenantID.String())
	}

	tenant.Name = name
	if err := s.tenantRepo.Update(tenant); err != nil {
		s.logger.WithError(err).WithField("tenant_id", tenantID).Error("failed to update tenant")
		return apierror.Internal("failed to update tenant")
	}
	return nil
}

// Compile-time check.
var _ TenantServicer = (*TenantService)(nil)
