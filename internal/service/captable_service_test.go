package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ascenda/internal/event"
	"ascenda/internal/model"
)

func newTestCapTableService() (*CapTableService, *MockCapTableRepo) {
	repo := NewMockCapTableRepo()
	settings := NewMockSettingsRepo()
	logger := logrus.NewEntry(logrus.New())
	emitter := event.NewEmitter(logger)
	return NewCapTableService(repo, settings, nil, emitter, logger), repo
}

// ─── Company ──────────────────────────────────────────────────────────────────

func TestCapTable_GetCompany_ReturnsDefaultWhenMissing(t *testing.T) {
	svc, _ := newTestCapTableService()
	tenantID, scenarioID := uuid.New(), uuid.New()

	company, err := svc.GetCompany(context.Background(), tenantID, scenarioID)
	require.NoError(t, err)
	// Returns a bare default with correct IDs even if no record stored
	assert.Equal(t, scenarioID, company.ScenarioID)
	assert.Equal(t, tenantID, company.TenantID)
}

func TestCapTable_UpsertCompany_PersistsAndReturns(t *testing.T) {
	svc, _ := newTestCapTableService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	company := &model.CapTableCompany{
		ScenarioID:  scenarioID,
		CompanyName: "Acme Corp",
		Currency:    "EUR",
	}
	company.TenantID = tenantID

	require.NoError(t, svc.UpsertCompany(ctx, company))

	got, err := svc.GetCompany(ctx, tenantID, scenarioID)
	require.NoError(t, err)
	assert.Equal(t, "Acme Corp", got.CompanyName)
}

// ─── ShareClasses ─────────────────────────────────────────────────────────────

