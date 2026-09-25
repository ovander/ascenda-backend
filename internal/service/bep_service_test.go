package service

import (
	"context"
	"errors"
	"testing"

	"ascenda/internal/event"
	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestBEPService() (*BEPService, *MockBEPRepo) {
	repo := NewMockBEPRepo()
	logger := logrus.NewEntry(logrus.New())
	emitter := event.NewEmitter(logger)
	return NewBEPService(repo, nil, emitter, logger), repo
}

// helpers

func newValidSnapshot(tenantID, scenarioID uuid.UUID, label string) *model.BEPSnapshot {
	snap := &model.BEPSnapshot{
		ScenarioID:            scenarioID,
		Label:                 label,
		FixedCostsTotal:       decimal.NewFromInt(100000),
		ContributionMarginPct: decimal.NewFromFloat(40),
		Source:                model.BEPSourceManual,
	}
	snap.TenantID = tenantID
	snap.ID = uuid.New()
	return snap
}

// ─── Snapshot CRUD ────────────────────────────────────────────────────────────

func TestBEP_ListSnapshots_EmptyInitially(t *testing.T) {
	svc, _ := newTestBEPService()
	rows, err := svc.ListSnapshots(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestBEP_CreateSnapshot_PersistsAndAutoSensConfigs(t *testing.T) {
	svc, repo := newTestBEPService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "Budget 2025")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	// Snapshot should be stored
	snapshots, err := svc.ListSnapshots(ctx, tenantID, scenarioID)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	assert.Equal(t, "Budget 2025", snapshots[0].Label)

	// Three default sensitivity configs should have been created
	cfgs, _ := repo.ListSensitivityConfigs(tenantID, snap.ID)
	assert.Len(t, cfgs, 3, "CreateSnapshot should auto-provision 3 default sensitivity configs")
}

func TestBEP_CreateSnapshot_RejectsEmptyLabel(t *testing.T) {
	svc, _ := newTestBEPService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "")
	err := svc.CreateSnapshot(ctx, snap)
	require.Error(t, err, "snapshot with empty label must be rejected")
}

func TestBEP_CreateSnapshot_RejectsNegativeFixedCosts(t *testing.T) {
	svc, _ := newTestBEPService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "Invalid")
	snap.FixedCostsTotal = decimal.NewFromInt(-1)
	err := svc.CreateSnapshot(ctx, snap)
	require.Error(t, err, "negative fixed costs must be rejected")
}

func TestBEP_CreateSnapshot_RejectsInvalidMargin(t *testing.T) {
	svc, _ := newTestBEPService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "Bad margin")
	snap.ContributionMarginPct = decimal.NewFromInt(150) // > 100 is invalid
	err := svc.CreateSnapshot(ctx, snap)
	require.Error(t, err, "margin > 100% must be rejected")
}

func TestBEP_GetSnapshot_NotFound(t *testing.T) {
	svc, _ := newTestBEPService()
	_, err := svc.GetSnapshot(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
}

func TestBEP_UpdateSnapshot_PersistsChange(t *testing.T) {
	svc, _ := newTestBEPService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "Original")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	snap.Label = "Updated"
	require.NoError(t, svc.UpdateSnapshot(ctx, snap))

	got, err := svc.GetSnapshot(ctx, tenantID, snap.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", got.Label)
}

func TestBEP_DeleteSnapshot_RemovesRecord(t *testing.T) {
	svc, _ := newTestBEPService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "ToDelete")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	require.NoError(t, svc.DeleteSnapshot(ctx, tenantID, scenarioID, snap.ID))

	snapshots, _ := svc.ListSnapshots(ctx, tenantID, scenarioID)
	assert.Empty(t, snapshots)
}

func TestBEP_DeleteSnapshot_NotFound(t *testing.T) {
	svc, _ := newTestBEPService()
	err := svc.DeleteSnapshot(context.Background(), uuid.New(), uuid.New(), uuid.New())
	require.Error(t, err)
}

func TestBEP_ScenarioIsolation_Snapshots(t *testing.T) {
	svc, _ := newTestBEPService()
	tenantID := uuid.New()
	s1, s2 := uuid.New(), uuid.New()
	ctx := context.Background()

	require.NoError(t, svc.CreateSnapshot(ctx, newValidSnapshot(tenantID, s1, "S1-snap")))
	require.NoError(t, svc.CreateSnapshot(ctx, newValidSnapshot(tenantID, s2, "S2-A")))
	require.NoError(t, svc.CreateSnapshot(ctx, newValidSnapshot(tenantID, s2, "S2-B")))

	r1, _ := svc.ListSnapshots(ctx, tenantID, s1)
	r2, _ := svc.ListSnapshots(ctx, tenantID, s2)
	assert.Len(t, r1, 1)
	assert.Len(t, r2, 2)
}

// ─── Fixed cost lines ─────────────────────────────────────────────────────────

