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

func newTestBudgetService() (*BudgetService, *MockBudgetRepo) {
	repo := NewMockBudgetRepo()
	logger := logrus.NewEntry(logrus.New())
	return NewBudgetService(repo, nil, nil, logger), repo
}

func TestBudget_ListOverrides_EmptyInitially(t *testing.T) {
	svc, _ := newTestBudgetService()
	rows, err := svc.ListOverrides(context.Background(), uuid.New(), uuid.New(), 1)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestBudget_UpdateOverrides_StampsIDs(t *testing.T) {
	svc, repo := newTestBudgetService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	overrides := []model.BudgetMonthlyOverride{
		{ScenarioID: scenarioID, LineID: model.BudgetSalesRevenue, YearIndex: 1, Month: 1, Amount: decimal.NewFromInt(20000)},
	}
	require.NoError(t, svc.UpdateOverrides(ctx, tenantID, scenarioID, 1, overrides))

	stored, _ := repo.ListByScenario(tenantID, scenarioID, 1)
	require.Len(t, stored, 1)
	assert.NotEqual(t, uuid.Nil, stored[0].ID)
	assert.Equal(t, tenantID, stored[0].TenantID)
	assert.Equal(t, scenarioID, stored[0].ScenarioID)
}

func TestBudget_ListOverrides_ReturnsAfterUpdate(t *testing.T) {
	svc, _ := newTestBudgetService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateOverrides(ctx, tenantID, scenarioID, 1, []model.BudgetMonthlyOverride{
		{ScenarioID: scenarioID, LineID: model.BudgetSalesRevenue, YearIndex: 1, Month: 1, Amount: decimal.NewFromInt(10000)},
		{ScenarioID: scenarioID, LineID: model.BudgetSalesRevenue, YearIndex: 1, Month: 2, Amount: decimal.NewFromInt(11000)},
		{ScenarioID: scenarioID, LineID: model.BudgetRawMaterials, YearIndex: 1, Month: 1, Amount: decimal.NewFromInt(3000)},
	})

	rows, err := svc.ListOverrides(ctx, tenantID, scenarioID, 1)
	require.NoError(t, err)
	assert.Len(t, rows, 3)
}

func TestBudget_ListOverrides_FiltersYear(t *testing.T) {
	svc, repo := newTestBudgetService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	// Write a single batch that covers both year 1 and year 2 entries
	svc.UpdateOverrides(ctx, tenantID, scenarioID, 1, []model.BudgetMonthlyOverride{
		{ScenarioID: scenarioID, LineID: model.BudgetSalesRevenue, YearIndex: 1, Month: 1, Amount: decimal.NewFromInt(10000)},
		{ScenarioID: scenarioID, LineID: model.BudgetSalesRevenue, YearIndex: 2, Month: 1, Amount: decimal.NewFromInt(12000)},
		{ScenarioID: scenarioID, LineID: model.BudgetSalesRevenue, YearIndex: 2, Month: 2, Amount: decimal.NewFromInt(13000)},
	})
	_ = repo // accessed through svc above

	y1, err1 := svc.ListOverrides(ctx, tenantID, scenarioID, 1)
	y2, err2 := svc.ListOverrides(ctx, tenantID, scenarioID, 2)
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Len(t, y1, 1, "only year-1 overrides should be returned for year=1")
	assert.Len(t, y2, 2, "only year-2 overrides should be returned for year=2")
}

func TestBudget_UpdateOverrides_Overwrites(t *testing.T) {
	svc, _ := newTestBudgetService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateOverrides(ctx, tenantID, scenarioID, 1, []model.BudgetMonthlyOverride{
		{ScenarioID: scenarioID, LineID: model.BudgetSalesRevenue, YearIndex: 1, Month: 1, Amount: decimal.NewFromInt(10000)},
		{ScenarioID: scenarioID, LineID: model.BudgetSalesRevenue, YearIndex: 1, Month: 2, Amount: decimal.NewFromInt(11000)},
	})
	svc.UpdateOverrides(ctx, tenantID, scenarioID, 1, []model.BudgetMonthlyOverride{
		{ScenarioID: scenarioID, LineID: model.BudgetPayroll, YearIndex: 1, Month: 1, Amount: decimal.NewFromInt(5000)},
	})

	rows, _ := svc.ListOverrides(ctx, tenantID, scenarioID, 1)
	assert.Len(t, rows, 1, "second upsert should replace first batch")
}

func TestBudget_ScenarioIsolation(t *testing.T) {
	svc, _ := newTestBudgetService()
	tenantID := uuid.New()
	s1, s2 := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateOverrides(ctx, tenantID, s1, 1, []model.BudgetMonthlyOverride{
		{ScenarioID: s1, LineID: model.BudgetSalesRevenue, YearIndex: 1, Month: 1, Amount: decimal.NewFromInt(5000)},
	})
	svc.UpdateOverrides(ctx, tenantID, s2, 1, []model.BudgetMonthlyOverride{
		{ScenarioID: s2, LineID: model.BudgetRawMaterials, YearIndex: 1, Month: 1, Amount: decimal.NewFromInt(2000)},
		{ScenarioID: s2, LineID: model.BudgetDirectLabor, YearIndex: 1, Month: 1, Amount: decimal.NewFromInt(1500)},
	})

	r1, _ := svc.ListOverrides(ctx, tenantID, s1, 1)
	r2, _ := svc.ListOverrides(ctx, tenantID, s2, 1)
	assert.Len(t, r1, 1)
	assert.Len(t, r2, 2)
}
