package compute

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"kerplan/internal/model"
)

func TestComputeOpexSummary(t *testing.T) {
	config := model.PlanConfig{
		DiscountRate:       decimal.NewFromFloat(0.1),
		CorporateTaxRate:   decimal.NewFromFloat(0.25),
		EmployerTaxRate: decimal.NewFromFloat(0.42),
	}

	scenarioID := uuid.New()
	opexPerHire := model.OpexPerHire{
		PropertyRentals:              decimal.NewFromInt(1000),
		PostageTelecom:               decimal.NewFromInt(500),
		SuppliesPurchases:            decimal.NewFromInt(300),
		StudiesDocumentation:         decimal.NewFromInt(200),
		InsuranceCostsPctSales:       ParseDecimal(0.02),
		RoyaltyPaymentsPctSales:      ParseDecimal(0.03),
		TravelTransportation:         decimal.NewFromInt(800),
		MissionRepresentation:        decimal.NewFromInt(400),
		RecruitTrainingPctPayroll:    ParseDecimal(0.05),
	}

	tests := []struct {
		name      string
		entries   []model.OpexManualEntry
		revenue   model.ConsolidatedRevenue
		staff     model.StaffPayrollSummary
		checkOpex func(*testing.T, model.OpexSummary)
	}{
		{
			name: "manual entries only",
			entries: []model.OpexManualEntry{
				{LineID: model.LineLeasingMovable, YearIndex: 0, Amount: decimal.NewFromInt(10000)},
				{LineID: model.LineProfessionalFees, YearIndex: 0, Amount: decimal.NewFromInt(5000)},
				{LineID: model.LineAdvertisingComms, YearIndex: 0, Amount: decimal.NewFromInt(3000)},
				{LineID: model.LineOtherExpenses, YearIndex: 0, Amount: decimal.NewFromInt(2000)},
				{LineID: model.LineLeasingMovable, YearIndex: 1, Amount: decimal.NewFromInt(12000)},
			},
			revenue: model.ConsolidatedRevenue{
				Totals: [5]model.ConsolidatedRevenueYear{
					{Year: 0, TotalTurnover: decimal.NewFromInt(100000)},
					{Year: 1, TotalTurnover: decimal.NewFromInt(120000)},
					{}, {}, {},
				},
			},
			staff: model.StaffPayrollSummary{
				Payroll: [5]model.StaffPayrollYear{
					{TotalPayroll: decimal.NewFromInt(40000), SubtotalPayroll: decimal.NewFromInt(40000)},
					{TotalPayroll: decimal.NewFromInt(48000), SubtotalPayroll: decimal.NewFromInt(48000)},
					{}, {}, {},
				},
			},
			checkOpex: func(t *testing.T, result model.OpexSummary) {
				// Year 0: manual(20000) + auto-computed(8000)
				// Manual: 10000 + 5000 + 3000 + 2000 = 20000
				// Auto-computed (per_capita all 0 due to no headcount):
				//   PropertyRentals: 1000 (special_rent base)
				//   InsuranceCosts: 100000 * 0.02 = 2000
				//   RoyaltyPatents: 100000 * 0.03 = 3000
				//   RecruitTraining: 40000 * 0.05 = 2000
				//   Auto subtotal: 8000
				expectedYear0 := decimal.NewFromInt(28000)
				assertDecEq(t, expectedYear0, result.GrandTotal[0],
					"Year 0 total should sum manual entries (20000) + auto-computed (8000)")

				// Year 1: manual(12000) + auto-computed(9400)
				// Manual: 12000 (leasing only, others carry forward)
				// Auto-computed:
				//   PropertyRentals: 1000
				//   InsuranceCosts: 120000 * 0.02 = 2400
				//   RoyaltyPatents: 120000 * 0.03 = 3600
				//   RecruitTraining: 48000 * 0.05 = 2400
				//   Auto subtotal: 9400
				expectedYear1 := decimal.NewFromInt(21400)
				assertDecEq(t, expectedYear1, result.GrandTotal[1],
					"Year 1 total should sum manual entries (12000) + auto-computed (9400)")

				// Find leasing subcategory and verify it aggregates correctly
				var leasingSubcat *model.OpexSubcategoryResult
				for i := range result.Subcategories {
					if result.Subcategories[i].Subcategory == model.OpexSubLeasing {
						leasingSubcat = &result.Subcategories[i]
						break
					}
				}
				assert.NotNil(t, leasingSubcat, "Leasing subcategory should exist")
				assertDecEq(t, decimal.NewFromInt(10000), leasingSubcat.Subtotal[0],
					"Year 0 Leasing should be 10000")
				assertDecEq(t, decimal.NewFromInt(12000), leasingSubcat.Subtotal[1],
					"Year 1 Leasing should be 12000")
			},
		},
		{
			name:    "auto-computed lines via OpexPerHire",
			entries: []model.OpexManualEntry{},
			revenue: model.ConsolidatedRevenue{
				Totals: [5]model.ConsolidatedRevenueYear{
					{Year: 0, TotalTurnover: decimal.NewFromInt(100000)},
					{Year: 1, TotalTurnover: decimal.NewFromInt(120000)},
					{Year: 2, TotalTurnover: decimal.NewFromInt(144000)},
					{}, {},
				},
			},
			staff: model.StaffPayrollSummary{
				Payroll: [5]model.StaffPayrollYear{
					{TotalPayroll: decimal.NewFromInt(40000), SubtotalPayroll: decimal.NewFromInt(40000)},
					{TotalPayroll: decimal.NewFromInt(48000), SubtotalPayroll: decimal.NewFromInt(48000)},
					{TotalPayroll: decimal.NewFromInt(57600), SubtotalPayroll: decimal.NewFromInt(57600)},
					{}, {},
				},
			},
			checkOpex: func(t *testing.T, result model.OpexSummary) {
				// Year 0: Note that per_capita lines are all 0 (Headcount.TotalStaff is all zeros)
				// PropertyRentals: 1000 (special_rent base)
				// PostageTelecom: 0 (per_capita, no headcount)
				// SuppliesPurchases: 0 (per_capita, no headcount)
				// StudiesDocumentation: 0 (per_capita, no headcount)
				// InsuranceCosts: 100000 * 0.02 = 2000 (pct_of_sales)
				// RoyaltyPatents: 100000 * 0.03 = 3000 (pct_of_sales)
				// TravelTransport: 0 (per_capita, no headcount)
				// MissionRepresentation: 0 (per_capita, no headcount)
				// RecruitTraining: 40000 * 0.05 = 2000 (pct_of_payroll)
				// Expected: 1000 + 0 + 0 + 0 + 2000 + 3000 + 0 + 0 + 2000 = 8000
				expectedYear0 := decimal.NewFromInt(8000)
				assertDecEq(t, expectedYear0, result.GrandTotal[0],
					"Year 0 should sum only special_rent, pct_of_sales, and pct_of_payroll lines")

				// Year 1:
				// PropertyRentals: 1000 (special_rent base)
				// Per_capita lines: 0 (no headcount)
				// InsuranceCosts: 120000 * 0.02 = 2400 (pct_of_sales)
				// RoyaltyPatents: 120000 * 0.03 = 3600 (pct_of_sales)
				// RecruitTraining: 48000 * 0.05 = 2400 (pct_of_payroll)
				// Expected: 1000 + 0 + 2400 + 3600 + 0 + 2400 = 9400
				expectedYear1 := decimal.NewFromInt(9400)
				assertDecEq(t, expectedYear1, result.GrandTotal[1],
					"Year 1 should reflect scaled special_rent, pct_of_sales, and pct_of_payroll")
			},
		},
		{
			name: "subcategory aggregation",
			entries: []model.OpexManualEntry{
				{LineID: model.LineLeasingMovable, YearIndex: 0, Amount: decimal.NewFromInt(5000)},
				{LineID: model.LineLeasingRealEstate, YearIndex: 0, Amount: decimal.NewFromInt(8000)},
				{LineID: model.LineProfessionalFees, YearIndex: 0, Amount: decimal.NewFromInt(3000)},
			},
			revenue: model.ConsolidatedRevenue{
				Totals: [5]model.ConsolidatedRevenueYear{
					{Year: 0, TotalTurnover: decimal.NewFromInt(100000)},
					{}, {}, {}, {},
				},
			},
			staff: model.StaffPayrollSummary{
				Payroll: [5]model.StaffPayrollYear{
					{TotalPayroll: decimal.NewFromInt(40000), SubtotalPayroll: decimal.NewFromInt(40000)},
					{}, {}, {}, {},
				},
			},
			checkOpex: func(t *testing.T, result model.OpexSummary) {
				// Find leasing subcategory (should have 3 lines: movable + real_estate + maintenance)
				var leasingSubcat *model.OpexSubcategoryResult
				for i := range result.Subcategories {
					if result.Subcategories[i].Subcategory == model.OpexSubLeasing {
						leasingSubcat = &result.Subcategories[i]
						break
					}
				}
				assert.NotNil(t, leasingSubcat, "Leasing subcategory should exist")
				assert.Equal(t, 3, len(leasingSubcat.Lines), "Leasing should have 3 lines (movable, real_estate, maintenance)")
				expectedLeasingTotal := decimal.NewFromInt(5000 + 8000)
				assertDecEq(t, expectedLeasingTotal, leasingSubcat.Subtotal[0],
					"Leasing subtotal should be 5000 + 8000")

				// Find professional subcategory (should have 2 lines: professional_fees + external_staff_rnd)
				var profSubcat *model.OpexSubcategoryResult
				for i := range result.Subcategories {
					if result.Subcategories[i].Subcategory == model.OpexSubProfessional {
						profSubcat = &result.Subcategories[i]
						break
					}
				}
				assert.NotNil(t, profSubcat, "Professional subcategory should exist")
				assert.Equal(t, 2, len(profSubcat.Lines), "Professional should have 2 lines (fees + external_staff)")
				assertDecEq(t, decimal.NewFromInt(3000), profSubcat.Subtotal[0],
					"Professional subtotal should be 3000")

				// Grand total: manual(16000) + auto-computed(1000 + 2000 + 3000 + 2000) = 24000
				// Manual: 5000 + 8000 + 3000 = 16000
				// Auto: PropertyRentals(1000) + InsuranceCosts(100000*0.02=2000) + RoyaltyPatents(100000*0.03=3000) + RecruitTraining(40000*0.05=2000) = 8000
				expectedGrandTotal := decimal.NewFromInt(24000)
				assertDecEq(t, expectedGrandTotal, result.GrandTotal[0],
					"Grand total should sum manual (16000) + auto-computed (8000)")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			revenueCopy := tt.revenue
			revenueCopy.ScenarioID = scenarioID
			staffCopy := tt.staff

			result := ComputeOpexSummary(tt.entries, opexPerHire, revenueCopy, staffCopy, config)
			tt.checkOpex(t, result)
		})
	}
}
