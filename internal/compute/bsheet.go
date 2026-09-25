package compute

import (
	"ascenda/internal/model"

	"github.com/shopspring/decimal"
)

// ComputeBSheet computes the complete balance sheet report
func ComputeBSheet(
	pnl model.PnlReport,
	wcr model.WCRReport,
	capex model.CapexSummary,
	fiplan model.FiplanReport,
	openBal model.OpeningBalance,
	config model.PlanConfig,
) model.BSheetReport {
	result := model.BSheetReport{}

	// Initialize arrays
	var detailedAssets model.BSheetDetailedAssets
	var detailedLiabilities model.BSheetDetailedLiabilities
	var condensedAssets model.BSheetCondensedAssets
	var condensedLiabilities model.BSheetCondensedLiabilities
	var analysisData model.BSheetAnalysis
	var capitalEmp model.BSheetCapital
	var wcEmployed, wcInvested model.BSheetWorkingCapital
	var charts model.BSheetCharts

	// ==================== YEAR 0 (OPENING BALANCE) ====================

	// Detailed Assets - Year 0
	detailedAssets.NoncurrentAssets[0] = openBal.NoncurrentAssets
	detailedAssets.Inventory[0] = openBal.Inventories
	detailedAssets.AccountsReceivable[0] = openBal.CustomerReceivables
	detailedAssets.Cash[0] = openBal.CashAndSecurities
	detailedAssets.TotalAssets[0] = openBal.NoncurrentAssets.
		Add(openBal.Inventories).
		Add(openBal.CustomerReceivables).
		Add(openBal.CashAndSecurities)

	// Detailed Liabilities - Year 0
	detailedLiabilities.ShareCapital[0] = openBal.ShareCapital
	detailedLiabilities.NetProfit[0] = decimal.Zero
	detailedLiabilities.RetainedEarnings[0] = openBal.RetainedEarnings
	detailedLiabilities.LongTermDebt[0] = openBal.LoansAndDebt
	detailedLiabilities.TradePayables[0] = openBal.SupplierPayables
	detailedLiabilities.SocialTaxDebts[0] = openBal.SocialAndTaxDebts

	// OtherPayables = balancing plug (assets - other liabilities)
	otherPayables0 := detailedAssets.TotalAssets[0].
		Sub(openBal.ShareCapital).
		Sub(openBal.RetainedEarnings).
		Sub(openBal.LoansAndDebt).
		Sub(openBal.SupplierPayables).
		Sub(openBal.SocialAndTaxDebts)
	// otherPayables0 may be negative when the explicit liabilities already exceed total
	// assets (equity-negative opening balance). The negative plug keeps Assets = Liabilities.
	detailedLiabilities.OtherPayables[0] = otherPayables0
	detailedLiabilities.TotalLiabilities[0] = detailedAssets.TotalAssets[0]

	// Equity - Year 0
	result.Equity[0] = openBal.ShareCapital.Add(openBal.RetainedEarnings)

	// ==================== YEARS 1-5 ====================

	for y := 0; y < 5; y++ {
		yearIdx := y // 0-based index for pnl/wcr/capex arrays

		// --- DETAILED ASSETS ---
		// NoncurrentAssets from capex (index y has the value for year 1-5)
		detailedAssets.NoncurrentAssets[y+1] = capex.Totals.NetAssets[y+1]

		// Inventory from wcr (year y refers to the status at end of year y)
		detailedAssets.Inventory[y+1] = wcr.Inventory.InventoryValue[yearIdx]

		// AccountsReceivable from wcr
		detailedAssets.AccountsReceivable[y+1] = wcr.Customers.TotalCustomers[yearIdx]

		// Cash from fiplan (CumulativeCash at year index)
		detailedAssets.Cash[y+1] = fiplan.Plan.Balance.CumulativeCash[yearIdx]

		// TotalAssets
		detailedAssets.TotalAssets[y+1] = detailedAssets.NoncurrentAssets[y+1].
			Add(detailedAssets.Inventory[y+1]).
			Add(detailedAssets.AccountsReceivable[y+1]).
			Add(detailedAssets.Cash[y+1])

		// --- DETAILED LIABILITIES ---
		// ShareCapital: accumulate from fiplan capital increases
		capitalIncrease := sumFiplanCapitalIncreases(fiplan, yearIdx)
		detailedLiabilities.ShareCapital[y+1] = detailedLiabilities.ShareCapital[y].Add(capitalIncrease)

		// NetProfit: current forecast year's net profit (shown separately on the BS)
		detailedLiabilities.NetProfit[y+1] = pnl.Years[yearIdx].NetProfit

		// RetainedEarnings: accumulate PRIOR years' profits only.
		// Current year profit is shown separately in NetProfit above, so do NOT
		// include it here — that would double-count it in Equity.
		if yearIdx == 0 {
			// First forecast year — no prior forecast profit to roll over
			detailedLiabilities.RetainedEarnings[y+1] = detailedLiabilities.RetainedEarnings[y]
		} else {
			// Roll the previous forecast year's net profit into retained earnings
			detailedLiabilities.RetainedEarnings[y+1] = detailedLiabilities.RetainedEarnings[y].
				Add(pnl.Years[yearIdx-1].NetProfit)
		}

		// LongTermDebt: updated from fiplan
		detailedLiabilities.LongTermDebt[y+1] = computeLongTermDebt(fiplan, detailedLiabilities.LongTermDebt[y], yearIdx)

		// TradePayables from wcr
		detailedLiabilities.TradePayables[y+1] = wcr.Suppliers.TotalSuppliers[yearIdx]

		// SocialTaxDebts from wcr
		detailedLiabilities.SocialTaxDebts[y+1] = wcr.FiscalSocial.TotalFiscalSocial[yearIdx]

		// OtherPayables: balancing plug
		otherPayablesY := detailedAssets.TotalAssets[y+1].
			Sub(detailedLiabilities.ShareCapital[y+1]).
			Sub(detailedLiabilities.RetainedEarnings[y+1]).
			Sub(detailedLiabilities.NetProfit[y+1]).
			Sub(detailedLiabilities.LongTermDebt[y+1]).
			Sub(detailedLiabilities.TradePayables[y+1]).
			Sub(detailedLiabilities.SocialTaxDebts[y+1])
		// otherPayablesY may be negative; the plug is signed to enforce Assets = Liabilities.
		detailedLiabilities.OtherPayables[y+1] = otherPayablesY
		detailedLiabilities.TotalLiabilities[y+1] = detailedAssets.TotalAssets[y+1]

		// --- EQUITY ---
		result.Equity[y+1] = detailedLiabilities.ShareCapital[y+1].
			Add(detailedLiabilities.RetainedEarnings[y+1]).
			Add(detailedLiabilities.NetProfit[y+1])
	}

	// ==================== CONDENSED VIEW ====================
	for y := 0; y < 6; y++ {
		// Condensed Assets
		condensedAssets.NoncurrentAssets[y] = detailedAssets.NoncurrentAssets[y]
		condensedAssets.CurrentAssets[y] = detailedAssets.Inventory[y].
			Add(detailedAssets.AccountsReceivable[y])
		condensedAssets.Cash[y] = detailedAssets.Cash[y]
		condensedAssets.Total[y] = detailedAssets.TotalAssets[y]

		// Condensed Liabilities
		equity := decimal.Zero
		if y < len(result.Equity) {
			equity = result.Equity[y]
		}
		condensedLiabilities.Equity[y] = equity
		condensedLiabilities.LongTermDebt[y] = detailedLiabilities.LongTermDebt[y]

		// ShortTermDebt = current liabilities - equity = total liabilities - long-term debt - equity
		shortTermDebt := detailedLiabilities.TotalLiabilities[y].
			Sub(detailedLiabilities.LongTermDebt[y]).
			Sub(equity)
		if shortTermDebt.IsNegative() {
			shortTermDebt = decimal.Zero
		}
		condensedLiabilities.ShortTermDebt[y] = shortTermDebt
		condensedLiabilities.Total[y] = detailedLiabilities.TotalLiabilities[y]
	}

	// ==================== ANALYSIS VIEW ====================
	for y := 0; y < 6; y++ {
		equity := result.Equity[y]
		ltDebt := detailedLiabilities.LongTermDebt[y]
		nca := detailedAssets.NoncurrentAssets[y]
		currentAssets := detailedAssets.Inventory[y].Add(detailedAssets.AccountsReceivable[y])
		cash := detailedAssets.Cash[y]

		// Sources
		analysisData.Equity[y] = equity
		analysisData.LongTermDebt[y] = ltDebt
		analysisData.PermanentCapital[y] = equity.Add(ltDebt)

		// Short-term debt (current liabilities - current assets with proper adjustment)
		stDebt := detailedLiabilities.TotalLiabilities[y].
			Sub(ltDebt).
			Sub(equity)
		if stDebt.IsNegative() {
			stDebt = decimal.Zero
		}
		analysisData.ShortTermDebt[y] = stDebt

		analysisData.TotalSources[y] = analysisData.PermanentCapital[y].Add(stDebt)

		// Uses
		analysisData.NoncurrentAssets[y] = nca
		analysisData.CurrentAssets[y] = currentAssets
		analysisData.Cash[y] = cash
		analysisData.TotalUses[y] = nca.Add(currentAssets).Add(cash)

		// Working Capital metrics
		permanentCap := analysisData.PermanentCapital[y]
		analysisData.WorkingCapital[y] = permanentCap.Sub(nca)
		analysisData.WCR[y] = currentAssets.Sub(stDebt)

		wcMinus := analysisData.WorkingCapital[y].Sub(analysisData.WCR[y])
		analysisData.WCMinusWCR[y] = wcMinus

		// Net Debt = Total Debt − Cash  (negative = net-cash position; never clamped)
		analysisData.NetDebt[y] = ltDebt.Add(stDebt).Sub(cash)
	}

	// ==================== CAPITAL EMPLOYED & INVESTED ====================
	for y := 0; y < 6; y++ {
		// Capital Employed = Equity + Long-term Debt
		capitalEmp.Employed[y] = result.Equity[y].Add(detailedLiabilities.LongTermDebt[y])

		// Capital Invested = NCA + Working Capital
		permanentCap := analysisData.PermanentCapital[y]
		capitalEmp.Invested[y] = detailedAssets.NoncurrentAssets[y].Add(permanentCap.Sub(detailedAssets.NoncurrentAssets[y]))

		// Working Capital Employed = PermanentCapital - NCA
		wcEmp := permanentCap.Sub(detailedAssets.NoncurrentAssets[y])
		if wcEmp.IsNegative() {
			wcEmp = decimal.Zero
		}
		wcEmployed.Employed[y] = wcEmp

		// Working Capital Invested = CurrentAssets - ShortTermDebt
		stDebt := analysisData.ShortTermDebt[y]
		currentAssets := detailedAssets.Inventory[y].Add(detailedAssets.AccountsReceivable[y])
		wcInvested.Invested[y] = currentAssets.Sub(stDebt)
		if wcInvested.Invested[y].IsNegative() {
			wcInvested.Invested[y] = decimal.Zero
		}
	}

	// ==================== NCA WARNING ====================
	for y := 0; y < 6; y++ {
		result.NCAWarning[y] = detailedAssets.NoncurrentAssets[y].GreaterThan(analysisData.PermanentCapital[y])
	}

	// ==================== CHARTS ====================
	charts.Years = [6]int{0, 1, 2, 3, 4, 5}

	// Asset structure, liability structure, capital percentages
	for y := 0; y < 6; y++ {
		totalAssets := detailedAssets.TotalAssets[y]
		if totalAssets.IsZero() {
			charts.AssetStructure[y] = decimal.Zero
			charts.LiabilityStructure[y] = decimal.Zero
		} else {
			// AssetStructure: NCA as % of total assets
			charts.AssetStructure[y] = SafeDiv(detailedAssets.NoncurrentAssets[y], totalAssets)
			// LiabilityStructure: Equity as % of total liabilities
			charts.LiabilityStructure[y] = SafeDiv(result.Equity[y], detailedLiabilities.TotalLiabilities[y])
		}

		// Capital Employed
		charts.CapitalEmployed[y] = capitalEmp.Employed[y]
		charts.CapitalInvested[y] = capitalEmp.Invested[y]

		// Working Capital metrics
		charts.WCEmployed[y] = wcEmployed.Employed[y]
		charts.WCInvested[y] = wcInvested.Invested[y]
	}

	// ==================== ROUND ALL OUTPUT TO 2 d.p. ====================
	r2 := roundBSheet6 // shorthand

	detailedAssets.NoncurrentAssets = r2(detailedAssets.NoncurrentAssets)
	detailedAssets.Inventory = r2(detailedAssets.Inventory)
	detailedAssets.AccountsReceivable = r2(detailedAssets.AccountsReceivable)
	detailedAssets.Cash = r2(detailedAssets.Cash)
	detailedAssets.TotalAssets = r2(detailedAssets.TotalAssets)

	detailedLiabilities.ShareCapital = r2(detailedLiabilities.ShareCapital)
	detailedLiabilities.NetProfit = r2(detailedLiabilities.NetProfit)
	detailedLiabilities.RetainedEarnings = r2(detailedLiabilities.RetainedEarnings)
	detailedLiabilities.LongTermDebt = r2(detailedLiabilities.LongTermDebt)
	detailedLiabilities.TradePayables = r2(detailedLiabilities.TradePayables)
	detailedLiabilities.SocialTaxDebts = r2(detailedLiabilities.SocialTaxDebts)
	detailedLiabilities.OtherPayables = r2(detailedLiabilities.OtherPayables)
	detailedLiabilities.TotalLiabilities = r2(detailedLiabilities.TotalLiabilities)

	for i := range result.Equity {
		result.Equity[i] = result.Equity[i].Round(2)
	}

	condensedAssets.NoncurrentAssets = r2(condensedAssets.NoncurrentAssets)
	condensedAssets.CurrentAssets = r2(condensedAssets.CurrentAssets)
	condensedAssets.Cash = r2(condensedAssets.Cash)
	condensedAssets.Total = r2(condensedAssets.Total)

	condensedLiabilities.Equity = r2(condensedLiabilities.Equity)
	condensedLiabilities.LongTermDebt = r2(condensedLiabilities.LongTermDebt)
	condensedLiabilities.ShortTermDebt = r2(condensedLiabilities.ShortTermDebt)
	condensedLiabilities.Total = r2(condensedLiabilities.Total)

	analysisData.Equity = r2(analysisData.Equity)
	analysisData.LongTermDebt = r2(analysisData.LongTermDebt)
	analysisData.PermanentCapital = r2(analysisData.PermanentCapital)
	analysisData.ShortTermDebt = r2(analysisData.ShortTermDebt)
	analysisData.TotalSources = r2(analysisData.TotalSources)
	analysisData.NoncurrentAssets = r2(analysisData.NoncurrentAssets)
	analysisData.CurrentAssets = r2(analysisData.CurrentAssets)
	analysisData.Cash = r2(analysisData.Cash)
	analysisData.TotalUses = r2(analysisData.TotalUses)
	analysisData.WorkingCapital = r2(analysisData.WorkingCapital)
	analysisData.WCR = r2(analysisData.WCR)
	analysisData.WCMinusWCR = r2(analysisData.WCMinusWCR)
	analysisData.NetDebt = r2(analysisData.NetDebt)

	capitalEmp.Employed = r2(capitalEmp.Employed)
	capitalEmp.Invested = r2(capitalEmp.Invested)
	wcEmployed.Employed = r2(wcEmployed.Employed)
	wcInvested.Invested = r2(wcInvested.Invested)

	// Round chart fields (computed before rounding block — must be rounded here)
	for i := range charts.AssetStructure {
		charts.AssetStructure[i] = charts.AssetStructure[i].Round(4)
		charts.LiabilityStructure[i] = charts.LiabilityStructure[i].Round(4)
	}
	charts.CapitalEmployed = r2(charts.CapitalEmployed)
	charts.CapitalInvested = r2(charts.CapitalInvested)
	charts.WCEmployed = r2(charts.WCEmployed)
	charts.WCInvested = r2(charts.WCInvested)

	// Assign computed structures to result
	result.Detailed.Assets = detailedAssets
	result.Detailed.Liabilities = detailedLiabilities
	result.Condensed.Assets = condensedAssets
	result.Condensed.Liabilities = condensedLiabilities
	result.Analysis = analysisData
	result.Capital = capitalEmp
	result.WorkingCapital.Employed = wcEmployed.Employed // fix: was result.WorkingCapital = wcEmployed
	result.WorkingCapital.Invested = wcInvested.Invested //      (wcInvested.Invested was discarded)
	result.Charts = charts

	return result
}

