package compute

import (
	"testing"

	"ascenda/internal/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func dec(v float64) decimal.Decimal {
	return decimal.NewFromFloat(v)
}

// minimalOutput returns a FullPlanOutput with only the fields consumed by
// ComputeGraphs / ComputeGraphs2. Every unused field is left at its zero value.
func minimalOutput() model.FullPlanOutput {
	out := model.FullPlanOutput{}

	// PnL — 5 annual rows
	out.PnL.Years[0] = model.PnlYear{
		Year:              2025,
		Sales:             dec(1000),
		ExportSalesMemo:   dec(200),
		COGS:              dec(300),
		ExternalExpenses:  dec(100),
		PayrollExpenses:   dec(150),
		Depreciation:      dec(50),
		TaxesAndDuties:    dec(20),
		OtherOperatingExp: dec(30),
		EBITDA:            dec(400),
		NetProfit:         dec(250),
		CashFlow:          dec(300),
	}
	out.PnL.Years[1] = model.PnlYear{
		Year:              2026,
		Sales:             dec(1200),
		ExportSalesMemo:   dec(300),
		COGS:              dec(360),
		ExternalExpenses:  dec(120),
		PayrollExpenses:   dec(180),
		Depreciation:      dec(60),
		TaxesAndDuties:    dec(25),
		OtherOperatingExp: dec(35),
		EBITDA:            dec(480),
		NetProfit:         dec(300),
		CashFlow:          dec(360),
	}
	// Years 2–4 stay at zero — enough to verify indexing does not overflow

	// FiPlan balance / requirements / resources
	for i := 0; i < 5; i++ {
		out.FiPlan.Plan.Balance.CumulativeCash[i] = dec(float64((i + 1) * 100))
		out.FiPlan.Plan.Requirements.Total[i] = dec(float64((i + 1) * 500))
		out.FiPlan.Plan.Resources.Total[i] = dec(float64((i + 1) * 510))
	}

	// Ratios — Years used for GraphsReport.Years
	out.Ratios.Years = [5]int{2025, 2026, 2027, 2028, 2029}
	for i := 0; i < 5; i++ {
		out.Ratios.EquityLeverage.LTLoans[i] = dec(float64((i + 1) * 50))
	}

	// BSheet.Equity — index 0 = opening, 1–5 = Year 1–5
	for i := 0; i < 6; i++ {
		out.BSheet.Equity[i] = dec(float64(i * 200))
	}

	// Cash — 3 years, 12 months each
	for y := 0; y < 3; y++ {
		for m := 0; m < 12; m++ {
			out.Cash.Years[y].Revenue.Total[m] = dec(float64((y*12 + m + 1) * 10))
			out.Cash.Years[y].Operating.Total[m] = dec(float64((y*12 + m + 1) * 4))
			out.Cash.Years[y].Tax.Total[m] = dec(float64((y*12 + m + 1) * 1))
			out.Cash.Years[y].NetCashFlow[m] = dec(float64((y*12 + m + 1) * 5))
			out.Cash.Years[y].ClosingBalance[m] = dec(float64((y*12 + m + 1) * 20))
		}
	}

	// Payroll — two categories: RnDEngineers and SalesTeam
	out.Payroll.Headcount.Categories = []model.StaffCategoryHeadcount{
		{
			Category: model.CategoryRnDEngineers,
			FTE:      [5]decimal.Decimal{dec(3), dec(4), dec(5), dec(6), dec(7)},
		},
		{
			Category: model.CategorySalesTeam,
			FTE:      [5]decimal.Decimal{dec(2), dec(3), dec(4), dec(5), dec(6)},
		},
		{
			Category: model.CategoryAdminManagers,
			FTE:      [5]decimal.Decimal{dec(1), dec(1), dec(2), dec(2), dec(2)},
		},
	}

	return out
}

// ── ComputeGraphs (annual) ────────────────────────────────────────────────────

func TestComputeGraphs_Years(t *testing.T) {
	out := minimalOutput()
	g := ComputeGraphs(&out)

	assert.Equal(t, [5]int{2025, 2026, 2027, 2028, 2029}, g.Years)
}