func TestBEP_FixedCostLines_EmptyInitially(t *testing.T) {
	svc, _ := newTestBEPService()
	rows, err := svc.ListFixedCostLines(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestBEP_UpsertFixedCostLines_PersistsLines(t *testing.T) {
	svc, _ := newTestBEPService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "Lines test")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	lines := []model.FixedCostLine{
		{SnapshotID: snap.ID, Category: model.FixedCostRent, Label: "Office Rent", AmountAnnual: decimal.NewFromInt(24000)},
		{SnapshotID: snap.ID, Category: model.FixedCostPayroll, Label: "Staff Cost", AmountAnnual: decimal.NewFromInt(60000)},
	}
	for i := range lines {
		lines[i].TenantID = tenantID
		lines[i].ID = uuid.New()
	}

	require.NoError(t, svc.UpsertFixedCostLines(ctx, tenantID, snap.ID, lines))

	stored, err := svc.ListFixedCostLines(ctx, tenantID, snap.ID)
	require.NoError(t, err)
	assert.Len(t, stored, 2)
}

func TestBEP_UpsertFixedCostLines_RejectsNegativeAmount(t *testing.T) {
	svc, _ := newTestBEPService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "Neg test")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	lines := []model.FixedCostLine{
		{SnapshotID: snap.ID, Category: model.FixedCostOther, Label: "Bad", AmountAnnual: decimal.NewFromInt(-100)},
	}
	lines[0].TenantID = tenantID
	lines[0].ID = uuid.New()

	err := svc.UpsertFixedCostLines(ctx, tenantID, snap.ID, lines)
	require.Error(t, err, "negative fixed cost amount must be rejected")
}

// ─── Variable cost lines ──────────────────────────────────────────────────────

func TestBEP_VariableCostLines_EmptyInitially(t *testing.T) {
	svc, _ := newTestBEPService()
	rows, err := svc.ListVariableCostLines(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestBEP_UpsertVariableCostLines_PersistsLines(t *testing.T) {
	svc, _ := newTestBEPService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "Var lines")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	lines := []model.VariableCostLine{
		{SnapshotID: snap.ID, Category: model.VarCostMaterials, Label: "Raw Materials", AmountPerUnit: decimal.NewFromFloat(5)},
	}
	lines[0].TenantID = tenantID
	lines[0].ID = uuid.New()

	require.NoError(t, svc.UpsertVariableCostLines(ctx, tenantID, snap.ID, lines))

	stored, err := svc.ListVariableCostLines(ctx, tenantID, snap.ID)
	require.NoError(t, err)
	assert.Len(t, stored, 1)
}

func TestBEP_UpsertVariableCostLines_RejectsNegativeAmount(t *testing.T) {
	svc, _ := newTestBEPService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "Neg var")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	lines := []model.VariableCostLine{
		{SnapshotID: snap.ID, Category: model.VarCostLogistics, Label: "Delivery", AmountPerUnit: decimal.NewFromFloat(-1)},
	}
	lines[0].TenantID = tenantID
	lines[0].ID = uuid.New()

	err := svc.UpsertVariableCostLines(ctx, tenantID, snap.ID, lines)
	require.Error(t, err, "negative variable cost must be rejected")
}

func TestBEP_UpsertVariableCostLines_RejectsAmountExceedingAvgOrderValue(t *testing.T) {
	svc, _ := newTestBEPService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	avgOrder := decimal.NewFromFloat(10)
	snap := newValidSnapshot(tenantID, scenarioID, "AOV constraint")
	snap.AvgOrderValue = &avgOrder
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	// Variable cost per unit >= avg order value violates constraint
	lines := []model.VariableCostLine{
		{SnapshotID: snap.ID, Category: model.VarCostMaterials, Label: "Material", AmountPerUnit: decimal.NewFromFloat(15)},
	}
	lines[0].TenantID = tenantID
	lines[0].ID = uuid.New()

	err := svc.UpsertVariableCostLines(ctx, tenantID, snap.ID, lines)
	require.Error(t, err, "variable cost >= avg order value must be rejected")
}

// ─── Sensitivity configs ──────────────────────────────────────────────────────

func TestBEP_UpsertSensitivityConfig_PersistsConfig(t *testing.T) {
	svc, _ := newTestBEPService()
	tenantID := uuid.New()
	snapID := uuid.New()
	ctx := context.Background()

	cfg := &model.SensitivityConfig{
		SnapshotID:   snapID,
		AnalysisType: model.SensRevenue,
		StepSizePct:  decimal.NewFromInt(5),
		RangePct:     decimal.NewFromInt(30),
	}
	cfg.TenantID = tenantID
	cfg.ID = uuid.New()

	require.NoError(t, svc.UpsertSensitivityConfig(ctx, cfg))

	cfgs, err := svc.ListSensitivityConfigs(ctx, tenantID, snapID)
	require.NoError(t, err)
	require.Len(t, cfgs, 1)
	assert.False(t, cfgs[0].IsDefault, "user-upserted configs should not be marked as default")
}

