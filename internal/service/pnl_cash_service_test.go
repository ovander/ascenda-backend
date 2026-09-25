package service

import (
	"context"
	"testing"

	"ascenda/internal/event"
	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPnlCashService() (*PnlCashService, *MockPnlCashRepo) {
	repo := NewMockPnlCashRepo()
	logger := logrus.NewEntry(logrus.New())
	emitter := event.NewEmitter(logger)
	return NewPnlCashService(repo, nil, emitter, logger), repo
}

func TestPnlCash_ListEntries_EmptyInitially(t *testing.T) {
	svc, _ := newTestPnlCashService()
	rows, err := svc.ListEntries(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestPnlCash_UpdateEntries_StampsIDs(t *testing.T) {
	svc, repo := newTestPnlCashService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	entries := []model.PnlCashEntry{
		{ScenarioID: scenarioID, LineID: "revenue_adjustment", YearIndex: 1, Amount: decimal.NewFromInt(5000)},
	}
	require.NoError(t, svc.UpdateEntries(ctx, tenantID, scenarioID, entries))

	stored, _ := repo.ListByScenario(tenantID, scenarioID)
	require.Len(t, stored, 1)
	assert.NotEqual(t, uuid.Nil, stored[0].ID)
	assert.Equal(t, tenantID, stored[0].TenantID)
	assert.Equal(t, scenarioID, stored[0].ScenarioID)
}

func TestPnlCash_ListEntries_ReturnsAfterUpdate(t *testing.T) {
	svc, _ := newTestPnlCashService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateEntries(ctx, tenantID, scenarioID, []model.PnlCashEntry{
		{ScenarioID: scenarioID, LineID: "revenue_adjustment", YearIndex: 1, Amount: decimal.NewFromInt(1000)},
		{ScenarioID: scenarioID, LineID: "revenue_adjustment", YearIndex: 2, Amount: decimal.NewFromInt(1100)},
	})

	rows, err := svc.ListEntries(ctx, tenantID, scenarioID)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}

func TestPnlCash_UpdateEntries_Overwrites(t *testing.T) {
	svc, _ := newTestPnlCashService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateEntries(ctx, tenantID, scenarioID, []model.PnlCashEntry{
		{ScenarioID: scenarioID, LineID: "revenue_adjustment", YearIndex: 1, Amount: decimal.NewFromInt(1000)},
		{ScenarioID: scenarioID, LineID: "revenue_adjustment", YearIndex: 2, Amount: decimal.NewFromInt(1100)},
	})
	svc.UpdateEntries(ctx, tenantID, scenarioID, []model.PnlCashEntry{
		{ScenarioID: scenarioID, LineID: "cost_adjustment", YearIndex: 1, Amount: decimal.NewFromInt(500)},
	})

	rows, _ := svc.ListEntries(ctx, tenantID, scenarioID)
	assert.Len(t, rows, 1)
}

func TestPnlCash_ScenarioIsolation(t *testing.T) {
	svc, _ := newTestPnlCashService()
	tenantID := uuid.New()
	s1, s2 := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateEntries(ctx, tenantID, s1, []model.PnlCashEntry{
		{ScenarioID: s1, LineID: "revenue_adjustment", YearIndex: 1, Amount: decimal.NewFromInt(100)},
	})
	svc.UpdateEntries(ctx, tenantID, s2, []model.PnlCashEntry{
		{ScenarioID: s2, LineID: "cost_adjustment", YearIndex: 1, Amount: decimal.NewFromInt(200)},
		{ScenarioID: s2, LineID: "cost_adjustment", YearIndex: 2, Amount: decimal.NewFromInt(300)},
	})

	r1, _ := svc.ListEntries(ctx, tenantID, s1)
	r2, _ := svc.ListEntries(ctx, tenantID, s2)
	assert.Len(t, r1, 1)
	assert.Len(t, r2, 2)
}
