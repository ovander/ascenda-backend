package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ascenda/internal/event"
	"ascenda/internal/model"
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
		TenantScoped:      model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:        scenarioID,
		CustomerPct30Days: decimal.NewFromFloat(0.5), // non-zero → not treated as unconfigured
		CustomerPct60Days: decimal.NewFromFloat(0.5),
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

func TestGetOpexPerHireReturnsDefaultsWhenAllZero(t *testing.T) {
	// Regression test: DB rows created before the defaults feature was introduced
	// contain all-zero values. GetOpexPerHire must detect this and return defaults
	// rather than the zero row, so that ComputeOpexSummary produces non-zero results.
	repo := NewExtendedMockSettingsRepo()
	svc := newTestSettingsService(repo)

	tenantID := uuid.New()
	scenarioID := uuid.New()

	// Simulate a legacy DB row: exists, but every field is zero.
	zeroRow := &model.OpexPerHire{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:   scenarioID,
		// all decimal fields default to zero
	}
	require.NoError(t, repo.UpsertOpexPerHire(zeroRow))

	result, err := svc.GetOpexPerHire(context.Background(), tenantID, scenarioID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	// Must return defaults, not zeros
	assert.False(t, result.PropertyRentals.IsZero(), "PropertyRentals should be non-zero default")
	assert.False(t, result.PostageTelecom.IsZero(), "PostageTelecom should be non-zero default")
	assert.False(t, result.TravelTransportation.IsZero(), "TravelTransportation should be non-zero default")
	assert.False(t, result.RecruitTrainingPctPayroll.IsZero(), "RecruitTrainingPctPayroll should be non-zero default")
}

func TestGetOpexPerHirePreservesNonZeroRow(t *testing.T) {
	// When a row has at least one non-zero field it must be returned as-is,
	// not overwritten by defaults.
	repo := NewExtendedMockSettingsRepo()
	svc := newTestSettingsService(repo)

	tenantID := uuid.New()
	scenarioID := uuid.New()

	customRow := &model.OpexPerHire{
		TenantScoped:     model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:       scenarioID,
		PropertyRentals:  decimal.NewFromInt(99), // custom non-zero value
		// all others remain zero
	}
	require.NoError(t, repo.UpsertOpexPerHire(customRow))

	result, err := svc.GetOpexPerHire(context.Background(), tenantID, scenarioID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, decimal.NewFromInt(99).Equal(result.PropertyRentals),
		"Non-zero custom row should be returned unchanged")
	assert.True(t, result.PostageTelecom.IsZero(),
		"Other fields should stay zero as stored")
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
