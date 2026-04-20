package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"ascenda/internal/event"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"ascenda/internal/repo"
)

// ── Test fixtures & mocks ──────────────────────────────────────────────────────

// inMemPlanRepo is a minimal in-memory implementation of PlanRepository.
type inMemPlanRepo struct {
	store map[uuid.UUID]*model.BusinessPlan
}

func newInMemPlanRepo() *inMemPlanRepo {
	return &inMemPlanRepo{
		store: make(map[uuid.UUID]*model.BusinessPlan),
	}
}

func (r *inMemPlanRepo) Create(plan *model.BusinessPlan) error {
	r.store[plan.ID] = plan
	return nil
}

func (r *inMemPlanRepo) GetByID(tenantID, planID uuid.UUID) (*model.BusinessPlan, error) {
	plan, ok := r.store[planID]
	if !ok || plan.TenantID != tenantID {
		return nil, nil
	}
	return plan, nil
}

func (r *inMemPlanRepo) ListByTenant(tenantID uuid.UUID, offset, limit int) ([]*model.BusinessPlan, error) {
	var plans []*model.BusinessPlan
	for _, p := range r.store {
		if p.TenantID == tenantID {
			cp := *p
			plans = append(plans, &cp)
		}
	}
	return plans, nil
}

func (r *inMemPlanRepo) CountByTenant(tenantID uuid.UUID) (int64, error) {
	count := 0
	for _, p := range r.store {
		if p.TenantID == tenantID {
			count++
		}
	}
	return int64(count), nil
}

func (r *inMemPlanRepo) Update(plan *model.BusinessPlan) error {
	r.store[plan.ID] = plan
	return nil
}

func (r *inMemPlanRepo) Delete(tenantID, planID uuid.UUID) error {
	if plan, ok := r.store[planID]; ok && plan.TenantID == tenantID {
		delete(r.store, planID)
		return nil
	}
	return errors.New("not found")
}

func (r *inMemPlanRepo) PurgeDemoPlans(tenantID uuid.UUID) error {
	return nil
}

// inMemSettingsRepo is a minimal in-memory implementation of SettingsRepository.
type inMemSettingsRepo struct {
	configs map[uuid.UUID]*model.PlanConfig
}

func newInMemSettingsRepo() *inMemSettingsRepo {
	return &inMemSettingsRepo{
		configs: make(map[uuid.UUID]*model.PlanConfig),
	}
}

func (r *inMemSettingsRepo) GetConfig(tenantID, scenarioID uuid.UUID) (*model.PlanConfig, error) {
	cfg, ok := r.configs[scenarioID]
	if !ok || cfg.TenantID != tenantID {
		return nil, nil
	}
	return cfg, nil
}

func (r *inMemSettingsRepo) UpsertConfig(cfg *model.PlanConfig) error {
	r.configs[cfg.ScenarioID] = cfg
	return nil
}

func (r *inMemSettingsRepo) GetOpeningBalance(tenantID, scenarioID uuid.UUID) (*model.OpeningBalance, error) {
	return nil, nil
}

func (r *inMemSettingsRepo) UpsertOpeningBalance(b *model.OpeningBalance) error {
	return nil
}

func (r *inMemSettingsRepo) GetWCConfig(tenantID, scenarioID uuid.UUID) (*model.WorkingCapitalConfig, error) {
	return nil, nil
}

func (r *inMemSettingsRepo) UpsertWCConfig(cfg *model.WorkingCapitalConfig) error {
	return nil
}

func (r *inMemSettingsRepo) GetOpexPerHire(tenantID, scenarioID uuid.UUID) (*model.OpexPerHire, error) {
	return nil, nil
}

func (r *inMemSettingsRepo) UpsertOpexPerHire(opexPerHire *model.OpexPerHire) error {
	return nil
}

func (r *inMemSettingsRepo) GetCapexPerHire(tenantID, scenarioID uuid.UUID) (*model.CapexPerHire, error) {
	return nil, nil
}

func (r *inMemSettingsRepo) UpsertCapexPerHire(capexPerHire *model.CapexPerHire) error {
	return nil
}

func (r *inMemSettingsRepo) ListMultiYearAdjustments(tenantID, scenarioID uuid.UUID) ([]*model.MultiYearAdjustment, error) {
	return nil, nil
}

