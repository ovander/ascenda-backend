package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"kerplan/internal/event"
	"kerplan/internal/model"
	"kerplan/internal/pkg/apierror"
	"kerplan/internal/pkg/pagination"
	"kerplan/internal/repo"
)

// PlanService orchestrates plan and scenario CRUD operations.
type PlanService struct {
	planRepo     repo.PlanRepository
	settingsRepo repo.SettingsRepository
	auditRepo    repo.AuditRepository
	repos        *repo.RepoBundle
	emitter      *event.Emitter
	logger       *logrus.Entry
}

// NewPlanService creates a new PlanService.
func NewPlanService(planRepo repo.PlanRepository, settingsRepo repo.SettingsRepository, auditRepo repo.AuditRepository, repos *repo.RepoBundle, emitter *event.Emitter, logger *logrus.Entry) *PlanService {
	return &PlanService{
		planRepo:     planRepo,
		settingsRepo: settingsRepo,
		auditRepo:    auditRepo,
		repos:        repos,
		emitter:      emitter,
		logger:       logger,
	}
}

// ListPlans lists all plans for a tenant with pagination.
func (s *PlanService) ListPlans(ctx context.Context, tenantID uuid.UUID, params pagination.Params) ([]model.BusinessPlan, int64, error) {
	ptrPlans, err := s.planRepo.ListByTenant(tenantID, params.Offset, params.PerPage)
	if err != nil {
		s.logger.WithError(err).Error("failed to list plans")
		return nil, 0, apierror.Internal("failed to list plans")
	}

	plans := make([]model.BusinessPlan, len(ptrPlans))
	for i, p := range ptrPlans {
		plans[i] = *p
	}

	return plans, int64(len(plans)), nil
}

// CreatePlan creates a new business plan with default scenario and configuration.
func (s *PlanService) CreatePlan(ctx context.Context, tenantID, createdBy uuid.UUID, name, description string) (*model.BusinessPlan, error) {
	plan := &model.BusinessPlan{
		TenantScoped: model.TenantScoped{ID: uuid.New()},
		Name:         name,
		Description:  description,
		Status:       "draft",
		CreatedBy:    createdBy,
	}
	plan.TenantID = tenantID

	if err := s.planRepo.Create(plan); err != nil {
		s.logger.WithError(err).Error("failed to create plan")
		return nil, apierror.Internal("failed to create plan")
	}

	// Create default scenario
	scenario := &model.Scenario{
		TenantScoped: model.TenantScoped{ID: uuid.New()},
		PlanID:       plan.ID,
		Name:         "Base",
		IsDefault:    true,
	}
	scenario.TenantID = tenantID

	if err := s.repos.Scenario.Create(scenario); err != nil {
		s.logger.WithError(err).Error("failed to create default scenario")
		return nil, apierror.Internal("failed to create default scenario")
	}

	// Create default PlanConfig
	config := &model.PlanConfig{
		TenantScoped:     model.TenantScoped{ID: uuid.New()},
		ScenarioID:       scenario.ID,
		FirstFiscalYearMonths: 12,
		SalaryMonthsPerYear:  12,
		Country:              "BE",
	}
	config.TenantID = tenantID

	if err := s.settingsRepo.UpsertConfig(config); err != nil {
		s.logger.WithError(err).Error("failed to create default config")
		return nil, apierror.Internal("failed to create default config")
	}

	s.logger.WithField("plan_id", plan.ID).Info("plan created with default scenario and config")
	s.emitter.Publish(event.Event{
		Type:       event.PlanCreated,
		TenantID:   tenantID,
		UserID:     createdBy,
		EntityType: "plan",
		EntityID:   plan.ID,
		Action:     event.ActionCreate,
	})
	return plan, nil
}

// GetPlan retrieves a plan by ID.
func (s *PlanService) GetPlan(ctx context.Context, tenantID, planID uuid.UUID) (*model.BusinessPlan, error) {
	plan, err := s.planRepo.GetByID(tenantID, planID)
	if err != nil || plan == nil || plan.TenantID != tenantID {
		s.logger.WithError(err).Warn("plan not found")
		return nil, apierror.NotFound("plan", planID.String())
	}

	return plan, nil
}