func TestCapTable_ShareClasses_EmptyInitially(t *testing.T) {
	svc, _ := newTestCapTableService()
	rows, err := svc.ListShareClasses(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestCapTable_UpsertShareClass_AssignsIDAndPersists(t *testing.T) {
	svc, _ := newTestCapTableService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	sc := &model.CapTableShareClass{
		ScenarioID:  scenarioID,
		ClassType:   model.ShareClassCommon,
		VotingRights: true,
	}
	sc.TenantID = tenantID

	require.NoError(t, svc.UpsertShareClass(ctx, sc))

	rows, err := svc.ListShareClasses(ctx, tenantID, scenarioID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.NotEqual(t, uuid.Nil, rows[0].ID)
	assert.Equal(t, model.ShareClassCommon, rows[0].ClassType)
}

func TestCapTable_DeleteShareClass_RemovesRecord(t *testing.T) {
	svc, _ := newTestCapTableService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	sc := &model.CapTableShareClass{ScenarioID: scenarioID, ClassType: model.ShareClassPreferredA}
	sc.TenantID = tenantID
	require.NoError(t, svc.UpsertShareClass(ctx, sc))

	classes, _ := svc.ListShareClasses(ctx, tenantID, scenarioID)
	require.Len(t, classes, 1)
	classID := classes[0].ID

	require.NoError(t, svc.DeleteShareClass(ctx, tenantID, classID))

	classes, _ = svc.ListShareClasses(ctx, tenantID, scenarioID)
	assert.Empty(t, classes)
}

// ─── Shareholders ─────────────────────────────────────────────────────────────

func TestCapTable_Shareholders_EmptyInitially(t *testing.T) {
	svc, _ := newTestCapTableService()
	rows, err := svc.ListShareholders(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestCapTable_CreateShareholder_AssignsIDAndPersists(t *testing.T) {
	svc, _ := newTestCapTableService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	sh := &model.CapTableShareholder{
		ScenarioID: scenarioID,
		Name:       "Alice Founder",
		Type:       model.ShareholderFounder,
		ClassType:  model.ShareClassCommon,
	}
	sh.TenantID = tenantID

	require.NoError(t, svc.CreateShareholder(ctx, sh))

	rows, err := svc.ListShareholders(ctx, tenantID, scenarioID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.NotEqual(t, uuid.Nil, rows[0].ID)
	assert.Equal(t, "Alice Founder", rows[0].Name)
}

func TestCapTable_UpdateShareholder_PersistsChange(t *testing.T) {
	svc, _ := newTestCapTableService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	sh := &model.CapTableShareholder{
		ScenarioID: scenarioID,
		Name:       "Bob",
		Type:       model.ShareholderFounder,
		ClassType:  model.ShareClassCommon,
	}
	sh.TenantID = tenantID
	require.NoError(t, svc.CreateShareholder(ctx, sh))

	sh.Name = "Bob Updated"
	require.NoError(t, svc.UpdateShareholder(ctx, sh))

	rows, _ := svc.ListShareholders(ctx, tenantID, scenarioID)
	assert.Equal(t, "Bob Updated", rows[0].Name)
}

func TestCapTable_DeleteShareholder_RemovesRecord(t *testing.T) {
	svc, _ := newTestCapTableService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	sh := &model.CapTableShareholder{ScenarioID: scenarioID, Name: "Charlie", Type: model.ShareholderInvestor, ClassType: model.ShareClassPreferredA}
	sh.TenantID = tenantID
	require.NoError(t, svc.CreateShareholder(ctx, sh))

	require.NoError(t, svc.DeleteShareholder(ctx, tenantID, sh.ID))

	rows, _ := svc.ListShareholders(ctx, tenantID, scenarioID)
	assert.Empty(t, rows)
}

// ─── Rounds ───────────────────────────────────────────────────────────────────

func TestCapTable_Rounds_EmptyInitially(t *testing.T) {
	svc, _ := newTestCapTableService()
	rows, err := svc.ListRounds(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestCapTable_CreateRound_AssignsIDAndPersists(t *testing.T) {
	svc, _ := newTestCapTableService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	rnd := &model.CapTableRound{
		ScenarioID:     scenarioID,
		PhaseNumber:    1,
		Label:          "Seed Round",
		EventType:      model.RoundEventFunding,
		ShareClassType: model.ShareClassPreferredA,
	}
	rnd.TenantID = tenantID

	require.NoError(t, svc.CreateRound(ctx, rnd))

	rounds, err := svc.ListRounds(ctx, tenantID, scenarioID)
	require.NoError(t, err)
	require.Len(t, rounds, 1)
	assert.NotEqual(t, uuid.Nil, rounds[0].ID)
	assert.Equal(t, "Seed Round", rounds[0].Label)
}

func TestCapTable_DeleteRound_RemovesRecord(t *testing.T) {
	svc, _ := newTestCapTableService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	rnd := &model.CapTableRound{ScenarioID: scenarioID, PhaseNumber: 1, Label: "R1", EventType: model.RoundEventFunding, ShareClassType: model.ShareClassCommon}
	rnd.TenantID = tenantID
	require.NoError(t, svc.CreateRound(ctx, rnd))

	require.NoError(t, svc.DeleteRound(ctx, tenantID, rnd.ID))

	rounds, _ := svc.ListRounds(ctx, tenantID, scenarioID)
	assert.Empty(t, rounds)
}

// ─── Option Plans ─────────────────────────────────────────────────────────────

func TestCapTable_Plans_EmptyInitially(t *testing.T) {
	svc, _ := newTestCapTableService()
	rows, err := svc.ListPlans(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestCapTable_CreatePlan_AssignsIDAndPersists(t *testing.T) {
	svc, _ := newTestCapTableService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	plan := &model.StockOptionPlan{
		ScenarioID:    scenarioID,
		PlanLabel:     "Management Plan 2024",
		Instrument:    model.SOIBSPCE,
		ExercisePrice: decimal.NewFromFloat(0.01),
		OptionsVoted:  10000,
	}
	plan.TenantID = tenantID

	require.NoError(t, svc.CreatePlan(ctx, plan))

	plans, err := svc.ListPlans(ctx, tenantID, scenarioID)
	require.NoError(t, err)
	require.Len(t, plans, 1)
	assert.NotEqual(t, uuid.Nil, plans[0].ID)
	assert.Equal(t, "Management Plan 2024", plans[0].PlanLabel)
}

func TestCapTable_DeletePlan_RemovesRecord(t *testing.T) {
	svc, _ := newTestCapTableService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	plan := &model.StockOptionPlan{ScenarioID: scenarioID, PlanLabel: "P1", Instrument: model.SOIBCE, ExercisePrice: decimal.NewFromInt(1)}
	plan.TenantID = tenantID
	require.NoError(t, svc.CreatePlan(ctx, plan))

	require.NoError(t, svc.DeletePlan(ctx, tenantID, plan.ID))

	plans, _ := svc.ListPlans(ctx, tenantID, scenarioID)
	assert.Empty(t, plans)
}

// ─── Grants ───────────────────────────────────────────────────────────────────

func TestCapTable_Grants_EmptyInitially(t *testing.T) {
	svc, _ := newTestCapTableService()
	rows, err := svc.ListGrants(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestCapTable_CreateGrant_AssignsIDAndPersists(t *testing.T) {
	svc, _ := newTestCapTableService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	// Create a plan first so that ListGrantsByScenario can find grants via plan lookup.
	plan := &model.StockOptionPlan{
		ScenarioID:    scenarioID,
		PlanLabel:     "Grant Plan",
		Instrument:    model.SOIBSPCE,
		ExercisePrice: decimal.NewFromFloat(0.01),
	}
	plan.TenantID = tenantID
	require.NoError(t, svc.CreatePlan(ctx, plan))

	grant := &model.OptionGrant{
		PlanID:        plan.ID,
		ShareholderID: uuid.New(),
		RoundID:       uuid.New(),
	}
	grant.TenantID = tenantID

	require.NoError(t, svc.CreateGrant(ctx, grant))

	grants, err := svc.ListGrants(ctx, tenantID, scenarioID)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.NotEqual(t, uuid.Nil, grants[0].ID)
}

func TestCapTable_DeleteGrant_RemovesRecord(t *testing.T) {
	svc, _ := newTestCapTableService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	plan := &model.StockOptionPlan{ScenarioID: scenarioID, PlanLabel: "P", Instrument: model.SOIBCE, ExercisePrice: decimal.NewFromFloat(0.01)}
	plan.TenantID = tenantID
	require.NoError(t, svc.CreatePlan(ctx, plan))

	grant := &model.OptionGrant{PlanID: plan.ID, ShareholderID: uuid.New(), RoundID: uuid.New()}
	grant.TenantID = tenantID
	require.NoError(t, svc.CreateGrant(ctx, grant))

	require.NoError(t, svc.DeleteGrant(ctx, tenantID, grant.ID))

	grants, _ := svc.ListGrants(ctx, tenantID, scenarioID)
	assert.Empty(t, grants)
}

// ─── Valuation Scenarios ──────────────────────────────────────────────────────

func TestCapTable_ValuationScenarios_EmptyInitially(t *testing.T) {
	svc, _ := newTestCapTableService()
	rows, err := svc.ListValuationScenarios(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestCapTable_CreateValuationScenario_AssignsIDAndPersists(t *testing.T) {
	svc, _ := newTestCapTableService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	newMoney := decimal.NewFromInt(1000)
	vs := &model.ValuationScenario{
		ScenarioID: scenarioID,
		Label:      "Series A — Base",
		CalcType:   model.ValScenNewMoneyToPremoney,
		NewMoneyK:  &newMoney,
	}
	vs.TenantID = tenantID
	vs.ID = uuid.New()

	require.NoError(t, svc.CreateValuationScenario(ctx, vs))

	vScens, err := svc.ListValuationScenarios(ctx, tenantID, scenarioID)
	require.NoError(t, err)
	require.Len(t, vScens, 1)
	assert.Equal(t, "Series A — Base", vScens[0].Label)
}

func TestCapTable_DeleteValuationScenario_RemovesRecord(t *testing.T) {
	svc, _ := newTestCapTableService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	vs := &model.ValuationScenario{ScenarioID: scenarioID, Label: "V1", CalcType: model.ValScenMultipleToIRR}
	vs.TenantID = tenantID
	vs.ID = uuid.New()
	require.NoError(t, svc.CreateValuationScenario(ctx, vs))

	require.NoError(t, svc.DeleteValuationScenario(ctx, tenantID, vs.ID))

	vScens, _ := svc.ListValuationScenarios(ctx, tenantID, scenarioID)
	assert.Empty(t, vScens)
}

// ─── ScenarioIsolation ────────────────────────────────────────────────────────

func TestCapTable_ScenarioIsolation_Shareholders(t *testing.T) {
	svc, _ := newTestCapTableService()
	tenantID := uuid.New()
	s1, s2 := uuid.New(), uuid.New()
	ctx := context.Background()

	sh1 := &model.CapTableShareholder{ScenarioID: s1, Name: "Alice", Type: model.ShareholderFounder, ClassType: model.ShareClassCommon}
	sh1.TenantID = tenantID
	sh2a := &model.CapTableShareholder{ScenarioID: s2, Name: "Bob", Type: model.ShareholderInvestor, ClassType: model.ShareClassPreferredA}
	sh2a.TenantID = tenantID
	sh2b := &model.CapTableShareholder{ScenarioID: s2, Name: "Carol", Type: model.ShareholderInvestor, ClassType: model.ShareClassPreferredA}
	sh2b.TenantID = tenantID

	require.NoError(t, svc.CreateShareholder(ctx, sh1))
	require.NoError(t, svc.CreateShareholder(ctx, sh2a))
	require.NoError(t, svc.CreateShareholder(ctx, sh2b))

	r1, _ := svc.ListShareholders(ctx, tenantID, s1)
	r2, _ := svc.ListShareholders(ctx, tenantID, s2)
	assert.Len(t, r1, 1)
	assert.Len(t, r2, 2)
}

// ─── FiPlan Sync ──────────────────────────────────────────────────────────────

// newTestCapTableServiceWithSyncer creates a CapTableService with a mock fiplanSyncer injected.
func newTestCapTableServiceWithSyncer() (*CapTableService, *MockCapTableRepo, *MockFiplanSyncer) {
	repo := NewMockCapTableRepo()
	settings := NewMockSettingsRepo()
	syncer := &MockFiplanSyncer{}
	logger := logrus.NewEntry(logrus.New())
	emitter := event.NewEmitter(logger)
	return NewCapTableService(repo, settings, syncer, emitter, logger), repo, syncer
}

// seedRound inserts a round into the mock repo and returns it.
func seedRound(t *testing.T, repo *MockCapTableRepo, tenantID, scenarioID uuid.UUID, amountRaisedK float64) *model.CapTableRound {
	t.Helper()
	rnd := &model.CapTableRound{
		ScenarioID:    scenarioID,
		Label:         "Series A",
		EventType:     model.RoundEventFunding,
		AmountRaisedK: decimal.NewFromFloat(amountRaisedK),
		PhaseNumber:   1,
	}
	rnd.ID = uuid.New()
	rnd.TenantID = tenantID
	repo.CreateRound(rnd)
	return rnd
}

func TestCapTable_SyncRoundToFiplan_CallsSyncer(t *testing.T) {
	svc, repo, syncer := newTestCapTableServiceWithSyncer()
	tenantID, scenarioID := uuid.New(), uuid.New()
	rnd := seedRound(t, repo, tenantID, scenarioID, 500) // 500 k€

	round, err := svc.SyncRoundToFiplan(context.Background(), tenantID, rnd.ID, 2)

	require.NoError(t, err)
	require.NotNil(t, round)
	require.Len(t, syncer.UpsertedEntries, 1)
	entry := syncer.UpsertedEntries[0]
	assert.Equal(t, tenantID, entry.TenantID)
	assert.Equal(t, scenarioID, entry.ScenarioID)
	assert.Equal(t, 2, entry.YearIndex)
	// 500 k€ × 1000 = 500 000 full €
	assert.True(t, decimal.NewFromInt(500_000).Equal(entry.Amount), "amount must be k€ × 1000")
	assert.Equal(t, rnd.ID, entry.RoundID)
	assert.Equal(t, "Series A", entry.RoundLabel)
}

func TestCapTable_SyncRoundToFiplan_UpdatesRoundSyncStatus(t *testing.T) {
	svc, repo, _ := newTestCapTableServiceWithSyncer()
	tenantID, scenarioID := uuid.New(), uuid.New()
	rnd := seedRound(t, repo, tenantID, scenarioID, 300)

	round, err := svc.SyncRoundToFiplan(context.Background(), tenantID, rnd.ID, 1)
	require.NoError(t, err)
	assert.True(t, round.FiplanSynced)
	require.NotNil(t, round.FiscalYearIndex)
	assert.Equal(t, 1, *round.FiscalYearIndex)

	// Verify persisted in repo
	persisted, _ := repo.GetRound(tenantID, rnd.ID)
	assert.True(t, persisted.FiplanSynced)
	require.NotNil(t, persisted.FiscalYearIndex)
	assert.Equal(t, 1, *persisted.FiscalYearIndex)
}

func TestCapTable_SyncRoundToFiplan_YearIndex0_IsValid(t *testing.T) {
	svc, repo, syncer := newTestCapTableServiceWithSyncer()
	tenantID, scenarioID := uuid.New(), uuid.New()
	rnd := seedRound(t, repo, tenantID, scenarioID, 100)

	_, err := svc.SyncRoundToFiplan(context.Background(), tenantID, rnd.ID, 0)
	require.NoError(t, err)
	assert.Len(t, syncer.UpsertedEntries, 1)
}

func TestCapTable_SyncRoundToFiplan_YearIndex4_IsValid(t *testing.T) {
	svc, repo, syncer := newTestCapTableServiceWithSyncer()
	tenantID, scenarioID := uuid.New(), uuid.New()
	rnd := seedRound(t, repo, tenantID, scenarioID, 100)

	_, err := svc.SyncRoundToFiplan(context.Background(), tenantID, rnd.ID, 4)
	require.NoError(t, err)
	assert.Len(t, syncer.UpsertedEntries, 1)
}

func TestCapTable_SyncRoundToFiplan_InvalidYearIndex_ReturnsError(t *testing.T) {
	svc, repo, syncer := newTestCapTableServiceWithSyncer()
	tenantID, scenarioID := uuid.New(), uuid.New()
	rnd := seedRound(t, repo, tenantID, scenarioID, 100)

	_, err := svc.SyncRoundToFiplan(context.Background(), tenantID, rnd.ID, 5)
	assert.Error(t, err)
	assert.Empty(t, syncer.UpsertedEntries)
}

func TestCapTable_SyncRoundToFiplan_NegativeYearIndex_ReturnsError(t *testing.T) {
	svc, repo, syncer := newTestCapTableServiceWithSyncer()
	tenantID, scenarioID := uuid.New(), uuid.New()
	rnd := seedRound(t, repo, tenantID, scenarioID, 100)

	_, err := svc.SyncRoundToFiplan(context.Background(), tenantID, rnd.ID, -1)
	assert.Error(t, err)
	assert.Empty(t, syncer.UpsertedEntries)
}

func TestCapTable_SyncRoundToFiplan_RoundNotFound_ReturnsError(t *testing.T) {
	svc, _, _ := newTestCapTableServiceWithSyncer()
	tenantID := uuid.New()

	_, err := svc.SyncRoundToFiplan(context.Background(), tenantID, uuid.New(), 2)
	assert.Error(t, err)
}

func TestCapTable_SyncRoundToFiplan_SyncerError_PropagatesError(t *testing.T) {
	svc, repo, syncer := newTestCapTableServiceWithSyncer()
	tenantID, scenarioID := uuid.New(), uuid.New()
	rnd := seedRound(t, repo, tenantID, scenarioID, 100)
	syncer.UpsertErr = errors.New("fiplan db error")

	_, err := svc.SyncRoundToFiplan(context.Background(), tenantID, rnd.ID, 2)
	assert.Error(t, err)
}

func TestCapTable_SyncRoundToFiplan_NilFiplanSyncer_ReturnsError(t *testing.T) {
	repo := NewMockCapTableRepo()
	settings := NewMockSettingsRepo()
	logger := logrus.NewEntry(logrus.New())
	emitter := event.NewEmitter(logger)
	svc := NewCapTableService(repo, settings, nil, emitter, logger)

	tenantID, scenarioID := uuid.New(), uuid.New()
	rnd := seedRound(t, repo, tenantID, scenarioID, 100)

	_, err := svc.SyncRoundToFiplan(context.Background(), tenantID, rnd.ID, 2)
	assert.Error(t, err)
}

func TestCapTable_UnlinkFromFiplan_ClearsSyncStatus(t *testing.T) {
	svc, repo, syncer := newTestCapTableServiceWithSyncer()
	tenantID, scenarioID := uuid.New(), uuid.New()
	rnd := seedRound(t, repo, tenantID, scenarioID, 400)

	// First sync
	_, err := svc.SyncRoundToFiplan(context.Background(), tenantID, rnd.ID, 3)
	require.NoError(t, err)

	// Then unlink
	round, err := svc.UnlinkFromFiplan(context.Background(), tenantID, rnd.ID)
	require.NoError(t, err)
	assert.False(t, round.FiplanSynced)
	assert.Nil(t, round.FiscalYearIndex)

	// Repo state updated
	persisted, _ := repo.GetRound(tenantID, rnd.ID)
	assert.False(t, persisted.FiplanSynced)
	assert.Nil(t, persisted.FiscalYearIndex)

	// Syncer was called with correct year
	require.Len(t, syncer.ClearedLinks, 1)
	assert.Equal(t, 3, syncer.ClearedLinks[0].YearIndex)
}

func TestCapTable_UnlinkFromFiplan_RoundNotSynced_ReturnsError(t *testing.T) {
	svc, repo, _ := newTestCapTableServiceWithSyncer()
	tenantID, scenarioID := uuid.New(), uuid.New()
	rnd := seedRound(t, repo, tenantID, scenarioID, 200)
	// Round was never synced

	_, err := svc.UnlinkFromFiplan(context.Background(), tenantID, rnd.ID)
	assert.Error(t, err)
}

func TestCapTable_UnlinkFromFiplan_RoundNotFound_ReturnsError(t *testing.T) {
	svc, _, _ := newTestCapTableServiceWithSyncer()
	tenantID := uuid.New()

	_, err := svc.UnlinkFromFiplan(context.Background(), tenantID, uuid.New())
	assert.Error(t, err)
}

func TestCapTable_UnlinkFromFiplan_SyncerError_PropagatesError(t *testing.T) {
	svc, repo, syncer := newTestCapTableServiceWithSyncer()
	tenantID, scenarioID := uuid.New(), uuid.New()
	rnd := seedRound(t, repo, tenantID, scenarioID, 100)

	// Pre-set as synced in the repo so we can reach the syncer call
	yearIdx := 2
	rnd.FiplanSynced = true
	rnd.FiscalYearIndex = &yearIdx
	repo.UpdateRound(rnd)

	syncer.ClearErr = errors.New("fiplan db error")
	_, err := svc.UnlinkFromFiplan(context.Background(), tenantID, rnd.ID)
	assert.Error(t, err)
}

func TestCapTable_SyncRoundToFiplan_AmountConversion_ZeroRaisedK(t *testing.T) {
	svc, repo, syncer := newTestCapTableServiceWithSyncer()
	tenantID, scenarioID := uuid.New(), uuid.New()
	rnd := seedRound(t, repo, tenantID, scenarioID, 0)

	_, err := svc.SyncRoundToFiplan(context.Background(), tenantID, rnd.ID, 0)
	require.NoError(t, err)
	require.Len(t, syncer.UpsertedEntries, 1)
	assert.True(t, decimal.Zero.Equal(syncer.UpsertedEntries[0].Amount))
}

// ── FiplanSyncedAmountK & IsSyncAmountDivergent ────────────────────────────────

func TestCapTable_SyncRoundToFiplan_SnapshotsAmountK(t *testing.T) {
	svc, repo, _ := newTestCapTableServiceWithSyncer()
	tenantID, scenarioID := uuid.New(), uuid.New()
	rnd := seedRound(t, repo, tenantID, scenarioID, 500)

	round, err := svc.SyncRoundToFiplan(context.Background(), tenantID, rnd.ID, 0)
	require.NoError(t, err)
	require.NotNil(t, round.FiplanSyncedAmountK, "FiplanSyncedAmountK must be set after sync")
	assert.True(t, round.AmountRaisedK.Equal(*round.FiplanSyncedAmountK),
		"FiplanSyncedAmountK should equal AmountRaisedK at time of sync")
}

func TestCapTable_ListRounds_IsSyncAmountDivergent_FalseWhenInSync(t *testing.T) {
	svc, repo, _ := newTestCapTableServiceWithSyncer()
	tenantID, scenarioID := uuid.New(), uuid.New()
	rnd := seedRound(t, repo, tenantID, scenarioID, 400)

	_, err := svc.SyncRoundToFiplan(context.Background(), tenantID, rnd.ID, 1)
	require.NoError(t, err)

	rounds, err := svc.ListRounds(context.Background(), tenantID, scenarioID)
	require.NoError(t, err)
	require.Len(t, rounds, 1)
	assert.False(t, rounds[0].IsSyncAmountDivergent, "divergence flag should be false when amounts match")
}

func TestCapTable_ListRounds_IsSyncAmountDivergent_TrueWhenAmountChanged(t *testing.T) {
	svc, repo, _ := newTestCapTableServiceWithSyncer()
	tenantID, scenarioID := uuid.New(), uuid.New()
	rnd := seedRound(t, repo, tenantID, scenarioID, 400)

	// Sync at 400
	_, err := svc.SyncRoundToFiplan(context.Background(), tenantID, rnd.ID, 1)
	require.NoError(t, err)

	// Simulate investor negotiation: amount changed to 600 in the repo directly
	persisted, _ := repo.GetRound(tenantID, rnd.ID)
	persisted.AmountRaisedK = decimal.NewFromInt(600)
	repo.UpdateRound(persisted)

	rounds, err := svc.ListRounds(context.Background(), tenantID, scenarioID)
	require.NoError(t, err)
	require.Len(t, rounds, 1)
	assert.True(t, rounds[0].IsSyncAmountDivergent, "divergence flag must be true when AmountRaisedK changed after sync")
}

func TestCapTable_UnlinkFromFiplan_ClearsFiplanSyncedAmountK(t *testing.T) {
	svc, repo, _ := newTestCapTableServiceWithSyncer()
	tenantID, scenarioID := uuid.New(), uuid.New()
	rnd := seedRound(t, repo, tenantID, scenarioID, 300)

	_, err := svc.SyncRoundToFiplan(context.Background(), tenantID, rnd.ID, 2)
	require.NoError(t, err)

	unlinked, err := svc.UnlinkFromFiplan(context.Background(), tenantID, rnd.ID)
	require.NoError(t, err)
	assert.Nil(t, unlinked.FiplanSyncedAmountK, "FiplanSyncedAmountK must be cleared after unlink")
	assert.False(t, unlinked.IsSyncAmountDivergent)
}
