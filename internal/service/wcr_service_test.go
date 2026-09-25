package service

import (
	"context"
	"testing"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestWCRService() (*WCRService, *MockWcrRepo) {
	repo := NewMockWcrRepo()
	logger := logrus.NewEntry(logrus.New())
	return NewWCRService(repo, nil, nil, logger), repo
}

func TestWCR_ListEntries_EmptyInitially(t *testing.T) {
	svc, _ := newTestWCRService()
	rows, err := svc.ListEntries(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestWCR_UpdateEntries_StampsIDs(t *testing.T) {
	svc, repo := newTestWCRService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	entries := []model.WCREntry{
		{ScenarioID: scenarioID, LineID: model.WCRPrepaidExpenses, YearIndex: 1, Amount: decimal.NewFromInt(8000)},
	}
	require.NoError(t, svc.UpdateEntries(ctx, tenantID, scenarioID, entries))

	stored, _ := repo.ListByScenario(tenantID, scenarioID)
	require.Len(t, stored, 1)
	assert.NotEqual(t, uuid.Nil, stored[0].ID)
	assert.Equal(t, tenantID, stored[0].TenantID)
	assert.Equal(t, scenarioID, stored[0].ScenarioID)
}

func TestWCR_ListEntries_ReturnsAfterUpdate(t *testing.T) {
	svc, _ := newTestWCRService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateEntries(ctx, tenantID, scenarioID, []model.WCREntry{
		{ScenarioID: scenarioID, LineID: model.WCRPrepaidExpenses, YearIndex: 1, Amount: decimal.NewFromInt(1000)},
		{ScenarioID: scenarioID, LineID: model.WCRDeferredRevenue, YearIndex: 1, Amount: decimal.NewFromInt(2000)},
		{ScenarioID: scenarioID, LineID: model.WCRTaxReceivables, YearIndex: 1, Amount: decimal.NewFromInt(500)},
	})

	rows, err := svc.ListEntries(ctx, tenantID, scenarioID)
	require.NoError(t, err)
	assert.Len(t, rows, 3)
}

func TestWCR_UpdateEntries_Overwrites(t *testing.T) {
	svc, _ := newTestWCRService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateEntries(ctx, tenantID, scenarioID, []model.WCREntry{
		{ScenarioID: scenarioID, LineID: model.WCRPrepaidExpenses, YearIndex: 1, Amount: decimal.NewFromInt(1000)},
		{ScenarioID: scenarioID, LineID: model.WCRDeferredRevenue, YearIndex: 1, Amount: decimal.NewFromInt(2000)},
	})
	svc.UpdateEntries(ctx, tenantID, scenarioID, []model.WCREntry{
		{ScenarioID: scenarioID, LineID: model.WCROtherAdjustPlus, YearIndex: 1, Amount: decimal.NewFromInt(300)},
	})

	rows, _ := svc.ListEntries(ctx, tenantID, scenarioID)
	assert.Len(t, rows, 1, "second upsert should replace first batch")
}

func TestWCR_ScenarioIsolation(t *testing.T) {
	svc, _ := newTestWCRService()
	tenantID := uuid.New()
	s1, s2 := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateEntries(ctx, tenantID, s1, []model.WCREntry{
		{ScenarioID: s1, LineID: model.WCRPrepaidExpenses, YearIndex: 1, Amount: decimal.NewFromInt(100)},
	})
	svc.UpdateEntries(ctx, tenantID, s2, []model.WCREntry{
		{ScenarioID: s2, LineID: model.WCRDeferredRevenue, YearIndex: 1, Amount: decimal.NewFromInt(200)},
		{ScenarioID: s2, LineID: model.WCRTaxReceivables, YearIndex: 2, Amount: decimal.NewFromInt(300)},
	})

	r1, _ := svc.ListEntries(ctx, tenantID, s1)
	r2, _ := svc.ListEntries(ctx, tenantID, s2)
	assert.Len(t, r1, 1)
	assert.Len(t, r2, 2)
}
