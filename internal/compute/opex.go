package compute

import (
	"ascenda/internal/model"

	"github.com/shopspring/decimal"
)

// ComputeOpexSummary computes total operating expenses with cost driver logic
func ComputeOpexSummary(
	entries []model.OpexManualEntry,
	opexPerHire model.OpexPerHire,
	revenue model.ConsolidatedRevenue,
	staff model.StaffPayrollSummary,
	config model.PlanConfig,
) model.OpexSummary {
	// Build lookup map: lineID/year -> amount.
	// YearIndex is 1-based (1–5); normalize to 0-based for array indexing.
	// Amounts are already in k€ (the displayed unit), matching all other opex lines.
	entryMap := make(map[model.OpexLineID]map[int]decimal.Decimal)
	for _, entry := range entries {
		if entry.YearIndex < 1 || entry.YearIndex > 5 {
			continue
		}
		if _, ok := entryMap[entry.LineID]; !ok {
			entryMap[entry.LineID] = make(map[int]decimal.Decimal)
		}
		entryMap[entry.LineID][entry.YearIndex-1] = entry.Amount
	}

	// Process each line and compute amounts
	lineResults := make(map[model.OpexLineID]model.OpexLineResult)
	for _, lineConfig := range model.AllOpexLines {
		lineResult := model.OpexLineResult{
			LineID:      lineConfig.ID,
			IsUserInput: lineConfig.IsUserInput,
			CostDriver:  lineConfig.CostDriver,
		}

		// Get values for each year
		for yearIdx := 0; yearIdx < 5; yearIdx++ {
			var amount decimal.Decimal

			// Check for manual entry
			if yearValues, ok := entryMap[lineConfig.ID]; ok {
				if val, ok := yearValues[yearIdx]; ok {
					amount = val
				}
			}

			// Apply cost driver logic if no manual entry
			if amount.IsZero() {
				amount = computeOpexAmount(
					lineConfig.CostDriver,
					lineConfig.SettingsField,
					yearIdx,
					opexPerHire,
					revenue,
					staff,
				)
			}

			lineResult.Years[yearIdx] = amount
		}

		lineResults[lineConfig.ID] = lineResult
	}

	// Group by subcategory
	subcategoryMap := make(map[model.OpexSubcategory][]model.OpexLineResult)
	for _, lineConfig := range model.AllOpexLines {
		lineResult := lineResults[lineConfig.ID]
		subcategoryMap[lineConfig.Subcategory] = append(subcategoryMap[lineConfig.Subcategory], lineResult)
	}

	// Build subcategory results
	var subcategoryResults []model.OpexSubcategoryResult
	var grandTotal [5]decimal.Decimal

	for _, lineConfig := range model.AllOpexLines {
		subcat := lineConfig.Subcategory
		// Only add once per subcategory
		if _, exists := subcategoryMap[subcat]; !exists {
			continue
		}

		lines := subcategoryMap[subcat]
		var subtotal [5]decimal.Decimal

		for _, line := range lines {
			for yearIdx := 0; yearIdx < 5; yearIdx++ {
				subtotal[yearIdx] = subtotal[yearIdx].Add(line.Years[yearIdx])
				grandTotal[yearIdx] = grandTotal[yearIdx].Add(line.Years[yearIdx])
			}
		}

		subcategoryResults = append(subcategoryResults, model.OpexSubcategoryResult{
			Subcategory: subcat,
			Lines:       lines,
			Subtotal:    subtotal,
		})

		// Remove from map so we don't process again
		delete(subcategoryMap, subcat)
	}

	return model.OpexSummary{
		Subcategories: subcategoryResults,
		GrandTotal:    grandTotal,
	}
}

// computeOpexAmount computes opex amount based on cost driver
func computeOpexAmount(
	costDriver string,
	settingsField string,
	yearIdx int,
	opexPerHire model.OpexPerHire,
	revenue model.ConsolidatedRevenue,
	staff model.StaffPayrollSummary,
) decimal.Decimal {
	switch costDriver {
	case "manual":
		// Manual entries are handled in caller; this shouldn't be called
		return decimal.Zero

	case "per_capita":
		// amount = settingsValue × totalStaff[year]
		settingValue := getOpexPerHireField(opexPerHire, settingsField)
		totalStaff := staff.Headcount.TotalStaff[yearIdx]
		return settingValue.Mul(totalStaff)

	case "pct_of_sales":
		// amount = settingsValue × totalTurnover[year]
		settingValue := getOpexPerHireField(opexPerHire, settingsField)
		totalTurnover := revenue.Totals[yearIdx].TotalTurnover
		return settingValue.Mul(totalTurnover)

	case "special_rent":
		// PropertyRentals base value (year 0), grows with headcount ratio
		// PropertyRentals × max(1, totalStaff[y] / totalStaff[0])
		baseValue := opexPerHire.PropertyRentals
		staffYear0 := staff.Headcount.TotalStaff[0]
		staffYearY := staff.Headcount.TotalStaff[yearIdx]

		if staffYear0.IsZero() {
			return baseValue
		}

		ratio := staffYearY.Div(staffYear0)
		if ratio.LessThan(decimal.NewFromInt(1)) {
			ratio = decimal.NewFromInt(1)
		}
		return baseValue.Mul(ratio)

	case "pct_of_payroll":
		// amount = settingsValue × totalPayroll[year]
		settingValue := getOpexPerHireField(opexPerHire, settingsField)
		totalPayroll := staff.Payroll[yearIdx].TotalPayroll
		return settingValue.Mul(totalPayroll)

	default:
		return decimal.Zero
	}
}

// getOpexPerHireField retrieves an OpexPerHire field by name
func getOpexPerHireField(opexPerHire model.OpexPerHire, fieldName string) decimal.Decimal {
	switch fieldName {
	case "PropertyRentals":
		return opexPerHire.PropertyRentals
	case "PostageTelecom":
		return opexPerHire.PostageTelecom
	case "SuppliesPurchases":
		return opexPerHire.SuppliesPurchases
	case "StudiesDocumentation":
		return opexPerHire.StudiesDocumentation
	case "InsuranceCostsPctSales":
		return opexPerHire.InsuranceCostsPctSales
	case "RoyaltyPaymentsPctSales":
		return opexPerHire.RoyaltyPaymentsPctSales
	case "TravelTransportation":
		return opexPerHire.TravelTransportation
	case "MissionRepresentation":
		return opexPerHire.MissionRepresentation
	case "RecruitTrainingPctPayroll":
		return opexPerHire.RecruitTrainingPctPayroll
	default:
		return decimal.Zero
	}
}