func TestComputeGraphs_SalesAnalysis(t *testing.T) {
	out := minimalOutput()
	g := ComputeGraphs(&out)

	// Year 0: export = 200, domestic = 1000 - 200 = 800
	assertDecEqual(t, dec(200), g.SalesAnalysis.ExportSales[0], "ExportSales[0]")
	assertDecEqual(t, dec(800), g.SalesAnalysis.DomesticSales[0], "DomesticSales[0]")
	assertDecEqual(t, dec(800), g.SalesAnalysis.DirectSales[0], "DirectSales[0]")
	assertDecEqual(t, decimal.Zero, g.SalesAnalysis.IndirectSales[0], "IndirectSales[0]")

	// Year 1: export = 300, domestic = 1200 - 300 = 900
	assertDecEqual(t, dec(300), g.SalesAnalysis.ExportSales[1], "ExportSales[1]")
	assertDecEqual(t, dec(900), g.SalesAnalysis.DomesticSales[1], "DomesticSales[1]")
}

func TestComputeGraphs_CostStructure(t *testing.T) {
	out := minimalOutput()
	g := ComputeGraphs(&out)

	assertDecEqual(t, dec(300), g.CostStructure.COGS[0], "COGS[0]")
	assertDecEqual(t, dec(100), g.CostStructure.ExternalExp[0], "ExternalExp[0]")
	assertDecEqual(t, dec(150), g.CostStructure.PayrollExp[0], "PayrollExp[0]")
	assertDecEqual(t, dec(50), g.CostStructure.Depreciation[0], "Depreciation[0]")
	// OtherOpex = TaxesAndDuties + OtherOperatingExp = 20 + 30 = 50
	assertDecEqual(t, dec(50), g.CostStructure.OtherOpex[0], "OtherOpex[0]")

	assertDecEqual(t, dec(360), g.CostStructure.COGS[1], "COGS[1]")
	assertDecEqual(t, dec(60), g.CostStructure.OtherOpex[1], "OtherOpex[1]") // 25+35
}

func TestComputeGraphs_RevProfitCash(t *testing.T) {
	out := minimalOutput()
	g := ComputeGraphs(&out)

	assertDecEqual(t, dec(1000), g.RevProfitCash.Revenue[0], "Revenue[0]")
	assertDecEqual(t, dec(250), g.RevProfitCash.NetProfit[0], "NetProfit[0]")
	assertDecEqual(t, dec(300), g.RevProfitCash.CashFlow[0], "CashFlow[0]")

	// CumulativeCash comes from FiPlan.Plan.Balance.CumulativeCash
	for i := 0; i < 5; i++ {
		assertDecEqual(t, dec(float64((i+1)*100)), g.RevProfitCash.CumulativeCash[i],
			"CumulativeCash[%d]", i)
	}
}

func TestComputeGraphs_FinRequirements(t *testing.T) {
	out := minimalOutput()
	g := ComputeGraphs(&out)

	for i := 0; i < 5; i++ {
		assertDecEqual(t, dec(float64((i+1)*500)), g.FinRequirements.Requirements[i],
			"Requirements[%d]", i)
		assertDecEqual(t, dec(float64((i+1)*510)), g.FinRequirements.Resources[i],
			"Resources[%d]", i)
		assertDecEqual(t, dec(float64((i+1)*100)), g.FinRequirements.CumulativeCash[i],
			"FinRequirements.CumulativeCash[%d]", i)
	}
}

func TestComputeGraphs_ZeroYearsAreZero(t *testing.T) {
	// Years 2–4 in minimalOutput have zero Sales, so all derived fields are zero.
	out := minimalOutput()
	g := ComputeGraphs(&out)

	assertDecEqual(t, decimal.Zero, g.SalesAnalysis.ExportSales[2], "ExportSales[2] should be zero")
	assertDecEqual(t, decimal.Zero, g.CostStructure.COGS[4], "COGS[4] should be zero")
}

// ── ComputeGraphs2 (monthly) ──────────────────────────────────────────────────

func TestComputeGraphs2_MonthLabels(t *testing.T) {
	out := minimalOutput()
	g := ComputeGraphs2(&out)

	// First label: Jan of PnL.Years[0].Year = Jan 2025
	assert.Equal(t, "Jan 2025", g.Months[0])
	assert.Equal(t, "Dec 2025", g.Months[11])
	assert.Equal(t, "Jan 2026", g.Months[12])
	assert.Equal(t, "Dec 2026", g.Months[23])
	assert.Equal(t, "Jan 2027", g.Months[24])
	assert.Equal(t, "Dec 2027", g.Months[35])
	assert.Equal(t, 36, len(g.Months))
}

