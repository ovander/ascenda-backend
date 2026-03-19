package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"kerplan/internal/model"
)

func TestSettingsRepoGetConfig(t *testing.T) {
	mockRepo := NewMockSettingsRepo()

	tenantID := uuid.New()
	scenarioID := uuid.New()

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

	err := mockRepo.UpsertConfig(config)
	assert.NoError(t, err)

	retrieved, err := mockRepo.GetConfig(tenantID, scenarioID)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "BE", retrieved.Country)
	assert.True(t, config.DiscountRate.Equal(retrieved.DiscountRate))
}

func TestSettingsRepoGetConfigNotFound(t *testing.T) {
	mockRepo := NewMockSettingsRepo()

	tenantID := uuid.New()
	scenarioID := uuid.New()

	retrieved, err := mockRepo.GetConfig(tenantID, scenarioID)
	assert.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestSettingsRepoUpdateConfig(t *testing.T) {
	mockRepo := NewMockSettingsRepo()

	tenantID := uuid.New()
	scenarioID := uuid.New()

	config := &model.PlanConfig{
		TenantScoped:          model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:            scenarioID,
		DiscountRate:          decimal.NewFromFloat(0.15),
		CorporateTaxRate:      decimal.NewFromFloat(0.30),
		EmployerTaxRate:       decimal.NewFromFloat(0.40),
		IncentiveCap:          decimal.NewFromFloat(0.20),
		SalaryMonthsPerYear:   12,
		FirstFiscalYearMonths: 12,
		Country:               "FR",
	}

	err := mockRepo.UpsertConfig(config)
	assert.NoError(t, err)

	retrieved, _ := mockRepo.GetConfig(tenantID, scenarioID)
	assert.Equal(t, "FR", retrieved.Country)
	assert.True(t, decimal.NewFromFloat(0.30).Equal(retrieved.CorporateTaxRate))
}

func TestSettingsRepoGetOpeningBalance(t *testing.T) {
	mockRepo := NewMockSettingsRepo()

	tenantID := uuid.New()
	scenarioID := uuid.New()

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

	mockRepo.UpsertOpeningBalance(balance)

	retrieved, err := mockRepo.GetOpeningBalance(tenantID, scenarioID)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.True(t, balance.ShareCapital.Equal(retrieved.ShareCapital))
	assert.True(t, balance.SupplierPayables.Equal(retrieved.SupplierPayables))
}

func TestSettingsRepoUpdateOpeningBalance(t *testing.T) {
	mockRepo := NewMockSettingsRepo()

	tenantID := uuid.New()
	scenarioID := uuid.New()

	balance := &model.OpeningBalance{
		TenantScoped:        model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:          scenarioID,
		ShareCapital:        decimal.NewFromInt(200000),
		RetainedEarnings:    decimal.NewFromInt(125000),
		CustomerReceivables: decimal.NewFromInt(15000),
		CashAndSecurities:   decimal.NewFromInt(10000),
		LoansAndDebt:        decimal.NewFromInt(80000),
		SupplierPayables:    decimal.NewFromInt(12000),
		SocialAndTaxDebts:   decimal.NewFromInt(8000),
	}

	err := mockRepo.UpsertOpeningBalance(balance)
	assert.NoError(t, err)

	retrieved, _ := mockRepo.GetOpeningBalance(tenantID, scenarioID)
	assert.True(t, balance.ShareCapital.Equal(retrieved.ShareCapital))
	assert.True(t, balance.LoansAndDebt.Equal(retrieved.LoansAndDebt))
}

func TestSettingsRepoGetWCConfig(t *testing.T) {
	mockRepo := NewMockSettingsRepo()

	tenantID := uuid.New()
	scenarioID := uuid.New()

	wcConfig := &model.WorkingCapitalConfig{
		TenantScoped:      model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:        scenarioID,
		CustomerPct0Days:  decimal.NewFromFloat(0.30),
		CustomerPct30Days: decimal.NewFromFloat(0.70),
		SupplierPct0Days:  decimal.NewFromFloat(0.20),
		SupplierPct30Days: decimal.NewFromFloat(0.50),
		SupplierPct60Days: decimal.NewFromFloat(0.30),
		InventoryPctYear1: decimal.NewFromFloat(0.10),
		InventoryPctYear2: decimal.NewFromFloat(0.10),
		InventoryPctYear3: decimal.NewFromFloat(0.10),
		InventoryPctYear4: decimal.NewFromFloat(0.10),
		InventoryPctYear5: decimal.NewFromFloat(0.10),
	}

	mockRepo.UpsertWCConfig(wcConfig)

	retrieved, err := mockRepo.GetWCConfig(tenantID, scenarioID)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.True(t, decimal.NewFromFloat(0.30).Equal(retrieved.CustomerPct0Days))
	assert.True(t, decimal.NewFromFloat(0.50).Equal(retrieved.SupplierPct30Days))
}

func TestSettingsRepoUpdateWCConfig(t *testing.T) {
	mockRepo := NewMockSettingsRepo()

	tenantID := uuid.New()
	scenarioID := uuid.New()

	wcConfig := &model.WorkingCapitalConfig{
		TenantScoped:      model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:        scenarioID,
		CustomerPct0Days:  decimal.NewFromFloat(0.10),
		CustomerPct30Days: decimal.NewFromFloat(0.40),
		CustomerPct60Days: decimal.NewFromFloat(0.50),
		SupplierPct0Days:  decimal.NewFromFloat(0.10),
		SupplierPct30Days: decimal.NewFromFloat(0.40),
		SupplierPct60Days: decimal.NewFromFloat(0.50),
		InventoryPctYear1: decimal.NewFromFloat(0.15),
		InventoryPctYear2: decimal.NewFromFloat(0.15),
		InventoryPctYear3: decimal.NewFromFloat(0.15),
		InventoryPctYear4: decimal.NewFromFloat(0.15),
		InventoryPctYear5: decimal.NewFromFloat(0.15),
	}

	err := mockRepo.UpsertWCConfig(wcConfig)
	assert.NoError(t, err)

	retrieved, _ := mockRepo.GetWCConfig(tenantID, scenarioID)
	assert.True(t, decimal.NewFromFloat(0.10).Equal(retrieved.CustomerPct0Days))
	assert.True(t, decimal.NewFromFloat(0.15).Equal(retrieved.InventoryPctYear1))
}

func TestSettingsRepoMultipleScenarios(t *testing.T) {
	mockRepo := NewMockSettingsRepo()

	tenantID := uuid.New()
	scenario1 := uuid.New()
	scenario2 := uuid.New()

	config1 := &model.PlanConfig{
		TenantScoped:     model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:       scenario1,
		CorporateTaxRate: decimal.NewFromFloat(0.25),
	}

	config2 := &model.PlanConfig{
		TenantScoped:     model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:       scenario2,
		CorporateTaxRate: decimal.NewFromFloat(0.30),
	}

	mockRepo.UpsertConfig(config1)
	mockRepo.UpsertConfig(config2)

	retrieved1, _ := mockRepo.GetConfig(tenantID, scenario1)
	retrieved2, _ := mockRepo.GetConfig(tenantID, scenario2)

	assert.True(t, decimal.NewFromFloat(0.25).Equal(retrieved1.CorporateTaxRate))
	assert.True(t, decimal.NewFromFloat(0.30).Equal(retrieved2.CorporateTaxRate))
}
