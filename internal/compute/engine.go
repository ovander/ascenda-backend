package compute

import (
	"sync"

	"ascenda/internal/model"
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
//
// Layer 1  : Revenue, Payroll, Capex (independent), Opex (depends on Revenue + Payroll)
// Layer 2a : P&L first pass — empty FiPlan/WCR (breaks the circular dependency)
// Layer 3  : WCR, FiPlan (depend on Layer 1 + Layer 2a P&L)
// Layer 2b : P&L second pass — real WCR inventory change, FiPlan loan interest & grants
// Layer 3b : FiPlan re-run — uses updated P&L CashFlow for accurate Requirements table
// Layer 3c : PnlCash (depends on Layer 1 + final P&L)
// Layer 4  : BSheet (depends on P&L + WCR + Capex + FiPlan)
// Layer 5  : Ratios (depends on all above)
// Layer 6  : Cash (depends on P&L + WCR + Capex + FiPlan + Revenue + Staff + Opex)
// Layer 7  : Budget1 + Budget2 (depends on P&L + Opex + Staff + Capex)
// Final    : Validation warnings
func ComputeFullPlan(input FullPlanInput) model.FullPlanOutput {
	output := model.FullPlanOutput{}

	// LAYER 1: Independent computations — Revenue, Payroll, Capex run in parallel.
	// Opex depends on Revenue + Payroll so it runs after the WaitGroup completes.
	// ============================================================================
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		output.Revenue = ComputeConsolidatedRevenue(
			input.Products,
			input.ProductData,
			input.Config,
		)
	}()

	go func() {
		defer wg.Done()
		output.Payroll = ComputeStaffPayroll(
			input.Headcounts,
			input.Salaries,
			input.Incentives,
			input.Config,
		)
	}()

	go func() {
		defer wg.Done()
		output.Capex = ComputeCapexSummary(
			input.CapexEntries,
			input.Config,
		)
	}()

	wg.Wait()

	// Compute opex (depends on Revenue and Payroll — must follow wg.Wait)
	output.Opex = ComputeOpexSummary(
		input.OpexEntries,
		input.OpexPerHire,
		output.Revenue,
		output.Payroll,
		input.Config,
	)

	// LAYER 2a: P&L first pass — empty FiPlan and WCR (circular dependency bootstrap)
	// =================================================================================
	// FiPlan needs P&L.CashFlow; P&L needs FiPlan.LoanInterest and WCR.InventoryValue.
	// We break the cycle with a first-pass using zero values, then recompute after
	// Layer 3 has produced the real WCR and FiPlan.

	output.PnL = ComputePnl(
		input.PnlEntries,
		output.Revenue,
		output.Payroll,
		output.Capex,
		output.Opex,
		model.FiplanReport{}, // placeholder — will be replaced in Layer 2b
		model.WCRReport{},    // placeholder — will be replaced in Layer 2b
		input.Config,
	)

	// LAYER 3: Finance statements (depend on Layer 1 + Layer 2a P&L)
	// ===============================================================

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

	// LAYER 2b: P&L second pass — now with real WCR and FiPlan
	// =========================================================
	// Wires: StoredProduction (WCR inventory Δ), GrantsOtherRevenue (FiPlan
	// subsidies+otherGrants), FinancialExpenses (FiPlan loan interest).

	output.PnL = ComputePnl(
		input.PnlEntries,
		output.Revenue,
		output.Payroll,
		output.Capex,
		output.Opex,
		output.FiPlan,
		output.WCR,
		input.Config,
	)

	// LAYER 3b: FiPlan re-run with the updated P&L CashFlow
	// =====================================================
	// The Requirements.NegativeCashFlow / Resources.PositiveCashFlow rows in
	// FiPlan depend on P&L.CashFlow.  Now that we have the final P&L we do one
	// more FiPlan pass so the Financing table is accurate.

	output.FiPlan = ComputeFiplan(
		input.FiplanEntries,
		output.Capex,
		output.PnL,
		output.WCR,
		input.OpeningBalance,
		input.Config,
	)

	// LAYER 3c: PnlCash (depends on Layer 1 + final P&L)
	// ===================================================

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