func TestComputeGraphs2_MonthLabels_ZeroYear(t *testing.T) {
	// When PnL.Years is empty (Year = 0) labels should still form without panic
	out := model.FullPlanOutput{}
	g := ComputeGraphs2(&out)
	assert.Equal(t, "Jan 0", g.Months[0])
}

func TestComputeGraphs2_CashEquityDebt(t *testing.T) {
	out := minimalOutput()
	g := ComputeGraphs2(&out)

	// Month 0 (y=0, m=0): ClosingBalance = (0*12+0+1)*20 = 20
	assertDecEqual(t, dec(20), g.CashEquityDebt.CashBalance[0], "CashBalance[0]")
	// Month 11 (y=0, m=11): ClosingBalance = (0*12+11+1)*20 = 240
	assertDecEqual(t, dec(240), g.CashEquityDebt.CashBalance[11], "CashBalance[11]")
	// Month 12 (y=1, m=0): ClosingBalance = (1*12+0+1)*20 = 260
	assertDecEqual(t, dec(260), g.CashEquityDebt.CashBalance[12], "CashBalance[12]")

	// Equity for year 0 = BSheet.Equity[1] = 1*200 = 200 (constant for all 12 months)
	for m := 0; m < 12; m++ {
		assertDecEqual(t, dec(200), g.CashEquityDebt.Equity[m], "Equity[%d]", m)
	}
	// Equity for year 1 = BSheet.Equity[2] = 2*200 = 400
	for m := 12; m < 24; m++ {
		assertDecEqual(t, dec(400), g.CashEquityDebt.Equity[m], "Equity[%d]", m)
	}

	// TotalDebt for year 0 = Ratios.EquityLeverage.LTLoans[0] = 1*50 = 50
	for m := 0; m < 12; m++ {
		assertDecEqual(t, dec(50), g.CashEquityDebt.TotalDebt[m], "TotalDebt[%d]", m)
	}
	// TotalDebt for year 1 = LTLoans[1] = 2*50 = 100
	for m := 12; m < 24; m++ {
		assertDecEqual(t, dec(100), g.CashEquityDebt.TotalDebt[m], "TotalDebt[%d]", m)
	}
}

func TestComputeGraphs2_OperatingCash(t *testing.T) {
	out := minimalOutput()
	g := ComputeGraphs2(&out)

	// Month 0: Revenue.Total[0] = (0+0+1)*10 = 10
	assertDecEqual(t, dec(10), g.OperatingCash.Inflows[0], "Inflows[0]")
	// Outflows = -(Operating.Total + Tax.Total) = -((0+1)*4 + (0+1)*1) = -5
	assertDecEqual(t, dec(-5), g.OperatingCash.Outflows[0], "Outflows[0]")
	// NetCash = NetCashFlow[0] = (0+0+1)*5 = 5
	assertDecEqual(t, dec(5), g.OperatingCash.NetCash[0], "NetCash[0]")

	// Month 13 (y=1, m=1): idx=13, Revenue.Total[1] = (1*12+1+1)*10 = 140
	assertDecEqual(t, dec(140), g.OperatingCash.Inflows[13], "Inflows[13]")
}

func TestComputeGraphs2_InvoicingEBITDA(t *testing.T) {
	out := minimalOutput()
	g := ComputeGraphs2(&out)

	// Month 0: MonthlyRevenue = 10, MonthlyEBITDA = EBITDA[0]/12 = 400/12 ≈ 33.33...
	assertDecEqual(t, dec(10), g.InvoicingEBITDA.MonthlyRevenue[0], "MonthlyRevenue[0]")

	expectedEBITDA := dec(400).Div(decimal.NewFromInt(12))
	assertDecEqual(t, expectedEBITDA, g.InvoicingEBITDA.MonthlyEBITDA[0], "MonthlyEBITDA[0]")

	// CumulRevenue after month 0 = 10
	assertDecEqual(t, dec(10), g.InvoicingEBITDA.CumulRevenue[0], "CumulRevenue[0]")
	// CumulRevenue after month 1: 10 + 20 = 30
	assertDecEqual(t, dec(30), g.InvoicingEBITDA.CumulRevenue[1], "CumulRevenue[1]")
	// CumulRevenue is strictly non-decreasing
	for i := 1; i < 36; i++ {
		assert.True(t,
			g.InvoicingEBITDA.CumulRevenue[i].GreaterThanOrEqual(g.InvoicingEBITDA.CumulRevenue[i-1]),
			"CumulRevenue should be non-decreasing at index %d", i)
	}

	// Year 1 months use PnL.Years[1].EBITDA = 480
	expectedEBITDA1 := dec(480).Div(decimal.NewFromInt(12))
	assertDecEqual(t, expectedEBITDA1, g.InvoicingEBITDA.MonthlyEBITDA[12], "MonthlyEBITDA[12]")
}

