package compute

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"kerplan/internal/model"
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
		name         string
		headcounts   []model.StaffHeadcount
		salaries     []model.StaffSalary
		incentives   []model.StaffIncentive
		config       model.PlanConfig
		expectedSal  [5]decimal.Decimal
		expectedInc  [5]decimal.Decimal
		expectedPay  [5]decimal.Decimal
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
			// Year 1: FTE=1, MonthlyGrossSalary=3000, SubtotalPayroll=1*3000*1.42=4260, IncentiveAmount=4260*0.10=426, TotalPayroll=4686
			// Year 2: FTE=2, SubtotalPayroll=2*3000*1.42=8520, IncentiveAmount=8520*0.10=852, TotalPayroll=9372
			// Year 3: FTE=2, SubtotalPayroll=2*3500*1.42=9940, IncentiveAmount=9940*0.10=994, TotalPayroll=10934
			// Year 4: FTE=3, SubtotalPayroll=3*3500*1.42=14910, IncentiveAmount=14910*0.12=1789.2, TotalPayroll=16699.2
			// Year 5: FTE=3, SubtotalPayroll=3*4000*1.42=17040, IncentiveAmount=17040*0.15=2556, TotalPayroll=19596
			expectedSal: [5]decimal.Decimal{
				decimal.NewFromFloat(4260), decimal.NewFromFloat(8520), decimal.NewFromFloat(9940),
				decimal.NewFromFloat(14910), decimal.NewFromFloat(17040),
			},
			expectedInc: [5]decimal.Decimal{
				decimal.NewFromFloat(426), decimal.NewFromFloat(852), decimal.NewFromFloat(994),
				decimal.NewFromFloat(1789.2), decimal.NewFromFloat(2556),
			},
			expectedPay: [5]decimal.Decimal{
				decimal.NewFromFloat(4686), decimal.NewFromFloat(9372), decimal.NewFromFloat(10934),
				decimal.NewFromFloat(16699.2), decimal.NewFromFloat(19596),
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
			// Year 1: Total FTE=3
			// Category 1 (RnD): SubtotalPayroll=1*5000*1.42=7100
			// Category 2 (Prod): SubtotalPayroll=2*3000*1.42=8520
			// Total SubtotalPayroll=15620, IncentiveAmount=15620*0.10=1562, TotalPayroll=17182
			expectedSal: [5]decimal.Decimal{
				decimal.NewFromFloat(15620), decimal.NewFromFloat(15620), decimal.NewFromFloat(15620),
				decimal.NewFromFloat(15620), decimal.NewFromFloat(15620),
			},
			expectedInc: [5]decimal.Decimal{
				decimal.NewFromFloat(1562), decimal.NewFromFloat(1562), decimal.NewFromFloat(1562),
				decimal.NewFromFloat(1562), decimal.NewFromFloat(1562),
			},
			expectedPay: [5]decimal.Decimal{
				decimal.NewFromFloat(17182), decimal.NewFromFloat(17182), decimal.NewFromFloat(17182),
				decimal.NewFromFloat(17182), decimal.NewFromFloat(17182),
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
			// Year 1: FTE=1, MonthlyGrossSalary=10000, SubtotalPayroll=1*10000*1.42=14200, IncentiveAmount=14200*0.15=2130 (capped from 0.25), TotalPayroll=16330
			expectedSal: [5]decimal.Decimal{
				decimal.NewFromFloat(14200), decimal.NewFromFloat(14200), decimal.NewFromFloat(14200),
				decimal.NewFromFloat(14200), decimal.NewFromFloat(14200),
			},
			expectedInc: [5]decimal.Decimal{
				decimal.NewFromFloat(2130), decimal.NewFromFloat(2130), decimal.NewFromFloat(2130),
				decimal.NewFromFloat(2130), decimal.NewFromFloat(2130),
			},
			expectedPay: [5]decimal.Decimal{
				decimal.NewFromFloat(16330), decimal.NewFromFloat(16330), decimal.NewFromFloat(16330),
				decimal.NewFromFloat(16330), decimal.NewFromFloat(16330),
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