func TestBEP_UpsertSensitivityConfig_RejectsZeroStepSize(t *testing.T) {
	svc, _ := newTestBEPService()
	ctx := context.Background()

	cfg := &model.SensitivityConfig{
		SnapshotID:   uuid.New(),
		AnalysisType: model.SensMargin,
		StepSizePct:  decimal.Zero,
		RangePct:     decimal.NewFromInt(25),
	}
	cfg.TenantID = uuid.New()

	err := svc.UpsertSensitivityConfig(ctx, cfg)
	require.Error(t, err, "zero step size must be rejected")
}

// ─── Optimisation plans ───────────────────────────────────────────────────────

func TestBEP_OptimisationPlans_EmptyInitially(t *testing.T) {
	svc, _ := newTestBEPService()
	rows, err := svc.ListOptimisationPlans(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestBEP_CreateOptimisationPlan_PersistsPlan(t *testing.T) {
	svc, _ := newTestBEPService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "Base")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	plan := &model.OptimisationPlan{
		SnapshotID: snap.ID,
		Name:       "Headcount Reduction Q3",
		Status:     model.BEPPlanDraft,
	}
	plan.TenantID = tenantID
	plan.ID = uuid.New()

	require.NoError(t, svc.CreateOptimisationPlan(ctx, plan))

	plans, err := svc.ListOptimisationPlans(ctx, tenantID, snap.ID)
	require.NoError(t, err)
	require.Len(t, plans, 1)
	assert.Equal(t, "Headcount Reduction Q3", plans[0].Name)
}

func TestBEP_CreateOptimisationPlan_RejectsEmptyName(t *testing.T) {
	svc, _ := newTestBEPService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "Base")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	plan := &model.OptimisationPlan{SnapshotID: snap.ID, Name: ""}
	plan.TenantID = tenantID
	plan.ID = uuid.New()

	err := svc.CreateOptimisationPlan(ctx, plan)
	require.Error(t, err, "plan with empty name must be rejected")
}

func TestBEP_CreateOptimisationPlan_RejectsUnknownSnapshot(t *testing.T) {
	svc, _ := newTestBEPService()
	ctx := context.Background()

	plan := &model.OptimisationPlan{SnapshotID: uuid.New(), Name: "Orphan"}
	plan.TenantID = uuid.New()
	plan.ID = uuid.New()

	err := svc.CreateOptimisationPlan(ctx, plan)
	require.Error(t, err, "plan linked to unknown snapshot must be rejected")
}

func TestBEP_DeleteOptimisationPlan_RemovesRecord(t *testing.T) {
	svc, _ := newTestBEPService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "Base")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	plan := &model.OptimisationPlan{SnapshotID: snap.ID, Name: "Plan to delete"}
	plan.TenantID = tenantID
	plan.ID = uuid.New()
	require.NoError(t, svc.CreateOptimisationPlan(ctx, plan))

	require.NoError(t, svc.DeleteOptimisationPlan(ctx, tenantID, plan.ID))

	plans, _ := svc.ListOptimisationPlans(ctx, tenantID, snap.ID)
	assert.Empty(t, plans)
}

// ─── Fixed cost savings ───────────────────────────────────────────────────────

func TestBEP_UpsertFixedCostSavings_PersistsSavings(t *testing.T) {
	svc, _ := newTestBEPService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "Base")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	plan := &model.OptimisationPlan{SnapshotID: snap.ID, Name: "Optimise"}
	plan.TenantID = tenantID
	plan.ID = uuid.New()
	require.NoError(t, svc.CreateOptimisationPlan(ctx, plan))

	savings := []model.FixedCostSaving{
		{PlanID: plan.ID, FixedCostLineID: uuid.New(), SavingAmount: decimal.NewFromInt(5000), NewAmount: decimal.NewFromInt(19000)},
	}
	savings[0].TenantID = tenantID
	savings[0].ID = uuid.New()

	require.NoError(t, svc.UpsertFixedCostSavings(ctx, tenantID, plan.ID, savings))

	stored, err := svc.ListFixedCostSavings(ctx, tenantID, plan.ID)
	require.NoError(t, err)
	assert.Len(t, stored, 1)
}

func TestBEP_UpsertFixedCostSavings_RejectsNegativeSaving(t *testing.T) {
	svc, _ := newTestBEPService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "Base")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	plan := &model.OptimisationPlan{SnapshotID: snap.ID, Name: "Plan"}
	plan.TenantID = tenantID
	plan.ID = uuid.New()
	require.NoError(t, svc.CreateOptimisationPlan(ctx, plan))

	savings := []model.FixedCostSaving{
		{PlanID: plan.ID, FixedCostLineID: uuid.New(), SavingAmount: decimal.NewFromInt(-100)},
	}
	savings[0].TenantID = tenantID

	err := svc.UpsertFixedCostSavings(ctx, tenantID, plan.ID, savings)
	require.Error(t, err, "negative saving amount must be rejected")
}

// ─── ImportFromPlan ───────────────────────────────────────────────────────────

// mockPlanReporter is a controllable stub of planReporter used in unit tests.
type mockPlanReporter struct {
	report *model.FullPlanOutput
	err    error
}

func (m *mockPlanReporter) GetFullReport(_ context.Context, _, _ uuid.UUID) (*model.FullPlanOutput, error) {
	return m.report, m.err
}

