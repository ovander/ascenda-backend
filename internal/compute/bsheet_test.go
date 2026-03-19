package compute

import (
	"testing"

	"github.com/shopspring/decimal"
	"kerplan/internal/model"
)

func TestComputeBSheet(t *testing.T) {
	config := model.PlanConfig{
		DiscountRate:     decimal.NewFromFloat(0.1),
		CorporateTaxRate: decimal.NewFromFloat(0.25),
		EmployerTaxRate:  decimal.NewFromFloat(0.42),
	}

	tests := []struct {
		name       string
		pnl        model.PnlReport
		wcr        model.WCRReport
		capex      model.CapexSummary
		fiplan     model.FiplanReport
		openBal    model.OpeningBalance
		checkAssets func(*testing.T, model.BSheetReport)
		checkLiab  func(*testing.T, model.BSheetReport)
	}{
		{
			name: "basic balance sheet with simple data",
			pnl: model.PnlReport{
				Years: [5]model.PnlYear{
					{Year: 2025, YearIndex: 0, NetProfit: decimal.NewFromInt(10000)},
					{Year: 2026, YearIndex: 1, NetProfit: decimal.NewFromInt(15000)},
					{Year: 2027, YearIndex: 2, NetProfit: decimal.NewFromInt(20000)},
					{Year: 2028, YearIndex: 3, NetProfit: decimal.NewFromInt(25000)},
					{Year: 2029, YearIndex: 4, NetProfit: decimal.NewFromInt(30000)},
				},
			},
			wcr: model.WCRReport{
				Summary: model.WCRSummary{
					BasicWCR: [5]decimal.Decimal{
						decimal.NewFromInt(5000),
						decimal.NewFromInt(6000),
						decimal.NewFromInt(7000),
						decimal.NewFromInt(8000),
						decimal.NewFromInt(9000),
					},
				},
			},
			capex: model.CapexSummary{
				Totals: model.CapexTotals{
					NetAssets: [6]decimal.Decimal{
						decimal.Zero,
						decimal.NewFromInt(20000),
						decimal.NewFromInt(35000),
						decimal.NewFromInt(48000),
						decimal.NewFromInt(60000),
						decimal.NewFromInt(70000),
					},
				},
			},
			fiplan: model.FiplanReport{
				Plan: model.FiplanPlan{
					Balance: model.FiplanBalance{
						CumulativeCash: [5]decimal.Decimal{
							decimal.NewFromInt(50000),
							decimal.NewFromInt(65000),
							decimal.NewFromInt(85000),
							decimal.NewFromInt(110000),
							decimal.NewFromInt(140000),
						},
					},
				},
			},
			openBal: model.OpeningBalance{
				NoncurrentAssets:    decimal.NewFromInt(22000),
				Inventories:         decimal.NewFromInt(3000),
				CustomerReceivables: decimal.NewFromInt(2000),
				CashAndSecurities:   decimal.NewFromInt(8000),
				ShareCapital:        decimal.NewFromInt(50000),
				RetainedEarnings:    decimal.NewFromInt(5000),
				LoansAndDebt:        decimal.NewFromInt(30000),
				SupplierPayables:    decimal.NewFromInt(8000),
				SocialAndTaxDebts:   decimal.NewFromInt(3000),
			},
			checkAssets: func(t *testing.T, result model.BSheetReport) {
				// Check year 0 opening noncurrent assets
				assertDecEq(t, decimal.NewFromInt(22000), result.Detailed.Assets.NoncurrentAssets[0],
					"Opening noncurrent assets mismatch")

				// Check year 0 opening current assets (inventory + receivables)
				expectedCurrent := decimal.NewFromInt(3000).Add(decimal.NewFromInt(2000))
				actualCurrent := result.Detailed.Assets.Inventory[0].Add(result.Detailed.Assets.AccountsReceivable[0])
				assertDecEq(t, expectedCurrent, actualCurrent,
					"Opening current assets mismatch")

				// Check year 1 noncurrent assets from capex
				assertDecEq(t, decimal.NewFromInt(20000), result.Detailed.Assets.NoncurrentAssets[1],
					"Year 1 noncurrent assets from capex mismatch")

				// Check total assets = noncurrent + current
				for year := 0; year < 6; year++ {
					expectedTotal := result.Detailed.Assets.NoncurrentAssets[year].
						Add(result.Detailed.Assets.Inventory[year]).
						Add(result.Detailed.Assets.AccountsReceivable[year]).
						Add(result.Detailed.Assets.Cash[year])
					assertDecEq(t, expectedTotal, result.Detailed.Assets.TotalAssets[year],
						"Total assets mismatch for year %d", year)
				}
			},
			checkLiab: func(t *testing.T, result model.BSheetReport) {
				// Check year 0 opening equity
				expectedEquity := decimal.NewFromInt(50000).Add(decimal.NewFromInt(5000))
				assertDecEq(t, expectedEquity, result.Detailed.Liabilities.ShareCapital[0].Add(result.Detailed.Liabilities.RetainedEarnings[0]),
					"Opening equity mismatch")

				// Check opening long-term debt
				assertDecEq(t, decimal.NewFromInt(30000), result.Detailed.Liabilities.LongTermDebt[0],
					"Opening long-term debt mismatch")

				// Check opening current liabilities
				expectedCurrentLiab := decimal.NewFromInt(8000).Add(decimal.NewFromInt(3000))
				actualCurrentLiab := result.Detailed.Liabilities.TradePayables[0].Add(result.Detailed.Liabilities.SocialTaxDebts[0])
				assertDecEq(t, expectedCurrentLiab, actualCurrentLiab,
					"Opening current liabilities mismatch")
			},
		},
		{
			name: "zero inputs",
			pnl: model.PnlReport{
				Years: [5]model.PnlYear{
					{Year: 2025, YearIndex: 0, NetProfit: decimal.Zero},
					{Year: 2026, YearIndex: 1, NetProfit: decimal.Zero},
					{Year: 2027, YearIndex: 2, NetProfit: decimal.Zero},
					{Year: 2028, YearIndex: 3, NetProfit: decimal.Zero},
					{Year: 2029, YearIndex: 4, NetProfit: decimal.Zero},
				},
			},
			wcr:    model.WCRReport{},
			capex:  model.CapexSummary{},
			fiplan: model.FiplanReport{},
			openBal: model.OpeningBalance{
				NoncurrentAssets:    decimal.Zero,
				Inventories:         decimal.Zero,
				CustomerReceivables: decimal.Zero,
				CashAndSecurities:   decimal.Zero,
				ShareCapital:        decimal.Zero,
				RetainedEarnings:    decimal.Zero,
				LoansAndDebt:        decimal.Zero,
				SupplierPayables:    decimal.Zero,
				SocialAndTaxDebts:   decimal.Zero,
			},
			checkAssets: func(t *testing.T, result model.BSheetReport) {
				// All balances should be zero
				for year := 0; year < 6; year++ {
					assertDecEq(t, decimal.Zero, result.Detailed.Assets.NoncurrentAssets[year],
						"Noncurrent assets year %d should be zero", year)
					assertDecEq(t, decimal.Zero, result.Detailed.Assets.Inventory[year],
						"Inventory year %d should be zero", year)
					assertDecEq(t, decimal.Zero, result.Detailed.Assets.AccountsReceivable[year],
						"Accounts receivable year %d should be zero", year)
					assertDecEq(t, decimal.Zero, result.Detailed.Assets.TotalAssets[year],
						"Total assets year %d should be zero", year)
				}
			},
			checkLiab: func(t *testing.T, result model.BSheetReport) {
				// All equity and liabilities should be zero
				for year := 0; year < 6; year++ {
					assertDecEq(t, decimal.Zero, result.Detailed.Liabilities.ShareCapital[year],
						"Share capital year %d should be zero", year)
					assertDecEq(t, decimal.Zero, result.Detailed.Liabilities.LongTermDebt[year],
						"Long-term debt year %d should be zero", year)
					assertDecEq(t, decimal.Zero, result.Detailed.Liabilities.TradePayables[year],
						"Trade payables year %d should be zero", year)
				}
			},
		},
		{
			name: "opening balance carries through to year 0",
			pnl: model.PnlReport{
				Years: [5]model.PnlYear{
					{Year: 2025, YearIndex: 0, NetProfit: decimal.NewFromInt(5000)},
					{Year: 2026, YearIndex: 1, NetProfit: decimal.NewFromInt(5000)},
					{Year: 2027, YearIndex: 2, NetProfit: decimal.NewFromInt(5000)},
					{Year: 2028, YearIndex: 3, NetProfit: decimal.NewFromInt(5000)},
					{Year: 2029, YearIndex: 4, NetProfit: decimal.NewFromInt(5000)},
				},
			},
			wcr:    model.WCRReport{},
			capex:  model.CapexSummary{},
			fiplan: model.FiplanReport{},
			openBal: model.OpeningBalance{
				NoncurrentAssets:    decimal.NewFromInt(350),
				Inventories:         decimal.NewFromInt(75),
				CustomerReceivables: decimal.NewFromInt(100),
				CashAndSecurities:   decimal.NewFromInt(125),
				ShareCapital:        decimal.NewFromInt(500),
				RetainedEarnings:    decimal.NewFromInt(50),
				LoansAndDebt:        decimal.NewFromInt(200),
				SupplierPayables:    decimal.NewFromInt(100),
				SocialAndTaxDebts:   decimal.NewFromInt(75),
			},
			checkAssets: func(t *testing.T, result model.BSheetReport) {
				// Check that opening balance is exactly transferred to year 0
				assertDecEq(t, decimal.NewFromInt(350), result.Detailed.Assets.NoncurrentAssets[0],
					"Opening noncurrent assets should match input")
				assertDecEq(t, decimal.NewFromInt(75), result.Detailed.Assets.Inventory[0],
					"Opening inventory should match input")
				assertDecEq(t, decimal.NewFromInt(100), result.Detailed.Assets.AccountsReceivable[0],
					"Opening accounts receivable should match input")
				assertDecEq(t, decimal.NewFromInt(125), result.Detailed.Assets.Cash[0],
					"Opening cash should match input")
			},
			checkLiab: func(t *testing.T, result model.BSheetReport) {
				// Check opening balance
				assertDecEq(t, decimal.NewFromInt(500), result.Detailed.Liabilities.ShareCapital[0],
					"Opening share capital should match input")
				assertDecEq(t, decimal.NewFromInt(50), result.Detailed.Liabilities.RetainedEarnings[0],
					"Opening retained earnings should match input")
				assertDecEq(t, decimal.NewFromInt(200), result.Detailed.Liabilities.LongTermDebt[0],
					"Opening long-term debt should match input")
				assertDecEq(t, decimal.NewFromInt(100), result.Detailed.Liabilities.TradePayables[0],
					"Opening trade payables should match input")
				assertDecEq(t, decimal.NewFromInt(75), result.Detailed.Liabilities.SocialTaxDebts[0],
					"Opening social and tax debts should match input")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeBSheet(tt.pnl, tt.wcr, tt.capex, tt.fiplan, tt.openBal, config)

			tt.checkAssets(t, result)
			tt.checkLiab(t, result)

			// Verify cash from fiplan and opening balance
			assertDecEq(t, tt.openBal.CashAndSecurities, result.Detailed.Assets.Cash[0],
				"Opening cash should be from CashAndSecurities")
		})
	}
}
