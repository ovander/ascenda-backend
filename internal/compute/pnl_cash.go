package compute

import (
	"kerplan/internal/model"

	"github.com/shopspring/decimal"
)

// ComputePnlCash computes detailed P&L with Anglo-Saxon functional structure
func ComputePnlCash(
	entries []model.PnlCashEntry,
	revenue model.ConsolidatedRevenue,
	opex model.OpexSummary,
	staff model.StaffPayrollSummary,
	capex model.CapexSummary,
	pnl model.PnlReport,
	config model.PlanConfig,
) model.PnlCashReport {
	result := model.PnlCashReport{}

	// Build lookup map: lineID/yearIndex -> amount
	entryMap := make(map[string]map[int]decimal.Decimal)
	for _, entry := range entries {
		if _, ok := entryMap[string(entry.LineID)]; !ok {
			entryMap[string(entry.LineID)] = make(map[int]decimal.Decimal)
		}
		entryMap[string(entry.LineID)][entry.YearIndex] = entry.Amount
	}

	// Helper to get entry value
	getEntry := func(lineID model.PnlCashLineID, yearIndex int) decimal.Decimal {
		if m, ok := entryMap[string(lineID)]; ok {
			if v, ok := m[yearIndex]; ok {
				return v
			}
		}
		return decimal.Zero
	}

	// Helper to extract opex line value for a year
	getOpexLineValue := func(lineID model.OpexLineID, yearIndex int) decimal.Decimal {
		for _, subcat := range opex.Subcategories {
			for _, line := range subcat.Lines {
				if line.LineID == lineID {
					if yearIndex < len(line.Years) {
						return line.Years[yearIndex]
					}
					return decimal.Zero
				}
			}
		}
		return decimal.Zero
	}

	// Process each year
	startYear := config.ForecastStart.Year()
	for yearIndex := 0; yearIndex < 5; yearIndex++ {
		year := startYear + yearIndex

		// 1-3: Sales, CostOfSales, GrossMargin
		sales := revenue.Totals[yearIndex].TotalTurnover
		costOfSales := revenue.Totals[yearIndex].TotalCOGS
		grossMargin := sales.Sub(costOfSales)

		// R&D section
		// 4. RDPayroll
		rdPayroll := staff.FunctionalBreakdown[yearIndex].RnD
		// 5. OutsourcedRD
		outsourcedRD := getOpexLineValue("external_staff_rnd", yearIndex)
		// 6. RoyaltiesMisc
		royaltiesMisc := getOpexLineValue("royalty_patents", yearIndex).
			Add(getOpexLineValue("royalty_trademarks", yearIndex))

		// Sales section
		// 7. SalesPayroll
		salesPayroll := staff.FunctionalBreakdown[yearIndex].Sales
		// 8. AdvertisingPromo (marketing opex lines)
		advertisingPromo := getOpexLineValue("advertising_comms", yearIndex).
			Add(getOpexLineValue("trade_shows", yearIndex)).
			Add(getOpexLineValue("promotion_merchandising", yearIndex)).
			Add(getOpexLineValue("design_creation", yearIndex)).
			Add(getOpexLineValue("tech_costs_web", yearIndex))
		// 9. MiscSalesCosts
		miscSalesCosts := getEntry(model.PnlCashMiscSalesCosts, yearIndex)

		// G&A section
		// 10. GAPayroll
		gaPayroll := staff.FunctionalBreakdown[yearIndex].GnA
		// 11. InsuranceRent
		insuranceRent := getOpexLineValue("insurance_costs", yearIndex).
			Add(getOpexLineValue("property_rentals", yearIndex))
		// 12. LeasedEquip
		leasedEquip := getOpexLineValue("leasing_movable", yearIndex).
			Add(getOpexLineValue("leasing_real_estate", yearIndex))
		// 13. LegalConsulting
		legalConsulting := getOpexLineValue("professional_fees", yearIndex).
			Add(getOpexLineValue("studies_documentation", yearIndex))
		// 14. TravelMisc
		travelMisc := getOpexLineValue("travel_transport", yearIndex).
			Add(getOpexLineValue("mission_representation", yearIndex)).
			Add(getOpexLineValue("supplies_purchases", yearIndex)).
			Add(getOpexLineValue("postage_telecom", yearIndex)).
			Add(getOpexLineValue("other_expenses", yearIndex)).
			Add(getOpexLineValue("maintenance_repairs", yearIndex)).
			Add(getOpexLineValue("recruitment_training", yearIndex))

		// 15. Depreciation
		depreciation := capex.Totals.TotalDepreciation[yearIndex]

		// 16. EBIT
		totalExpenses := rdPayroll.Add(outsourcedRD).Add(royaltiesMisc).
			Add(salesPayroll).Add(advertisingPromo).Add(miscSalesCosts).
			Add(gaPayroll).Add(insuranceRent).Add(leasedEquip).
			Add(legalConsulting).Add(travelMisc).Add(depreciation)
		ebit := grossMargin.Sub(totalExpenses)

		// 17. InterestExpense
		interestExpense := pnl.Years[yearIndex].FinancialExpenses
		// 18. Subsidies
		subsidies := pnl.Years[yearIndex].GrantsOtherRevenue
		// 19. TaxesIncurred
		taxesIncurred := pnl.Years[yearIndex].CorporateTax
		// 20. NetProfit
		netProfit := ebit.Sub(interestExpense).Add(subsidies).Sub(taxesIncurred)

		// 21. SalesPct
		salesPct := safeDiv(netProfit, sales)

		result.Years[yearIndex] = model.PnlCashYear{
			Year:             year,
			YearIndex:        yearIndex,
			Sales:            sales,
			CostOfSales:      costOfSales,
			GrossMargin:      grossMargin,
			RDPayroll:        rdPayroll,
			OutsourcedRD:     outsourcedRD,
			RoyaltiesMisc:    royaltiesMisc,
			SalesPayroll:     salesPayroll,
			AdvertisingPromo: advertisingPromo,
			MiscSalesCosts:   miscSalesCosts,
			GAPayroll:        gaPayroll,
			InsuranceRent:    insuranceRent,
			LeasedEquip:      leasedEquip,
			LegalConsulting:  legalConsulting,
			TravelMisc:       travelMisc,
			Depreciation:     depreciation,
			EBIT:             ebit,
			InterestExpense:  interestExpense,
			Subsidies:        subsidies,
			TaxesIncurred:    taxesIncurred,
			NetProfit:        netProfit,
			SalesPct:         salesPct,
		}
	}

	// Compute chart data
	result.ChartData = computeChartData(result.Years)

	return result
}

