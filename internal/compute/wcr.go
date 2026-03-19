package compute

import (
	"kerplan/internal/model"

	"github.com/shopspring/decimal"
)

// ComputeWCR computes working capital requirement with a 5-tranche payment deferral model
func ComputeWCR(
	entries []model.WCREntry,
	revenue model.ConsolidatedRevenue,
	opex model.OpexSummary,
	staff model.StaffPayrollSummary,
	wcConfig model.WorkingCapitalConfig,
	openBal model.OpeningBalance,
	capex model.CapexSummary,
	pnl model.PnlReport,
	config model.PlanConfig,
) model.WCRReport {
	result := model.WCRReport{}

	// Extract VAT rate from config
	result.VATRate = config.VATRate

	// Build lookup map: lineID/yearIndex -> amount (for overrides)
	entryMap := make(map[model.WCRLineID]map[int]decimal.Decimal)
	for _, entry := range entries {
		if _, ok := entryMap[entry.LineID]; !ok {
			entryMap[entry.LineID] = make(map[int]decimal.Decimal)
		}
		entryMap[entry.LineID][entry.YearIndex] = entry.Amount
	}

	// Helper to get override or compute default
	getOrCompute := func(lineID model.WCRLineID, yearIdx int, computed decimal.Decimal) decimal.Decimal {
		if yearVals, ok := entryMap[lineID]; ok {
			if val, ok := yearVals[yearIdx]; ok {
				return val
			}
		}
		return computed
	}

	// Days per year constant
	daysPerYear := decimal.NewFromInt(360)

	// Compute working capital percentages with 120-day tranche
	wcComputed := model.WorkingCapitalComputed{
		WorkingCapitalConfig: wcConfig,
		CustomerPct120Days:   decimal.NewFromInt(1).
			Sub(wcConfig.CustomerPct0Days).
			Sub(wcConfig.CustomerPct30Days).
			Sub(wcConfig.CustomerPct60Days).
			Sub(wcConfig.CustomerPct90Days),
		SupplierPct120Days: decimal.NewFromInt(1).
			Sub(wcConfig.SupplierPct0Days).
			Sub(wcConfig.SupplierPct30Days).
			Sub(wcConfig.SupplierPct60Days).
			Sub(wcConfig.SupplierPct90Days),
	}

	// Process each year
	for yearIdx := 0; yearIdx < MaxYears; yearIdx++ {
		// ===== CUSTOMERS =====
		rev := revenue.Totals[yearIdx]
		salesExclVAT := rev.TotalTurnover
		exportExclVAT := rev.EuropeExportSales
		domesticSales := salesExclVAT.Sub(exportExclVAT)

		// SalesInclTax = DomesticSales × (1 + VATRate) + ExportExclVAT (export is VAT-free)
		salesInclTax := domesticSales.Mul(result.VATRate.Add(decimal.NewFromInt(1))).Add(exportExclVAT)

		result.Customers.SalesExclVAT[yearIdx] = salesExclVAT
		result.Customers.ExportExclVAT[yearIdx] = exportExclVAT
		result.Customers.SalesInclTax[yearIdx] = salesInclTax

		// 5-tranche model for customers (0, 30, 60, 90, 120 days)
		daysArray := [5]decimal.Decimal{
			decimal.Zero,
			decimal.NewFromInt(30),
			decimal.NewFromInt(60),
			decimal.NewFromInt(90),
			decimal.NewFromInt(120),
		}
		pctArray := [5]decimal.Decimal{
			wcComputed.CustomerPct0Days,
			wcComputed.CustomerPct30Days,
			wcComputed.CustomerPct60Days,
			wcComputed.CustomerPct90Days,
			wcComputed.CustomerPct120Days,
		}

		for tranche := 0; tranche < 5; tranche++ {
			result.Customers.Tranches[yearIdx][tranche] = salesInclTax.
				Mul(pctArray[tranche]).
				Mul(daysArray[tranche]).
				Div(daysPerYear)
		}

		// TotalCustomers = sum of tranches (this is the customer receivable balance)
		totalCustomers := decimal.Zero
		for tranche := 0; tranche < 5; tranche++ {
			totalCustomers = totalCustomers.Add(result.Customers.Tranches[yearIdx][tranche])
		}
		result.Customers.TotalCustomers[yearIdx] = totalCustomers

		// ===== INVENTORY =====
		result.Inventory.COGSBase[yearIdx] = rev.TotalCOGS
		invPct := wcConfig.InventoryPctYear1
		if yearIdx > 0 {
			invPct = wcConfig.InventoryPctYear1 // TODO: use year-specific if available
		}
		result.Inventory.InventoryPct[yearIdx] = invPct
		result.Inventory.InventoryValue[yearIdx] = rev.TotalCOGS.Mul(invPct)

		// ===== SUPPLIERS =====
		cogsExclVAT := rev.TotalCOGS
		externalExclVAT := opex.GrandTotal[yearIdx]
		capexExclVAT := capex.Totals.TotalCapex[yearIdx]

		result.Suppliers.COGSExclVAT[yearIdx] = cogsExclVAT
		result.Suppliers.ExternalExclVAT[yearIdx] = externalExclVAT
		result.Suppliers.CapexExclVAT[yearIdx] = capexExclVAT

		// TotalInclVAT = (COGS + Opex + Capex) × (1 + VATRate)
		totalInclVAT := cogsExclVAT.Add(externalExclVAT).Add(capexExclVAT).
			Mul(result.VATRate.Add(decimal.NewFromInt(1)))
		result.Suppliers.TotalInclVAT[yearIdx] = totalInclVAT

		// 5-tranche model for suppliers
		supplierPctArray := [5]decimal.Decimal{
			wcComputed.SupplierPct0Days,
			wcComputed.SupplierPct30Days,
			wcComputed.SupplierPct60Days,
			wcComputed.SupplierPct90Days,
			wcComputed.SupplierPct120Days,
		}

		for tranche := 0; tranche < 5; tranche++ {
			result.Suppliers.Tranches[yearIdx][tranche] = totalInclVAT.
				Mul(supplierPctArray[tranche]).
				Mul(daysArray[tranche]).
				Div(daysPerYear)
		}

		// TotalSuppliers = sum of tranches
		totalSuppliers := decimal.Zero
		for tranche := 0; tranche < 5; tranche++ {
			totalSuppliers = totalSuppliers.Add(result.Suppliers.Tranches[yearIdx][tranche])
		}
		result.Suppliers.TotalSuppliers[yearIdx] = totalSuppliers

		// ===== SUMMARY =====
		result.Summary.CustomerWCR[yearIdx] = totalCustomers
		result.Summary.InventoryWCR[yearIdx] = result.Inventory.InventoryValue[yearIdx]
		result.Summary.SupplierWCR[yearIdx] = totalSuppliers
		result.Summary.BasicWCR[yearIdx] = totalCustomers.
			Add(result.Inventory.InventoryValue[yearIdx]).
			Sub(totalSuppliers)

		// BasicWCRDays = BasicWCR / (SalesExclVAT / 360)
		result.Summary.BasicWCRDays[yearIdx] = SafeDiv(result.Summary.BasicWCR[yearIdx],
			SafeDiv(salesExclVAT, daysPerYear))

		// WCRChange calculation
		if yearIdx == 0 {
			result.Summary.WCRChange[yearIdx] = result.Summary.BasicWCR[yearIdx]
		} else {
			result.Summary.WCRChange[yearIdx] = result.Summary.BasicWCR[yearIdx].
				Sub(result.Summary.BasicWCR[yearIdx-1])
		}

		// ===== FISCAL & SOCIAL =====
		// VAT calculations
		domesticRatio := SafeDiv(domesticSales, salesExclVAT)
		domesticCOGS := rev.TotalCOGS.Mul(domesticRatio)
		domesticOpex := opex.GrandTotal[yearIdx].Mul(domesticRatio)
		domesticCapex := capex.Totals.TotalCapex[yearIdx].Mul(domesticRatio)

		vatCollected := domesticSales.Mul(result.VATRate)
		vatDeductible := domesticCOGS.Add(domesticOpex).Add(domesticCapex).Mul(result.VATRate)
		netVATPayable := vatCollected.Sub(vatDeductible)

		result.FiscalSocial.VATCollected[yearIdx] = vatCollected
		result.FiscalSocial.VATDeductible[yearIdx] = vatDeductible
		result.FiscalSocial.NetVATPayable[yearIdx] = netVATPayable

		// VAT liability (30-day deferral)
		result.FiscalSocial.VATDaysOutstanding[yearIdx] = decimal.NewFromInt(30)
		result.FiscalSocial.VATLiability[yearIdx] = SafeDiv(netVATPayable.Mul(decimal.NewFromInt(30)), daysPerYear)

		// Social charges (derived from payroll and employer tax rate)
		payrollYear := staff.Payroll[yearIdx]
		employerCharges := payrollYear.SubtotalPayroll.Mul(config.EmployerTaxRate)
		result.FiscalSocial.EmployerCharges[yearIdx] = employerCharges
		result.FiscalSocial.EmployeeCharges[yearIdx] = decimal.Zero // employee deductions not tracked separately
		totalSocialCharges := employerCharges
		result.FiscalSocial.TotalSocialCharges[yearIdx] = totalSocialCharges

		// Social liability (15-day deferral average)
		result.FiscalSocial.SocialDaysOutstand[yearIdx] = decimal.NewFromInt(15)
		result.FiscalSocial.SocialLiability[yearIdx] = SafeDiv(totalSocialCharges.Mul(decimal.NewFromInt(15)), daysPerYear)

		// Corporate tax (from PnL PreTaxEarnings, 30-day deferral)
		estimatedTaxLiability := pnl.Years[yearIdx].PreTaxEarnings.Mul(config.CorporateTaxRate)
		result.FiscalSocial.CorporateTaxLiab[yearIdx] = SafeDiv(estimatedTaxLiability.Mul(decimal.NewFromInt(30)), daysPerYear)

		// Total fiscal social
		result.FiscalSocial.TotalFiscalSocial[yearIdx] = result.FiscalSocial.VATLiability[yearIdx].
			Add(result.FiscalSocial.SocialLiability[yearIdx]).
			Add(result.FiscalSocial.CorporateTaxLiab[yearIdx])

		// ===== ADJUSTMENTS (from WCREntry) =====
		result.Adjustments.PrepaidExpenses[yearIdx] = getOrCompute(
			model.WCRPrepaidExpenses, yearIdx, decimal.Zero)
		result.Adjustments.DeferredRevenue[yearIdx] = getOrCompute(
			model.WCRDeferredRevenue, yearIdx, decimal.Zero)
		result.Adjustments.TaxReceivables[yearIdx] = getOrCompute(
			model.WCRTaxReceivables, yearIdx, decimal.Zero)
		result.Adjustments.OtherAdjustPlus[yearIdx] = getOrCompute(
			model.WCROtherAdjustPlus, yearIdx, decimal.Zero)
		result.Adjustments.OtherAdjustMinus[yearIdx] = getOrCompute(
			model.WCROtherAdjustMinus, yearIdx, decimal.Zero)

		result.Adjustments.NetAdjustment[yearIdx] = result.Adjustments.PrepaidExpenses[yearIdx].
			Add(result.Adjustments.DeferredRevenue[yearIdx]).
			Add(result.Adjustments.TaxReceivables[yearIdx]).
			Add(result.Adjustments.OtherAdjustPlus[yearIdx]).
			Sub(result.Adjustments.OtherAdjustMinus[yearIdx])

		// ===== ADJUSTED WCR =====
		result.Adjusted.AdjustedWCR[yearIdx] = result.Summary.BasicWCR[yearIdx].
			Add(result.FiscalSocial.TotalFiscalSocial[yearIdx]).
			Add(result.Adjustments.NetAdjustment[yearIdx])

		result.Adjusted.AdjustedWCRDays[yearIdx] = SafeDiv(result.Adjusted.AdjustedWCR[yearIdx],
			SafeDiv(salesExclVAT, daysPerYear))

		if yearIdx == 0 {
			result.Adjusted.AdjustedWCRChange[yearIdx] = result.Adjusted.AdjustedWCR[yearIdx]
		} else {
			result.Adjusted.AdjustedWCRChange[yearIdx] = result.Adjusted.AdjustedWCR[yearIdx].
				Sub(result.Adjusted.AdjustedWCR[yearIdx-1])
		}
	}

	// Set initial values from opening balance
	result.Customers.InitialTradeRecv = openBal.CustomerReceivables
	result.Inventory.InitialInventory = openBal.Inventories
	result.Suppliers.InitialTradePay = openBal.SupplierPayables
	result.Summary.InitialWCR = openBal.CustomerReceivables.
		Add(openBal.Inventories).
		Sub(openBal.SupplierPayables)
	result.Adjusted.InitialAdjWCR = result.Summary.InitialWCR

	// ===== CHARTS =====
	for yearIdx := 0; yearIdx < MaxYears; yearIdx++ {
		result.Charts.Years[yearIdx] = yearIdx + 1
		result.Charts.CustomerWCR[yearIdx] = result.Summary.CustomerWCR[yearIdx]
		result.Charts.InventoryWCR[yearIdx] = result.Summary.InventoryWCR[yearIdx]
		result.Charts.SupplierWCR[yearIdx] = result.Summary.SupplierWCR[yearIdx]
		result.Charts.FiscalSocialWCR[yearIdx] = result.FiscalSocial.TotalFiscalSocial[yearIdx]
		result.Charts.WCRChange[yearIdx] = result.Summary.WCRChange[yearIdx]
	}

	return result
}