func (r *inMemSettingsRepo) BatchUpsertMultiYearAdjustments(tenantID, scenarioID uuid.UUID, adjustments []model.MultiYearAdjustment) error {
	return nil
}

// mockAuditRepo is a minimal in-memory implementation of AuditRepository.
type mockAuditRepo struct{}

func (r *mockAuditRepo) Create(auditLog *model.AuditLog) error                                            { return nil }
func (r *mockAuditRepo) ListByEntity(tenantID uuid.UUID, entityType string, entityID uuid.UUID) ([]*model.AuditLog, error) {
	return nil, nil
}
func (r *mockAuditRepo) ListByTenant(tenantID uuid.UUID, offset, limit int) ([]*model.AuditLog, error) {
	return nil, nil
}
func (r *mockAuditRepo) CountByTenant(tenantID uuid.UUID) (int64, error)                             { return 0, nil }
func (r *mockAuditRepo) GetByID(tenantID, entryID uuid.UUID) (*model.AuditLog, error)               { return nil, nil }
func (r *mockAuditRepo) ListByEntityID(tenantID, entityID uuid.UUID) ([]*model.AuditLog, error)     { return nil, nil }
func (r *mockAuditRepo) ListByUser(tenantID, userID uuid.UUID, offset, limit int) ([]*model.AuditLog, error) {
	return nil, nil
}

// mockPlanDeps provides minimal mocks for PlanDeps interface.
type mockPlanDeps struct {
	scenarioRepo repo.ScenarioRepository
	db           *gorm.DB
}

func (m *mockPlanDeps) GetScenario() repo.ScenarioRepository { return m.scenarioRepo }
func (m *mockPlanDeps) GetProduct() repo.ProductRepository  { return nil }
func (m *mockPlanDeps) GetStaff() repo.StaffRepository      { return nil }
func (m *mockPlanDeps) GetCapex() repo.CapexRepository      { return nil }
func (m *mockPlanDeps) GetOpex() repo.OpexRepository        { return nil }
func (m *mockPlanDeps) GetPnL() repo.PnLRepository          { return nil }
func (m *mockPlanDeps) GetFiPlan() repo.FiplanRepository    { return nil }
func (m *mockPlanDeps) GetPnlCash() repo.PnlCashRepository  { return nil }
func (m *mockPlanDeps) GetWCR() repo.WCRRepository          { return nil }
func (m *mockPlanDeps) GetCash() repo.CashRepository        { return nil }
func (m *mockPlanDeps) GetBudget() repo.BudgetRepository    { return nil }
func (m *mockPlanDeps) GetDB() *gorm.DB                     { return m.db }

// newTestPlanService creates a PlanService for testing.
func newTestPlanService(planRepo repo.PlanRepository, settingsRepo repo.SettingsRepository, deps repo.PlanDeps) *PlanService {
	logger := logrus.NewEntry(logrus.New())
	emitter := event.NewEmitter(logger)
	return NewPlanService(planRepo, settingsRepo, &mockAuditRepo{}, deps, nil, emitter, logger)
}

// ── GetPlan tests ──────────────────────────────────────────────────────────────

func TestPlanService_GetPlan_NotFound(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	planRepo := newInMemPlanRepo()
	settingsRepo := newInMemSettingsRepo()
	scenarioRepo := newInMemScenarioRepo()
	deps := &mockPlanDeps{scenarioRepo: scenarioRepo}

	svc := newTestPlanService(planRepo, settingsRepo, deps)
	ctx := context.Background()

	plan, err := svc.GetPlan(ctx, tenantID, planID)

	assert.Nil(t, plan)
	assert.Error(t, err)
	// Verify it's a NotFound error
	apiErr := err.(*apierror.AppError)
	assert.Equal(t, "not_found", apiErr.Code)
}

func TestPlanService_GetPlan_Success(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	plan := &model.BusinessPlan{
		TenantScoped: model.TenantScoped{ID: planID, TenantID: tenantID},
		Name:         "Q1 Plan",
		Description:  "First quarter projection",
		Status:       "draft",
	}

	planRepo := newInMemPlanRepo()
	planRepo.Create(plan)

	settingsRepo := newInMemSettingsRepo()
	scenarioRepo := newInMemScenarioRepo()
	deps := &mockPlanDeps{scenarioRepo: scenarioRepo}

	svc := newTestPlanService(planRepo, settingsRepo, deps)
	ctx := context.Background()

	got, err := svc.GetPlan(ctx, tenantID, planID)

	require.NoError(t, err)
	assert.Equal(t, plan.Name, got.Name)
	assert.Equal(t, "Q1 Plan", got.Name)
}

