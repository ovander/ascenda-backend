package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"ascenda/internal/model"
)

func TestSettingsRepoIntegration(t *testing.T) {
	// Integration test: Create config, balance, and WC config for a scenario using mock repos directly
	mockSettingsRepo := NewMockSettingsRepo()

	tenantID := uuid.New()
	scenarioID := uuid.New()

	// Create initial config
	config := &model.PlanConfig{
		TenantScoped:          model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:            scenarioID,
		DiscountRate:          decimal.NewFromFloat(0.10),
		CorporateTaxRate:      decimal.NewFromFloat(0.25),
		EmployerTaxRate:       decimal.NewFromFloat(0.42),
		IncentiveCap:          decimal.NewFromFloat(0.15),
		SalaryMonthsPerYear:   12,
		FirstFiscalYearMonths: 12,
		Country:               "BE",
	}

	err := mockSettingsRepo.UpsertConfig(config)
	assert.NoError(t, err)

	// Create opening balance
	balance := &model.OpeningBalance{
		TenantScoped:        model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:          scenarioID,
		ShareCapital:        decimal.NewFromInt(100000),
		RetainedEarnings:    decimal.NewFromInt(75000),
		CustomerReceivables: decimal.NewFromInt(10000),
		CashAndSecurities:   decimal.NewFromInt(5000),
		LoansAndDebt:        decimal.NewFromInt(45000),
		SupplierPayables:    decimal.NewFromInt(8000),
		SocialAndTaxDebts:   decimal.NewFromInt(4000),
	}

	err = mockSettingsRepo.UpsertOpeningBalance(balance)
	assert.NoError(t, err)

	// Create WC config
	wcConfig := &model.WorkingCapitalConfig{
		TenantScoped:      model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:        scenarioID,
		CustomerPct0Days:  decimal.NewFromFloat(0.30),
		CustomerPct30Days: decimal.NewFromFloat(0.70),
		SupplierPct0Days:  decimal.NewFromFloat(0.20),
		SupplierPct30Days: decimal.NewFromFloat(0.80),
		InventoryPctYear1: decimal.NewFromFloat(0.10),
		InventoryPctYear2: decimal.NewFromFloat(0.10),
		InventoryPctYear3: decimal.NewFromFloat(0.10),
		InventoryPctYear4: decimal.NewFromFloat(0.10),
		InventoryPctYear5: decimal.NewFromFloat(0.10),
	}

	err = mockSettingsRepo.UpsertWCConfig(wcConfig)
	assert.NoError(t, err)

	// Verify all settings are retrievable
	retrievedConfig, err := mockSettingsRepo.GetConfig(tenantID, scenarioID)
	assert.NoError(t, err)
	assert.NotNil(t, retrievedConfig)
	assert.Equal(t, "BE", retrievedConfig.Country)

	retrievedBalance, err := mockSettingsRepo.GetOpeningBalance(tenantID, scenarioID)
	assert.NoError(t, err)
	assert.NotNil(t, retrievedBalance)
	assert.Equal(t, decimal.NewFromInt(100000), retrievedBalance.ShareCapital)

	retrievedWC, err := mockSettingsRepo.GetWCConfig(tenantID, scenarioID)
	assert.NoError(t, err)
	assert.NotNil(t, retrievedWC)
	assert.Equal(t, decimal.NewFromFloat(0.30), retrievedWC.CustomerPct0Days)
}

