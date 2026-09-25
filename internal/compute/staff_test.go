package compute

import (
	"testing"
	"time"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestComputeStaffPayroll(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	now := time.Now()

	config := model.PlanConfig{
		ScenarioID:            scenarioID,
		CorporateTaxRate:      decimal.NewFromFloat(0.25),
		EmployerTaxRate:       decimal.NewFromFloat(0.42),
		IncentiveCap:          decimal.NewFromFloat(0.15),
		SalaryMonthsPerYear:   12,
		FirstFiscalYearMonths: 12,
		ForecastStart:         now,
		PreviousStaff:         0,
	}

	tests := []struct {
		name        string
		headcounts  []model.StaffHeadcount
		salaries    []model.StaffSalary
		incentives  []model.StaffIncentive
		config      model.PlanConfig
		expectedSal [5]decimal.Decimal
		expectedInc [5]decimal.Decimal
		expectedPay [5]decimal.Decimal
	}{
		{
			name: "single role - 5 entries per role one per year",
			headcounts: []model.StaffHeadcount{
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 1, FTE: decimal.NewFromInt(1)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 2, FTE: decimal.NewFromInt(2)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 3, FTE: decimal.NewFromInt(2)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 4, FTE: decimal.NewFromInt(3)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 5, FTE: decimal.NewFromInt(3)},
			},
			salaries: []model.StaffSalary{
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 1, MonthlyGrossSalary: decimal.NewFromInt(3000)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 2, MonthlyGrossSalary: decimal.NewFromInt(3000)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 3, MonthlyGrossSalary: decimal.NewFromInt(3500)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 4, MonthlyGrossSalary: decimal.NewFromInt(3500)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 5, MonthlyGrossSalary: decimal.NewFromInt(4000)},
			},
			incentives: []model.StaffIncentive{
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 1, IncentivePct: decimal.NewFromFloat(0.10)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 2, IncentivePct: decimal.NewFromFloat(0.10)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 3, IncentivePct: decimal.NewFromFloat(0.10)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 4, IncentivePct: decimal.NewFromFloat(0.12)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 5, IncentivePct: decimal.NewFromFloat(0.15)},
			},
			config: config,
			// Formula: SubtotalPayroll (€) = FTE × MonthlyGrossSalary(€) × (1 + EmployerTaxRate) × Months
			// Year 1: 1 × 3000 × 1.42 × 12 = 51120   IncentiveAmount = 51120 × 0.10 = 5112   Total = 56232
			// Year 2: 2 × 3000 × 1.42 × 12 = 102240  IncentiveAmount = 102240 × 0.10 = 10224 Total = 112464
			// Year 3: 2 × 3500 × 1.42 × 12 = 119280  IncentiveAmount = 119280 × 0.10 = 11928 Total = 131208
			// Year 4: 3 × 3500 × 1.42 × 12 = 178920  IncentiveAmount = 178920 × 0.12 = 21470.4 Total = 200390.4
			// Year 5: 3 × 4000 × 1.42 × 12 = 204480  IncentiveAmount = 204480 × 0.15 = 30672 Total = 235152
			expectedSal: [5]decimal.Decimal{
				decimal.NewFromFloat(51120), decimal.NewFromFloat(102240), decimal.NewFromFloat(119280),
				decimal.NewFromFloat(178920), decimal.NewFromFloat(204480),
			},
			expectedInc: [5]decimal.Decimal{
				decimal.NewFromFloat(5112), decimal.NewFromFloat(10224), decimal.NewFromFloat(11928),
				decimal.NewFromFloat(21470.4), decimal.NewFromFloat(30672),
			},
			expectedPay: [5]decimal.Decimal{
				decimal.NewFromFloat(56232), decimal.NewFromFloat(112464), decimal.NewFromFloat(131208),
				decimal.NewFromFloat(200390.4), decimal.NewFromFloat(235152),
			},
		},
		{
			name: "zero headcounts",
			headcounts: []model.StaffHeadcount{
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 1, FTE: decimal.Zero},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 2, FTE: decimal.Zero},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 3, FTE: decimal.Zero},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 4, FTE: decimal.Zero},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 5, FTE: decimal.Zero},
			},
			salaries: []model.StaffSalary{
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 1, MonthlyGrossSalary: decimal.NewFromInt(3000)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 2, MonthlyGrossSalary: decimal.NewFromInt(3000)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 3, MonthlyGrossSalary: decimal.NewFromInt(3000)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 4, MonthlyGrossSalary: decimal.NewFromInt(3000)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 5, MonthlyGrossSalary: decimal.NewFromInt(3000)},
			},
			incentives: []model.StaffIncentive{
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 1, IncentivePct: decimal.NewFromFloat(0.10)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 2, IncentivePct: decimal.NewFromFloat(0.10)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 3, IncentivePct: decimal.NewFromFloat(0.10)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 4, IncentivePct: decimal.NewFromFloat(0.10)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 5, IncentivePct: decimal.NewFromFloat(0.10)},
			},
			config: config,
			expectedSal: [5]decimal.Decimal{
				decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
			},
			expectedInc: [5]decimal.Decimal{
				decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
			},
			expectedPay: [5]decimal.Decimal{
				decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero,
			},
		},
		{
			name: "multiple roles - 5 entries per role one per year",
			headcounts: []model.StaffHeadcount{
				// Category 1 (RnD Engineers)
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 1, FTE: decimal.NewFromInt(1)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 2, FTE: decimal.NewFromInt(1)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 3, FTE: decimal.NewFromInt(1)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 4, FTE: decimal.NewFromInt(1)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 5, FTE: decimal.NewFromInt(1)},
				// Category 2 (Prod Engineers)
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryProdEngineers, YearIndex: 1, FTE: decimal.NewFromInt(2)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryProdEngineers, YearIndex: 2, FTE: decimal.NewFromInt(2)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryProdEngineers, YearIndex: 3, FTE: decimal.NewFromInt(2)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryProdEngineers, YearIndex: 4, FTE: decimal.NewFromInt(2)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryProdEngineers, YearIndex: 5, FTE: decimal.NewFromInt(2)},
			},
			salaries: []model.StaffSalary{
				// Category 1 (RnD Engineers)
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 1, MonthlyGrossSalary: decimal.NewFromInt(5000)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 2, MonthlyGrossSalary: decimal.NewFromInt(5000)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 3, MonthlyGrossSalary: decimal.NewFromInt(5000)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 4, MonthlyGrossSalary: decimal.NewFromInt(5000)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 5, MonthlyGrossSalary: decimal.NewFromInt(5000)},
				// Category 2 (Prod Engineers)
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryProdEngineers, YearIndex: 1, MonthlyGrossSalary: decimal.NewFromInt(3000)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryProdEngineers, YearIndex: 2, MonthlyGrossSalary: decimal.NewFromInt(3000)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryProdEngineers, YearIndex: 3, MonthlyGrossSalary: decimal.NewFromInt(3000)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryProdEngineers, YearIndex: 4, MonthlyGrossSalary: decimal.NewFromInt(3000)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryProdEngineers, YearIndex: 5, MonthlyGrossSalary: decimal.NewFromInt(3000)},
			},
			incentives: []model.StaffIncentive{
				// Incentives are per-year now (not per-role)
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 1, IncentivePct: decimal.NewFromFloat(0.10)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 2, IncentivePct: decimal.NewFromFloat(0.10)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 3, IncentivePct: decimal.NewFromFloat(0.10)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 4, IncentivePct: decimal.NewFromFloat(0.10)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 5, IncentivePct: decimal.NewFromFloat(0.10)},
			},
			config: config,
			// Formula: SubtotalPayroll (k€) = FTE × MonthlyGrossSalary(€) × (1 + EmployerTaxRate) × Months ÷ 1000
			// RnD:  1 × 5000 × 1.42 × 12 = 85200
			// Prod: 2 × 3000 × 1.42 × 12 = 102240
			// SubtotalPayroll = 187440 (constant across all years)
			// IncentiveAmount = 187440 × 0.10 = 18744
			// TotalPayroll = 206184
			expectedSal: [5]decimal.Decimal{
				decimal.NewFromFloat(187440), decimal.NewFromFloat(187440), decimal.NewFromFloat(187440),
				decimal.NewFromFloat(187440), decimal.NewFromFloat(187440),
			},
			expectedInc: [5]decimal.Decimal{
				decimal.NewFromFloat(18744), decimal.NewFromFloat(18744), decimal.NewFromFloat(18744),
				decimal.NewFromFloat(18744), decimal.NewFromFloat(18744),
			},
			expectedPay: [5]decimal.Decimal{
				decimal.NewFromFloat(206184), decimal.NewFromFloat(206184), decimal.NewFromFloat(206184),
				decimal.NewFromFloat(206184), decimal.NewFromFloat(206184),
			},
		},
		{
			name: "incentive cap enforcement",
			headcounts: []model.StaffHeadcount{
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 1, FTE: decimal.NewFromInt(1)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 2, FTE: decimal.NewFromInt(1)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 3, FTE: decimal.NewFromInt(1)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 4, FTE: decimal.NewFromInt(1)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 5, FTE: decimal.NewFromInt(1)},
			},
			salaries: []model.StaffSalary{
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 1, MonthlyGrossSalary: decimal.NewFromInt(10000)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 2, MonthlyGrossSalary: decimal.NewFromInt(10000)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 3, MonthlyGrossSalary: decimal.NewFromInt(10000)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 4, MonthlyGrossSalary: decimal.NewFromInt(10000)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, Category: model.CategoryRnDEngineers, YearIndex: 5, MonthlyGrossSalary: decimal.NewFromInt(10000)},
			},
			incentives: []model.StaffIncentive{
				// Incentive exceeds cap (0.15)
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 1, IncentivePct: decimal.NewFromFloat(0.25)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 2, IncentivePct: decimal.NewFromFloat(0.25)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 3, IncentivePct: decimal.NewFromFloat(0.25)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 4, IncentivePct: decimal.NewFromFloat(0.25)},
				{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID, YearIndex: 5, IncentivePct: decimal.NewFromFloat(0.25)},
			},
			config: config,
			// Formula: SubtotalPayroll (k€) = FTE × MonthlyGrossSalary(€) × (1 + EmployerTaxRate) × Months ÷ 1000
			// SubtotalPayroll = 1 × 10000 × 1.42 × 12 = 170400 (constant)
			// IncentiveAmount = 170400 × 0.15 = 25560 (capped from 0.25)
			// TotalPayroll = 195960
			expectedSal: [5]decimal.Decimal{
				decimal.NewFromFloat(170400), decimal.NewFromFloat(170400), decimal.NewFromFloat(170400),
				decimal.NewFromFloat(170400), decimal.NewFromFloat(170400),
			},
			expectedInc: [5]decimal.Decimal{
				decimal.NewFromFloat(25560), decimal.NewFromFloat(25560), decimal.NewFromFloat(25560),
				decimal.NewFromFloat(25560), decimal.NewFromFloat(25560),
			},
			expectedPay: [5]decimal.Decimal{
				decimal.NewFromFloat(195960), decimal.NewFromFloat(195960), decimal.NewFromFloat(195960),
				decimal.NewFromFloat(195960), decimal.NewFromFloat(195960),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeStaffPayroll(tt.headcounts, tt.salaries, tt.incentives, tt.config)

			for yearIdx := 0; yearIdx < 5; yearIdx++ {
				assertDecEq(t, tt.expectedSal[yearIdx], result.Payroll[yearIdx].SubtotalPayroll,
					"SubtotalPayroll mismatch in year %d", yearIdx+1)
				assertDecEq(t, tt.expectedInc[yearIdx], result.Payroll[yearIdx].IncentiveAmount,
					"IncentiveAmount mismatch in year %d", yearIdx+1)
				assertDecEq(t, tt.expectedPay[yearIdx], result.Payroll[yearIdx].TotalPayroll,
					"TotalPayroll mismatch in year %d", yearIdx+1)
			}
		})
	}
}

