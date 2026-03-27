package compute

import (
	"fmt"
	"ascenda/internal/model"

	"github.com/shopspring/decimal"
)

// ProductInputBundle groups all per-product inputs across 5 years
type ProductInputBundle struct {
	Assumptions [MaxYears]model.ProductAssumption  // indexed by yearIndex-1
	Volumes     []model.ProductSalesVolume        // all zone×channel combinations
	Margins     []model.ProductDistributorMargin  // per zone per year
	// SurplusInventoryValue[y] is populated by applyIndustryDriver when the
	// product is DriverIndustry and UnitsProduced[y] > 0.  It represents the
	// cumulative finished-goods inventory value (at effective unit cost) at the
	// end of year y.  Zero for all other driver types.
	SurplusInventoryValue [MaxYears]decimal.Decimal
}

// ComputeProductRevenue computes revenue summary for a single product
// Returns ProductRevenueSummary with detailed breakdown by zone and year
func ComputeProductRevenue(product model.Product, bundle ProductInputBundle, config model.PlanConfig) model.ProductRevenueSummary {
	summary := model.ProductRevenueSummary{
		ProductID:   product.ID,
		ProductName: product.Name,
	}

	// Build zone indices map
	zoneIndices := map[model.GeoZone]int{
		model.ZoneFrance: 0,
		model.ZoneEurope: 1,
		model.ZoneExport: 2,
	}

	// Build indices for volumes and margins for quick lookup
	volumeByYearZoneChannel := make(map[string]int64)
	for _, vol := range bundle.Volumes {
		key := makeVolumeKey(vol.YearIndex, vol.Zone, vol.Channel)
		volumeByYearZoneChannel[key] += vol.UnitsSold
	}

	marginByYearZone := make(map[string]decimal.Decimal)
	for _, margin := range bundle.Margins {
		key := makeMarginKey(margin.YearIndex, margin.Zone)
		marginByYearZone[key] = margin.MarginPercent
	}

	// Compute each year
	cumulativeUnitSales := int64(0)
	for yearIndex := 0; yearIndex < MaxYears; yearIndex++ {
		year := yearIndex + 1
		yearRevenue := model.ProductRevenueYear{
			Year:      config.ForecastStart.Year() + yearIndex, // calendar year (ForecastStart + 0..4)
			YearIndex: year,
		}

		assumption := bundle.Assumptions[yearIndex]

		// Compute total unit cost
		totalUnitCost := assumption.RawMaterialCost.
			Add(assumption.RoyaltiesCost).
			Add(assumption.LogisticsCost).
			Mul(assumption.CostCoefficient)

		// Compute final unit price
		finalUnitPrice := assumption.BaseUnitPrice.Mul(assumption.PriceCoefficient)

		// Process each zone
		directUnitSalesTotal := int64(0)
		indirectUnitSalesTotal := int64(0)
		directRevenueTotal := decimal.Zero
		indirectRevenueTotal := decimal.Zero

		for zone, zoneIdx := range zoneIndices {
			zoneRevenue := model.ProductRevenueZoneYear{
				Zone: zone,
			}

			// Direct sales for this zone
			directVolume := volumeByYearZoneChannel[makeVolumeKey(year, zone, model.ChannelDirect)]
			zoneRevenue.DirectUnitsSold = directVolume
			zoneRevenue.DirectUnitPrice = finalUnitPrice
			zoneRevenue.DirectRevenue = decimal.NewFromInt(directVolume).
				Mul(finalUnitPrice)

			directUnitSalesTotal += directVolume
			directRevenueTotal = directRevenueTotal.Add(zoneRevenue.DirectRevenue)

			// Indirect sales for this zone
			indirectVolume := volumeByYearZoneChannel[makeVolumeKey(year, zone, model.ChannelIndirect)]
			zoneRevenue.IndirectUnitsSold = indirectVolume
			zoneRevenue.EndUserPrice = finalUnitPrice

			// Get distributor margin for this zone/year
			marginKey := makeMarginKey(year, zone)
			marginPercent := marginByYearZone[marginKey]
			zoneRevenue.DistributorMarginPct = marginPercent

			// DistributorPrice = FinalUnitPrice * (1 - MarginPercent)
			marginComplement := decimal.NewFromInt(1).Sub(marginPercent)
			zoneRevenue.DistributorPrice = finalUnitPrice.Mul(marginComplement)

			// CustDistributorCoeff = 1 / (1 - margin%)
			zoneRevenue.CustDistributorCoeff = SafeDiv(decimal.NewFromInt(1), marginComplement)

			// IndirectRevenue = IndirectUnitsSold * DistributorPrice
			zoneRevenue.IndirectRevenue = decimal.NewFromInt(indirectVolume).
				Mul(zoneRevenue.DistributorPrice)

			// CompanyGrossMarginPct = (DistributorPrice - TotalUnitCost) / DistributorPrice
			grossMarginAmount := zoneRevenue.DistributorPrice.Sub(totalUnitCost)
			zoneRevenue.CompanyGrossMarginPct = SafeDiv(grossMarginAmount, zoneRevenue.DistributorPrice)

			indirectUnitSalesTotal += indirectVolume
			indirectRevenueTotal = indirectRevenueTotal.Add(zoneRevenue.IndirectRevenue)

			yearRevenue.Zones[zoneIdx] = zoneRevenue
		}

		// Aggregate unit sales
		totalUnitSales := directUnitSalesTotal + indirectUnitSalesTotal
		yearRevenue.TotalUnitSales = totalUnitSales
		cumulativeUnitSales += totalUnitSales
		yearRevenue.CumulatedUnitSales = cumulativeUnitSales

		yearRevenue.DirectUnitSales = directUnitSalesTotal
		yearRevenue.IndirectUnitSales = indirectUnitSalesTotal
		yearRevenue.FranceUnitSales = yearRevenue.Zones[0].DirectUnitsSold + yearRevenue.Zones[0].IndirectUnitsSold
		yearRevenue.EuropeUnitSales = yearRevenue.Zones[1].DirectUnitsSold + yearRevenue.Zones[1].IndirectUnitsSold
		yearRevenue.ExportUnitSales = yearRevenue.Zones[2].DirectUnitsSold + yearRevenue.Zones[2].IndirectUnitsSold

		// Total turnover = direct + indirect revenue
		yearRevenue.Turnover = directRevenueTotal.Add(indirectRevenueTotal)

		// EuropeExportSales = europe zone revenue + export zone revenue
		yearRevenue.EuropeExportSales = yearRevenue.Zones[1].DirectRevenue.
			Add(yearRevenue.Zones[1].IndirectRevenue).
			Add(yearRevenue.Zones[2].DirectRevenue).
			Add(yearRevenue.Zones[2].IndirectRevenue)

		// DirectSalesTotal = france direct + europe direct + export direct
		yearRevenue.DirectSalesTotal = yearRevenue.Zones[0].DirectRevenue.
			Add(yearRevenue.Zones[1].DirectRevenue).
			Add(yearRevenue.Zones[2].DirectRevenue)

		// COGS = TotalUnitSales * TotalUnitCost (base €)
		yearRevenue.COGS = decimal.NewFromInt(totalUnitSales).
			Mul(totalUnitCost)

		// GrossMargin = Turnover - COGS
		yearRevenue.GrossMargin = yearRevenue.Turnover.Sub(yearRevenue.COGS)

		// GrossMarginPct = GrossMargin / Turnover
		yearRevenue.GrossMarginPct = SafeDiv(yearRevenue.GrossMargin, yearRevenue.Turnover)

		summary.Years[yearIndex] = yearRevenue
		// Carry the industry surplus inventory value (zero for non-industry products).
		summary.SurplusInventoryValue[yearIndex] = bundle.SurplusInventoryValue[yearIndex]
	}

	return summary
}

