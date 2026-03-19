package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"kerplan/internal/event"
	"kerplan/internal/model"
)

// ExtendedMockSettingsRepo adds OpexPerHire and CapexPerHire support
// to the existing MockSettingsRepo for testing the full SettingsService.
type ExtendedMockSettingsRepo struct {
	*MockSettingsRepo
	opexPerHire  map[string]*model.OpexPerHire
	capexPerHire map[string]*model.CapexPerHire
}

func NewExtendedMockSettingsRepo() *ExtendedMockSettingsRepo {
	return &ExtendedMockSettingsRepo{
		MockSettingsRepo: NewMockSettingsRepo(),
		opexPerHire:      make(map[string]*model.OpexPerHire),
		capexPerHire:     make(map[string]*model.CapexPerHire),
	}
}

func (m *ExtendedMockSettingsRepo) GetOpexPerHire(tenantID, scenarioID uuid.UUID) (*model.OpexPerHire, error) {
	key := tenantID.String() + ":" + scenarioID.String()
	return m.opexPerHire[key], nil
}

func (m *ExtendedMockSettingsRepo) UpsertOpexPerHire(oph *model.OpexPerHire) error {
	key := oph.TenantID.String() + ":" + oph.ScenarioID.String()
	m.opexPerHire[key] = oph
	return nil
}

func (m *ExtendedMockSettingsRepo) GetCapexPerHire(tenantID, scenarioID uuid.UUID) (*model.CapexPerHire, error) {
	key := tenantID.String() + ":" + scenarioID.String()
	return m.capexPerHire[key], nil
}

func (m *ExtendedMockSettingsRepo) UpsertCapexPerHire(cph *model.CapexPerHire) error {
	key := cph.TenantID.String() + ":" + cph.ScenarioID.String()
	m.capexPerHire[key] = cph
	return nil
}

func (m *ExtendedMockSettingsRepo) ListMultiYearAdjustments(tenantID, scenarioID uuid.UUID) ([]*model.MultiYearAdjustment, error) {
	return nil, nil
}

func (m *ExtendedMockSettingsRepo) BatchUpsertMultiYearAdjustments(tenantID, scenarioID uuid.UUID, adjustments []model.MultiYearAdjustment) error {
	return nil
}

func newTestSettingsService(repo *ExtendedMockSettingsRepo) *SettingsService {
	logger := logrus.NewEntry(logrus.New())
	emitter := event.NewEmitter(logger)
	return NewSettingsService(repo, emitter, logger)
}

// ── Tests for empty defaults when records don't exist ──────────────

func TestGetOpeningBalanceReturnsDefaultsWhenNotFound(t *testing.T) {
	repo := NewExtendedMockSettingsRepo()
	svc := newTestSettingsService(repo)

	tenantID := uuid.New()
	scenarioID := uuid.New()

	balance, err := svc.GetOpeningBalance(context.Background(), tenantID, scenarioID)

	assert.NoError(t, err)
	assert.NotNil(t, balance)
	assert.Equal(t, scenarioID, balance.ScenarioID)
	assert.NotEqual(t, uuid.Nil, balance.ID)
}

func TestGetOpeningBalanceReturnsExistingWhenFound(t *testing.T) {
	repo := NewExtendedMockSettingsRepo()
	svc := newTestSettingsService(repo)

	tenantID := uuid.New()
	scenarioID := uuid.New()

	// Pre-populate
	existing := &model.OpeningBalance{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:   scenarioID,
	}
	repo.UpsertOpeningBalance(existing)

	balance, err := svc.GetOpeningBalance(context.Background(), tenantID, scenarioID)

	assert.NoError(t, err)
	assert.NotNil(t, balance)
	assert.Equal(t, existing.ID, balance.ID)
}

func TestGetWCConfigReturnsDefaultsWhenNotFound(t *testing.T) {
	repo := NewExtendedMockSettingsRepo()
	svc := newTestSettingsService(repo)

	tenantID := uuid.New()
	scenarioID := uuid.New()

	wcConfig, err := svc.GetWCConfig(context.Background(), tenantID, scenarioID)

	assert.NoError(t, err)
	assert.NotNil(t, wcConfig)
	assert.Equal(t, scenarioID, wcConfig.ScenarioID)
}

func TestGetWCConfigReturnsExistingWhenFound(t *testing.T) {
	repo := NewExtendedMockSettingsRepo()
	svc := newTestSettingsService(repo)

	tenantID := uuid.New()
	scenarioID := uuid.New()

	existing := &model.WorkingCapitalConfig{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:   scenarioID,
	}
	repo.UpsertWCConfig(existing)

	wcConfig, err := svc.GetWCConfig(context.Background(), tenantID, scenarioID)

	assert.NoError(t, err)
	assert.Equal(t, existing.ID, wcConfig.ID)
}

func TestGetOpexPerHireReturnsDefaultsWhenNotFound(t *testing.T) {
	repo := NewExtendedMockSettingsRepo()
	svc := newTestSettingsService(repo)

	tenantID := uuid.New()
	scenarioID := uuid.New()

	result, err := svc.GetOpexPerHire(context.Background(), tenantID, scenarioID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, scenarioID, result.ScenarioID)
}

func TestGetCapexPerHireReturnsDefaultsWhenNotFound(t *testing.T) {
	repo := NewExtendedMockSettingsRepo()
	svc := newTestSettingsService(repo)

	tenantID := uuid.New()
	scenarioID := uuid.New()

	result, err := svc.GetCapexPerHire(context.Background(), tenantID, scenarioID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, scenarioID, result.ScenarioID)
}

func TestGetConfigReturnsErrorWhenNotFound(t *testing.T) {
	repo := NewExtendedMockSettingsRepo()
	svc := newTestSettingsService(repo)

	tenantID := uuid.New()
	scenarioID := uuid.New()

	// Config still returns NotFound error (not defaults)
	config, err := svc.GetConfig(context.Background(), tenantID, scenarioID)

	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestUpdateOpeningBalanceSetsTenanAndScenario(t *testing.T) {
	repo := NewExtendedMockSettingsRepo()
	svc := newTestSettingsService(repo)

	tenantID := uuid.New()
	scenarioID := uuid.New()

	balance := &model.OpeningBalance{
		TenantScoped: model.TenantScoped{ID: uuid.New()},
	}

	err := svc.UpdateOpeningBalance(context.Background(), tenantID, scenarioID, balance)

	assert.NoError(t, err)
	assert.Equal(t, tenantID, balance.TenantID)
	assert.Equal(t, scenarioID, balance.ScenarioID)
}

func TestUpdateWCConfigSetsTenanAndScenario(t *testing.T) {
	repo := NewExtendedMockSettingsRepo()
	svc := newTestSettingsService(repo)

	tenantID := uuid.New()
	scenarioID := uuid.New()

	wcConfig := &model.WorkingCapitalConfig{
		TenantScoped: model.TenantScoped{ID: uuid.New()},
	}

	err := svc.UpdateWCConfig(context.Background(), tenantID, scenarioID, wcConfig)

	assert.NoError(t, err)
	assert.Equal(t, tenantID, wcConfig.TenantID)
	assert.Equal(t, scenarioID, wcConfig.ScenarioID)
}
