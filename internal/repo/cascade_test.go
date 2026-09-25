//go:build integration

package repo_test

// Verifies migration 000016: deleting a scenario or a plan removes every
// dependent row through ON DELETE CASCADE, instead of orphaning it as the
// parent-only deletes did before (audit §3.1).

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"ascenda/internal/model"
	"ascenda/internal/repo"
)

// cascadeFixture creates one plan with one scenario and a row in every
// scenario-scoped table family (settings, products, staff, capex, opex,
// snapshots, BEP chain, cap-table chain) plus plan-level members.
type cascadeFixture struct {
	tenantID uuid.UUID
	planID   uuid.UUID
	scenID   uuid.UUID
}

func newCascadeFixture(t *testing.T, db *gorm.DB) cascadeFixture {
	t.Helper()
	tenant := makeTenant(t, db)
	owner := makeOwner(t, db, tenant.ID)
	plan := makePlan(t, db, tenant.ID, owner.ID)
	return populatePlan(t, db, tenant.ID, owner.ID, plan.ID)
}

// populatePlan gives plan one scenario with a row in every dependent table
// family and one plan member, and returns the fixture describing it.
func populatePlan(t *testing.T, db *gorm.DB, tenantID, ownerID, planID uuid.UUID) cascadeFixture {
	t.Helper()
	scen := makeScenario(t, db, tenantID, planID)
	f := cascadeFixture{tenantID: tenantID, planID: planID, scenID: scen.ID}
	owner := &model.User{ID: ownerID}

	scoped := func() model.TenantScoped { return model.TenantScoped{ID: uuid.New(), TenantID: tenantID} }
	mustCreate := func(v interface{}) {
		t.Helper()
		require.NoError(t, db.Create(v).Error, "%T", v)
	}

	// Plan level
	mustCreate(&model.PlanMember{ID: uuid.New(), TenantID: tenantID, PlanID: planID, UserID: uuid.New(), Role: "editor", GrantedBy: owner.ID})

	// Settings / entries
	mustCreate(&model.PlanConfig{TenantScoped: scoped(), ScenarioID: scen.ID, ForecastStart: time.Now()})
	mustCreate(&model.StaffHeadcount{TenantScoped: scoped(), ScenarioID: scen.ID, Category: model.StaffCategory("rnd_engineers"), YearIndex: 1})
	mustCreate(&model.CapexEntry{TenantScoped: scoped(), ScenarioID: scen.ID, Category: model.AssetCategory("buildings"), YearIndex: 1, DepreciationYears: 5})
	mustCreate(&model.OpexManualEntry{TenantScoped: scoped(), ScenarioID: scen.ID, LineID: model.OpexLineID("other_expenses"), YearIndex: 1})
	mustCreate(&model.PlanSnapshot{TenantScoped: scoped(), ScenarioID: scen.ID, Version: 1, CreatedBy: owner.ID, Data: json.RawMessage(`{}`)})

	// Products (+ child through the pre-existing product FK from 000005)
	product := &model.Product{TenantScoped: scoped(), ScenarioID: scen.ID, Name: "Widget"}
	mustCreate(product)
	mustCreate(&model.ProductAssumption{TenantScoped: scoped(), ProductID: product.ID, YearIndex: 1})

	// BEP chain: snapshot → fixed cost line + optimisation plan → saving
	bep := &model.BEPSnapshot{TenantScoped: scoped(), ScenarioID: scen.ID, Label: "BEP"}
	mustCreate(bep)
	line := &model.FixedCostLine{TenantScoped: scoped(), SnapshotID: bep.ID, Category: model.FixedCostCategory("rent"), Label: "Office"}
	mustCreate(line)
	opt := &model.OptimisationPlan{TenantScoped: scoped(), SnapshotID: bep.ID, Name: "Plan A"}
	mustCreate(opt)
	mustCreate(&model.FixedCostSaving{TenantScoped: scoped(), PlanID: opt.ID, FixedCostLineID: line.ID})

	// Cap-table chain: round + shareholder → option plan → grant
	round := &model.CapTableRound{TenantScoped: scoped(), ScenarioID: scen.ID, PhaseNumber: 1, Label: "Seed", EventType: model.RoundEventType("capital_increase"), ShareClassType: model.ShareClassType("ordinary")}
	mustCreate(round)
	sh := &model.CapTableShareholder{TenantScoped: scoped(), ScenarioID: scen.ID, Name: "Founder", Type: model.ShareholderType("founder"), ClassType: model.ShareClassType("ordinary")}
	mustCreate(sh)
	roundID := round.ID
	sop := &model.StockOptionPlan{TenantScoped: scoped(), ScenarioID: scen.ID, PlanLabel: "BSPCE", Instrument: model.StockOptionInstrument("bspce"), ExercisePrice: decimal.NewFromInt(1), RoundID: &roundID}
	mustCreate(sop)
	mustCreate(&model.OptionGrant{TenantScoped: scoped(), PlanID: sop.ID, ShareholderID: sh.ID, RoundID: round.ID})

	return f
}

