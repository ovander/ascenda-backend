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

func newTestOpexService() (*OpexService, *MockOpexRepo) {
	repo := NewMockOpexRepo()
	logger := logrus.NewEntry(logrus.New())
	emitter := event.NewEmitter(logger)
	return NewOpexService(repo, nil, emitter, logger), repo
}

func TestOpex_ListEntries_EmptyInitially(t *testing.T) {
	svc, _ := newTestOpexService()
	rows, err := svc.ListEntries(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestOpex_UpdateEntries_StampsIDs(t *testing.T) {
	svc, repo := newTestOpexService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	entries := []model.OpexManualEntry{
		{ScenarioID: scenarioID, LineID: model.OpexLineID("marketing"), YearIndex: 1, Amount: decimal.NewFromInt(12000)},
	}
	require.NoError(t, svc.UpdateEntries(ctx, tenantID, scenarioID, entries))

	stored, _ := repo.ListByScenario(tenantID, scenarioID)
	require.Len(t, stored, 1)
	assert.NotEqual(t, uuid.Nil, stored[0].ID)
	assert.Equal(t, tenantID, stored[0].TenantID)
	assert.Equal(t, scenarioID, stored[0].ScenarioID)
}

func TestOpex_ListEntries_ReturnsAfterUpdate(t *testing.T) {
	svc, _ := newTestOpexService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateEntries(ctx, tenantID, scenarioID, []model.OpexManualEntry{
		{ScenarioID: scenarioID, LineID: model.OpexLineID("marketing"), YearIndex: 1, Amount: decimal.NewFromInt(5000)},
		{ScenarioID: scenarioID, LineID: model.OpexLineID("marketing"), YearIndex: 2, Amount: decimal.NewFromInt(6000)},
		{ScenarioID: scenarioID, LineID: model.OpexLineID("marketing"), YearIndex: 3, Amount: decimal.NewFromInt(7000)},
	})

	rows, err := svc.ListEntries(ctx, tenantID, scenarioID)
	require.NoError(t, err)
	assert.Len(t, rows, 3)
}

func TestOpex_UpdateEntries_Overwrites(t *testing.T) {
	svc, _ := newTestOpexService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateEntries(ctx, tenantID, scenarioID, []model.OpexManualEntry{
		{ScenarioID: scenarioID, LineID: model.OpexLineID("marketing"), YearIndex: 1, Amount: decimal.NewFromInt(5000)},
		{ScenarioID: scenarioID, LineID: model.OpexLineID("marketing"), YearIndex: 2, Amount: decimal.NewFromInt(6000)},
	})
	svc.UpdateEntries(ctx, tenantID, scenarioID, []model.OpexManualEntry{
		{ScenarioID: scenarioID, LineID: model.OpexLineID("it"), YearIndex: 1, Amount: decimal.NewFromInt(3000)},
	})

	rows, _ := svc.ListEntries(ctx, tenantID, scenarioID)
	assert.Len(t, rows, 1, "second upsert should replace first batch")
}

func TestOpex_ScenarioIsolation(t *testing.T) {
	svc, _ := newTestOpexService()
	tenantID := uuid.New()
	s1, s2 := uuid.New(), uuid.New()
	ctx := context.Background()

	svc.UpdateEntries(ctx, tenantID, s1, []model.OpexManualEntry{
		{ScenarioID: s1, LineID: model.OpexLineID("marketing"), YearIndex: 1, Amount: decimal.NewFromInt(1000)},
	})
	svc.UpdateEntries(ctx, tenantID, s2, []model.OpexManualEntry{
		{ScenarioID: s2, LineID: model.OpexLineID("it"), YearIndex: 1, Amount: decimal.NewFromInt(2000)},
		{ScenarioID: s2, LineID: model.OpexLineID("it"), YearIndex: 2, Amount: decimal.NewFromInt(2500)},
	})

	r1, _ := svc.ListEntries(ctx, tenantID, s1)
	r2, _ := svc.ListEntries(ctx, tenantID, s2)
	assert.Len(t, r1, 1)
	assert.Len(t, r2, 2)
}