// newTestBEPServiceWithReporter creates a BEPService wired to a stub reporter.
func newTestBEPServiceWithReporter(reporter planReporter) (*BEPService, *MockBEPRepo) {
	repo := NewMockBEPRepo()
	logger := logrus.NewEntry(logrus.New())
	emitter := event.NewEmitter(logger)
	return NewBEPService(repo, reporter, emitter, logger), repo
}

// buildFullPlanOutput populates only the fields ImportFromPlan reads, at the
// requested year index (0-based).
// marginRatio must be a ratio in [0, 1] — e.g. 0.425 for 42.5% — matching the
// way GrossMarginPct is stored in the real plan report.
// ImportFromPlan/PreviewFromPlan multiply it by 100 before storing in the BEP snapshot.
func buildFullPlanOutput(yearIdx int, payroll, opex, marginRatio, turnover decimal.Decimal, units int64) *model.FullPlanOutput {
	out := &model.FullPlanOutput{}
	out.Payroll.Payroll[yearIdx].TotalPayroll = payroll
	out.Opex.GrandTotal[yearIdx] = opex
	out.Revenue.Totals[yearIdx].GrossMarginPct = marginRatio
	out.Revenue.Totals[yearIdx].TotalTurnover = turnover
	out.Revenue.Totals[yearIdx].TotalUnitSales = units
	return out
}

// TestBEP_ImportFromPlan_HappyPath_Year1 verifies the common case where
// Year 1 plan data fully populates all three BEP inputs.
func TestBEP_ImportFromPlan_HappyPath_Year1(t *testing.T) {
	reporter := &mockPlanReporter{
		report: buildFullPlanOutput(
			0,
			decimal.NewFromInt(80000),   // payroll
			decimal.NewFromInt(20000),   // opex
			decimal.NewFromFloat(0.425), // gross margin ratio (42.5%)
			decimal.NewFromInt(500000),  // turnover
			1000,                        // unit sales
		),
	}
	svc, repo := newTestBEPServiceWithReporter(reporter)
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "FY2025")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	got, err := svc.ImportFromPlan(ctx, tenantID, scenarioID, snap.ID, 0)
	require.NoError(t, err)

	assert.True(t, decimal.NewFromInt(100000).Equal(got.FixedCostsTotal),
		"fixed costs = payroll (80000) + opex (20000): got %s", got.FixedCostsTotal)
	assert.True(t, decimal.NewFromFloat(42.5).Equal(got.ContributionMarginPct),
		"margin: got %s", got.ContributionMarginPct)
	require.NotNil(t, got.AvgOrderValue)
	assert.True(t, decimal.NewFromInt(500).Equal(*got.AvgOrderValue),
		"avg order = 500000 / 1000 = 500: got %s", got.AvgOrderValue)
	assert.Equal(t, model.BEPSourceImported, got.Source)

	// Confirm the update was persisted in the repo
	stored, _ := repo.GetSnapshot(tenantID, snap.ID)
	assert.True(t, decimal.NewFromInt(100000).Equal(stored.FixedCostsTotal),
		"persisted fixed costs: got %s", stored.FixedCostsTotal)
}

// TestBEP_ImportFromPlan_HappyPath_Year5 verifies that the correct year index
// is used when the caller requests the 5th year (index 4).
func TestBEP_ImportFromPlan_HappyPath_Year5(t *testing.T) {
	out := &model.FullPlanOutput{}
	out.Payroll.Payroll[4].TotalPayroll = decimal.NewFromInt(120000)
	out.Opex.GrandTotal[4] = decimal.NewFromInt(30000)
	out.Revenue.Totals[4].GrossMarginPct = decimal.NewFromFloat(0.55) // ratio → 55%
	out.Revenue.Totals[4].TotalTurnover = decimal.NewFromInt(1000000)
	out.Revenue.Totals[4].TotalUnitSales = 2000

	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{report: out})
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "Y5")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	got, err := svc.ImportFromPlan(ctx, tenantID, scenarioID, snap.ID, 4)
	require.NoError(t, err)

	assert.True(t, decimal.NewFromInt(150000).Equal(got.FixedCostsTotal),
		"fixed costs = 120000 + 30000: got %s", got.FixedCostsTotal)
	assert.True(t, decimal.NewFromFloat(55).Equal(got.ContributionMarginPct),
		"margin: got %s", got.ContributionMarginPct)
	require.NotNil(t, got.AvgOrderValue)
	assert.True(t, decimal.NewFromInt(500).Equal(*got.AvgOrderValue),
		"1 000 000 / 2000 = 500: got %s", got.AvgOrderValue)
}

// TestBEP_ImportFromPlan_NoAvgOrderValue_WhenZeroUnits verifies that AvgOrderValue
// is NOT updated when TotalUnitSales is 0 (avoids division by zero).
func TestBEP_ImportFromPlan_NoAvgOrderValue_WhenZeroUnits(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{
		report: buildFullPlanOutput(0,
			decimal.NewFromInt(50000), decimal.NewFromInt(10000),
			decimal.NewFromFloat(0.30), decimal.NewFromInt(200000), 0),
	})
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "NoUnits")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	got, err := svc.ImportFromPlan(ctx, tenantID, scenarioID, snap.ID, 0)
	require.NoError(t, err)
	assert.Nil(t, got.AvgOrderValue, "AvgOrderValue must remain nil when unit sales = 0")
}

