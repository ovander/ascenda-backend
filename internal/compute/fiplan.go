package compute

import (
	"ascenda/internal/model"

	"github.com/shopspring/decimal"
)

// round2Array rounds every element of a [5]decimal.Decimal to 2 decimal places.
// Applied to all output arrays so that repeating-decimal intermediates (e.g. WCR
// values derived from day-count / 365 divisions) never surface in API responses.
func round2Array(arr [5]decimal.Decimal) [5]decimal.Decimal {
	var out [5]decimal.Decimal
	for i, v := range arr {
		out[i] = v.Round(2)
	}
	return out
}

// ComputeFiplan computes the financial planning report including requirements, resources, balance, and cash flow
func ComputeFiplan(
	entries []model.FiplanEntry,
	capex model.CapexSummary,
	pnl model.PnlReport,
	wcr model.WCRReport,
	openBal model.OpeningBalance,
	config model.PlanConfig,
) model.FiplanReport {
	result := model.FiplanReport{}

	// Build lookup map: lineID/yearIndex -> amount
	entryMap := make(map[model.FiplanLineID]map[int]decimal.Decimal)
	for _, entry := range entries {
		if _, ok := entryMap[entry.LineID]; !ok {
			entryMap[entry.LineID] = make(map[int]decimal.Decimal)
		}
		entryMap[entry.LineID][entry.YearIndex] = entry.Amount
	}

	// Helper to get value from map or use zero
	getEntry := func(lineID model.FiplanLineID, yearIndex int) decimal.Decimal {
		if yearVals, ok := entryMap[lineID]; ok {
			if val, ok := yearVals[yearIndex]; ok {
				return val
			}
		}
		return decimal.Zero
	}

	// loanTerm is the number of years over which any tranche of MLT debt is repaid.
	loanTerm := int64(config.MLTLoanTermYears)
	if loanTerm <= 0 {
		loanTerm = 5 // safe fallback
	}
	termDec := decimal.NewFromInt(loanTerm)

	// repaymentSchedule[y] accumulates the total annual principal repayment due in
	// year y, summing across all debt tranches (opening balance + new draws).
	//
	// Opening-balance debt: repaid equally over loanTerm years starting in Year 0.
	//   Opening repayment each year = LoansAndDebt / loanTerm
	// New LT loans drawn in year d: repaid equally over loanTerm years starting
	// in year d+1 (drawn mid-year convention: interest charged immediately, but
	// first principal instalment is the following year).
	//   New draw in year d → repayment[d+1 .. d+loanTerm] += draw / loanTerm
	repaymentSchedule := [5]decimal.Decimal{}

	openRepayment := openBal.LoansAndDebt.Div(termDec)
	for y := 0; y < 5 && int64(y) < loanTerm; y++ {
		repaymentSchedule[y] = repaymentSchedule[y].Add(openRepayment)
	}

	// Pre-scan new loan draws so we can build the full repayment schedule before
	// the main loop (needed because draw[y] affects repayments at y+1..y+loanTerm).
	for y := 0; y < 5; y++ {
		draw := getEntry(model.FiplanLTLoans, y+1)
		if draw.IsZero() {
			continue
		}
		installment := draw.Div(termDec)
		for r := y + 1; r < 5 && int64(r-y) <= loanTerm; r++ {
			repaymentSchedule[r] = repaymentSchedule[r].Add(installment)
		}
	}

	// Loan interest tracking: outstanding balance accrues interest at
	// MLTInterestRate applied to the start-of-year balance.
	loanBalance := openBal.LoansAndDebt

	// Process each year (5 years, 0-indexed)
	for yearIndex := 0; yearIndex < 5; yearIndex++ {
		// Interest on outstanding balance at the start of this year
		result.LoanInterest[yearIndex] = loanBalance.Mul(config.MLTInterestRate).Round(2)
		// New LT loans drawn this year increase the balance for subsequent years
		newLoans := getEntry(model.FiplanLTLoans, yearIndex+1)
		// Advance balance: add new loans drawn, subtract principal repayment for this year
		loanBalance = loanBalance.Add(newLoans).Sub(repaymentSchedule[yearIndex])
		if loanBalance.IsNegative() {
			loanBalance = decimal.Zero
		}

		// ==================== REQUIREMENTS (Uses) ====================
		result.Plan.Requirements.Capex[yearIndex] = capex.Totals.TotalCapex[yearIndex]
		result.Plan.Requirements.Dividends[yearIndex] = getEntry(model.FiplanDividends, yearIndex+1)
		result.Plan.Requirements.WCRChange[yearIndex] = wcr.Summary.WCRChange[yearIndex]
		result.Plan.Requirements.LoanRepayments[yearIndex] = repaymentSchedule[yearIndex]
		result.Plan.Requirements.GrantRepayments[yearIndex] = getEntry(model.FiplanGrantRepayments, yearIndex+1)

		// CAF (operating cash generation before WCR): split into requirement when
		// negative (loss-making operations burn cash) or resource when positive.
		cashFlow := pnl.Years[yearIndex].CashFlow
		if cashFlow.IsNegative() {
			result.Plan.Requirements.NegativeCashFlow[yearIndex] = cashFlow.Abs()
			result.Plan.Resources.PositiveCashFlow[yearIndex] = decimal.Zero
		} else {
			result.Plan.Requirements.NegativeCashFlow[yearIndex] = decimal.Zero
			result.Plan.Resources.PositiveCashFlow[yearIndex] = cashFlow
		}

		result.Plan.Requirements.Total[yearIndex] = result.Plan.Requirements.Capex[yearIndex].
			Add(result.Plan.Requirements.Dividends[yearIndex]).
			Add(result.Plan.Requirements.NegativeCashFlow[yearIndex]).
			Add(result.Plan.Requirements.WCRChange[yearIndex]).
			Add(result.Plan.Requirements.LoanRepayments[yearIndex]).
			Add(result.Plan.Requirements.GrantRepayments[yearIndex])

		// ==================== RESOURCES (Sources) ====================
		result.Plan.Resources.CapitalIncrease[yearIndex] = getEntry(model.FiplanCapitalIncrease, yearIndex+1)
		result.Plan.Resources.CurrentAccountCont[yearIndex] = getEntry(model.FiplanCurrentAccountContrib, yearIndex+1)

		result.Plan.Resources.Subsidies[yearIndex] = getEntry(model.FiplanSubsidies, yearIndex+1)
		result.Plan.Resources.OtherGrants[yearIndex] = getEntry(model.FiplanOtherGrants, yearIndex+1)
		result.Plan.Resources.RepayableGrants[yearIndex] = getEntry(model.FiplanRepayableGrants, yearIndex+1)
		result.Plan.Resources.LTLoans[yearIndex] = getEntry(model.FiplanLTLoans, yearIndex+1)
		result.Plan.Resources.AssetSales[yearIndex] = getEntry(model.FiplanAssetSales, yearIndex+1)

		result.Plan.Resources.Total[yearIndex] = result.Plan.Resources.CapitalIncrease[yearIndex].
			Add(result.Plan.Resources.CurrentAccountCont[yearIndex]).
			Add(result.Plan.Resources.PositiveCashFlow[yearIndex]).
			Add(result.Plan.Resources.Subsidies[yearIndex]).
			Add(result.Plan.Resources.OtherGrants[yearIndex]).
			Add(result.Plan.Resources.RepayableGrants[yearIndex]).
			Add(result.Plan.Resources.LTLoans[yearIndex]).
			Add(result.Plan.Resources.AssetSales[yearIndex])

		// ==================== BALANCE ====================
		result.Plan.Balance.AnnualBalance[yearIndex] = result.Plan.Resources.Total[yearIndex].
			Sub(result.Plan.Requirements.Total[yearIndex])

		// Cumulative cash (running sum from initial cash)
		if yearIndex == 0 {
			result.Plan.Balance.CumulativeCash[yearIndex] = openBal.CashAndSecurities.Add(result.Plan.Balance.AnnualBalance[yearIndex])
		} else {
			result.Plan.Balance.CumulativeCash[yearIndex] = result.Plan.Balance.CumulativeCash[yearIndex-1].
				Add(result.Plan.Balance.AnnualBalance[yearIndex])
		}

		// Warning if cumulative cash is negative
		result.Warning[yearIndex] = result.Plan.Balance.CumulativeCash[yearIndex].IsNegative()

		// B-sheet cash check (same as cumulative cash for now)
		result.Plan.Balance.BSheetCashCheck[yearIndex] = result.Plan.Balance.CumulativeCash[yearIndex]
	}

	result.Plan.Balance.InitialCash = openBal.CashAndSecurities

	// ==================== CASH FLOW STATEMENT ====================
	for yearIndex := 0; yearIndex < 5; yearIndex++ {
		// Operating Cash Flow
		result.CashFlow.Operating.NetProfit[yearIndex] = pnl.Years[yearIndex].NetProfit
		result.CashFlow.Operating.Depreciation[yearIndex] = capex.Totals.TotalDepreciation[yearIndex]
		result.CashFlow.Operating.DisposalGainLoss[yearIndex] = getEntry(model.FiplanDisposalGainsLosses, yearIndex+1)

		result.CashFlow.Operating.CashFlowCAF[yearIndex] = result.CashFlow.Operating.NetProfit[yearIndex].
			Add(result.CashFlow.Operating.Depreciation[yearIndex]).
			Sub(result.CashFlow.Operating.DisposalGainLoss[yearIndex])

		result.CashFlow.Operating.WCRChange[yearIndex] = wcr.Summary.WCRChange[yearIndex]

		result.CashFlow.Operating.OperatingFlows[yearIndex] = result.CashFlow.Operating.CashFlowCAF[yearIndex].
			Sub(result.CashFlow.Operating.WCRChange[yearIndex])

		// Investing Cash Flow
		result.CashFlow.Investing.CapexOutflow[yearIndex] = capex.Totals.TotalCapex[yearIndex]
		result.CashFlow.Investing.AssetDisposals[yearIndex] = getEntry(model.FiplanAssetSales, yearIndex+1)

		result.CashFlow.Investing.InvestmentFlows[yearIndex] = result.CashFlow.Investing.AssetDisposals[yearIndex].
			Sub(result.CashFlow.Investing.CapexOutflow[yearIndex])

		// Financing Cash Flow
		result.CashFlow.Financing.CapitalIncrease[yearIndex] = getEntry(model.FiplanCapitalIncrease, yearIndex+1)
		result.CashFlow.Financing.CurrentAccountCont[yearIndex] = getEntry(model.FiplanCurrentAccountContrib, yearIndex+1)
		// NewLoansAndGrants contains only REPAYABLE financing flows:
		//   - LT loans (debt)
		//   - Repayable grants (quasi-debt)
		// Non-repayable subsidies and other grants are recognised as P&L operating
		// income (GrantsOtherRevenue), so they already flow through the operating
		// section via NetProfit.  Including them here would double-count them.
		result.CashFlow.Financing.NewLoansAndGrants[yearIndex] = getEntry(model.FiplanLTLoans, yearIndex+1).
			Add(getEntry(model.FiplanRepayableGrants, yearIndex+1))
		result.CashFlow.Financing.Dividends[yearIndex] = getEntry(model.FiplanDividends, yearIndex+1)
		result.CashFlow.Financing.LoanGrantRepayments[yearIndex] = repaymentSchedule[yearIndex].
			Add(getEntry(model.FiplanGrantRepayments, yearIndex+1))

		result.CashFlow.Financing.FinancingFlows[yearIndex] = result.CashFlow.Financing.CapitalIncrease[yearIndex].
			Add(result.CashFlow.Financing.CurrentAccountCont[yearIndex]).
			Add(result.CashFlow.Financing.NewLoansAndGrants[yearIndex]).
			Sub(result.CashFlow.Financing.Dividends[yearIndex]).
			Sub(result.CashFlow.Financing.LoanGrantRepayments[yearIndex])

		// Cash Flow Summary
		result.CashFlow.Summary.ChangeInCash[yearIndex] = result.CashFlow.Operating.OperatingFlows[yearIndex].
			Add(result.CashFlow.Investing.InvestmentFlows[yearIndex]).
			Add(result.CashFlow.Financing.FinancingFlows[yearIndex])

		// Cumulative cash in summary
		if yearIndex == 0 {
			result.CashFlow.Summary.CumulativeCash[yearIndex] = openBal.CashAndSecurities.Add(result.CashFlow.Summary.ChangeInCash[yearIndex])
		} else {
			result.CashFlow.Summary.CumulativeCash[yearIndex] = result.CashFlow.Summary.CumulativeCash[yearIndex-1].
				Add(result.CashFlow.Summary.ChangeInCash[yearIndex])
		}
	}

	result.CashFlow.Summary.InitialCash = openBal.CashAndSecurities

	// Round all output arrays to 2 decimal places.
	// This prevents repeating-decimal intermediates (e.g. WCR ÷ 365 day counts)
	// from leaking into API responses and causing spurious precision mismatches.
	result.Plan.Requirements.Capex = round2Array(result.Plan.Requirements.Capex)
	result.Plan.Requirements.Dividends = round2Array(result.Plan.Requirements.Dividends)
	result.Plan.Requirements.NegativeCashFlow = round2Array(result.Plan.Requirements.NegativeCashFlow)
	result.Plan.Requirements.WCRChange = round2Array(result.Plan.Requirements.WCRChange)
	result.Plan.Requirements.LoanRepayments = round2Array(result.Plan.Requirements.LoanRepayments)
	result.Plan.Requirements.GrantRepayments = round2Array(result.Plan.Requirements.GrantRepayments)
	result.Plan.Requirements.Total = round2Array(result.Plan.Requirements.Total)

	result.Plan.Resources.CapitalIncrease = round2Array(result.Plan.Resources.CapitalIncrease)
	result.Plan.Resources.CurrentAccountCont = round2Array(result.Plan.Resources.CurrentAccountCont)
	result.Plan.Resources.PositiveCashFlow = round2Array(result.Plan.Resources.PositiveCashFlow)
	result.Plan.Resources.Subsidies = round2Array(result.Plan.Resources.Subsidies)
	result.Plan.Resources.OtherGrants = round2Array(result.Plan.Resources.OtherGrants)
	result.Plan.Resources.RepayableGrants = round2Array(result.Plan.Resources.RepayableGrants)
	result.Plan.Resources.LTLoans = round2Array(result.Plan.Resources.LTLoans)
	result.Plan.Resources.AssetSales = round2Array(result.Plan.Resources.AssetSales)
	result.Plan.Resources.Total = round2Array(result.Plan.Resources.Total)

	result.Plan.Balance.AnnualBalance = round2Array(result.Plan.Balance.AnnualBalance)
	result.Plan.Balance.CumulativeCash = round2Array(result.Plan.Balance.CumulativeCash)
	result.Plan.Balance.BSheetCashCheck = round2Array(result.Plan.Balance.BSheetCashCheck)

	result.CashFlow.Operating.NetProfit = round2Array(result.CashFlow.Operating.NetProfit)
	result.CashFlow.Operating.Depreciation = round2Array(result.CashFlow.Operating.Depreciation)
	result.CashFlow.Operating.DisposalGainLoss = round2Array(result.CashFlow.Operating.DisposalGainLoss)
	result.CashFlow.Operating.CashFlowCAF = round2Array(result.CashFlow.Operating.CashFlowCAF)
	result.CashFlow.Operating.WCRChange = round2Array(result.CashFlow.Operating.WCRChange)
	result.CashFlow.Operating.OperatingFlows = round2Array(result.CashFlow.Operating.OperatingFlows)

	result.CashFlow.Investing.CapexOutflow = round2Array(result.CashFlow.Investing.CapexOutflow)
	result.CashFlow.Investing.AssetDisposals = round2Array(result.CashFlow.Investing.AssetDisposals)
	result.CashFlow.Investing.InvestmentFlows = round2Array(result.CashFlow.Investing.InvestmentFlows)

	result.CashFlow.Financing.CapitalIncrease = round2Array(result.CashFlow.Financing.CapitalIncrease)
	result.CashFlow.Financing.CurrentAccountCont = round2Array(result.CashFlow.Financing.CurrentAccountCont)
	result.CashFlow.Financing.NewLoansAndGrants = round2Array(result.CashFlow.Financing.NewLoansAndGrants)
	result.CashFlow.Financing.Dividends = round2Array(result.CashFlow.Financing.Dividends)
	result.CashFlow.Financing.LoanGrantRepayments = round2Array(result.CashFlow.Financing.LoanGrantRepayments)
	result.CashFlow.Financing.FinancingFlows = round2Array(result.CashFlow.Financing.FinancingFlows)

	result.CashFlow.Summary.ChangeInCash = round2Array(result.CashFlow.Summary.ChangeInCash)
	result.CashFlow.Summary.CumulativeCash = round2Array(result.CashFlow.Summary.CumulativeCash)

	return result
}