func TestComputeGraphs2_HeadcountByFunc(t *testing.T) {
	out := minimalOutput()
	// Categories: RnDEngineers(RnD), SalesTeam(Sales), AdminManagers(GnA)
	g := ComputeGraphs2(&out)

	// Year 0 (months 0–11): RnD=3, Sales=2, GnA=1, Production=0, Total=6
	for m := 0; m < 12; m++ {
		assertDecEqual(t, dec(3), g.HeadcountByFunc.RnD[m], "RnD[%d]", m)
		assertDecEqual(t, dec(2), g.HeadcountByFunc.Sales[m], "Sales[%d]", m)
		assertDecEqual(t, dec(1), g.HeadcountByFunc.GnA[m], "GnA[%d]", m)
		assertDecEqual(t, decimal.Zero, g.HeadcountByFunc.Production[m], "Production[%d]", m)
		assertDecEqual(t, dec(6), g.HeadcountByFunc.Total[m], "Total[%d]", m)
	}

	// Year 1 (months 12–23): RnD=4, Sales=3, GnA=1, Total=8
	for m := 12; m < 24; m++ {
		assertDecEqual(t, dec(4), g.HeadcountByFunc.RnD[m], "RnD[%d]", m)
		assertDecEqual(t, dec(3), g.HeadcountByFunc.Sales[m], "Sales[%d]", m)
		assertDecEqual(t, dec(1), g.HeadcountByFunc.GnA[m], "GnA[%d]", m)
		assertDecEqual(t, dec(8), g.HeadcountByFunc.Total[m], "Total[%d]", m)
	}
}

func TestComputeGraphs2_HeadcountMultipleProductionCategories(t *testing.T) {
	out := model.FullPlanOutput{}
	out.PnL.Years[0] = model.PnlYear{Year: 2025}
	// Two production categories should sum correctly
	out.Payroll.Headcount.Categories = []model.StaffCategoryHeadcount{
		{Category: model.CategoryProdEngineers, FTE: [5]decimal.Decimal{dec(2), dec(2), dec(2), dec(2), dec(2)}},
		{Category: model.CategoryProdTechnicians, FTE: [5]decimal.Decimal{dec(3), dec(3), dec(3), dec(3), dec(3)}},
	}
	// Cash.Years[0] is already zero-valued; the loop will read it for y=0.

	g := ComputeGraphs2(&out)

	// Production = 2+3 = 5 for all 12 months of year 0
	for m := 0; m < 12; m++ {
		assertDecEqual(t, dec(5), g.HeadcountByFunc.Production[m], "Production[%d]", m)
		assertDecEqual(t, dec(5), g.HeadcountByFunc.Total[m], "Total[%d]", m)
	}
}

func TestComputeGraphs2_ZeroCashYears(t *testing.T) {
	// Cash.Years is a [3]CashYear fixed array; zero values must produce
	// all-zero monthly slots without panicking.
	out := model.FullPlanOutput{}
	out.PnL.Years[0] = model.PnlYear{Year: 2025}

	g := ComputeGraphs2(&out)

	for i := 0; i < 36; i++ {
		assertDecEqual(t, decimal.Zero, g.CashEquityDebt.CashBalance[i], "CashBalance[%d]", i)
		assertDecEqual(t, decimal.Zero, g.OperatingCash.Inflows[i], "Inflows[%d]", i)
	}
}

// ── assertion helper ──────────────────────────────────────────────────────────

func assertDecEqual(t *testing.T, want, got decimal.Decimal, msg string, args ...interface{}) {
	t.Helper()
	if !want.Equal(got) {
		t.Errorf(msg+" — want %s, got %s", append(args, want.String(), got.String())...)
	}
}