// TestBEP_ImportFromPlan_NoAvgOrderValue_WhenZeroTurnover verifies that
// AvgOrderValue is not set when turnover is zero, even if units > 0.
func TestBEP_ImportFromPlan_NoAvgOrderValue_WhenZeroTurnover(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{
		report: buildFullPlanOutput(0,
			decimal.NewFromInt(50000), decimal.NewFromInt(10000),
			decimal.NewFromFloat(0.30), decimal.Zero, 500),
	})
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "NoTurnover")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	got, err := svc.ImportFromPlan(ctx, tenantID, scenarioID, snap.ID, 0)
	require.NoError(t, err)
	assert.Nil(t, got.AvgOrderValue, "AvgOrderValue must remain nil when turnover = 0")
}

// TestBEP_ImportFromPlan_SetsSourceToImported confirms the source field is
// always overwritten to "imported" regardless of the previous value.
func TestBEP_ImportFromPlan_SetsSourceToImported(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{
		report: buildFullPlanOutput(0,
			decimal.NewFromInt(40000), decimal.NewFromInt(10000),
			decimal.NewFromFloat(0.35), decimal.Zero, 0),
	})
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "Manual")
	snap.Source = model.BEPSourceManual
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	got, err := svc.ImportFromPlan(ctx, tenantID, scenarioID, snap.ID, 0)
	require.NoError(t, err)
	assert.Equal(t, model.BEPSourceImported, got.Source,
		"source must be overwritten to 'imported'")
}

// TestBEP_ImportFromPlan_PreservesSnapshotMetadata ensures that label, ID,
// tenant, and scenario IDs are unchanged after import.
func TestBEP_ImportFromPlan_PreservesSnapshotMetadata(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{
		report: buildFullPlanOutput(2,
			decimal.NewFromInt(60000), decimal.NewFromInt(15000),
			decimal.NewFromFloat(0.40), decimal.NewFromInt(300000), 600),
	})
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "Metadata check")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))
	origID := snap.ID

	got, err := svc.ImportFromPlan(ctx, tenantID, scenarioID, snap.ID, 2)
	require.NoError(t, err)

	assert.Equal(t, origID, got.ID)
	assert.Equal(t, tenantID, got.TenantID)
	assert.Equal(t, scenarioID, got.ScenarioID)
	assert.Equal(t, "Metadata check", got.Label)
}

// TestBEP_ImportFromPlan_FixedCostArithmetic verifies that FixedCostsTotal is
// exactly Payroll + Opex with no rounding.
func TestBEP_ImportFromPlan_FixedCostArithmetic(t *testing.T) {
	cases := []struct {
		payroll  int64
		opex     int64
		expected int64
	}{
		{0, 0, 0},
		{100000, 0, 100000},
		{0, 50000, 50000},
		{123456, 78901, 202357},
	}
	for _, tc := range cases {
		svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{
			report: buildFullPlanOutput(0,
				decimal.NewFromInt(tc.payroll), decimal.NewFromInt(tc.opex),
				decimal.NewFromFloat(0.30), decimal.Zero, 0),
		})
		tenantID, scenarioID := uuid.New(), uuid.New()
		ctx := context.Background()
		snap := newValidSnapshot(tenantID, scenarioID, "Arithmetic")
		require.NoError(t, svc.CreateSnapshot(ctx, snap))

		got, err := svc.ImportFromPlan(ctx, tenantID, scenarioID, snap.ID, 0)
		require.NoError(t, err)
		assert.Equal(t, decimal.NewFromInt(tc.expected), got.FixedCostsTotal,
			"payroll=%d opex=%d", tc.payroll, tc.opex)
	}
}

// TestBEP_ImportFromPlan_AvgOrderValueDivision verifies the avg order value
// calculation: turnover / units.
func TestBEP_ImportFromPlan_AvgOrderValueDivision(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{
		report: buildFullPlanOutput(1,
			decimal.NewFromInt(50000), decimal.NewFromInt(10000),
			decimal.NewFromFloat(0.25), // ratio → 25%
			decimal.NewFromInt(750000), // turnover
			3000,                       // units → avg = 250
		),
	})
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "AvgCalc")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	got, err := svc.ImportFromPlan(ctx, tenantID, scenarioID, snap.ID, 1)
	require.NoError(t, err)
	require.NotNil(t, got.AvgOrderValue)
	expected := decimal.NewFromInt(750000).Div(decimal.NewFromInt(3000))
	assert.True(t, expected.Equal(*got.AvgOrderValue),
		"expected %s, got %s", expected, got.AvgOrderValue)
}

// TestBEP_ImportFromPlan_InvalidYearIndex_BelowZero ensures yearIndex=-1 is rejected.
func TestBEP_ImportFromPlan_InvalidYearIndex_BelowZero(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{report: &model.FullPlanOutput{}})
	_, err := svc.ImportFromPlan(context.Background(), uuid.New(), uuid.New(), uuid.New(), -1)
	require.Error(t, err, "yearIndex=-1 must be rejected")
}