// UpdatePlan updates plan details.
func (s *PlanService) UpdatePlan(ctx context.Context, tenantID, planID uuid.UUID, name, description, status string) error {
	plan, err := s.GetPlan(ctx, tenantID, planID)
	if err != nil {
		return err
	}

	if name != "" {
		plan.Name = name
	}
	if description != "" {
		plan.Description = description
	}
	if status != "" && (status == "draft" || status == "review" || status == "approved" || status == "archived") {
		plan.Status = status
	}

	if err := s.planRepo.Update(plan); err != nil {
		s.logger.WithError(err).Error("failed to update plan")
		return apierror.Internal("failed to update plan")
	}

	s.logger.WithField("plan_id", planID).Info("plan updated")
	s.emitter.Publish(event.Event{
		Type:       event.DataChanged,
		TenantID:   tenantID,
		EntityType: "plan",
		EntityID:   planID,
		Action:     event.ActionUpdate,
	})
	return nil
}

// DeletePlan removes a plan and all associated data.
func (s *PlanService) DeletePlan(ctx context.Context, tenantID, planID uuid.UUID) error {
	if _, err := s.GetPlan(ctx, tenantID, planID); err != nil {
		return err
	}

	if err := s.planRepo.Delete(tenantID, planID); err != nil {
		s.logger.WithError(err).Error("failed to delete plan")
		return apierror.Internal("failed to delete plan")
	}

	s.logger.WithField("plan_id", planID).Info("plan deleted")
	s.emitter.Publish(event.Event{
		Type:       event.PlanDeleted,
		TenantID:   tenantID,
		EntityType: "plan",
		EntityID:   planID,
		Action:     event.ActionDelete,
	})
	return nil
}

// GetScenario retrieves a scenario by ID.
func (s *PlanService) GetScenario(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.Scenario, error) {
	scenario, err := s.repos.Scenario.GetByID(tenantID, scenarioID)
	if err != nil || scenario == nil || scenario.TenantID != tenantID {
		s.logger.WithError(err).Warn("scenario not found")
		return nil, apierror.NotFound("scenario", scenarioID.String())
	}

	return scenario, nil
}

// ListScenarios lists all scenarios for a plan.
func (s *PlanService) ListScenarios(ctx context.Context, tenantID, planID uuid.UUID) ([]model.Scenario, error) {
	ptrScenarios, err := s.repos.Scenario.ListByPlan(tenantID, planID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list scenarios")
		return nil, apierror.Internal("failed to list scenarios")
	}

	scenarios := make([]model.Scenario, len(ptrScenarios))
	for i, s := range ptrScenarios {
		scenarios[i] = *s
	}

	return scenarios, nil
}

// CreateScenario creates a new scenario for a plan.
func (s *PlanService) CreateScenario(ctx context.Context, tenantID, planID uuid.UUID, name string) (*model.Scenario, error) {
	// Verify plan exists
	if _, err := s.GetPlan(ctx, tenantID, planID); err != nil {
		return nil, err
	}

	scenario := &model.Scenario{
		TenantScoped: model.TenantScoped{ID: uuid.New()},
		PlanID:       planID,
		Name:         name,
		IsDefault:    false,
	}
	scenario.TenantID = tenantID

	if err := s.repos.Scenario.Create(scenario); err != nil {
		s.logger.WithError(err).Error("failed to create scenario")
		return nil, apierror.Internal("failed to create scenario")
	}

	// Create default PlanConfig for new scenario
	config := &model.PlanConfig{
		TenantScoped:     model.TenantScoped{ID: uuid.New()},
		ScenarioID:       scenario.ID,
		FirstFiscalYearMonths: 12,
		SalaryMonthsPerYear:  12,
		Country:              "BE",
	}
	config.TenantID = tenantID

	if err := s.settingsRepo.UpsertConfig(config); err != nil {
		s.logger.WithError(err).Error("failed to create config for new scenario")
		return nil, apierror.Internal("failed to create config for new scenario")
	}

	s.logger.WithField("scenario_id", scenario.ID).WithField("plan_id", planID).Info("scenario created")
	s.emitter.Publish(event.Event{
		Type:       event.DataChanged,
		TenantID:   tenantID,
		ScenarioID: scenario.ID,
		EntityType: "scenario",
		EntityID:   scenario.ID,
		Action:     event.ActionCreate,
	})
	return scenario, nil
}