// computeChartData generates chart-ready aggregations
func computeChartData(years [5]model.PnlCashYear) model.PnlCashChartData {
	chartData := model.PnlCashChartData{}

	for i := 0; i < 5; i++ {
		chartData.Years[i] = years[i].Year
		chartData.CostOfSales[i] = years[i].CostOfSales
		// RDProduction = RDPayroll + OutsourcedRD + RoyaltiesMisc
		chartData.RDProduction[i] = years[i].RDPayroll.
			Add(years[i].OutsourcedRD).
			Add(years[i].RoyaltiesMisc)
		// SalesMarketing = SalesPayroll + AdvertisingPromo + MiscSalesCosts
		chartData.SalesMarketing[i] = years[i].SalesPayroll.
			Add(years[i].AdvertisingPromo).
			Add(years[i].MiscSalesCosts)
		// GeneralAdmin = GAPayroll + InsuranceRent + LeasedEquip + LegalConsulting + TravelMisc
		chartData.GeneralAdmin[i] = years[i].GAPayroll.
			Add(years[i].InsuranceRent).
			Add(years[i].LeasedEquip).
			Add(years[i].LegalConsulting).
			Add(years[i].TravelMisc)
		// EBIT (split positive/negative)
		if years[i].EBIT.IsPositive() {
			chartData.EBITPositive[i] = years[i].EBIT
			chartData.EBITNegative[i] = decimal.Zero
		} else {
			chartData.EBITPositive[i] = decimal.Zero
			chartData.EBITNegative[i] = years[i].EBIT
		}
	}

	return chartData
}

// safeDiv performs division with zero-denominator protection
func safeDiv(numerator, denominator decimal.Decimal) decimal.Decimal {
	if denominator.IsZero() {
		return decimal.Zero
	}
	return numerator.Div(denominator)
}