// TestBEP_ImportFromPlan_InvalidYearIndex_EqualToMaxYears ensures yearIndex=5 is
// rejected (valid range is 0–4).
func TestBEP_ImportFromPlan_InvalidYearIndex_EqualToMaxYears(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{report: &model.FullPlanOutput{}})
	_, err := svc.ImportFromPlan(context.Background(), uuid.New(), uuid.New(), uuid.New(), 5)
	require.Error(t, err, "yearIndex=5 is out of range and must be rejected")
}

// TestBEP_ImportFromPlan_SnapshotNotFound returns a not-found error when the
// snapshot does not exist for the given tenant.
func TestBEP_ImportFromPlan_SnapshotNotFound(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{report: &model.FullPlanOutput{}})
	_, err := svc.ImportFromPlan(context.Background(), uuid.New(), uuid.New(), uuid.New(), 0)
	require.Error(t, err, "unknown snapshotID must return an error")
}

// TestBEP_ImportFromPlan_ReportServiceFailure propagates errors from GetFullReport.
func TestBEP_ImportFromPlan_ReportServiceFailure(t *testing.T) {
	reporter := &mockPlanReporter{err: errors.New("compute timeout")}
	svc, _ := newTestBEPServiceWithReporter(reporter)
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "ReportFail")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	_, err := svc.ImportFromPlan(ctx, tenantID, scenarioID, snap.ID, 0)
	require.Error(t, err, "GetFullReport failure must propagate as an error")
}

// TestBEP_ImportFromPlan_NilReportService returns an internal error when
// reportSvc was not wired (nil).
func TestBEP_ImportFromPlan_NilReportService(t *testing.T) {
	svc, _ := newTestBEPService() // uses nil reportSvc
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	snap := newValidSnapshot(tenantID, scenarioID, "NoSvc")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	_, err := svc.ImportFromPlan(ctx, tenantID, scenarioID, snap.ID, 0)
	require.Error(t, err, "nil reportSvc must return an error before touching the repo")
}

// ─── PreviewFromPlan ──────────────────────────────────────────────────────────

// TestBEP_PreviewFromPlan_ReturnsCorrectValues verifies the three derived scalars
// match the plan data for the requested year.
func TestBEP_PreviewFromPlan_ReturnsCorrectValues(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{
		report: buildFullPlanOutput(
			2,
			decimal.NewFromInt(90000),   // payroll year 3
			decimal.NewFromInt(25000),   // opex year 3
			decimal.NewFromFloat(0.385), // gross margin ratio (38.5%)
			decimal.NewFromInt(600000),  // turnover
			2000,                        // units → avg = 300
		),
	})
	ctx := context.Background()

	preview, err := svc.PreviewFromPlan(ctx, uuid.New(), uuid.New(), 2)
	require.NoError(t, err)

	assert.True(t, decimal.NewFromInt(115000).Equal(preview.FixedCostsTotal),
		"fixed = 90000 + 25000: got %s", preview.FixedCostsTotal)
	assert.True(t, decimal.NewFromFloat(38.5).Equal(preview.ContributionMarginPct),
		"margin: got %s", preview.ContributionMarginPct)
	require.NotNil(t, preview.AvgOrderValue)
	assert.True(t, decimal.NewFromInt(300).Equal(*preview.AvgOrderValue),
		"avg order = 600000 / 2000 = 300: got %s", preview.AvgOrderValue)
	assert.Equal(t, 2, preview.YearIndex)
}

// TestBEP_PreviewFromPlan_DoesNotWriteToRepo confirms that a preview call
// never touches the BEP repository.
func TestBEP_PreviewFromPlan_DoesNotWriteToRepo(t *testing.T) {
	reporter := &mockPlanReporter{
		report: buildFullPlanOutput(0,
			decimal.NewFromInt(50000), decimal.NewFromInt(10000),
			decimal.NewFromFloat(0.30), decimal.Zero, 0),
	}
	svc, repo := newTestBEPServiceWithReporter(reporter)
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	// Create one snapshot so we can verify it is unchanged after preview.
	snap := newValidSnapshot(tenantID, scenarioID, "Unchanged")
	require.NoError(t, svc.CreateSnapshot(ctx, snap))

	_, err := svc.PreviewFromPlan(ctx, tenantID, scenarioID, 0)
	require.NoError(t, err)

	// Snapshot must be unmodified.
	stored, _ := repo.GetSnapshot(tenantID, snap.ID)
	assert.True(t, decimal.NewFromInt(100000).Equal(stored.FixedCostsTotal),
		"preview must not overwrite the existing snapshot")
	assert.Equal(t, model.BEPSourceManual, stored.Source,
		"source must remain 'manual' after preview")
}

