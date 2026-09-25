package service

import (
	"ascenda/internal/compute"
	"ascenda/internal/model"
	"ascenda/internal/repo"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/sirupsen/logrus"
)

// PlanComputeOrchestrator assembles a FullPlanInput from the repository layer
// and runs ComputeFullPlan.  It is the single source of truth for "how all
// plan data is loaded and fed into the compute engine."
//
// Both ReportService (cached, HTTP-facing) and ScenarioAnalysisService
// (decision-intelligence layer) delegate to this type, avoiding duplication
// of the complex multi-repo loading logic.
type PlanComputeOrchestrator struct {
	repos  *repo.RepoBundle
	logger *logrus.Entry
}

// NewPlanComputeOrchestrator creates a PlanComputeOrchestrator.
func NewPlanComputeOrchestrator(repos *repo.RepoBundle, logger *logrus.Entry) *PlanComputeOrchestrator {
	return &PlanComputeOrchestrator{repos: repos, logger: logger}
}

// Compute loads all scenario inputs and runs the full compute engine.
// Callers that want caching should go through ReportService.GetFullReport.
// This method is intentionally cache-free so it can be used in contexts where
// fresh results are required (e.g. analysis after a data mutation).
func (o *PlanComputeOrchestrator) Compute(tenantID, scenarioID uuid.UUID) (*model.FullPlanOutput, error) {
	input, err := o.LoadInputs(tenantID, scenarioID)
	if err != nil {
		return nil, err
	}
	result := compute.ComputeFullPlan(*input)
	return &result, nil
}

// ComputeFromInput runs the compute engine on a pre-loaded FullPlanInput.
// This enables in-memory perturbations (e.g. for sensitivity analysis) without
// incurring additional database round-trips: call LoadInputs once, then mutate
// the input struct and call ComputeFromInput for each perturbation.
func (o *PlanComputeOrchestrator) ComputeFromInput(input compute.FullPlanInput) *model.FullPlanOutput {
	result := compute.ComputeFullPlan(input)
	return &result
}

