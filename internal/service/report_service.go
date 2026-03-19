package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"kerplan/internal/compute"
	"kerplan/internal/model"
	"kerplan/internal/pkg/apierror"
	"kerplan/internal/repo"
)

// ReportService orchestrates full plan computation with optional caching.
type ReportService struct {
	repos  *repo.RepoBundle
	cache  *ReportCache
	logger *logrus.Entry
}

// NewReportService creates a new ReportService.
func NewReportService(repos *repo.RepoBundle, logger *logrus.Entry) *ReportService {
	return &ReportService{
		repos:  repos,
		logger: logger,
	}
}

// SetCache attaches a ReportCache. Called after the cache is created in ServiceBundle.
func (s *ReportService) SetCache(cache *ReportCache) {
	s.cache = cache
}

// GetFullReport loads all data and computes the complete plan.
// Results are cached per scenario and invalidated on data mutations.
func (s *ReportService) GetFullReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.FullPlanOutput, error) {
	// Check cache first
	if s.cache != nil {
		if cached, ok := s.cache.Get(scenarioID); ok {
			s.logger.WithField("scenario_id", scenarioID).Debug("report cache hit")
			return cached, nil
		}
	}

	input, err := s.loadAllInputs(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to load all inputs")
		return nil, apierror.Internal("failed to load all inputs")
	}

	// Call compute engine
	result := compute.ComputeFullPlan(*input)

	// Store in cache
	if s.cache != nil {
		s.cache.Put(scenarioID, &result)
	}

	s.logger.WithField("scenario_id", scenarioID).Info("full plan report computed")
	return &result, nil
}