// ── CreatePlan tests ───────────────────────────────────────────────────────────

func TestPlanService_CreatePlan_Success(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	planRepo := newInMemPlanRepo()
	settingsRepo := newInMemSettingsRepo()
	scenarioRepo := newInMemScenarioRepo()
	deps := &mockPlanDeps{scenarioRepo: scenarioRepo}

	svc := newTestPlanService(planRepo, settingsRepo, deps)
	ctx := context.Background()

	plan, err := svc.CreatePlan(ctx, tenantID, userID, "Growth Plan", "2024 growth strategy", "US")

	require.NoError(t, err)
	assert.NotNil(t, plan)
	assert.Equal(t, "Growth Plan", plan.Name)
	assert.Equal(t, "2024 growth strategy", plan.Description)
	assert.Equal(t, "draft", plan.Status)
	assert.Equal(t, tenantID, plan.TenantID)

	// Verify plan was persisted
	retrieved, err := svc.GetPlan(ctx, tenantID, plan.ID)
	require.NoError(t, err)
	assert.Equal(t, plan.Name, retrieved.Name)
}

func TestPlanService_CreatePlan_FreemiumLimitedTo1(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	planRepo := newInMemPlanRepo()
	settingsRepo := newInMemSettingsRepo()
	scenarioRepo := newInMemScenarioRepo()
	deps := &mockPlanDeps{scenarioRepo: scenarioRepo}
	svc := newTestPlanService(planRepo, settingsRepo, deps)

	ctx := ctxutil.WithUserPlan(context.Background(), "freemium")

	// First plan must succeed.
	_, err := svc.CreatePlan(ctx, tenantID, userID, "Plan 1", "", "BE")
	require.NoError(t, err)

	// Second plan must be rejected.
	_, err = svc.CreatePlan(ctx, tenantID, userID, "Plan 2", "", "BE")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1")
}

func TestPlanService_CreatePlan_ProLimitedTo3(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	planRepo := newInMemPlanRepo()
	settingsRepo := newInMemSettingsRepo()
	scenarioRepo := newInMemScenarioRepo()
	deps := &mockPlanDeps{scenarioRepo: scenarioRepo}
	svc := newTestPlanService(planRepo, settingsRepo, deps)

	ctx := ctxutil.WithUserPlan(context.Background(), "pro")

	// Three plans must succeed.
	for i := 1; i <= 3; i++ {
		_, err := svc.CreatePlan(ctx, tenantID, userID, fmt.Sprintf("Plan %d", i), "", "BE")
		require.NoError(t, err, "plan %d should be allowed on Pro", i)
	}

	// Fourth plan must be rejected.
	_, err := svc.CreatePlan(ctx, tenantID, userID, "Plan 4", "", "BE")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "3")
}

func TestPlanService_CreatePlan_EnterpriseUnlimited(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	planRepo := newInMemPlanRepo()
	settingsRepo := newInMemSettingsRepo()
	scenarioRepo := newInMemScenarioRepo()
	deps := &mockPlanDeps{scenarioRepo: scenarioRepo}
	svc := newTestPlanService(planRepo, settingsRepo, deps)

	ctx := ctxutil.WithUserPlan(context.Background(), "enterprise")

	// Enterprise users must be able to create more than 3 plans.
	for i := 1; i <= 5; i++ {
		_, err := svc.CreatePlan(ctx, tenantID, userID, fmt.Sprintf("Plan %d", i), "", "BE")
		require.NoError(t, err, "plan %d should be allowed on Enterprise", i)
	}
}

// ── UpdatePlan tests ───────────────────────────────────────────────────────────