// roundBSheet6 rounds every element of a [6]decimal.Decimal array to 2 d.p.
func roundBSheet6(arr [6]decimal.Decimal) [6]decimal.Decimal {
	var out [6]decimal.Decimal
	for i, v := range arr {
		out[i] = v.Round(2)
	}
	return out
}

// sumFiplanCapitalIncreases sums capital increases from fiplan up to yearIdx
func sumFiplanCapitalIncreases(fiplan model.FiplanReport, yearIdx int) decimal.Decimal {
	if yearIdx < 0 || yearIdx >= 5 {
		return decimal.Zero
	}
	return fiplan.Plan.Resources.CapitalIncrease[yearIdx]
}

// computeLongTermDebt updates long-term debt from fiplan loans and repayments
func computeLongTermDebt(fiplan model.FiplanReport, priorDebt decimal.Decimal, yearIdx int) decimal.Decimal {
	if yearIdx < 0 || yearIdx >= 5 {
		return priorDebt
	}

	// New LT loans from fiplan
	newLoans := fiplan.Plan.Resources.LTLoans[yearIdx]

	// Loan repayments from fiplan requirements
	repayments := fiplan.Plan.Requirements.LoanRepayments[yearIdx]

	// Updated debt = prior + new loans - repayments
	updated := priorDebt.Add(newLoans).Sub(repayments)
	if updated.IsNegative() {
		updated = decimal.Zero
	}

	return updated
}