// TestCascade_PurgeDemoPlansRemovesDemoHierarchyOnly checks PurgeDemoPlans
// through the same cascade: the demo plan and every row under it are gone,
// while a real plan of the same tenant keeps all of its data.
func TestCascade_PurgeDemoPlansRemovesDemoHierarchyOnly(t *testing.T) {
	db := testDB(t)
	tenant := makeTenant(t, db)
	owner := makeOwner(t, db, tenant.ID)
	demo := populatePlan(t, db, tenant.ID, owner.ID, makeDemoPlan(t, db, tenant.ID, owner.ID).ID)
	real := populatePlan(t, db, tenant.ID, owner.ID, makePlan(t, db, tenant.ID, owner.ID).ID)

	for _, tbl := range dependentTables {
		require.Equal(t, int64(2), countRows(t, db, tbl.model, tenant.ID), "fixture should have two %s rows", tbl.name)
	}

	require.NoError(t, repo.NewPlanRepo(db).PurgeDemoPlans(tenant.ID))

	require.Equal(t, int64(1), countRows(t, db, &model.BusinessPlan{}, tenant.ID))
	require.Equal(t, int64(1), countRows(t, db, &model.Scenario{}, tenant.ID))
	require.Equal(t, int64(1), countRows(t, db, &model.PlanMember{}, tenant.ID))
	for _, tbl := range dependentTables {
		var n int64
		require.NoError(t, db.Model(tbl.model).Where("tenant_id = ?", tenant.ID).Count(&n).Error)
		require.Equal(t, int64(1), n, "%s must keep the real plan's row only", tbl.name)
	}
	var survivor model.Scenario
	require.NoError(t, db.Where("tenant_id = ?", tenant.ID).First(&survivor).Error)
	assert.Equal(t, real.scenID, survivor.ID, "the surviving scenario is the real plan's")
	_, err := repo.NewPlanRepo(db).GetByID(tenant.ID, demo.planID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// dependentTables lists every table the fixture populated below the scenario,
// paired with the model used to count its rows.
var dependentTables = []struct {
	name  string
	model interface{}
}{
	{"plan_configs", &model.PlanConfig{}},
	{"staff_headcounts", &model.StaffHeadcount{}},
	{"capex_entries", &model.CapexEntry{}},
	{"opex_manual_entries", &model.OpexManualEntry{}},
	{"plan_snapshots", &model.PlanSnapshot{}},
	{"products", &model.Product{}},
	{"product_assumptions", &model.ProductAssumption{}},
	{"bep_snapshots", &model.BEPSnapshot{}},
	{"fixed_cost_lines", &model.FixedCostLine{}},
	{"bep_optimisation_plans", &model.OptimisationPlan{}},
	{"bep_fixed_cost_savings", &model.FixedCostSaving{}},
	{"cap_table_rounds", &model.CapTableRound{}},
	{"cap_table_shareholders", &model.CapTableShareholder{}},
	{"stock_option_plans", &model.StockOptionPlan{}},
	{"option_grants", &model.OptionGrant{}},
}

func countRows(t *testing.T, db *gorm.DB, m interface{}, tenantID uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(m).Where("tenant_id = ?", tenantID).Count(&n).Error)
	return n
}

func TestCascade_DeletingScenarioRemovesAllDependentRows(t *testing.T) {
	db := testDB(t)
	f := newCascadeFixture(t, db)

	for _, tbl := range dependentTables {
		require.Equal(t, int64(1), countRows(t, db, tbl.model, f.tenantID), "fixture should have one %s row", tbl.name)
	}

	require.NoError(t, repo.NewScenarioRepo(db).Delete(f.tenantID, f.scenID))

	for _, tbl := range dependentTables {
		require.Equal(t, int64(0), countRows(t, db, tbl.model, f.tenantID), "%s must be emptied by the scenario cascade", tbl.name)
	}
	// The plan and its membership survive a scenario delete.
	require.Equal(t, int64(1), countRows(t, db, &model.BusinessPlan{}, f.tenantID))
	require.Equal(t, int64(1), countRows(t, db, &model.PlanMember{}, f.tenantID))
}

func TestCascade_DeletingPlanRemovesScenariosMembersAndData(t *testing.T) {
	db := testDB(t)
	f := newCascadeFixture(t, db)

	require.NoError(t, repo.NewPlanRepo(db).Delete(f.tenantID, f.planID))

	require.Equal(t, int64(0), countRows(t, db, &model.BusinessPlan{}, f.tenantID))
	require.Equal(t, int64(0), countRows(t, db, &model.Scenario{}, f.tenantID))
	require.Equal(t, int64(0), countRows(t, db, &model.PlanMember{}, f.tenantID))
	for _, tbl := range dependentTables {
		require.Equal(t, int64(0), countRows(t, db, tbl.model, f.tenantID), "%s must be emptied by the plan cascade", tbl.name)
	}
}

func TestCascade_DeletingRoundNullsOptionPlanReference(t *testing.T) {
	db := testDB(t)
	f := newCascadeFixture(t, db)

	var round model.CapTableRound
	require.NoError(t, db.Where("tenant_id = ?", f.tenantID).First(&round).Error)
	require.NoError(t, db.Delete(&round).Error)

	// Grants tied to the round are gone; the option plan survives with the
	// reference cleared (ON DELETE SET NULL).
	require.Equal(t, int64(0), countRows(t, db, &model.OptionGrant{}, f.tenantID))
	var sop model.StockOptionPlan
	require.NoError(t, db.Where("tenant_id = ?", f.tenantID).First(&sop).Error)
	require.Nil(t, sop.RoundID)
}