// TestIncentiveCapZeroMeansNoCap is a regression test for the semantic bug where
// IncentiveCap = 0 (unset/default) was treated as "cap at 0%", silently zeroing all
// percentage-based incentives.  Zero must mean "no cap configured" — the full
// IncentivePct must be applied.
func TestIncentiveCapZeroMeansNoCap(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	now := time.Now()

	cfg := model.PlanConfig{
		ScenarioID:            scenarioID,
		EmployerTaxRate:       decimal.NewFromFloat(0.42),
		IncentiveCap:          decimal.Zero, // unset — must NOT cap incentives to 0
		SalaryMonthsPerYear:   12,
		FirstFiscalYearMonths: 12,
		ForecastStart:         now,
	}

	headcounts := []model.StaffHeadcount{
		{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID,
			Category: model.CategoryRnDEngineers, YearIndex: 1, FTE: decimal.NewFromInt(1)},
	}
	salaries := []model.StaffSalary{
		{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID,
			Category: model.CategoryRnDEngineers, YearIndex: 1, MonthlyGrossSalary: decimal.NewFromInt(3000)},
	}
	incentives := []model.StaffIncentive{
		{TenantScoped: model.TenantScoped{TenantID: tenantID}, ScenarioID: scenarioID,
			YearIndex: 1, IncentivePct: decimal.NewFromFloat(0.10)},
	}

	result := ComputeStaffPayroll(headcounts, salaries, incentives, cfg)

	// SubtotalPayroll (€) = 1 × 3000 × 1.42 × 12 = 51120
	// With zero cap (= no cap), IncentiveAmount = 51120 × 0.10 = 5112
	expectedSubtotal := decimal.NewFromFloat(51120)
	expectedIncentive := decimal.NewFromFloat(5112)

	assertDecEq(t, expectedSubtotal, result.Payroll[0].SubtotalPayroll,
		"SubtotalPayroll should be 51120 €")
	assertDecEq(t, expectedIncentive, result.Payroll[0].IncentiveAmount,
		"IncentiveAmount must not be zeroed when IncentiveCap is 0 (no cap configured)")
}