// loadAllInputs loads all necessary data from repos for complete plan computation.
func (s *ReportService) loadAllInputs(tenantID, scenarioID uuid.UUID) (*compute.FullPlanInput, error) {
	input := &compute.FullPlanInput{}

	// Load PlanConfig
	config, err := s.repos.Settings.GetConfig(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to load config")
		return nil, apierror.Internal("failed to load config")
	}
	input.Config = *config

	// Load OpeningBalance
	openingBalance, err := s.repos.Settings.GetOpeningBalance(tenantID, scenarioID)
	if err == nil && openingBalance != nil {
		input.OpeningBalance = *openingBalance
	}

	// Load WorkingCapitalConfig
	wcConfig, err := s.repos.Settings.GetWCConfig(tenantID, scenarioID)
	if err == nil && wcConfig != nil {
		input.WCConfig = *wcConfig
	}

	// Load OpexPerHire
	opexPerHire, err := s.repos.Settings.GetOpexPerHire(tenantID, scenarioID)
	if err == nil && opexPerHire != nil {
		input.OpexPerHire = *opexPerHire
	}

	// Load CapexPerHire (stored separately)
	capexPerHire, err := s.repos.Settings.GetCapexPerHire(tenantID, scenarioID)
	if err == nil && capexPerHire != nil {
		input.CapexPerHire = *capexPerHire
	}

	// Load Products
	ptrProducts, err := s.repos.Product.ListProductsByScenario(tenantID, scenarioID)
	if err == nil {
		input.Products = make([]model.Product, len(ptrProducts))
		for i, p := range ptrProducts {
			input.Products[i] = *p
		}
		// Load product data for each product
		input.ProductData = make([]compute.ProductInputBundle, len(input.Products))
		for i, product := range input.Products {
			bundle := compute.ProductInputBundle{}

			// Load assumptions
			assumptions, err := s.repos.Product.GetAssumptionsByProduct(tenantID, product.ID)
			if err == nil {
				for _, a := range assumptions {
					yearIdx := a.YearIndex - 1
					if yearIdx >= 0 && yearIdx < compute.MaxYears {
						bundle.Assumptions[yearIdx] = *a
					}
				}
			}

			// Load volumes (all zone×channel combinations)
			volumes, err := s.repos.Product.GetVolumesByProduct(tenantID, product.ID)
			if err == nil {
				for _, v := range volumes {
					bundle.Volumes = append(bundle.Volumes, *v)
				}
			}

			// Load margins (per zone per year)
			margins, err := s.repos.Product.GetMarginsByProduct(tenantID, product.ID)
			if err == nil {
				for _, m := range margins {
					bundle.Margins = append(bundle.Margins, *m)
				}
			}

			input.ProductData[i] = bundle
		}
	}

	// Load Staff
	ptrHeadcounts, err := s.repos.Staff.ListHeadcountsByScenario(tenantID, scenarioID)
	if err == nil {
		input.Headcounts = make([]model.StaffHeadcount, len(ptrHeadcounts))
		for i, h := range ptrHeadcounts {
			input.Headcounts[i] = *h
		}
	}

	ptrSalaries, err := s.repos.Staff.ListSalariesByScenario(tenantID, scenarioID)
	if err == nil {
		input.Salaries = make([]model.StaffSalary, len(ptrSalaries))
		for i, s := range ptrSalaries {
			input.Salaries[i] = *s
		}
	}

	ptrIncentives, err := s.repos.Staff.ListIncentivesByScenario(tenantID, scenarioID)
	if err == nil {
		input.Incentives = make([]model.StaffIncentive, len(ptrIncentives))
		for i, inc := range ptrIncentives {
			input.Incentives[i] = *inc
		}
	}

	// Load Capex
	ptrCapexEntries, err := s.repos.Capex.ListByScenario(tenantID, scenarioID)
	if err == nil {
		input.CapexEntries = make([]model.CapexEntry, len(ptrCapexEntries))
		for i, e := range ptrCapexEntries {
			input.CapexEntries[i] = *e
		}
	}

	// Load Opex
	ptrOpexEntries, err := s.repos.Opex.ListByScenario(tenantID, scenarioID)
	if err == nil {
		input.OpexEntries = make([]model.OpexManualEntry, len(ptrOpexEntries))
		for i, e := range ptrOpexEntries {
			input.OpexEntries[i] = *e
		}
	}

	// Load PnL
	ptrPnlEntries, err := s.repos.PnL.ListByScenario(tenantID, scenarioID)
	if err == nil {
		input.PnlEntries = make([]model.PnlManualEntry, len(ptrPnlEntries))
		for i, e := range ptrPnlEntries {
			input.PnlEntries[i] = *e
		}
	}

	// Load FiPlan
	ptrFiplanEntries, err := s.repos.FiPlan.ListByScenario(tenantID, scenarioID)
	if err == nil {
		input.FiplanEntries = make([]model.FiplanEntry, len(ptrFiplanEntries))
		for i, e := range ptrFiplanEntries {
			input.FiplanEntries[i] = *e
		}
	}

	// Load PnL Cash
	ptrPnlCashEntries, err := s.repos.PnlCash.ListByScenario(tenantID, scenarioID)
	if err == nil {
		input.PnlCashEntries = make([]model.PnlCashEntry, len(ptrPnlCashEntries))
		for i, e := range ptrPnlCashEntries {
			input.PnlCashEntries[i] = *e
		}
	}

	// Load WCR
	ptrWcrEntries, err := s.repos.WCR.ListByScenario(tenantID, scenarioID)
	if err == nil {
		input.WCREntries = make([]model.WCREntry, len(ptrWcrEntries))
		for i, e := range ptrWcrEntries {
			input.WCREntries[i] = *e
		}
	}

	// Load Cash Overrides
	ptrCashOverrides, err := s.repos.Cash.ListByScenario(tenantID, scenarioID)
	if err == nil {
		input.CashOverrides = make([]model.CashMonthlyOverride, len(ptrCashOverrides))
		for i, c := range ptrCashOverrides {
			input.CashOverrides[i] = *c
		}
	}

	// Load Budget Overrides (all years)
	var allBudgets []model.BudgetMonthlyOverride
	for year := 1; year <= compute.MaxYears; year++ {
		ptrEntries, err := s.repos.Budget.ListByScenario(tenantID, scenarioID, year)
		if err == nil {
			for _, e := range ptrEntries {
				allBudgets = append(allBudgets, *e)
			}
		}
	}
	input.BudgetOverrides = allBudgets

	return input, nil
}
