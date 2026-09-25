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

func newTestStaffService() (*StaffService, *MockStaffRepo) {
	repo := NewMockStaffRepo()
	logger := logrus.NewEntry(logrus.New())
	emitter := event.NewEmitter(logger)
	return NewStaffService(repo, nil, emitter, logger), repo
}

// ── Headcounts ────────────────────────────────────────────────────────────────

func TestStaff_ListHeadcounts_EmptyInitially(t *testing.T) {
	svc, _ := newTestStaffService()
	rows, err := svc.ListHeadcounts(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestStaff_UpdateHeadcounts_StampsIDs(t *testing.T) {
	svc, repo := newTestStaffService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	hcs := []model.StaffHeadcount{
		{ScenarioID: scenarioID, Category: model.CategorySalesTeam, YearIndex: 1, FTE: decimal.NewFromFloat(2)},
	}
	require.NoError(t, svc.UpdateHeadcounts(ctx, tenantID, scenarioID, hcs))

	stored, _ := repo.ListHeadcountsByScenario(tenantID, scenarioID)
	require.Len(t, stored, 1)
	assert.NotEqual(t, uuid.Nil, stored[0].ID, "ID should be assigned")
	assert.Equal(t, tenantID, stored[0].TenantID)
	assert.Equal(t, scenarioID, stored[0].ScenarioID)
}

func TestStaff_ListHeadcounts_ReturnsAfterUpdate(t *testing.T) {
	svc, _ := newTestStaffService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	hcs := []model.StaffHeadcount{
		{ScenarioID: scenarioID, Category: model.CategorySalesTeam, YearIndex: 1, FTE: decimal.NewFromFloat(3)},
		{ScenarioID: scenarioID, Category: model.CategorySalesTeam, YearIndex: 2, FTE: decimal.NewFromFloat(4)},
	}
	require.NoError(t, svc.UpdateHeadcounts(ctx, tenantID, scenarioID, hcs))

	rows, err := svc.ListHeadcounts(ctx, tenantID, scenarioID)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}

func TestStaff_Headcounts_ScenarioIsolation(t *testing.T) {
	svc, _ := newTestStaffService()
	tenantID := uuid.New()
	s1, s2 := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateHeadcounts(ctx, tenantID, s1, []model.StaffHeadcount{
		{ScenarioID: s1, Category: model.CategorySalesTeam, YearIndex: 1, FTE: decimal.NewFromFloat(1)},
	})
	svc.UpdateHeadcounts(ctx, tenantID, s2, []model.StaffHeadcount{
		{ScenarioID: s2, Category: model.CategorySalesTeam, YearIndex: 1, FTE: decimal.NewFromFloat(5)},
		{ScenarioID: s2, Category: model.CategorySalesTeam, YearIndex: 2, FTE: decimal.NewFromFloat(6)},
	})

	r1, _ := svc.ListHeadcounts(ctx, tenantID, s1)
	r2, _ := svc.ListHeadcounts(ctx, tenantID, s2)
	assert.Len(t, r1, 1)
	assert.Len(t, r2, 2)
}

// ── Salaries ──────────────────────────────────────────────────────────────────

func TestStaff_ListSalaries_EmptyInitially(t *testing.T) {
	svc, _ := newTestStaffService()
	rows, err := svc.ListSalaries(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestStaff_UpdateSalaries_StampsIDs(t *testing.T) {
	svc, repo := newTestStaffService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	sals := []model.StaffSalary{
		{ScenarioID: scenarioID, Category: model.CategorySalesTeam, YearIndex: 1, MonthlyGrossSalary: decimal.NewFromInt(4000)},
	}
	require.NoError(t, svc.UpdateSalaries(ctx, tenantID, scenarioID, sals))

	stored, _ := repo.ListSalariesByScenario(tenantID, scenarioID)
	require.Len(t, stored, 1)
	assert.NotEqual(t, uuid.Nil, stored[0].ID)
	assert.Equal(t, tenantID, stored[0].TenantID)
}

func TestStaff_UpdateSalaries_OverwritesPrevious(t *testing.T) {
	svc, _ := newTestStaffService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateSalaries(ctx, tenantID, scenarioID, []model.StaffSalary{
		{ScenarioID: scenarioID, Category: model.CategorySalesTeam, YearIndex: 1, MonthlyGrossSalary: decimal.NewFromInt(3000)},
		{ScenarioID: scenarioID, Category: model.CategorySalesTeam, YearIndex: 2, MonthlyGrossSalary: decimal.NewFromInt(3200)},
	})
	svc.UpdateSalaries(ctx, tenantID, scenarioID, []model.StaffSalary{
		{ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 1, MonthlyGrossSalary: decimal.NewFromInt(5000)},
	})

	rows, _ := svc.ListSalaries(ctx, tenantID, scenarioID)
	assert.Len(t, rows, 1, "second upsert should replace first")
}

// ── Incentives ────────────────────────────────────────────────────────────────

func TestStaff_ListIncentives_EmptyInitially(t *testing.T) {
	svc, _ := newTestStaffService()
	rows, err := svc.ListIncentives(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestStaff_UpdateIncentives_StampsIDs(t *testing.T) {
	svc, repo := newTestStaffService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	incs := []model.StaffIncentive{
		{ScenarioID: scenarioID, YearIndex: 1, IncentivePct: decimal.NewFromFloat(0.10)},
	}
	require.NoError(t, svc.UpdateIncentives(ctx, tenantID, scenarioID, incs))

	stored, _ := repo.ListIncentivesByScenario(tenantID, scenarioID)
	require.Len(t, stored, 1)
	assert.NotEqual(t, uuid.Nil, stored[0].ID)
	assert.Equal(t, tenantID, stored[0].TenantID)
}

func TestStaff_ListIncentives_ReturnsAfterUpdate(t *testing.T) {
	svc, _ := newTestStaffService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateIncentives(ctx, tenantID, scenarioID, []model.StaffIncentive{
		{ScenarioID: scenarioID, YearIndex: 1, IncentivePct: decimal.NewFromFloat(0.05)},
		{ScenarioID: scenarioID, YearIndex: 2, IncentivePct: decimal.NewFromFloat(0.08)},
		{ScenarioID: scenarioID, YearIndex: 3, IncentivePct: decimal.NewFromFloat(0.10)},
	})

	rows, err := svc.ListIncentives(ctx, tenantID, scenarioID)
	require.NoError(t, err)
	assert.Len(t, rows, 3)
}
