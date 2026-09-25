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

func newTestFiplanService() (*FiplanService, *MockFiplanRepo) {
	repo := NewMockFiplanRepo()
	logger := logrus.NewEntry(logrus.New())
	emitter := event.NewEmitter(logger)
	return NewFiplanService(repo, nil, emitter, logger), repo
}

func TestFiplan_ListEntries_EmptyInitially(t *testing.T) {
	svc, _ := newTestFiplanService()
	rows, err := svc.ListEntries(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestFiplan_UpdateEntries_StampsIDs(t *testing.T) {
	svc, repo := newTestFiplanService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	entries := []model.FiplanEntry{
		{ScenarioID: scenarioID, LineID: model.FiplanSubsidies, YearIndex: 1, Amount: decimal.NewFromInt(50000)},
	}
	require.NoError(t, svc.UpdateEntries(ctx, tenantID, scenarioID, entries))

	stored, _ := repo.ListByScenario(tenantID, scenarioID)
	require.Len(t, stored, 1)
	assert.NotEqual(t, uuid.Nil, stored[0].ID)
	assert.Equal(t, tenantID, stored[0].TenantID)
	assert.Equal(t, scenarioID, stored[0].ScenarioID)
}

func TestFiplan_ListEntries_ReturnsAfterUpdate(t *testing.T) {
	svc, _ := newTestFiplanService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateEntries(ctx, tenantID, scenarioID, []model.FiplanEntry{
		{ScenarioID: scenarioID, LineID: model.FiplanSubsidies, YearIndex: 1, Amount: decimal.NewFromInt(10000)},
		{ScenarioID: scenarioID, LineID: model.FiplanLTLoans, YearIndex: 1, Amount: decimal.NewFromInt(200000)},
	})

	rows, err := svc.ListEntries(ctx, tenantID, scenarioID)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}

func TestFiplan_GetGrantsForPnL_FiltersGrantLines(t *testing.T) {
	svc, _ := newTestFiplanService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	// Mix of grant and non-grant lines
	svc.UpdateEntries(ctx, tenantID, scenarioID, []model.FiplanEntry{
		{ScenarioID: scenarioID, LineID: model.FiplanSubsidies, YearIndex: 1, Amount: decimal.NewFromInt(10000)},
		{ScenarioID: scenarioID, LineID: model.FiplanOtherGrants, YearIndex: 1, Amount: decimal.NewFromInt(5000)},
		{ScenarioID: scenarioID, LineID: model.FiplanRepayableGrants, YearIndex: 1, Amount: decimal.NewFromInt(3000)},
		{ScenarioID: scenarioID, LineID: model.FiplanLTLoans, YearIndex: 1, Amount: decimal.NewFromInt(200000)},
		{ScenarioID: scenarioID, LineID: model.FiplanDividends, YearIndex: 1, Amount: decimal.NewFromInt(0)},
	})

	grants, err := svc.GetGrantsForPnL(ctx, tenantID, scenarioID)
	require.NoError(t, err)
	assert.Len(t, grants, 3, "only subsidies, other grants and repayable grants should be returned")

	lineIDs := map[model.FiplanLineID]bool{}
	for _, g := range grants {
		lineIDs[g.LineID] = true
	}
	assert.True(t, lineIDs[model.FiplanSubsidies])
	assert.True(t, lineIDs[model.FiplanOtherGrants])
	assert.True(t, lineIDs[model.FiplanRepayableGrants])
	assert.False(t, lineIDs[model.FiplanLTLoans])
	assert.False(t, lineIDs[model.FiplanDividends])
}

func TestFiplan_GetGrantsForPnL_EmptyWhenNoGrants(t *testing.T) {
	svc, _ := newTestFiplanService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateEntries(ctx, tenantID, scenarioID, []model.FiplanEntry{
		{ScenarioID: scenarioID, LineID: model.FiplanLTLoans, YearIndex: 1, Amount: decimal.NewFromInt(100000)},
	})

	grants, err := svc.GetGrantsForPnL(ctx, tenantID, scenarioID)
	require.NoError(t, err)
	assert.Empty(t, grants)
}

// ── GetCapitalIncreaseEntry ──────────────────────────────────────────────────

func TestFiplan_GetCapitalIncreaseEntry_ReturnsNilWhenNoEntry(t *testing.T) {
	svc, _ := newTestFiplanService()
	tenantID, scenarioID := uuid.New(), uuid.New()

	entry, err := svc.GetCapitalIncreaseEntry(context.Background(), tenantID, scenarioID, 0)
	require.NoError(t, err)
	assert.Nil(t, entry, "should return nil when no capital_increase entry exists")
}

func TestFiplan_GetCapitalIncreaseEntry_ReturnsEntryForCorrectYear(t *testing.T) {
	svc, _ := newTestFiplanService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	// Insert capital_increase for year 1 (yearIndex=1, stored 1-based)
	svc.UpdateEntries(ctx, tenantID, scenarioID, []model.FiplanEntry{
		{ScenarioID: scenarioID, LineID: model.FiplanCapitalIncrease, YearIndex: 1, Amount: decimal.NewFromInt(500000)},
		{ScenarioID: scenarioID, LineID: model.FiplanCapitalIncrease, YearIndex: 3, Amount: decimal.NewFromInt(200000)},
	})

	// Request fiscal year 0 (→ yearIndex 1, stored as year=1)
	entry, err := svc.GetCapitalIncreaseEntry(ctx, tenantID, scenarioID, 0)
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, decimal.NewFromInt(500000), entry.Amount)

	// Request fiscal year 2 (→ yearIndex 3, stored as year=3)
	entry2, err := svc.GetCapitalIncreaseEntry(ctx, tenantID, scenarioID, 2)
	require.NoError(t, err)
	require.NotNil(t, entry2)
	assert.Equal(t, decimal.NewFromInt(200000), entry2.Amount)
}

func TestFiplan_GetCapitalIncreaseEntry_WrongYearReturnsNil(t *testing.T) {
	svc, _ := newTestFiplanService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateEntries(ctx, tenantID, scenarioID, []model.FiplanEntry{
		{ScenarioID: scenarioID, LineID: model.FiplanCapitalIncrease, YearIndex: 1, Amount: decimal.NewFromInt(500000)},
	})

	// Year 2 has no entry — should return nil without error
	entry, err := svc.GetCapitalIncreaseEntry(ctx, tenantID, scenarioID, 1)
	require.NoError(t, err)
	assert.Nil(t, entry)
}

// ── UpsertCapitalIncreaseEntry / ClearCapTableLink ───────────────────────────

func TestFiplan_UpsertCapitalIncreaseEntry_StoresLinkFields(t *testing.T) {
	svc, repo := newTestFiplanService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	roundID := uuid.New()
	ctx := context.Background()

	err := svc.UpsertCapitalIncreaseEntry(ctx, tenantID, scenarioID, 0, decimal.NewFromInt(500000), roundID, "Series A")
	require.NoError(t, err)

	stored, _ := repo.ListByScenario(tenantID, scenarioID)
	require.Len(t, stored, 1)
	assert.Equal(t, model.FiplanCapitalIncrease, stored[0].LineID)
	assert.Equal(t, 1, stored[0].YearIndex) // 0-based → 1-based conversion
	require.NotNil(t, stored[0].CapTableRoundID)
	assert.Equal(t, roundID, *stored[0].CapTableRoundID)
	assert.Equal(t, "Series A", stored[0].CapTableRoundLabel)
}

func TestFiplan_ClearCapTableLink_RemovesLinkFields(t *testing.T) {
	svc, repo := newTestFiplanService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	roundID := uuid.New()
	ctx := context.Background()

	// First upsert a linked entry
	require.NoError(t, svc.UpsertCapitalIncreaseEntry(ctx, tenantID, scenarioID, 0, decimal.NewFromInt(500000), roundID, "Series A"))

	// Then clear the link
	require.NoError(t, svc.ClearCapTableLink(ctx, tenantID, scenarioID, 0))

	stored, _ := repo.ListByScenario(tenantID, scenarioID)
	require.Len(t, stored, 1)
	assert.Nil(t, stored[0].CapTableRoundID, "cap table round ID should be nil after clearing")
	assert.Empty(t, stored[0].CapTableRoundLabel)
	assert.Equal(t, decimal.NewFromInt(500000), stored[0].Amount, "amount must be preserved")
}

func TestFiplan_ScenarioIsolation(t *testing.T) {
	svc, _ := newTestFiplanService()
	tenantID := uuid.New()
	s1, s2 := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateEntries(ctx, tenantID, s1, []model.FiplanEntry{
		{ScenarioID: s1, LineID: model.FiplanSubsidies, YearIndex: 1, Amount: decimal.NewFromInt(5000)},
	})
	svc.UpdateEntries(ctx, tenantID, s2, []model.FiplanEntry{
		{ScenarioID: s2, LineID: model.FiplanLTLoans, YearIndex: 1, Amount: decimal.NewFromInt(100000)},
		{ScenarioID: s2, LineID: model.FiplanCapitalIncrease, YearIndex: 1, Amount: decimal.NewFromInt(50000)},
	})

	r1, _ := svc.ListEntries(ctx, tenantID, s1)
	r2, _ := svc.ListEntries(ctx, tenantID, s2)
	assert.Len(t, r1, 1)
	assert.Len(t, r2, 2)
}