// TestBEP_PreviewFromPlan_OmitsAvgOrderValueWhenZeroUnits mirrors the
// ImportFromPlan behaviour for the no-units case.
func TestBEP_PreviewFromPlan_OmitsAvgOrderValueWhenZeroUnits(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{
		report: buildFullPlanOutput(0,
			decimal.NewFromInt(50000), decimal.NewFromInt(10000),
			decimal.NewFromFloat(0.30), decimal.NewFromInt(200000), 0),
	})
	preview, err := svc.PreviewFromPlan(context.Background(), uuid.New(), uuid.New(), 0)
	require.NoError(t, err)
	assert.Nil(t, preview.AvgOrderValue)
}

// TestBEP_PreviewFromPlan_InvalidYear rejects out-of-range year indices.
func TestBEP_PreviewFromPlan_InvalidYear(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{report: &model.FullPlanOutput{}})
	ctx := context.Background()

	_, err := svc.PreviewFromPlan(ctx, uuid.New(), uuid.New(), -1)
	require.Error(t, err, "yearIndex=-1 must be rejected")

	_, err = svc.PreviewFromPlan(ctx, uuid.New(), uuid.New(), 5)
	require.Error(t, err, "yearIndex=5 must be rejected")
}

// TestBEP_PreviewFromPlan_ReportServiceFailure propagates GetFullReport errors.
func TestBEP_PreviewFromPlan_ReportServiceFailure(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{err: errors.New("timeout")})
	_, err := svc.PreviewFromPlan(context.Background(), uuid.New(), uuid.New(), 0)
	require.Error(t, err)
}

// TestBEP_PreviewFromPlan_NilReportService returns an internal error when
// the service was not wired with a report provider.
func TestBEP_PreviewFromPlan_NilReportService(t *testing.T) {
	svc, _ := newTestBEPService() // nil reportSvc
	_, err := svc.PreviewFromPlan(context.Background(), uuid.New(), uuid.New(), 0)
	require.Error(t, err, "nil reportSvc must be caught")
}

// ─── GetMultiYearBEPReport ────────────────────────────────────────────────────

// buildMultiYearPlan creates a FullPlanOutput with distinct per-year values so we
// can assert which year's data ends up in which row.
//
// Plan layout (marginRatio is a ratio 0–1, costs/turnover in €):
//
//	Year  Payroll   Opex     Turnover   MarginRatio
//	  1   400 000  200 000  1 000 000   0.40
//	  2   480 000  220 000  2 000 000   0.42
//	  3   510 000  240 000  2 500 000   0.44
//	  4   540 000  260 000  3 000 000   0.46
//	  5   570 000  280 000  3 500 000   0.48
//
// Year 1 EBE = 1 000 000 × 0.40 − 600 000 = −200 000  (loss)
// Year 2 EBE = 2 000 000 × 0.42 − 700 000 = +140 000  (first profitable year)
// Year 3 EBE = 2 500 000 × 0.44 − 750 000 = +350 000
// Cumulative at end of year 2 = −60 000 → crossover falls in year 3
func buildMultiYearPlan() *model.FullPlanOutput {
	payrolls := [5]int64{400_000, 480_000, 510_000, 540_000, 570_000}
	opexes := [5]int64{200_000, 220_000, 240_000, 260_000, 280_000}
	turnovers := [5]int64{1_000_000, 2_000_000, 2_500_000, 3_000_000, 3_500_000}
	margins := [5]float64{0.40, 0.42, 0.44, 0.46, 0.48}

	out := &model.FullPlanOutput{}
	for i := 0; i < 5; i++ {
		out.Payroll.Payroll[i].TotalPayroll = decimal.NewFromInt(payrolls[i])
		out.Opex.GrandTotal[i] = decimal.NewFromInt(opexes[i])
		out.Revenue.Totals[i].TotalTurnover = decimal.NewFromInt(turnovers[i])
		out.Revenue.Totals[i].GrossMarginPct = decimal.NewFromFloat(margins[i])
	}
	return out
}

// TestBEP_GetMultiYearBEPReport_HappyPath verifies the end-to-end service call:
// plan is fetched, compute is applied, and the caller receives a valid report.
func TestBEP_GetMultiYearBEPReport_HappyPath(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{report: buildMultiYearPlan()})
	ctx := context.Background()

	report, err := svc.GetMultiYearBEPReport(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)
	require.NotNil(t, report)

	// Should always return exactly 5 rows
	assert.Len(t, report.Years, 5, "must return a row for each plan year")
}

// TestBEP_GetMultiYearBEPReport_YearLabels asserts each row carries the correct
// 1-based year number.
func TestBEP_GetMultiYearBEPReport_YearLabels(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{report: buildMultiYearPlan()})
	report, err := svc.GetMultiYearBEPReport(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)

	for i, row := range report.Years {
		assert.Equal(t, i+1, row.Year, "year %d row must have Year = %d", i+1, i+1)
	}
}

// TestBEP_GetMultiYearBEPReport_FixedCosts verifies payroll+opex aggregation.
func TestBEP_GetMultiYearBEPReport_FixedCosts(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{report: buildMultiYearPlan()})
	report, err := svc.GetMultiYearBEPReport(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)

	// Year 1: payroll 400k + opex 200k = 600k
	assert.Equal(t, "600000", report.Years[0].FixedCosts.StringFixed(0))
	// Year 5: payroll 570k + opex 280k = 850k
	assert.Equal(t, "850000", report.Years[4].FixedCosts.StringFixed(0))
}

