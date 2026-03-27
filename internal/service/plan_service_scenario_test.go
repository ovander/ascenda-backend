package service

// Tests for PlanService.CreateScenario and PlanService.UpdateScenario,
// focusing on the newly-added Description field.

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ascenda/internal/event"
	"ascenda/internal/model"
	"ascenda/internal/pkg/apierror"
	"ascenda/internal/repo"
)

// ── minimal repo mocks ────────────────────────────────────────────────────────

// inMemScenarioRepo satisfies repo.ScenarioRepository with an in-memory map.
type inMemScenarioRepo struct {
	mu       sync.Mutex
	store    map[string]*model.Scenario
	createFn func(*model.Scenario) error // optional hook to inject errors
	updateFn func(*model.Scenario) error
}

func newInMemScenarioRepo() *inMemScenarioRepo {
	return &inMemScenarioRepo{store: make(map[string]*model.Scenario)}
}

func (r *inMemScenarioRepo) Create(s *model.Scenario) error {
	if r.createFn != nil {
		return r.createFn(s)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store[s.ID.String()] = s
	return nil
}

func (r *inMemScenarioRepo) GetByID(tenantID, scenarioID uuid.UUID) (*model.Scenario, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.store[scenarioID.String()]
	if !ok || s.TenantID != tenantID {
		return nil, errors.New("not found")
	}
	return s, nil
}

func (r *inMemScenarioRepo) ListByPlan(tenantID, planID uuid.UUID) ([]*model.Scenario, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*model.Scenario
	for _, s := range r.store {
		if s.TenantID == tenantID && s.PlanID == planID {
			cp := *s
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *inMemScenarioRepo) Update(s *model.Scenario) error {
	if r.updateFn != nil {
		return r.updateFn(s)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store[s.ID.String()] = s
	return nil
}

func (r *inMemScenarioRepo) Delete(tenantID, scenarioID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.store, scenarioID.String())
	return nil
}

// stubPlanRepo satisfies repo.PlanRepository with minimal in-memory logic.
type stubPlanRepo struct {
	plans map[string]*model.BusinessPlan
}

func newStubPlanRepo() *stubPlanRepo {
	return &stubPlanRepo{plans: make(map[string]*model.BusinessPlan)}
}

func (r *stubPlanRepo) Create(p *model.BusinessPlan) error {
	r.plans[p.ID.String()] = p
	return nil
}

func (r *stubPlanRepo) GetByID(tenantID, planID uuid.UUID) (*model.BusinessPlan, error) {
	p, ok := r.plans[planID.String()]
	if !ok || p.TenantID != tenantID {
		return nil, errors.New("plan not found")
	}
	return p, nil
}

func (r *stubPlanRepo) ListByTenant(tenantID uuid.UUID, offset, limit int) ([]*model.BusinessPlan, error) {
	return nil, nil
}

func (r *stubPlanRepo) CountByTenant(tenantID uuid.UUID) (int64, error) { return 0, nil }

func (r *stubPlanRepo) Update(p *model.BusinessPlan) error { return nil }

func (r *stubPlanRepo) Delete(tenantID, planID uuid.UUID) error { return nil }

func (r *stubPlanRepo) PurgeDemoPlans(tenantID uuid.UUID) error { return nil }

// stubAuditRepo satisfies repo.AuditRepository — all operations are no-ops.
type stubAuditRepo struct{}

func (r *stubAuditRepo) Create(a *model.AuditLog) error                                    { return nil }
func (r *stubAuditRepo) GetByID(tenantID, id uuid.UUID) (*model.AuditLog, error)           { return nil, nil }
func (r *stubAuditRepo) ListByEntity(tenantID uuid.UUID, entityType string, entityID uuid.UUID) ([]*model.AuditLog, error) {
	return nil, nil
}
func (r *stubAuditRepo) ListByTenant(tenantID uuid.UUID, offset, limit int) ([]*model.AuditLog, error) {
	return nil, nil
}
func (r *stubAuditRepo) CountByTenant(tenantID uuid.UUID) (int64, error)             { return 0, nil }
func (r *stubAuditRepo) ListByEntityID(tenantID, entityID uuid.UUID) ([]*model.AuditLog, error) {
	return nil, nil
}
func (r *stubAuditRepo) ListByUser(tenantID, userID uuid.UUID, offset, limit int) ([]*model.AuditLog, error) {
	return nil, nil
}

// ── test factory ──────────────────────────────────────────────────────────────

type scenarioTestBed struct {
	svc         *PlanService
	scenarioRepo *inMemScenarioRepo
	planRepo    *stubPlanRepo
	settingsRepo *MockSettingsRepo
	tenantID    uuid.UUID
	planID      uuid.UUID
}

func newScenarioTestBed(t *testing.T) *scenarioTestBed {
	t.Helper()

	scenarioRepo := newInMemScenarioRepo()
	planRepo := newStubPlanRepo()
	settingsRepo := NewMockSettingsRepo()

	emitter := event.NewEmitter(logrus.NewEntry(logrus.New()))
	t.Cleanup(emitter.Close)

	// Build a minimal RepoBundle with the scenario repo and an empty settings repo.
	// Other fields in RepoBundle are nil — PlanService.CreateScenario and
	// UpdateScenario only touch repos.Scenario, planRepo, settingsRepo, and emitter.
	repos := &repo.RepoBundle{
		Scenario: scenarioRepo,
	}

	svc := NewPlanService(planRepo, settingsRepo, &stubAuditRepo{}, repos, nil, emitter, logrus.NewEntry(logrus.New()))

	tenantID := uuid.New()
	planID := uuid.New()

	// Seed a plan so GetPlan succeeds.
	plan := &model.BusinessPlan{
		TenantScoped: model.TenantScoped{ID: planID, TenantID: tenantID},
		Name:         "Test Plan",
	}
	require.NoError(t, planRepo.Create(plan))

	return &scenarioTestBed{
		svc:          svc,
		scenarioRepo: scenarioRepo,
		planRepo:     planRepo,
		settingsRepo: settingsRepo,
		tenantID:     tenantID,
		planID:       planID,
	}
}

// ── CreateScenario ────────────────────────────────────────────────────────────

func TestPlanService_CreateScenario_NameAndDescription(t *testing.T) {
	tb := newScenarioTestBed(t)
	ctx := context.Background()

	sc, err := tb.svc.CreateScenario(ctx, tb.tenantID, tb.planID, "Base Case", "Conservative growth assumptions")
	require.NoError(t, err)
	require.NotNil(t, sc)

	assert.Equal(t, "Base Case", sc.Name)
	assert.Equal(t, "Conservative growth assumptions", sc.Description)
	assert.Equal(t, tb.tenantID, sc.TenantID)
	assert.Equal(t, tb.planID, sc.PlanID)
	assert.False(t, sc.IsDefault)
	assert.NotEqual(t, uuid.Nil, sc.ID)
}

func TestPlanService_CreateScenario_EmptyDescriptionStored(t *testing.T) {
	tb := newScenarioTestBed(t)
	ctx := context.Background()

	sc, err := tb.svc.CreateScenario(ctx, tb.tenantID, tb.planID, "Optimistic", "")
	require.NoError(t, err)
	assert.Equal(t, "", sc.Description)
}

func TestPlanService_CreateScenario_DescriptionPersisted(t *testing.T) {
	// Verify the description is actually written to the repo, not just returned.
	tb := newScenarioTestBed(t)
	ctx := context.Background()

	sc, err := tb.svc.CreateScenario(ctx, tb.tenantID, tb.planID, "Worst Case", "Pessimistic outlook")
	require.NoError(t, err)

	stored, err := tb.scenarioRepo.GetByID(tb.tenantID, sc.ID)
	require.NoError(t, err)
	assert.Equal(t, "Pessimistic outlook", stored.Description)
}

func TestPlanService_CreateScenario_DefaultConfigSeeded(t *testing.T) {
	// A default PlanConfig must be upserted for the new scenario.
	tb := newScenarioTestBed(t)
	ctx := context.Background()

	sc, err := tb.svc.CreateScenario(ctx, tb.tenantID, tb.planID, "X", "")
	require.NoError(t, err)

	cfg, err := tb.settingsRepo.GetConfig(tb.tenantID, sc.ID)
	require.NoError(t, err, "default config should have been upserted")
	require.NotNil(t, cfg)
	assert.Equal(t, sc.ID, cfg.ScenarioID)
}

func TestPlanService_CreateScenario_PlanNotFound(t *testing.T) {
	tb := newScenarioTestBed(t)
	ctx := context.Background()

	_, err := tb.svc.CreateScenario(ctx, tb.tenantID, uuid.New(), "X", "")
	require.Error(t, err)

	var apiErr *apierror.AppError
	if assert.ErrorAs(t, err, &apiErr) {
		assert.Equal(t, 404, apiErr.StatusCode)
	}
}

func TestPlanService_CreateScenario_RepoError(t *testing.T) {
	tb := newScenarioTestBed(t)
	tb.scenarioRepo.createFn = func(_ *model.Scenario) error {
		return errors.New("db constraint violation")
	}
	ctx := context.Background()

	_, err := tb.svc.CreateScenario(ctx, tb.tenantID, tb.planID, "X", "")
	require.Error(t, err)
	// service wraps in apierror.Internal
	var apiErr *apierror.AppError
	if assert.ErrorAs(t, err, &apiErr) {
		assert.Equal(t, 500, apiErr.StatusCode)
	}
}

func TestPlanService_CreateScenario_MultipleScenariosIndependent(t *testing.T) {
	tb := newScenarioTestBed(t)
	ctx := context.Background()

	sc1, err := tb.svc.CreateScenario(ctx, tb.tenantID, tb.planID, "Base", "Base desc")
	require.NoError(t, err)
	sc2, err := tb.svc.CreateScenario(ctx, tb.tenantID, tb.planID, "High", "High desc")
	require.NoError(t, err)

	assert.NotEqual(t, sc1.ID, sc2.ID)
	assert.Equal(t, "Base", sc1.Name)
	assert.Equal(t, "High", sc2.Name)
	assert.Equal(t, "Base desc", sc1.Description)
	assert.Equal(t, "High desc", sc2.Description)
}

// ── UpdateScenario ────────────────────────────────────────────────────────────

func TestPlanService_UpdateScenario_UpdatesNameAndDescription(t *testing.T) {
	tb := newScenarioTestBed(t)
	ctx := context.Background()

	sc, err := tb.svc.CreateScenario(ctx, tb.tenantID, tb.planID, "Old Name", "Old desc")
	require.NoError(t, err)

	err = tb.svc.UpdateScenario(ctx, tb.tenantID, sc.ID, "New Name", "New desc")
	require.NoError(t, err)

	stored, err := tb.scenarioRepo.GetByID(tb.tenantID, sc.ID)
	require.NoError(t, err)
	assert.Equal(t, "New Name", stored.Name)
	assert.Equal(t, "New desc", stored.Description)
}

func TestPlanService_UpdateScenario_EmptyNamePreservesExisting(t *testing.T) {
	tb := newScenarioTestBed(t)
	ctx := context.Background()

	sc, err := tb.svc.CreateScenario(ctx, tb.tenantID, tb.planID, "Keep This Name", "Original desc")
	require.NoError(t, err)

	// Pass empty name — service logic should leave the name unchanged.
	err = tb.svc.UpdateScenario(ctx, tb.tenantID, sc.ID, "", "Revised desc")
	require.NoError(t, err)

	stored, err := tb.scenarioRepo.GetByID(tb.tenantID, sc.ID)
	require.NoError(t, err)
	assert.Equal(t, "Keep This Name", stored.Name, "name must be preserved when empty string passed")
	assert.Equal(t, "Revised desc", stored.Description)
}

func TestPlanService_UpdateScenario_EmptyDescriptionPreservesExisting(t *testing.T) {
	tb := newScenarioTestBed(t)
	ctx := context.Background()

	sc, err := tb.svc.CreateScenario(ctx, tb.tenantID, tb.planID, "My Scenario", "Keep this description")
	require.NoError(t, err)

	// Pass empty description — service logic should leave description unchanged.
	err = tb.svc.UpdateScenario(ctx, tb.tenantID, sc.ID, "Renamed", "")
	require.NoError(t, err)

	stored, err := tb.scenarioRepo.GetByID(tb.tenantID, sc.ID)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", stored.Name)
	assert.Equal(t, "Keep this description", stored.Description, "description must be preserved when empty string passed")
}

func TestPlanService_UpdateScenario_BothFieldsUpdated(t *testing.T) {
	tb := newScenarioTestBed(t)
	ctx := context.Background()

	sc, err := tb.svc.CreateScenario(ctx, tb.tenantID, tb.planID, "Draft", "Draft notes")
	require.NoError(t, err)

	err = tb.svc.UpdateScenario(ctx, tb.tenantID, sc.ID, "Final", "Final assumptions locked")
	require.NoError(t, err)

	stored, err := tb.scenarioRepo.GetByID(tb.tenantID, sc.ID)
	require.NoError(t, err)
	assert.Equal(t, "Final", stored.Name)
	assert.Equal(t, "Final assumptions locked", stored.Description)
}

func TestPlanService_UpdateScenario_ScenarioNotFound(t *testing.T) {
	tb := newScenarioTestBed(t)
	ctx := context.Background()

	err := tb.svc.UpdateScenario(ctx, tb.tenantID, uuid.New(), "X", "Y")
	require.Error(t, err)

	var apiErr *apierror.AppError
	if assert.ErrorAs(t, err, &apiErr) {
		assert.Equal(t, 404, apiErr.StatusCode)
	}
}

func TestPlanService_UpdateScenario_TenantIsolation(t *testing.T) {
	tb := newScenarioTestBed(t)
	ctx := context.Background()

	sc, err := tb.svc.CreateScenario(ctx, tb.tenantID, tb.planID, "Mine", "My desc")
	require.NoError(t, err)

	// Another tenant tries to update the same scenario ID — should fail.
	otherTenant := uuid.New()
	err = tb.svc.UpdateScenario(ctx, otherTenant, sc.ID, "Hijacked", "Hijacked desc")
	require.Error(t, err)

	// Original data must be intact.
	stored, _ := tb.scenarioRepo.GetByID(tb.tenantID, sc.ID)
	assert.Equal(t, "Mine", stored.Name)
}

func TestPlanService_UpdateScenario_RepoError(t *testing.T) {
	tb := newScenarioTestBed(t)
	ctx := context.Background()

	sc, err := tb.svc.CreateScenario(ctx, tb.tenantID, tb.planID, "X", "")
	require.NoError(t, err)

	tb.scenarioRepo.updateFn = func(_ *model.Scenario) error {
		return errors.New("db write failed")
	}

	err = tb.svc.UpdateScenario(ctx, tb.tenantID, sc.ID, "Y", "")
	require.Error(t, err)

	var apiErr *apierror.AppError
	if assert.ErrorAs(t, err, &apiErr) {
		assert.Equal(t, 500, apiErr.StatusCode)
	}
}

// ── Description round-trip ────────────────────────────────────────────────────

func TestPlanService_Scenario_DescriptionRoundTrip(t *testing.T) {
	// End-to-end: create with description, update description, verify get returns updated value.
	tb := newScenarioTestBed(t)
	ctx := context.Background()

	const origDesc = "Original description with special chars: éàü & <>"
	const newDesc = "Revised: 2025 outlook — upside scenario"

	sc, err := tb.svc.CreateScenario(ctx, tb.tenantID, tb.planID, "Scenario Alpha", origDesc)
	require.NoError(t, err)
	assert.Equal(t, origDesc, sc.Description)

	err = tb.svc.UpdateScenario(ctx, tb.tenantID, sc.ID, "Scenario Alpha", newDesc)
	require.NoError(t, err)

	fetched, err := tb.svc.GetScenario(ctx, tb.tenantID, sc.ID)
	require.NoError(t, err)
	assert.Equal(t, newDesc, fetched.Description)
}

// ── countryForPlan fallback ───────────────────────────────────────────────────

func TestPlanService_CreateScenario_InheritsCountryFromPlan(t *testing.T) {
	// Seed a scenario with a config so countryForPlan returns "FR".
	tb := newScenarioTestBed(t)
	ctx := context.Background()

	// Create first scenario, mark it as default, and set its config country to "FR".
	sc1, err := tb.svc.CreateScenario(ctx, tb.tenantID, tb.planID, "First", "")
	require.NoError(t, err)
	// Mark sc1 as the plan's default so countryForPlan deterministically picks it.
	sc1stored, _ := tb.scenarioRepo.GetByID(tb.tenantID, sc1.ID)
	sc1stored.IsDefault = true
	_ = tb.scenarioRepo.Update(sc1stored)
	cfg, _ := tb.settingsRepo.GetConfig(tb.tenantID, sc1.ID)
	cfg.Country = "FR"
	_ = tb.settingsRepo.UpsertConfig(cfg)

	// Create a second scenario — it should inherit "FR" country for its default config.
	sc2, err := tb.svc.CreateScenario(ctx, tb.tenantID, tb.planID, "Second", "")
	require.NoError(t, err)

	cfg2, err := tb.settingsRepo.GetConfig(tb.tenantID, sc2.ID)
	require.NoError(t, err)
	assert.Equal(t, "FR", cfg2.Country, "second scenario should inherit country from first")
}
