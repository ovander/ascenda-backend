package compute

import (
	"ascenda/internal/model"

	"github.com/shopspring/decimal"
)

// ComputeRatios computes financial ratios from P&L, balance sheet, and other reports
// Implements the new RatiosReport model with comprehensive financial metrics.
func ComputeRatios(
	pnl model.PnlReport,
	bsheet model.BSheetReport,
	wcr model.WCRReport,
	fiplan model.FiplanReport,
	revenue model.ConsolidatedRevenue,
	staff model.StaffPayrollSummary,
	capex model.CapexSummary,
	config model.PlanConfig,
) model.RatiosReport {
	result := model.RatiosReport{}

	// Initialize years array (1-5 calendar years)
	for i := 0; i < MaxYears; i++ {
		result.Years[i] = i + 1
	}

	// Scaling factor — all values are now in base €; frontend applies ÷1000 (or other unit).
	result.ScalingFactor = decimal.NewFromInt(1)

	// Process each year (0-4 array indices)
	for y := 0; y < MaxYears; y++ {
		// ────────────────────────────────────────────────────────────
		// SALES & MARGINS
		// ────────────────────────────────────────────────────────────

		// Sales[y] = revenue.Totals[y].TotalTurnover
		result.Sales.Sales[y] = revenue.Totals[y].TotalTurnover

		// GrowthRate[y] = (Sales[y] - Sales[y-1]) / Sales[y-1]
		// For y=0: use config.PriorYearTurnover
		if y == 0 {
			result.Sales.GrowthRate[y] = SafeDiv(
				result.Sales.Sales[y].Sub(config.PriorYearTurnover),
				config.PriorYearTurnover,
			)
		} else {
			result.Sales.GrowthRate[y] = SafeDiv(
				result.Sales.Sales[y].Sub(result.Sales.Sales[y-1]),
				result.Sales.Sales[y-1],
			)
		}

		// ExportSales[y] = revenue.Totals[y].EuropeExportSales
		result.Sales.ExportSales[y] = revenue.Totals[y].EuropeExportSales

		// ExportPct[y] = ExportSales / Sales
		result.Sales.ExportPct[y] = SafeDiv(result.Sales.ExportSales[y], result.Sales.Sales[y])

		// COGS[y] = revenue.Totals[y].TotalCOGS
		result.Sales.COGS[y] = revenue.Totals[y].TotalCOGS

		// COGSPct[y] = COGS / Sales
		result.Sales.COGSPct[y] = SafeDiv(result.Sales.COGS[y], result.Sales.Sales[y])

		// GrossMarginPct[y] = revenue.Totals[y].GrossMarginPct
		result.Sales.GrossMarginPct[y] = revenue.Totals[y].GrossMarginPct

		// ────────────────────────────────────────────────────────────
		// OPERATIONAL METRICS
		// ────────────────────────────────────────────────────────────

		// StaffHeadcount[y] = staff.Headcount.TotalStaff[y]
		result.Operational.StaffHeadcount[y] = staff.Headcount.TotalStaff[y]

		// SalesPerStaff[y] = Sales / StaffHeadcount
		result.Operational.SalesPerStaff[y] = SafeDiv(
			result.Sales.Sales[y],
			result.Operational.StaffHeadcount[y],
		)

		// PayrollExpenses[y] = staff.Payroll[y].TotalPayroll
		result.Operational.PayrollExpenses[y] = staff.Payroll[y].TotalPayroll

		// PayrollPct[y] = PayrollExpenses / Sales
		result.Operational.PayrollPct[y] = SafeDiv(
			result.Operational.PayrollExpenses[y],
			result.Sales.Sales[y],
		)

		// CapitalExpenditure[y] = capex.Totals.TotalCapex[y]
		result.Operational.CapitalExpenditure[y] = capex.Totals.TotalCapex[y]

		// CapexPct[y] = CapitalExpenditure / Sales
		result.Operational.CapexPct[y] = SafeDiv(
			result.Operational.CapitalExpenditure[y],
			result.Sales.Sales[y],
		)

		// Depreciation[y] = capex.Totals.TotalDepreciation[y]
		result.Operational.Depreciation[y] = capex.Totals.TotalDepreciation[y]

		// ExternalExpenses[y] = pnl.Years[y].ExternalExpenses
		result.Operational.ExternalExpenses[y] = pnl.Years[y].ExternalExpenses

		// AdvertisingPromo[y] = approximation from pnl (using OtherOperatingExp as proxy)
		result.Operational.AdvertisingPromo[y] = pnl.Years[y].OtherOperatingExp

		// AdPromoPct[y] = AdvertisingPromo / Sales
		result.Operational.AdPromoPct[y] = SafeDiv(
			result.Operational.AdvertisingPromo[y],
			result.Sales.Sales[y],
		)

		// ────────────────────────────────────────────────────────────
		// PROFITABILITY
		// ────────────────────────────────────────────────────────────

		// AddedValue[y] = pnl.Years[y].AddedValue
		result.Profitability.AddedValue[y] = pnl.Years[y].AddedValue

		// AddedValuePct = AddedValue / Sales
		result.Profitability.AddedValuePct[y] = SafeDiv(
			result.Profitability.AddedValue[y],
			result.Sales.Sales[y],
		)

		// EBITDA[y] = pnl.Years[y].EBITDA
		result.Profitability.EBITDA[y] = pnl.Years[y].EBITDA

		// EBITDAPct = EBITDA / Sales
		result.Profitability.EBITDAPct[y] = SafeDiv(
			result.Profitability.EBITDA[y],
			result.Sales.Sales[y],
		)

		// NetProfit[y] = pnl.Years[y].NetProfit
		result.Profitability.NetProfit[y] = pnl.Years[y].NetProfit

		// NetProfitPct = NetProfit / Sales
		result.Profitability.NetProfitPct[y] = SafeDiv(
			result.Profitability.NetProfit[y],
			result.Sales.Sales[y],
		)

		// CashFlow[y] = pnl.Years[y].CashFlow
		result.Profitability.CashFlow[y] = pnl.Years[y].CashFlow

		// CashFlowPct = CashFlow / Sales
		result.Profitability.CashFlowPct[y] = SafeDiv(
			result.Profitability.CashFlow[y],
			result.Sales.Sales[y],
		)

		// CashAtEOY[y] = fiplan.Balance.CumulativeCash[y]
		result.Profitability.CashAtEOY[y] = fiplan.Plan.Balance.CumulativeCash[y]

		// ────────────────────────────────────────────────────────────
		// EQUITY & LEVERAGE
		// ────────────────────────────────────────────────────────────

		// CapitalIncrease[y] = fiplan.Plan.Resources.CapitalIncrease[y]
		result.EquityLeverage.CapitalIncrease[y] = fiplan.Plan.Resources.CapitalIncrease[y]

		// TotalEquityEOY[y] = bsheet.Equity[y+1]
		result.EquityLeverage.TotalEquityEOY[y] = bsheet.Equity[y+1]

		// NetProfitMinusCap[y] = NetProfit - CapitalIncrease
		result.EquityLeverage.NetProfitMinusCap[y] = result.Profitability.NetProfit[y].Sub(
			result.EquityLeverage.CapitalIncrease[y],
		)

		// FinancialReturn[y] = NetProfit / TotalEquityEOY
		result.EquityLeverage.FinancialReturn[y] = SafeDiv(
			result.Profitability.NetProfit[y],
			result.EquityLeverage.TotalEquityEOY[y],
		)

		// TotalAssets[y] = bsheet.Detailed.Assets.TotalAssets[y+1]
		// Stored explicitly so EquityToAssets is auditable.
		result.EquityLeverage.TotalAssets[y] = bsheet.Detailed.Assets.TotalAssets[y+1]

		// EquityToAssets[y] = TotalEquityEOY / TotalAssets
		result.EquityLeverage.EquityToAssets[y] = SafeDiv(
			result.EquityLeverage.TotalEquityEOY[y],
			result.EquityLeverage.TotalAssets[y],
		)

		// LTLoans[y] = bsheet.Detailed.Liabilities.LongTermDebt[y+1]
		result.EquityLeverage.LTLoans[y] = bsheet.Detailed.Liabilities.LongTermDebt[y+1]

		// LTLoansToEquity = LTLoans / Equity
		result.EquityLeverage.LTLoansToEquity[y] = SafeDiv(
			result.EquityLeverage.LTLoans[y],
			result.EquityLeverage.TotalEquityEOY[y],
		)

		// CashFlowToLoans = CashFlow / LTLoans
		result.EquityLeverage.CashFlowToLoans[y] = SafeDiv(
			result.Profitability.CashFlow[y],
			result.EquityLeverage.LTLoans[y],
		)

		// FinExpToEBITDA = pnl.Years[y].FinancialExpenses / EBITDA
		result.EquityLeverage.FinExpToEBITDA[y] = SafeDiv(
			pnl.Years[y].FinancialExpenses,
			result.Profitability.EBITDA[y],
		)

		// WCR[y] = bsheet.Analysis.WCR[y+1]  (working capital requirement in currency)
		// Stored explicitly so WCRRotationDays is auditable.
		result.EquityLeverage.WCR[y] = bsheet.Analysis.WCR[y+1]

		// WCRRotationDays = wcr.Summary.BasicWCRDays[y]
		result.EquityLeverage.WCRRotationDays[y] = wcr.Summary.BasicWCRDays[y]
	}

	// ────────────────────────────────────────────────────────────
	// FREE CASH FLOW  (computed after per-year loop; needs all years)
	// FCF[y] = CashFlow[y] − CapEx[y] − ΔWCR[y]
	// ΔWCR[y] = WCR[y] − WCR[y−1]  (y=0 uses opening-balance WCR from bsheet)
	// ────────────────────────────────────────────────────────────
	for y := 0; y < MaxYears; y++ {
		prevWCR := bsheet.Analysis.WCR[y] // index 0 = opening period, y = prior year-end
		deltaWCR := result.EquityLeverage.WCR[y].Sub(prevWCR)

		result.Profitability.FreeCashFlow[y] = result.Profitability.CashFlow[y].
			Sub(result.Operational.CapitalExpenditure[y]).
			Sub(deltaWCR)

		// FreeCashFlowPct[y] = FreeCashFlow / Sales
		result.Profitability.FreeCashFlowPct[y] = SafeDiv(
			result.Profitability.FreeCashFlow[y],
			result.Sales.Sales[y],
		)
	}

	// ────────────────────────────────────────────────────────────
	// INITIAL EQUITY (scalar, not per-year)
	// ────────────────────────────────────────────────────────────
	// InitialEquity = bsheet.Equity[0]
	result.EquityLeverage.InitialEquity = bsheet.Equity[0]

	// ────────────────────────────────────────────────────────────
	// VALUATION METRICS
	// ────────────────────────────────────────────────────────────

	// DiscountRate = config.DiscountRate
	result.Valuation.DiscountRate = config.DiscountRate

	// Use Free Cash Flow (= CashFlow − CapEx − ΔWCR) for NPV and IRR.
	cashFlows := [MaxYears]decimal.Decimal{}
	for y := 0; y < MaxYears; y++ {
		cashFlows[y] = result.Profitability.FreeCashFlow[y]
	}

	// NPV = Σ FCF[y] / (1+r)^(y+1)  for y = 0..4
	result.Valuation.NPV = ComputeNPV(cashFlows, config.DiscountRate)

	// IRR: initial investment = equity injected at t=0 (opening equity + Y1 capital increase).
	// Falls back to Y1 capex when no equity data is available.
	initialInvestment := bsheet.Equity[0].Add(fiplan.Plan.Resources.CapitalIncrease[0])
	if initialInvestment.IsZero() {
		initialInvestment = capex.Totals.TotalCapex[0]
	}
	if initialInvestment.IsZero() {
		initialInvestment = decimal.NewFromInt(1) // prevent divide-by-zero in Newton step
	}
	result.Valuation.IRR, result.Valuation.IRRValid = ComputeIRR(cashFlows, initialInvestment)

	// Terminal Value = FCF[4] / tvRate  (Gordon perpetuity, no long-term growth)
	// tvRate = max(discountRate, 5%) to avoid unreasonably large terminal values
	// when the analyst sets a near-zero discount rate.
	tvRate := MaxDecimal(config.DiscountRate, decimal.NewFromFloat(0.05))
	tv := SafeDiv(result.Profitability.FreeCashFlow[MaxYears-1], tvRate)
	// Discount TV back to today: TV / (1+discountRate)^5
	// Use max(discountRate, 0) — no negative discounting.
	tvDiscountBase := decimal.NewFromInt(1).Add(MaxDecimal(config.DiscountRate, decimal.Zero))
	tvDiscountFactor := SafeDiv(
		decimal.NewFromInt(1),
		tvDiscountBase.Pow(decimal.NewFromInt(MaxYears)),
	)
	result.Valuation.TerminalValue = tv.Mul(tvDiscountFactor)

	// DiscountedValue = NPV + TerminalValue  (enterprise value estimate)
	result.Valuation.DiscountedValue = result.Valuation.NPV.Add(result.Valuation.TerminalValue)

	// PEMultiple = DiscountedValue / NetProfit[last year] — DCF-implied earnings multiple.
	result.Valuation.PEMultiple = SafeDiv(
		result.Valuation.DiscountedValue,
		result.Profitability.NetProfit[MaxYears-1],
	)

	// ────────────────────────────────────────────────────────────
	// CHARTS
	// ────────────────────────────────────────────────────────────
	// Set to nil for now (interface{} fields)
	result.Charts.Summary = nil
	result.Charts.Waterfall = nil
	result.Charts.RevenueByProduct = nil
	result.Charts.CashFlowComponents = nil
	result.Charts.ProfitabilityChart = nil

	return result
}
