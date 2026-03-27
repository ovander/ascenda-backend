package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ascenda/internal/model"
)

func newTestCashService() (*CashService, *MockCashRepo) {
	repo := NewMockCashRepo()
	logger := logrus.NewEntry(logrus.New())
	return NewCashService(repo, nil, nil, logger), repo
}

func TestCash_ListOverrides_EmptyInitially(t *testing.T) {
	svc, _ := newTestCashService()
	rows, err := svc.ListOverrides(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestCash_UpdateOverrides_StampsIDs(t *testing.T) {
	svc, repo := newTestCashService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	overrides := []model.CashMonthlyOverride{
		{ScenarioID: scenarioID, LineID: model.CashOtherRevenues, YearIndex: 1, Month: 1, Amount: decimal.NewFromInt(5000)},
	}
	require.NoError(t, svc.UpdateOverrides(ctx, tenantID, scenarioID, overrides))

	stored, _ := repo.ListByScenario(tenantID, scenarioID)
	require.Len(t, stored, 1)
	assert.NotEqual(t, uuid.Nil, stored[0].ID)
	assert.Equal(t, tenantID, stored[0].TenantID)
	assert.Equal(t, scenarioID, stored[0].ScenarioID)
}

func TestCash_ListOverrides_ReturnsAfterUpdate(t *testing.T) {
	svc, _ := newTestCashService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateOverrides(ctx, tenantID, scenarioID, []model.CashMonthlyOverride{
		{ScenarioID: scenarioID, LineID: model.CashOtherRevenues, YearIndex: 1, Month: 1, Amount: decimal.NewFromInt(1000)},
		{ScenarioID: scenarioID, LineID: model.CashOtherRevenues, YearIndex: 1, Month: 2, Amount: decimal.NewFromInt(1100)},
		{ScenarioID: scenarioID, LineID: model.CashOpexRentTelecom, YearIndex: 1, Month: 1, Amount: decimal.NewFromInt(800)},
	})

	rows, err := svc.ListOverrides(ctx, tenantID, scenarioID)
	require.NoError(t, err)
	assert.Len(t, rows, 3)
}

func TestCash_UpdateOverrides_Overwrites(t *testing.T) {
	svc, _ := newTestCashService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateOverrides(ctx, tenantID, scenarioID, []model.CashMonthlyOverride{
		{ScenarioID: scenarioID, LineID: model.CashOtherRevenues, YearIndex: 1, Month: 1, Amount: decimal.NewFromInt(1000)},
		{ScenarioID: scenarioID, LineID: model.CashOtherRevenues, YearIndex: 1, Month: 2, Amount: decimal.NewFromInt(1100)},
	})
	svc.UpdateOverrides(ctx, tenantID, scenarioID, []model.CashMonthlyOverride{
		{ScenarioID: scenarioID, LineID: model.CashLTLoans, YearIndex: 1, Month: 6, Amount: decimal.NewFromInt(50000)},
	})

	rows, _ := svc.ListOverrides(ctx, tenantID, scenarioID)
	assert.Len(t, rows, 1, "second upsert should replace first batch")
}

func TestCash_ScenarioIsolation(t *testing.T) {
	svc, _ := newTestCashService()
	tenantID := uuid.New()
	s1, s2 := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateOverrides(ctx, tenantID, s1, []model.CashMonthlyOverride{
		{ScenarioID: s1, LineID: model.CashOtherRevenues, YearIndex: 1, Month: 1, Amount: decimal.NewFromInt(500)},
	})
	svc.UpdateOverrides(ctx, tenantID, s2, []model.CashMonthlyOverride{
		{ScenarioID: s2, LineID: model.CashOpexFees, YearIndex: 1, Month: 1, Amount: decimal.NewFromInt(300)},
		{ScenarioID: s2, LineID: model.CashCapexITVehicles, YearIndex: 1, Month: 3, Amount: decimal.NewFromInt(10000)},
	})

	r1, _ := svc.ListOverrides(ctx, tenantID, s1)
	r2, _ := svc.ListOverrides(ctx, tenantID, s2)
	assert.Len(t, r1, 1)
	assert.Len(t, r2, 2)
}
