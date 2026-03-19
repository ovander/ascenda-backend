package compute

import (
	"kerplan/internal/model"

	"github.com/shopspring/decimal"
)

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

	// Calculate loan repayment per year (simplified: total loans / term years)
	loanRepaymentPerYear := decimal.Zero
	if config.MLTLoanTermYears > 0 {
		loanRepaymentPerYear = openBal.LoansAndDebt.Div(decimal.NewFromInt(int64(config.MLTLoanTermYears)))
	}

	// Process each year (5 years, 0-indexed)
	for yearIndex := 0; yearIndex < 5; yearIndex++ {

		// ==================== REQUIREMENTS (Uses) ====================
		result.Plan.Requirements.Capex[yearIndex] = capex.Totals.TotalCapex[yearIndex]
		result.Plan.Requirements.Dividends[yearIndex] = getEntry(model.FiplanDividends, yearIndex+1)
		result.Plan.Requirements.NegativeCashFlow[yearIndex] = decimal.Zero // Placeholder
		result.Plan.Requirements.WCRChange[yearIndex] = wcr.Summary.WCRChange[yearIndex]
		result.Plan.Requirements.LoanRepayments[yearIndex] = loanRepaymentPerYear
		result.Plan.Requirements.GrantRepayments[yearIndex] = getEntry(model.FiplanGrantRepayments, yearIndex+1)

		result.Plan.Requirements.Total[yearIndex] = result.Plan.Requirements.Capex[yearIndex].
			Add(result.Plan.Requirements.Dividends[yearIndex]).
			Add(result.Plan.Requirements.NegativeCashFlow[yearIndex]).
			Add(result.Plan.Requirements.WCRChange[yearIndex]).
			Add(result.Plan.Requirements.LoanRepayments[yearIndex]).
			Add(result.Plan.Requirements.GrantRepayments[yearIndex])

		// ==================== RESOURCES (Sources) ====================
		result.Plan.Resources.CapitalIncrease[yearIndex] = getEntry(model.FiplanCapitalIncrease, yearIndex+1)
		result.Plan.Resources.CurrentAccountCont[yearIndex] = getEntry(model.FiplanCurrentAccountContrib, yearIndex+1)

		// Positive cash flow from P&L
		cashFlow := pnl.Years[yearIndex].CashFlow
		if cashFlow.IsPositive() {
			result.Plan.Resources.PositiveCashFlow[yearIndex] = cashFlow
		} else {
			result.Plan.Resources.PositiveCashFlow[yearIndex] = decimal.Zero
		}

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
		result.CashFlow.Financing.NewLoansAndGrants[yearIndex] = getEntry(model.FiplanLTLoans, yearIndex+1).
			Add(getEntry(model.FiplanRepayableGrants, yearIndex+1)).
			Add(getEntry(model.FiplanSubsidies, yearIndex+1)).
			Add(getEntry(model.FiplanOtherGrants, yearIndex+1))
		result.CashFlow.Financing.Dividends[yearIndex] = getEntry(model.FiplanDividends, yearIndex+1)
		result.CashFlow.Financing.LoanGrantRepayments[yearIndex] = loanRepaymentPerYear.
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

	return result
}