// UpdateScenario updates a scenario's name.
func (s *PlanService) UpdateScenario(ctx context.Context, tenantID, scenarioID uuid.UUID, name string) error {
	scenario, err := s.GetScenario(ctx, tenantID, scenarioID)
	if err != nil {
		return err
	}

	if name != "" {
		scenario.Name = name
	}

	if err := s.repos.Scenario.Update(scenario); err != nil {
		s.logger.WithError(err).Error("failed to update scenario")
		return apierror.Internal("failed to update scenario")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("scenario updated")
	s.emitter.Publish(event.Event{
		Type:       event.DataChanged,
		TenantID:   tenantID,
		ScenarioID: scenarioID,
		EntityType: "scenario",
		EntityID:   scenarioID,
		Action:     event.ActionUpdate,
	})
	return nil
}

// CloneScenario clones a scenario with all its data to a new scenario.
func (s *PlanService) CloneScenario(ctx context.Context, tenantID, sourceScenarioID uuid.UUID, newName string) (*model.Scenario, error) {
	// Get source scenario
	sourceScenario, err := s.repos.Scenario.GetByID(tenantID, sourceScenarioID)
	if err != nil || sourceScenario == nil || sourceScenario.TenantID != tenantID {
		s.logger.WithError(err).Warn("source scenario not found")
		return nil, apierror.NotFound("scenario", sourceScenarioID.String())
	}

	// Create new scenario
	newScenario := &model.Scenario{
		TenantScoped: model.TenantScoped{ID: uuid.New()},
		PlanID:       sourceScenario.PlanID,
		Name:         newName,
		IsDefault:    false,
	}
	newScenario.TenantID = tenantID

	if err := s.repos.Scenario.Create(newScenario); err != nil {
		s.logger.WithError(err).Error("failed to create cloned scenario")
		return nil, apierror.Internal("failed to create cloned scenario")
	}

	// Deep copy all scenario data
	if err := s.deepCopyScenarioData(tenantID, sourceScenarioID, newScenario.ID); err != nil {
		s.logger.WithError(err).Error("failed to copy scenario data")
		return nil, apierror.Internal("failed to copy scenario data")
	}

	s.logger.WithField("source_scenario_id", sourceScenarioID).WithField("new_scenario_id", newScenario.ID).Info("scenario cloned")
	s.emitter.Publish(event.Event{
		Type:       event.DataChanged,
		TenantID:   tenantID,
		ScenarioID: newScenario.ID,
		EntityType: "scenario",
		EntityID:   newScenario.ID,
		Action:     event.ActionCreate,
	})
	return newScenario, nil
}

// DeleteScenario removes a scenario and all associated data.
func (s *PlanService) DeleteScenario(ctx context.Context, tenantID, scenarioID uuid.UUID) error {
	scenario, err := s.repos.Scenario.GetByID(tenantID, scenarioID)
	if err != nil || scenario == nil || scenario.TenantID != tenantID {
		s.logger.WithError(err).Warn("scenario not found")
		return apierror.NotFound("scenario", scenarioID.String())
	}

	if err := s.repos.Scenario.Delete(tenantID, scenarioID); err != nil {
		s.logger.WithError(err).Error("failed to delete scenario")
		return apierror.Internal("failed to delete scenario")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("scenario deleted")
	s.emitter.Publish(event.Event{
		Type:       event.DataChanged,
		TenantID:   tenantID,
		ScenarioID: scenarioID,
		EntityType: "scenario",
		EntityID:   scenarioID,
		Action:     event.ActionDelete,
	})
	return nil
}

// deepCopyScenarioData performs a deep copy of all data from source to destination scenario.
func (s *PlanService) deepCopyScenarioData(tenantID, sourceScenarioID, destScenarioID uuid.UUID) error {
	// Copy Settings
	if config, err := s.settingsRepo.GetConfig(tenantID, sourceScenarioID); err == nil && config != nil {
		newConfig := *config
		newConfig.ID = uuid.New()
		newConfig.ScenarioID = destScenarioID
		_ = s.settingsRepo.UpsertConfig(&newConfig)
	}

	if balance, err := s.settingsRepo.GetOpeningBalance(tenantID, sourceScenarioID); err == nil && balance != nil {
		newBalance := *balance
		newBalance.ID = uuid.New()
		newBalance.ScenarioID = destScenarioID
		_ = s.settingsRepo.UpsertOpeningBalance(&newBalance)
	}

	if wcConfig, err := s.settingsRepo.GetWCConfig(tenantID, sourceScenarioID); err == nil && wcConfig != nil {
		newWCConfig := *wcConfig
		newWCConfig.ID = uuid.New()
		newWCConfig.ScenarioID = destScenarioID
		_ = s.settingsRepo.UpsertWCConfig(&newWCConfig)
	}

	// Copy Products with mapping
	if ptrProducts, err := s.repos.Product.ListProductsByScenario(tenantID, sourceScenarioID); err == nil {
		for _, p := range ptrProducts {
			newProduct := *p
			newProduct.ID = uuid.New()
			newProduct.ScenarioID = destScenarioID
			_ = s.repos.Product.CreateProduct(&newProduct)

			// Copy assumptions
			if ptrAssumptions, err := s.repos.Product.GetAssumptionsByProduct(tenantID, p.ID); err == nil {
				vals := make([]model.ProductAssumption, len(ptrAssumptions))
				for i, a := range ptrAssumptions {
					v := *a
					v.ID = uuid.New()
					v.ProductID = newProduct.ID
					vals[i] = v
				}
				_ = s.repos.Product.BatchUpsertAssumptions(tenantID, destScenarioID, vals)
			}

			// Copy volumes
			if ptrVolumes, err := s.repos.Product.GetVolumesByProduct(tenantID, p.ID); err == nil {
				vals := make([]model.ProductSalesVolume, len(ptrVolumes))
				for i, v := range ptrVolumes {
					nv := *v
					nv.ID = uuid.New()
					nv.ProductID = newProduct.ID
					vals[i] = nv
				}
				_ = s.repos.Product.BatchUpsertVolumes(tenantID, destScenarioID, vals)
			}

			// Copy margins
			if ptrMargins, err := s.repos.Product.GetMarginsByProduct(tenantID, p.ID); err == nil {
				vals := make([]model.ProductDistributorMargin, len(ptrMargins))
				for i, m := range ptrMargins {
					nm := *m
					nm.ID = uuid.New()
					nm.ProductID = newProduct.ID
					vals[i] = nm
				}
				_ = s.repos.Product.BatchUpsertMargins(tenantID, destScenarioID, vals)
			}
		}
	}

	// Copy Staff
	if ptrHC, err := s.repos.Staff.ListHeadcountsByScenario(tenantID, sourceScenarioID); err == nil {
		vals := make([]model.StaffHeadcount, len(ptrHC))
		for i, h := range ptrHC {
			v := *h
			v.ID = uuid.New()
			v.ScenarioID = destScenarioID
			vals[i] = v
		}
		_ = s.repos.Staff.BatchUpsertHeadcounts(tenantID, destScenarioID, vals)
	}

	if ptrSal, err := s.repos.Staff.ListSalariesByScenario(tenantID, sourceScenarioID); err == nil {
		vals := make([]model.StaffSalary, len(ptrSal))
		for i, s := range ptrSal {
			v := *s
			v.ID = uuid.New()
			v.ScenarioID = destScenarioID
			vals[i] = v
		}
		_ = s.repos.Staff.BatchUpsertSalaries(tenantID, destScenarioID, vals)
	}

	if ptrInc, err := s.repos.Staff.ListIncentivesByScenario(tenantID, sourceScenarioID); err == nil {
		vals := make([]model.StaffIncentive, len(ptrInc))
		for i, inc := range ptrInc {
			v := *inc
			v.ID = uuid.New()
			v.ScenarioID = destScenarioID
			vals[i] = v
		}
		_ = s.repos.Staff.BatchUpsertIncentives(tenantID, destScenarioID, vals)
	}

	// Copy Capex, Opex, PnL, FiPlan, PnlCash, WCR, Cash, Budget
	if ptr, err := s.repos.Capex.ListByScenario(tenantID, sourceScenarioID); err == nil {
		vals := make([]model.CapexEntry, len(ptr))
		for i, e := range ptr {
			v := *e; v.ID = uuid.New(); v.ScenarioID = destScenarioID; vals[i] = v
		}
		_ = s.repos.Capex.BatchUpsert(tenantID, destScenarioID, vals)
	}

	if ptr, err := s.repos.Opex.ListByScenario(tenantID, sourceScenarioID); err == nil {
		vals := make([]model.OpexManualEntry, len(ptr))
		for i, e := range ptr {
			v := *e; v.ID = uuid.New(); v.ScenarioID = destScenarioID; vals[i] = v
		}
		_ = s.repos.Opex.BatchUpsert(tenantID, destScenarioID, vals)
	}

	if ptr, err := s.repos.PnL.ListByScenario(tenantID, sourceScenarioID); err == nil {
		vals := make([]model.PnlManualEntry, len(ptr))
		for i, e := range ptr {
			v := *e; v.ID = uuid.New(); v.ScenarioID = destScenarioID; vals[i] = v
		}
		_ = s.repos.PnL.BatchUpsert(tenantID, destScenarioID, vals)
	}

	if ptr, err := s.repos.FiPlan.ListByScenario(tenantID, sourceScenarioID); err == nil {
		vals := make([]model.FiplanEntry, len(ptr))
		for i, e := range ptr {
			v := *e; v.ID = uuid.New(); v.ScenarioID = destScenarioID; vals[i] = v
		}
		_ = s.repos.FiPlan.BatchUpsert(tenantID, destScenarioID, vals)
	}

	if ptr, err := s.repos.PnlCash.ListByScenario(tenantID, sourceScenarioID); err == nil {
		vals := make([]model.PnlCashEntry, len(ptr))
		for i, e := range ptr {
			v := *e; v.ID = uuid.New(); v.ScenarioID = destScenarioID; vals[i] = v
		}
		_ = s.repos.PnlCash.BatchUpsert(tenantID, destScenarioID, vals)
	}

	if ptr, err := s.repos.WCR.ListByScenario(tenantID, sourceScenarioID); err == nil {
		vals := make([]model.WCREntry, len(ptr))
		for i, e := range ptr {
			v := *e; v.ID = uuid.New(); v.ScenarioID = destScenarioID; vals[i] = v
		}
		_ = s.repos.WCR.BatchUpsert(tenantID, destScenarioID, vals)
	}

	if ptr, err := s.repos.Cash.ListByScenario(tenantID, sourceScenarioID); err == nil {
		vals := make([]model.CashMonthlyOverride, len(ptr))
		for i, e := range ptr {
			v := *e; v.ID = uuid.New(); v.ScenarioID = destScenarioID; vals[i] = v
		}
		_ = s.repos.Cash.BatchUpsert(tenantID, destScenarioID, vals)
	}

	// Copy budget for all years
	for year := 1; year <= 5; year++ {
		if ptr, err := s.repos.Budget.ListByScenario(tenantID, sourceScenarioID, year); err == nil {
			vals := make([]model.BudgetMonthlyOverride, len(ptr))
			for i, e := range ptr {
				v := *e; v.ID = uuid.New(); v.ScenarioID = destScenarioID; vals[i] = v
			}
			_ = s.repos.Budget.BatchUpsert(tenantID, destScenarioID, vals)
		}
	}

	return nil
}