// LoadInputs loads all necessary data from repositories for a complete plan
// computation.  It is exported so that callers that need the raw inputs (e.g.
// for driver heuristics that inspect assumptions directly) can access them
// without running the full compute engine again.
func (o *PlanComputeOrchestrator) LoadInputs(tenantID, scenarioID uuid.UUID) (*compute.FullPlanInput, error) {
	input := &compute.FullPlanInput{}

	// ── Configuration ─────────────────────────────────────────────────────
	config, err := o.repos.Settings.GetConfig(tenantID, scenarioID)
	if err != nil {
		o.logger.WithError(err).Error("orchestrator: failed to load config")
		return nil, apierror.Internal("failed to load plan config")
	}
	input.Config = *config

	openingBalance, err := o.repos.Settings.GetOpeningBalance(tenantID, scenarioID)
	if err == nil && openingBalance != nil {
		input.OpeningBalance = *openingBalance
	}

	wcConfig, err := o.repos.Settings.GetWCConfig(tenantID, scenarioID)
	if err == nil && wcConfig != nil {
		input.WCConfig = *wcConfig
	}

	// OpexPerHire — fall back to defaults when the stored row is all-zero
	// (rows created before the defaults feature was introduced).
	opexPerHire, err := o.repos.Settings.GetOpexPerHire(tenantID, scenarioID)
	if err != nil || opexPerHire == nil || isZeroOpexPerHire(opexPerHire) {
		input.OpexPerHire = *defaultOpexPerHire(tenantID, scenarioID)
	} else {
		input.OpexPerHire = *opexPerHire
	}

	capexPerHire, err := o.repos.Settings.GetCapexPerHire(tenantID, scenarioID)
	if err == nil && capexPerHire != nil {
		input.CapexPerHire = *capexPerHire
	}

	// ── Products ──────────────────────────────────────────────────────────
	ptrProducts, err := o.repos.Product.ListProductsByScenario(tenantID, scenarioID)
	if err == nil {
		input.Products = make([]model.Product, len(ptrProducts))
		for i, p := range ptrProducts {
			input.Products[i] = *p
		}

		// Scenario-level facts some drivers read from other products
		// (contract bonuses are paid on the competition wins).
		driverCtx := compute.BuildDriverContext(input.Products)

		input.ProductData = make([]compute.ProductInputBundle, len(input.Products))
		for i, product := range input.Products {
			bundle := compute.ProductInputBundle{}

			assumptions, err := o.repos.Product.GetAssumptionsByProduct(tenantID, product.ID)
			if err == nil {
				for _, a := range assumptions {
					yearIdx := a.YearIndex - 1
					if yearIdx >= 0 && yearIdx < compute.MaxYears {
						bundle.Assumptions[yearIdx] = *a
					}
				}
			}

			volumes, err := o.repos.Product.GetVolumesByProduct(tenantID, product.ID)
			if err == nil {
				for _, v := range volumes {
					bundle.Volumes = append(bundle.Volumes, *v)
				}
			}

			margins, err := o.repos.Product.GetMarginsByProduct(tenantID, product.ID)
			if err == nil {
				for _, m := range margins {
					bundle.Margins = append(bundle.Margins, *m)
				}
			}

			if derived, dErr := compute.ApplyDriverComputeWithContext(product, bundle, driverCtx); dErr != nil {
				o.logger.WithError(dErr).WithField("product_id", product.ID).
					Warn("orchestrator: driver compute failed — falling back to stored assumptions")
			} else {
				bundle = derived
			}

			input.ProductData[i] = bundle
		}
	}

	// ── Staff ─────────────────────────────────────────────────────────────
	ptrHeadcounts, err := o.repos.Staff.ListHeadcountsByScenario(tenantID, scenarioID)
	if err == nil {
		input.Headcounts = make([]model.StaffHeadcount, len(ptrHeadcounts))
		for i, h := range ptrHeadcounts {
			input.Headcounts[i] = *h
		}
	}

	ptrSalaries, err := o.repos.Staff.ListSalariesByScenario(tenantID, scenarioID)
	if err == nil {
		input.Salaries = make([]model.StaffSalary, len(ptrSalaries))
		for i, s := range ptrSalaries {
			input.Salaries[i] = *s
		}
	}

	ptrIncentives, err := o.repos.Staff.ListIncentivesByScenario(tenantID, scenarioID)
	if err == nil {
		input.Incentives = make([]model.StaffIncentive, len(ptrIncentives))
		for i, inc := range ptrIncentives {
			input.Incentives[i] = *inc
		}
	}

	// ── Capex / Opex ──────────────────────────────────────────────────────
	ptrCapexEntries, err := o.repos.Capex.ListByScenario(tenantID, scenarioID)
	if err == nil {
		input.CapexEntries = make([]model.CapexEntry, len(ptrCapexEntries))
		for i, e := range ptrCapexEntries {
			input.CapexEntries[i] = *e
		}
	}

	ptrOpexEntries, err := o.repos.Opex.ListByScenario(tenantID, scenarioID)
	if err == nil {
		input.OpexEntries = make([]model.OpexManualEntry, len(ptrOpexEntries))
		for i, e := range ptrOpexEntries {
			input.OpexEntries[i] = *e
		}
	}

	// ── P&L / Finance ─────────────────────────────────────────────────────
	ptrPnlEntries, err := o.repos.PnL.ListByScenario(tenantID, scenarioID)
	if err == nil {
		input.PnlEntries = make([]model.PnlManualEntry, len(ptrPnlEntries))
		for i, e := range ptrPnlEntries {
			input.PnlEntries[i] = *e
		}
	}

	ptrFiplanEntries, err := o.repos.FiPlan.ListByScenario(tenantID, scenarioID)
	if err == nil {
		input.FiplanEntries = make([]model.FiplanEntry, len(ptrFiplanEntries))
		for i, e := range ptrFiplanEntries {
			input.FiplanEntries[i] = *e
		}
	}

	ptrPnlCashEntries, err := o.repos.PnlCash.ListByScenario(tenantID, scenarioID)
	if err == nil {
		input.PnlCashEntries = make([]model.PnlCashEntry, len(ptrPnlCashEntries))
		for i, e := range ptrPnlCashEntries {
			input.PnlCashEntries[i] = *e
		}
	}

	ptrWcrEntries, err := o.repos.WCR.ListByScenario(tenantID, scenarioID)
	if err == nil {
		input.WCREntries = make([]model.WCREntry, len(ptrWcrEntries))
		for i, e := range ptrWcrEntries {
			input.WCREntries[i] = *e
		}
	}

	// ── Monthly Overrides ─────────────────────────────────────────────────
	ptrCashOverrides, err := o.repos.Cash.ListByScenario(tenantID, scenarioID)
	if err == nil {
		input.CashOverrides = make([]model.CashMonthlyOverride, len(ptrCashOverrides))
		for i, c := range ptrCashOverrides {
			input.CashOverrides[i] = *c
		}
	}

	var allBudgets []model.BudgetMonthlyOverride
	for year := 1; year <= compute.MaxYears; year++ {
		ptrEntries, err := o.repos.Budget.ListByScenario(tenantID, scenarioID, year)
		if err == nil {
			for _, e := range ptrEntries {
				allBudgets = append(allBudgets, *e)
			}
		}
	}
	input.BudgetOverrides = allBudgets

	return input, nil
}