func TestPlanService_UpdatePlan_Name(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	plan := &model.BusinessPlan{
		TenantScoped: model.TenantScoped{ID: planID, TenantID: tenantID},
		Name:         "Original Name",
		Description:  "Original description",
		Status:       "draft",
	}

	planRepo := newInMemPlanRepo()
	planRepo.Create(plan)

	settingsRepo := newInMemSettingsRepo()
	scenarioRepo := newInMemScenarioRepo()
	deps := &mockPlanDeps{scenarioRepo: scenarioRepo}

	svc := newTestPlanService(planRepo, settingsRepo, deps)
	ctx := context.Background()

	err := svc.UpdatePlan(ctx, tenantID, planID, "Updated Name", "", "")

	require.NoError(t, err)

	updated, err := svc.GetPlan(ctx, tenantID, planID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.Equal(t, "Original description", updated.Description)
}

func TestPlanService_UpdatePlan_Description(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	plan := &model.BusinessPlan{
		TenantScoped: model.TenantScoped{ID: planID, TenantID: tenantID},
		Name:         "Plan Name",
		Description:  "Original description",
		Status:       "draft",
	}

	planRepo := newInMemPlanRepo()
	planRepo.Create(plan)

	settingsRepo := newInMemSettingsRepo()
	scenarioRepo := newInMemScenarioRepo()
	deps := &mockPlanDeps{scenarioRepo: scenarioRepo}

	svc := newTestPlanService(planRepo, settingsRepo, deps)
	ctx := context.Background()

	err := svc.UpdatePlan(ctx, tenantID, planID, "", "Updated description", "")

	require.NoError(t, err)

	updated, err := svc.GetPlan(ctx, tenantID, planID)
	require.NoError(t, err)
	assert.Equal(t, "Plan Name", updated.Name)
	assert.Equal(t, "Updated description", updated.Description)
}

func TestPlanService_UpdatePlan_Status(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	plan := &model.BusinessPlan{
		TenantScoped: model.TenantScoped{ID: planID, TenantID: tenantID},
		Name:         "Plan Name",
		Description:  "Description",
		Status:       "draft",
	}

	planRepo := newInMemPlanRepo()
	planRepo.Create(plan)

	settingsRepo := newInMemSettingsRepo()
	scenarioRepo := newInMemScenarioRepo()
	deps := &mockPlanDeps{scenarioRepo: scenarioRepo}

	svc := newTestPlanService(planRepo, settingsRepo, deps)
	ctx := context.Background()

	err := svc.UpdatePlan(ctx, tenantID, planID, "", "", "review")

	require.NoError(t, err)

	updated, err := svc.GetPlan(ctx, tenantID, planID)
	require.NoError(t, err)
	assert.Equal(t, "review", updated.Status)
}

func TestPlanService_UpdatePlan_NotFound(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	planRepo := newInMemPlanRepo()
	settingsRepo := newInMemSettingsRepo()
	scenarioRepo := newInMemScenarioRepo()
	deps := &mockPlanDeps{scenarioRepo: scenarioRepo}

	svc := newTestPlanService(planRepo, settingsRepo, deps)
	ctx := context.Background()

	err := svc.UpdatePlan(ctx, tenantID, planID, "New Name", "", "")

	assert.Error(t, err)
	apiErr := err.(*apierror.AppError)
	assert.Equal(t, "not_found", apiErr.Code)
}

func TestPlanService_UpdatePlan_InvalidStatus(t *testing.T) {
	tenantID := uuid.New()
	planID := uuid.New()

	plan := &model.BusinessPlan{
		TenantScoped: model.TenantScoped{ID: planID, TenantID: tenantID},
		Name:         "Plan Name",
		Description:  "Description",
		Status:       "draft",
	}

	planRepo := newInMemPlanRepo()
	planRepo.Create(plan)

	settingsRepo := newInMemSettingsRepo()
	scenarioRepo := newInMemScenarioRepo()
	deps := &mockPlanDeps{scenarioRepo: scenarioRepo}

	svc := newTestPlanService(planRepo, settingsRepo, deps)
	ctx := context.Background()

	// Try to set invalid status — should be ignored
	err := svc.UpdatePlan(ctx, tenantID, planID, "", "", "invalid_status")

	require.NoError(t, err)

	updated, err := svc.GetPlan(ctx, tenantID, planID)
	require.NoError(t, err)
	// Status should remain unchanged when invalid status is provided
	assert.Equal(t, "draft", updated.Status)
}
