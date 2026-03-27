package compute

import (
	"fmt"

	"github.com/shopspring/decimal"
	"ascenda/internal/model"
)

var monthAbbr = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// ── Annual Charts ─────────────────────────────────────────────────────────────

// ComputeGraphs derives the 5-year annual chart data from a fully computed plan.
func ComputeGraphs(output *model.FullPlanOutput) model.GraphsReport {
	g := model.GraphsReport{}
	g.Years = output.Ratios.Years

	for i, py := range output.PnL.Years {
		// Sales Analysis
		export := py.ExportSalesMemo
		domestic := py.Sales.Sub(export)
		g.SalesAnalysis.ExportSales[i] = export
		g.SalesAnalysis.DomesticSales[i] = domestic
		g.SalesAnalysis.DirectSales[i] = domestic // domestic treated as direct channel
		g.SalesAnalysis.IndirectSales[i] = decimal.Zero

		// Cost Structure
		g.CostStructure.COGS[i] = py.COGS
		g.CostStructure.ExternalExp[i] = py.ExternalExpenses
		g.CostStructure.PayrollExp[i] = py.PayrollExpenses
		g.CostStructure.Depreciation[i] = py.Depreciation
		g.CostStructure.OtherOpex[i] = py.TaxesAndDuties.Add(py.OtherOperatingExp)

		// Revenue / Profit / Cash
		g.RevProfitCash.Revenue[i] = py.Sales
		g.RevProfitCash.NetProfit[i] = py.NetProfit
		g.RevProfitCash.CashFlow[i] = py.CashFlow
	}

	// Cumulative cash from FiPlan balance sheet
	g.RevProfitCash.CumulativeCash = output.FiPlan.Plan.Balance.CumulativeCash

	// Financial Requirements vs Resources
	g.FinRequirements.Requirements = output.FiPlan.Plan.Requirements.Total
	g.FinRequirements.Resources = output.FiPlan.Plan.Resources.Total
	g.FinRequirements.CumulativeCash = output.FiPlan.Plan.Balance.CumulativeCash

	// Balance Sheet Structure (indices 1–5: Year 1–5, skipping opening index 0)
	for i := 0; i < 5; i++ {
		g.BalanceSheet.Equity[i]        = output.BSheet.Condensed.Liabilities.Equity[i+1]
		g.BalanceSheet.LongTermDebt[i]  = output.BSheet.Condensed.Liabilities.LongTermDebt[i+1]
		g.BalanceSheet.ShortTermDebt[i] = output.BSheet.Condensed.Liabilities.ShortTermDebt[i+1]
	}

	// Annual Headcount by Function (5 years)
	for _, cat := range output.Payroll.Headcount.Categories {
		for y := 0; y < 5; y++ {
			if y >= len(cat.FTE) {
				continue
			}
			fte := cat.FTE[y]
			switch model.CategoryFunction[cat.Category] {
			case model.FunctionRnD:
				g.HeadcountAnnual.RnD[y] = g.HeadcountAnnual.RnD[y].Add(fte)
			case model.FunctionProduction:
				g.HeadcountAnnual.Production[y] = g.HeadcountAnnual.Production[y].Add(fte)
			case model.FunctionSales:
				g.HeadcountAnnual.Sales[y] = g.HeadcountAnnual.Sales[y].Add(fte)
			case model.FunctionGnA:
				g.HeadcountAnnual.GnA[y] = g.HeadcountAnnual.GnA[y].Add(fte)
			}
		}
	}
	for y := 0; y < 5; y++ {
		g.HeadcountAnnual.Total[y] = g.HeadcountAnnual.RnD[y].
			Add(g.HeadcountAnnual.Production[y]).
			Add(g.HeadcountAnnual.Sales[y]).
			Add(g.HeadcountAnnual.GnA[y])
	}

	// P&L Cascade (Revenue → Gross Margin → EBITDA → EBIT → Net Profit)
	for i, py := range output.PnL.Years {
		g.PnLCascade.Revenue[i]     = py.Sales
		g.PnLCascade.GrossMargin[i] = py.Sales.Sub(py.COGS)
		g.PnLCascade.EBITDA[i]      = py.EBITDA
		g.PnLCascade.EBIT[i]        = py.EBIT
		g.PnLCascade.NetProfit[i]   = py.NetProfit
	}

	return g
}

// ── Monthly Charts ────────────────────────────────────────────────────────────