func TestSnapshotRepoIntegration(t *testing.T) {
	// Integration test: Create multiple snapshots and verify isolation
	mockSnapshotRepo := NewMockSnapshotRepo()

	tenantID := uuid.New()
	scenario1 := uuid.New()
	scenario2 := uuid.New()

	// Create snapshots for scenario 1
	snap1a := &model.PlanSnapshot{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:   scenario1,
		Version:      1,
		Label:        "Snapshot 1A",
	}

	snap1b := &model.PlanSnapshot{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:   scenario1,
		Version:      2,
		Label:        "Snapshot 1B",
	}

	// Create snapshot for scenario 2
	snap2 := &model.PlanSnapshot{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:   scenario2,
		Version:      1,
		Label:        "Snapshot 2A",
	}

	mockSnapshotRepo.CreateSnapshot(snap1a)
	mockSnapshotRepo.CreateSnapshot(snap1b)
	mockSnapshotRepo.CreateSnapshot(snap2)

	// List snapshots for scenario 1
	list1, _ := mockSnapshotRepo.ListSnapshots(tenantID, scenario1)
	assert.Equal(t, 2, len(list1))

	// List snapshots for scenario 2
	list2, _ := mockSnapshotRepo.ListSnapshots(tenantID, scenario2)
	assert.Equal(t, 1, len(list2))

	// Verify snapshots can be retrieved individually
	retrieved1, _ := mockSnapshotRepo.GetSnapshot(tenantID, snap1a.ID)
	assert.Equal(t, "Snapshot 1A", retrieved1.Label)

	retrieved2, _ := mockSnapshotRepo.GetSnapshot(tenantID, snap2.ID)
	assert.Equal(t, "Snapshot 2A", retrieved2.Label)
}

func TestMultiTenantIsolation(t *testing.T) {
	// Integration test: Verify data from different tenants doesn't mix
	mockSettingsRepo := NewMockSettingsRepo()

	tenant1 := uuid.New()
	tenant2 := uuid.New()
	scenario := uuid.New()

	// Create config for tenant 1
	config1 := &model.PlanConfig{
		TenantScoped:     model.TenantScoped{ID: uuid.New(), TenantID: tenant1},
		ScenarioID:       scenario,
		CorporateTaxRate: decimal.NewFromFloat(0.25),
		Country:          "BE",
	}

	// Create config for tenant 2
	config2 := &model.PlanConfig{
		TenantScoped:     model.TenantScoped{ID: uuid.New(), TenantID: tenant2},
		ScenarioID:       scenario,
		CorporateTaxRate: decimal.NewFromFloat(0.30),
		Country:          "FR",
	}

	mockSettingsRepo.UpsertConfig(config1)
	mockSettingsRepo.UpsertConfig(config2)

	// Retrieve configs for both tenants
	retrieved1, _ := mockSettingsRepo.GetConfig(tenant1, scenario)
	retrieved2, _ := mockSettingsRepo.GetConfig(tenant2, scenario)

	// Verify they're different
	assert.Equal(t, "BE", retrieved1.Country)
	assert.Equal(t, "FR", retrieved2.Country)
	assert.NotEqual(t, retrieved1.CorporateTaxRate, retrieved2.CorporateTaxRate)
}

func TestScenarioSpecificSettings(t *testing.T) {
	// Integration test: Verify settings are scenario-specific
	mockSettingsRepo := NewMockSettingsRepo()

	tenantID := uuid.New()
	scenario1 := uuid.New()
	scenario2 := uuid.New()

	// Create different opening balances for each scenario
	balance1 := &model.OpeningBalance{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:   scenario1,
		ShareCapital: decimal.NewFromInt(100000),
	}

	balance2 := &model.OpeningBalance{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:   scenario2,
		ShareCapital: decimal.NewFromInt(200000),
	}

	mockSettingsRepo.UpsertOpeningBalance(balance1)
	mockSettingsRepo.UpsertOpeningBalance(balance2)

	// Retrieve balances for each scenario
	retrieved1, _ := mockSettingsRepo.GetOpeningBalance(tenantID, scenario1)
	retrieved2, _ := mockSettingsRepo.GetOpeningBalance(tenantID, scenario2)

	// Verify they're different
	assert.Equal(t, decimal.NewFromInt(100000), retrieved1.ShareCapital)
	assert.Equal(t, decimal.NewFromInt(200000), retrieved2.ShareCapital)
}
