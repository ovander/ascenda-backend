package compute

import (
	"ascenda/internal/model"

	"github.com/shopspring/decimal"
)

// ComputeCapexSummary aggregates capex across categories and computes depreciation.
// It builds an investment grid per category, calculates straight-line depreciation,
// and produces CapexCategoryRow for each asset category plus totals and depreciation schedule.
func ComputeCapexSummary(entries []model.CapexEntry, config model.PlanConfig) model.CapexSummary {
	result := model.CapexSummary{
		Investments:          make([]model.CapexCategoryRow, 0, len(model.AllAssetCategories)),
		DepreciationSchedule: make([]model.CapexDepreciationRow, 0),
	}

	// Build investment grid: map category to [5]decimal.Decimal
	investmentGrid := make(map[model.AssetCategory][MaxYears]decimal.Decimal)

	// Accumulate entries into investment grid.
	// YearIndex is 1-based (1–MaxYears); convert to 0-based array index.
	// Amounts are already in k€ (the displayed unit), matching revenue and payroll.
	for _, entry := range entries {
		if entry.YearIndex < 1 || entry.YearIndex > MaxYears {
			continue
		}
		idx := entry.YearIndex - 1
		grid := investmentGrid[entry.Category]
		grid[idx] = grid[idx].Add(entry.Amount)
		investmentGrid[entry.Category] = grid
	}

	// Build depreciation grid: map category to [5]decimal.Decimal
	depreciationGrid := computeDepreciationGrid(investmentGrid)

	// Compute totals
	totalCapex := [MaxYears]decimal.Decimal{}
	totalDepreciation := [MaxYears]decimal.Decimal{}

	for _, cat := range model.AllAssetCategories {
		invGrid := investmentGrid[cat]
		for y := 0; y < MaxYears; y++ {
			totalCapex[y] = totalCapex[y].Add(invGrid[y])
			totalDepreciation[y] = totalDepreciation[y].Add(depreciationGrid[cat][y])
		}
	}

	// Compute net assets: NetAssets[0] = PreviousNetAssets, then accumulate
	netAssets := [6]decimal.Decimal{}
	netAssets[0] = result.Totals.PreviousNetAssets // defaults to zero
	for y := 0; y < MaxYears; y++ {
		netAssets[y+1] = netAssets[y].Add(totalCapex[y]).Sub(totalDepreciation[y])
	}

	// Build Investments (one row per category in AllAssetCategories order)
	for _, cat := range model.AllAssetCategories {
		isDepreciable := !model.NonDepreciableCategories[cat]
		isStaffLinked := model.StaffLinkedCategories[cat]

		depYears := 0
		if isDepreciable {
			if v, ok := model.DefaultDepreciationYears[cat]; ok {
				depYears = v
			}
		}

		invGrid := investmentGrid[cat]
		row := model.CapexCategoryRow{
			Category:          cat,
			IsDepreciable:     isDepreciable,
			IsStaffLinked:     isStaffLinked,
			DepreciationYears: depYears,
			Years:             invGrid,
		}
		result.Investments = append(result.Investments, row)
	}

	// Build DepreciationSchedule (one row per depreciable category)
	for _, cat := range model.AllAssetCategories {
		if model.NonDepreciableCategories[cat] {
			continue
		}

		depYears := 0
		if v, ok := model.DefaultDepreciationYears[cat]; ok {
			depYears = v
		}

		depRow := model.CapexDepreciationRow{
			Category:          cat,
			DepreciationYears: depYears,
			Years:             depreciationGrid[cat],
		}
		result.DepreciationSchedule = append(result.DepreciationSchedule, depRow)
	}

	// Populate totals
	result.Totals = model.CapexTotals{
		TotalCapex:        totalCapex,
		PreviousNetAssets: result.Totals.PreviousNetAssets,
		PriorDepreciation: [MaxYears]decimal.Decimal{}, // placeholder
		TotalDepreciation: totalDepreciation,
		NetAssets:         netAssets,
	}

	return result
}

// computeDepreciationGrid computes straight-line depreciation for all categories.
// For each investment, depreciation is spread evenly over its DepreciationYears,
// starting in the year after investment and capped at MaxYears.
func computeDepreciationGrid(investmentGrid map[model.AssetCategory][MaxYears]decimal.Decimal) map[model.AssetCategory][MaxYears]decimal.Decimal {
	depreciationGrid := make(map[model.AssetCategory][MaxYears]decimal.Decimal)

	for _, cat := range model.AllAssetCategories {
		// Skip non-depreciable categories
		if model.NonDepreciableCategories[cat] {
			continue
		}

		depYears, ok := model.DefaultDepreciationYears[cat]
		if !ok || depYears <= 0 {
			continue
		}

		invGrid := investmentGrid[cat]
		depGrid := [MaxYears]decimal.Decimal{}

		// For each investment year, apply depreciation over depYears
		for investYear := 0; investYear < MaxYears; investYear++ {
			amount := invGrid[investYear]
			if amount.IsZero() {
				continue
			}

			// Annual depreciation expense (straight-line)
			yearlyDep := SafeDiv(amount, decimal.NewFromInt(int64(depYears)))

			// Apply from year after investment through min(investYear+depYears, MaxYears-1).
			// Use +1 on endYear so the < comparison is exclusive and the full
			// depYears worth of depreciation entries are included within the horizon.
			endYear := investYear + depYears + 1
			if endYear > MaxYears {
				endYear = MaxYears
			}
			for depYear := investYear + 1; depYear < endYear; depYear++ {
				depGrid[depYear] = depGrid[depYear].Add(yearlyDep)
			}
		}

		depreciationGrid[cat] = depGrid
	}

	return depreciationGrid
}
