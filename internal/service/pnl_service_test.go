package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"kerplan/internal/model"
)

// MockPnlRepo is a manual mock implementation for testing PnL entries
type MockPnlRepo struct {
	entries map[string][]*model.PnlManualEntry
}

// NewMockPnlRepo creates a new mock P&L repository
func NewMockPnlRepo() *MockPnlRepo {
	return &MockPnlRepo{
		entries: make(map[string][]*model.PnlManualEntry),
	}
}

// ListByScenario lists PnL entries for a scenario
func (m *MockPnlRepo) ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.PnlManualEntry, error) {
	key := tenantID.String() + ":" + scenarioID.String()
	return m.entries[key], nil
}

// BatchUpsert saves PnL entries for a scenario
func (m *MockPnlRepo) BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.PnlManualEntry) error {
	key := tenantID.String() + ":" + scenarioID.String()
	ptrs := make([]*model.PnlManualEntry, len(entries))
	for i := range entries {
		e := entries[i]
		ptrs[i] = &e
	}
	m.entries[key] = ptrs
	return nil
}

func TestPnlRepoListByScenario(t *testing.T) {
	mockRepo := NewMockPnlRepo()

	tenantID := uuid.New()
	scenarioID := uuid.New()

	entries := []model.PnlManualEntry{
		{
			TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			ScenarioID:   scenarioID,
			LineID:       "operating_subsidy",
			YearIndex:         1,
			Amount:       decimal.NewFromInt(10000),
		},
		{
			TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			ScenarioID:   scenarioID,
			LineID:       "operating_subsidy",
			YearIndex:         2,
			Amount:       decimal.NewFromInt(15000),
		},
	}

	err := mockRepo.BatchUpsert(tenantID, scenarioID, entries)
	assert.NoError(t, err)

	retrieved, err := mockRepo.ListByScenario(tenantID, scenarioID)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(retrieved))
}

func TestPnlRepoScenarioIsolation(t *testing.T) {
	mockRepo := NewMockPnlRepo()

	tenantID := uuid.New()
	scenario1 := uuid.New()
	scenario2 := uuid.New()

	entries1 := []model.PnlManualEntry{
		{
			TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			ScenarioID:   scenario1,
			LineID:       "operating_subsidy",
			YearIndex:         1,
			Amount:       decimal.NewFromInt(10000),
		},
	}

	entries2 := []model.PnlManualEntry{
		{
			TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			ScenarioID:   scenario2,
			LineID:       "financial_income",
			YearIndex:         1,
			Amount:       decimal.NewFromInt(5000),
		},
		{
			TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			ScenarioID:   scenario2,
			LineID:       "financial_income",
			YearIndex:         2,
			Amount:       decimal.NewFromInt(7000),
		},
	}

	mockRepo.BatchUpsert(tenantID, scenario1, entries1)
	mockRepo.BatchUpsert(tenantID, scenario2, entries2)

	list1, _ := mockRepo.ListByScenario(tenantID, scenario1)
	list2, _ := mockRepo.ListByScenario(tenantID, scenario2)

	assert.Equal(t, 1, len(list1))
	assert.Equal(t, 2, len(list2))
}

func TestPnlRepoEmptyScenario(t *testing.T) {
	mockRepo := NewMockPnlRepo()

	tenantID := uuid.New()
	scenarioID := uuid.New()

	retrieved, err := mockRepo.ListByScenario(tenantID, scenarioID)
	assert.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestPnlRepoBatchUpsertOverwrite(t *testing.T) {
	mockRepo := NewMockPnlRepo()

	tenantID := uuid.New()
	scenarioID := uuid.New()

	// First batch
	entries1 := []model.PnlManualEntry{
		{
			TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			ScenarioID:   scenarioID,
			LineID:       "operating_subsidy",
			YearIndex:         1,
			Amount:       decimal.NewFromInt(10000),
		},
	}
	mockRepo.BatchUpsert(tenantID, scenarioID, entries1)

	// Second batch overwrites
	entries2 := []model.PnlManualEntry{
		{
			TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			ScenarioID:   scenarioID,
			LineID:       "financial_income",
			YearIndex:         1,
			Amount:       decimal.NewFromInt(5000),
		},
		{
			TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			ScenarioID:   scenarioID,
			LineID:       "financial_income",
			YearIndex:         2,
			Amount:       decimal.NewFromInt(7000),
		},
	}
	mockRepo.BatchUpsert(tenantID, scenarioID, entries2)

	retrieved, _ := mockRepo.ListByScenario(tenantID, scenarioID)
	assert.Equal(t, 2, len(retrieved))
}