// TestBEP_GetMultiYearBEPReport_MarginIsPercentage confirms the ratio→percent
// conversion (same rule as ImportFromPlan: GrossMarginPct × 100).
func TestBEP_GetMultiYearBEPReport_MarginIsPercentage(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{report: buildMultiYearPlan()})
	report, err := svc.GetMultiYearBEPReport(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)

	assert.Equal(t, "40.00", report.Years[0].ContributionMarginPct.StringFixed(2),
		"GrossMarginPct ratio 0.40 must become ContributionMarginPct 40.00%%")
	assert.Equal(t, "48.00", report.Years[4].ContributionMarginPct.StringFixed(2),
		"GrossMarginPct ratio 0.48 must become ContributionMarginPct 48.00%%")
}

// TestBEP_GetMultiYearBEPReport_FirstProfitableYear verifies the correct year
// is flagged (year 2 in the reference plan).
func TestBEP_GetMultiYearBEPReport_FirstProfitableYear(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{report: buildMultiYearPlan()})
	report, err := svc.GetMultiYearBEPReport(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)

	require.NotNil(t, report.FirstProfitableYear)
	assert.Equal(t, 2, *report.FirstProfitableYear,
		"year 2 is the first year with positive annual EBE")
}

// TestBEP_GetMultiYearBEPReport_CumulativeBEPYear verifies the payback year
// (year 3 in the reference plan: cumulative EBE turns positive).
func TestBEP_GetMultiYearBEPReport_CumulativeBEPYear(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{report: buildMultiYearPlan()})
	report, err := svc.GetMultiYearBEPReport(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)

	require.NotNil(t, report.CumulativeBEPYear)
	assert.Equal(t, 3, *report.CumulativeBEPYear,
		"cumulative EBE first turns non-negative in year 3")
	require.NotNil(t, report.CumulativeBEPMonth)
	assert.Equal(t, 3, *report.CumulativeBEPMonth,
		"crossover estimated at month 3 of year 3 (prevCum=-60k, yearEBE=350k → t≈0.17 → month=3)")
}

// TestBEP_GetMultiYearBEPReport_TotalCumulativeEBE confirms the running total
// matches the year-5 cumulative EBE.
func TestBEP_GetMultiYearBEPReport_TotalCumulativeEBE(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{report: buildMultiYearPlan()})
	report, err := svc.GetMultiYearBEPReport(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)

	lastRow := report.Years[4]
	assert.True(t, report.TotalCumulativeEBE.Equal(lastRow.CumulativeEBE),
		"TotalCumulativeEBE must equal year-5 cumulative EBE; got %s vs %s",
		report.TotalCumulativeEBE, lastRow.CumulativeEBE)
}

// TestBEP_GetMultiYearBEPReport_NilReportService returns an internal error
// when the service has no report provider wired.
func TestBEP_GetMultiYearBEPReport_NilReportService(t *testing.T) {
	svc, _ := newTestBEPService() // nil reportSvc
	_, err := svc.GetMultiYearBEPReport(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err, "nil reportSvc must be caught")
}

// TestBEP_GetMultiYearBEPReport_PropagatesReporterError verifies that a failure
// from GetFullReport is surfaced as a service error.
func TestBEP_GetMultiYearBEPReport_PropagatesReporterError(t *testing.T) {
	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{err: errors.New("db timeout")})
	_, err := svc.GetMultiYearBEPReport(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err, "reporter failure must be propagated")
}

// TestBEP_GetMultiYearBEPReport_NoProfitableYear_NilFields confirms that when
// the plan is never profitable the optional fields remain nil.
func TestBEP_GetMultiYearBEPReport_NoProfitableYear_NilFields(t *testing.T) {
	// All years lose money: margin 20%, turnover 300k, fixed costs 200k/yr
	lossPlan := &model.FullPlanOutput{}
	for i := 0; i < 5; i++ {
		lossPlan.Payroll.Payroll[i].TotalPayroll = decimal.NewFromInt(150_000)
		lossPlan.Opex.GrandTotal[i] = decimal.NewFromInt(50_000)
		lossPlan.Revenue.Totals[i].TotalTurnover = decimal.NewFromInt(300_000)
		lossPlan.Revenue.Totals[i].GrossMarginPct = decimal.NewFromFloat(0.20)
	}

	svc, _ := newTestBEPServiceWithReporter(&mockPlanReporter{report: lossPlan})
	report, err := svc.GetMultiYearBEPReport(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)

	assert.Nil(t, report.FirstProfitableYear,
		"FirstProfitableYear must be nil when never profitable")
	assert.Nil(t, report.CumulativeBEPYear,
		"CumulativeBEPYear must be nil when never reached")
	assert.True(t, report.TotalCumulativeEBE.LessThan(decimal.Zero),
		"TotalCumulativeEBE must be negative when always losing")
}
