package compute

import (
	"kerplan/internal/model"
)

// FullPlanInput aggregates all inputs needed for full plan computation
type FullPlanInput struct {
	// Configuration
	Config         model.PlanConfig
	OpeningBalance model.OpeningBalance
	WCConfig       model.WorkingCapitalConfig
	OpexPerHire    model.OpexPerHire
	CapexPerHire   model.CapexPerHire

	// Products
	Products    []model.Product
	ProductData []ProductInputBundle

	// Staff
	Headcounts []model.StaffHeadcount
	Salaries   []model.StaffSalary
	Incentives []model.StaffIncentive

	// Capex & Opex
	CapexEntries []model.CapexEntry
	OpexEntries  []model.OpexManualEntry

	// P&L & Finance
	PnlEntries     []model.PnlManualEntry
	FiplanEntries  []model.FiplanEntry
	PnlCashEntries []model.PnlCashEntry
	WCREntries     []model.WCREntry

	// Monthly Overrides
	CashOverrides   []model.CashMonthlyOverride
	BudgetOverrides []model.BudgetMonthlyOverride
}

// ComputeFullPlan orchestrates the 7-layer dependency graph
// Layer 1: Revenue, Payroll, Capex (independent), Opex (depends on Revenue + Payroll)
// Layer 2: P&L (depends on Layer 1 + empty FiPlan)
// Layer 3: WCR, FiPlan, PnlCash (depends on Layer 1 + PnL)
// Layer 4: BSheet (depends on PnL + WCR + Capex + FiPlan)
// Layer 5: Ratios (depends on all above)
// Layer 6: Cash (depends on PnL + WCR + Capex + FiPlan + Revenue + Staff + Opex)
// Layer 7: Budget1 + Budget2 (depends on PnL + Opex + Staff + Capex)
// Final: Validation warnings
func ComputeFullPlan(input FullPlanInput) model.FullPlanOutput {
	output := model.FullPlanOutput{}

	// LAYER 1: Independent computations
	// ================================

	// Compute revenue
	output.Revenue = ComputeConsolidatedRevenue(
		input.Products,
		input.ProductData,
		input.Config,
	)

	// Compute staff payroll
	output.Payroll = ComputeStaffPayroll(
		input.Headcounts,
		input.Salaries,
		input.Incentives,
		input.Config,
	)

	// Compute capex
	output.Capex = ComputeCapexSummary(
		input.CapexEntries,
		input.Config,
	)

	// Compute opex (depends on Revenue and Payroll)
	output.Opex = ComputeOpexSummary(
		input.OpexEntries,
		input.OpexPerHire,
		output.Revenue,
		output.Payroll,
		input.Config,
	)

	// LAYER 2: P&L computation (depends on Layer 1)
	// =============================================

	output.PnL = ComputePnl(
		input.PnlEntries,
		output.Revenue,
		output.Payroll,
		output.Capex,
		output.Opex,
		model.FiplanReport{}, // Will be computed in Layer 3, pass empty for now
		input.Config,
	)

	// LAYER 3: Finance statements (depend on Layer 1 + Layer 2)
	// ========================================================

	output.WCR = ComputeWCR(
		input.WCREntries,
		output.Revenue,
		output.Opex,
		output.Payroll,
		input.WCConfig,
		input.OpeningBalance,
		output.Capex,
		output.PnL,
		input.Config,
	)

	output.FiPlan = ComputeFiplan(
		input.FiplanEntries,
		output.Capex,
		output.PnL,
		output.WCR,
		input.OpeningBalance,
		input.Config,
	)

	output.PnlCash = ComputePnlCash(
		input.PnlCashEntries,
		output.Revenue,
		output.Opex,
		output.Payroll,
		output.Capex,
		output.PnL,
		input.Config,
	)

	// LAYER 4: Balance sheet (depends on PnL + WCR + Capex + FiPlan)
	// =============================================================

	output.BSheet = ComputeBSheet(
		output.PnL,
		output.WCR,
		output.Capex,
		output.FiPlan,
		input.OpeningBalance,
		input.Config,
	)

	// LAYER 5: Ratios (depends on all previous layers)
	// ==============================================

	output.Ratios = ComputeRatios(
		output.PnL,
		output.BSheet,
		output.WCR,
		output.FiPlan,
		output.Revenue,
		output.Payroll,
		output.Capex,
		input.Config,
	)

	// LAYER 6: Cash (depends on PnL + WCR + Capex + FiPlan + Revenue + Staff + Opex)
	// =============================================================================

	output.Cash = ComputeCash(
		input.CashOverrides,
		output.PnL,
		output.WCR,
		output.Capex,
		output.FiPlan,
		output.Revenue,
		output.Payroll,
		output.Opex,
		input.Config,
	)

	// LAYER 7: Budgets (depends on PnL + Opex + Staff + Capex)
	// =======================================================

	output.Budget1 = ComputeBudget1(
		input.BudgetOverrides,
		output.PnL,
		output.Opex,
		output.Payroll,
		output.Capex,
		input.Config,
	)

	output.Budget2 = ComputeBudget2(
		output.PnL,
		output.Opex,
		output.Payroll,
		input.Config,
	)

	// FINAL: Validation
	// ================

	output.Warnings = ValidateOutput(output)

	return output
}
