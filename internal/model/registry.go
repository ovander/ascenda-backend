package model

// TableModels returns one instance of every model that owns a database table.
//
// The SQL migrations are the source of truth for the schema; this list is
// what GORM AutoMigrate (DB_AUTO_MIGRATE, development only) walks, and what
// the schema drift test in internal/repo compares the migrations against: on
// a freshly migrated database, AutoMigrate over this list must not issue a
// single DDL statement. Add every new table model here.
func TableModels() []interface{} {
	return []interface{}{
		// Tenant and user models
		&Organization{},
		&Tenant{},
		&User{},
		&PlanMember{},
		&BusinessPlan{},
		&Scenario{},

		// Settings
		&PlanConfig{},
		&OpeningBalance{},
		&WorkingCapitalConfig{},
		&OpexPerHire{},
		&CapexPerHire{},
		&MultiYearAdjustment{},

		// Product
		&Product{},
		&ProductAssumption{},
		&ProductSalesVolume{},
		&ProductDistributorMargin{},

		// Staff
		&StaffHeadcount{},
		&StaffSalary{},
		&StaffIncentive{},

		// Financial
		&CapexEntry{},
		&OpexManualEntry{},
		&PnlManualEntry{},
		&FiplanEntry{},
		&PnlCashEntry{},
		&WCREntry{},
		&CashMonthlyOverride{},
		&BudgetMonthlyOverride{},

		// Reporting
		&Report{},
		&PlanSnapshot{},
		&AuditLog{},

		// Platform configs
		&CountryRateConfig{},

		// AI
		&AINarrationCache{},
		&AIUsagePolicy{},
		&AIUsageRecord{},

		// BEP
		&BEPSnapshot{},
		&FixedCostLine{},
		&VariableCostLine{},
		&SensitivityConfig{},
		&OptimisationPlan{},
		&FixedCostSaving{},
		&VariableCostSaving{},
		&PCGReviewItem{},

		// Cap table
		&PlanShareholder{},
		&CapTableCompany{},
		&CapTableShareClass{},
		&CapTableShareholder{},
		&CapTableRound{},
		&CapTablePosition{},
		&StockOptionPlan{},
		&OptionGrant{},
		&ValuationScenario{},
		&CapTableScenarioBranch{},

		// Auth
		&MagicLinkToken{},

		// Feature policies
		&FeaturePolicy{},
	}
}