// ComputeGraphs2 derives the 36-month chart data from a fully computed plan.
func ComputeGraphs2(output *model.FullPlanOutput) model.Graphs2Report {
	g := model.Graphs2Report{}

	// 36-month labels derived from PnL base year
	startYear := 0
	if len(output.PnL.Years) > 0 {
		startYear = output.PnL.Years[0].Year
	}
	for y := 0; y < 3; y++ {
		yr := startYear + y
		for m := 0; m < 12; m++ {
			g.Months[y*12+m] = fmt.Sprintf("%s %d", monthAbbr[m], yr)
		}
	}

	// ── Cash / Equity / Debt ──────────────────────────────────────────────────
	// BSheet.Equity: index 0 = opening, 1-5 = Year 1–5
	// LTLoans: Ratios.EquityLeverage.LTLoans[5] — one per year
	for y := 0; y < 3; y++ {
		if y >= len(output.Cash.Years) {
			break
		}
		cashYear := output.Cash.Years[y]

		equityAnnual := decimal.Zero
		if y+1 < len(output.BSheet.Equity) {
			equityAnnual = output.BSheet.Equity[y+1]
		}
		debtAnnual := decimal.Zero
		if y < len(output.Ratios.EquityLeverage.LTLoans) {
			debtAnnual = output.Ratios.EquityLeverage.LTLoans[y]
		}

		for m := 0; m < 12; m++ {
			idx := y*12 + m
			g.CashEquityDebt.CashBalance[idx] = cashYear.ClosingBalance[m]
			g.CashEquityDebt.Equity[idx] = equityAnnual
			g.CashEquityDebt.TotalDebt[idx] = debtAnnual
		}
	}

	// ── Operating Cash Flows ──────────────────────────────────────────────────
	for y := 0; y < 3; y++ {
		if y >= len(output.Cash.Years) {
			break
		}
		cashYear := output.Cash.Years[y]
		for m := 0; m < 12; m++ {
			idx := y*12 + m
			g.OperatingCash.Inflows[idx] = cashYear.Revenue.Total[m]
			// Outflows as negative (cash out)
			g.OperatingCash.Outflows[idx] = cashYear.Operating.Total[m].Add(cashYear.Tax.Total[m]).Neg()
			g.OperatingCash.NetCash[idx] = cashYear.NetCashFlow[m]
		}
	}

	// ── Invoicing & EBITDA ────────────────────────────────────────────────────
	cumRevenue := decimal.Zero
	for y := 0; y < 3; y++ {
		if y >= len(output.Cash.Years) {
			break
		}
		cashYear := output.Cash.Years[y]

		// Monthly EBITDA approximated as annual EBITDA ÷ 12
		monthlyEBITDA := decimal.Zero
		if y < len(output.PnL.Years) {
			monthlyEBITDA = output.PnL.Years[y].EBITDA.Div(decimal.NewFromInt(12))
		}

		for m := 0; m < 12; m++ {
			idx := y*12 + m
			rev := cashYear.Revenue.Total[m]
			cumRevenue = cumRevenue.Add(rev)
			g.InvoicingEBITDA.MonthlyRevenue[idx] = rev
			g.InvoicingEBITDA.MonthlyEBITDA[idx] = monthlyEBITDA
			g.InvoicingEBITDA.CumulRevenue[idx] = cumRevenue
		}
	}

	// ── Headcount by Function ─────────────────────────────────────────────────
	// Repeat annual FTE values for each month of the corresponding year (y=0..2)
	for y := 0; y < 3; y++ {
		rnd := decimal.Zero
		production := decimal.Zero
		sales := decimal.Zero
		gna := decimal.Zero

		for _, cat := range output.Payroll.Headcount.Categories {
			if y >= len(cat.FTE) {
				continue
			}
			fte := cat.FTE[y]
			switch model.CategoryFunction[cat.Category] {
			case model.FunctionRnD:
				rnd = rnd.Add(fte)
			case model.FunctionProduction:
				production = production.Add(fte)
			case model.FunctionSales:
				sales = sales.Add(fte)
			case model.FunctionGnA:
				gna = gna.Add(fte)
			}
		}
		total := rnd.Add(production).Add(sales).Add(gna)

		for m := 0; m < 12; m++ {
			idx := y*12 + m
			g.HeadcountByFunc.RnD[idx] = rnd
			g.HeadcountByFunc.Production[idx] = production
			g.HeadcountByFunc.Sales[idx] = sales
			g.HeadcountByFunc.GnA[idx] = gna
			g.HeadcountByFunc.Total[idx] = total
		}
	}

	return g
}
