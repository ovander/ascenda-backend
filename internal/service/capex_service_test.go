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

func newTestCapexService() (*CapexService, *MockCapexRepo) {
	repo := NewMockCapexRepo()
	logger := logrus.NewEntry(logrus.New())
	emitter := event.NewEmitter(logger)
	return NewCapexService(repo, nil, emitter, logger), repo
}

func TestCapex_ListEntries_EmptyInitially(t *testing.T) {
	svc, _ := newTestCapexService()
	rows, err := svc.ListEntries(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestCapex_UpdateEntries_StampsIDs(t *testing.T) {
	svc, repo := newTestCapexService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	entries := []model.CapexEntry{
		{ScenarioID: scenarioID, Category: model.AssetComputerHWSW, YearIndex: 1, Amount: decimal.NewFromInt(50), DepreciationYears: 3},
	}
	require.NoError(t, svc.UpdateEntries(ctx, tenantID, scenarioID, entries))

	stored, _ := repo.ListByScenario(tenantID, scenarioID)
	require.Len(t, stored, 1)
	assert.NotEqual(t, uuid.Nil, stored[0].ID, "ID should be assigned")
	assert.Equal(t, tenantID, stored[0].TenantID)
	assert.Equal(t, scenarioID, stored[0].ScenarioID)
}

func TestCapex_ListEntries_ReturnsAfterUpdate(t *testing.T) {
	svc, _ := newTestCapexService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateEntries(ctx, tenantID, scenarioID, []model.CapexEntry{
		{ScenarioID: scenarioID, Category: model.AssetComputerHWSW, YearIndex: 1, Amount: decimal.NewFromInt(50), DepreciationYears: 3},
		{ScenarioID: scenarioID, Category: model.AssetOfficeFurniture, YearIndex: 1, Amount: decimal.NewFromInt(30), DepreciationYears: 5},
	})

	rows, err := svc.ListEntries(ctx, tenantID, scenarioID)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}

func TestCapex_UpdateEntries_PreservesExistingID(t *testing.T) {
	svc, repo := newTestCapexService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	existingID := uuid.New()
	entries := []model.CapexEntry{
		{TenantScoped: model.TenantScoped{ID: existingID, TenantID: tenantID}, ScenarioID: scenarioID, Category: model.AssetVehicles, YearIndex: 1, Amount: decimal.NewFromInt(20), DepreciationYears: 5},
	}
	require.NoError(t, svc.UpdateEntries(ctx, tenantID, scenarioID, entries))

	stored, _ := repo.ListByScenario(tenantID, scenarioID)
	assert.Equal(t, existingID, stored[0].ID, "pre-existing ID must not be overwritten")
}

func TestCapex_ScenarioIsolation(t *testing.T) {
	svc, _ := newTestCapexService()
	tenantID := uuid.New()
	s1, s2 := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateEntries(ctx, tenantID, s1, []model.CapexEntry{
		{ScenarioID: s1, Category: model.AssetBuildings, YearIndex: 1, Amount: decimal.NewFromInt(10), DepreciationYears: 10},
	})
	svc.UpdateEntries(ctx, tenantID, s2, []model.CapexEntry{
		{ScenarioID: s2, Category: model.AssetVehicles, YearIndex: 1, Amount: decimal.NewFromInt(20), DepreciationYears: 5},
		{ScenarioID: s2, Category: model.AssetComputerHWSW, YearIndex: 1, Amount: decimal.NewFromInt(30), DepreciationYears: 3},
	})

	r1, _ := svc.ListEntries(ctx, tenantID, s1)
	r2, _ := svc.ListEntries(ctx, tenantID, s2)
	assert.Len(t, r1, 1)
	assert.Len(t, r2, 2)
}