// ComputeConsolidatedRevenue aggregates revenues from all products
func ComputeConsolidatedRevenue(products []model.Product, bundles []ProductInputBundle, config model.PlanConfig) model.ConsolidatedRevenue {
	consolidated := model.ConsolidatedRevenue{}

	if len(products) > 0 {
		consolidated.ScenarioID = products[0].ScenarioID
	}

	// Compute each product and collect summaries
	summaries := make([]model.ProductRevenueSummary, 0, len(products))
	totalByYear := [MaxYears]model.ConsolidatedRevenueYear{}

	for i := 0; i < len(products); i++ {
		if i >= len(bundles) {
			continue
		}

		productSummary := ComputeProductRevenue(products[i], bundles[i], config)
		summaries = append(summaries, productSummary)

		// Aggregate by year
		for yearIdx := 0; yearIdx < MaxYears; yearIdx++ {
			productYear := productSummary.Years[yearIdx]

			totalByYear[yearIdx].Year = config.ForecastStart.Year() + yearIdx
			totalByYear[yearIdx].TotalTurnover = totalByYear[yearIdx].TotalTurnover.Add(productYear.Turnover)
			totalByYear[yearIdx].TotalDirectSales = totalByYear[yearIdx].TotalDirectSales.Add(productYear.DirectSalesTotal)
			totalByYear[yearIdx].TotalIndirectSales = totalByYear[yearIdx].TotalIndirectSales.Add(productYear.Turnover.Sub(productYear.DirectSalesTotal))
			totalByYear[yearIdx].EuropeExportSales = totalByYear[yearIdx].EuropeExportSales.Add(productYear.EuropeExportSales)
			totalByYear[yearIdx].TotalCOGS = totalByYear[yearIdx].TotalCOGS.Add(productYear.COGS)
			totalByYear[yearIdx].TotalUnitSales += productYear.TotalUnitSales
			// Aggregate finished-goods surplus inventory (non-zero for industry products only).
			totalByYear[yearIdx].TotalSurplusInventory = totalByYear[yearIdx].TotalSurplusInventory.
				Add(productSummary.SurplusInventoryValue[yearIdx])
		}
	}

	consolidated.Products = summaries

	// Calculate aggregated gross margin metrics
	for yearIdx := 0; yearIdx < MaxYears; yearIdx++ {
		totalByYear[yearIdx].TotalGrossMargin = totalByYear[yearIdx].TotalTurnover.Sub(totalByYear[yearIdx].TotalCOGS)
		totalByYear[yearIdx].GrossMarginPct = SafeDiv(totalByYear[yearIdx].TotalGrossMargin, totalByYear[yearIdx].TotalTurnover)
		consolidated.Totals[yearIdx] = totalByYear[yearIdx]
	}

	return consolidated
}

// Helper functions for volume and margin key generation
func makeVolumeKey(yearIndex int, zone model.GeoZone, channel model.SalesChannel) string {
	return fmt.Sprintf("%s_%s_%d", zone, channel, yearIndex)
}

func makeMarginKey(yearIndex int, zone model.GeoZone) string {
	return fmt.Sprintf("%s_%d", zone, yearIndex)
}
