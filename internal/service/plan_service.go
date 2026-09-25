package service

import (
	"context"
	"fmt"

	"ascenda/internal/event"
	"ascenda/internal/model"
	"ascenda/internal/repo"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/pagination"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// PlanService orchestrates plan and scenario CRUD operations.
type PlanService struct {
	planRepo     repo.PlanRepository
	settingsRepo repo.SettingsRepository
	auditRepo    repo.AuditRepository
	// deps is the narrow dependency interface; the real app injects *repo.RepoBundle,
	// tests can inject any struct that implements repo.PlanDeps.
	deps             repo.PlanDeps
	countryRateSvc   *CountryRateConfigService // nil-safe: falls back to hard-coded defaults
	featurePolicySvc *FeaturePolicyService     // nil-safe: falls back to hard-coded limits
	emitter          *event.Emitter
	logger           *logrus.Entry
}

// NewPlanService creates a new PlanService.
func NewPlanService(planRepo repo.PlanRepository, settingsRepo repo.SettingsRepository, auditRepo repo.AuditRepository, deps repo.PlanDeps, countryRateSvc *CountryRateConfigService, emitter *event.Emitter, logger *logrus.Entry) *PlanService {
	return &PlanService{
		planRepo:       planRepo,
		settingsRepo:   settingsRepo,
		auditRepo:      auditRepo,
		deps:           deps,
		countryRateSvc: countryRateSvc,
		emitter:        emitter,
		logger:         logger,
	}
}

// WithFeaturePolicyService attaches the FeaturePolicyService so that plan/scenario
// limits are read from the DB rather than hardcoded switch statements.
// Called during service-bundle construction after both services are created.
func (s *PlanService) WithFeaturePolicyService(fp *FeaturePolicyService) *PlanService {
	s.featurePolicySvc = fp
	return s
}

// ListPlans lists all plans for a tenant with pagination.
// The returned total reflects the full DB count for the tenant, not merely the
// length of the current page (fixes the pagination total count bug).
func (s *PlanService) ListPlans(ctx context.Context, tenantID uuid.UUID, params pagination.Params) ([]model.BusinessPlan, int64, error) {
	total, err := s.planRepo.CountByTenant(tenantID)
	if err != nil {
		s.logger.WithError(err).Error("failed to count plans")
		return nil, 0, apierror.Internal("failed to count plans")
	}

	ptrPlans, err := s.planRepo.ListByTenant(tenantID, params.Offset, params.PerPage)
	if err != nil {
		s.logger.WithError(err).Error("failed to list plans")
		return nil, 0, apierror.Internal("failed to list plans")
	}

	plans := make([]model.BusinessPlan, len(ptrPlans))
	for i, p := range ptrPlans {
		plans[i] = *p
	}

	return plans, total, nil
}

// defaultPlanConfig returns a PlanConfig seeded with country-specific statutory
// rates. Uses DB-backed rates when available, falls back to hard-coded defaults.
func (s *PlanService) defaultPlanConfig(tenantID, scenarioID uuid.UUID, country string) *model.PlanConfig {
	var rates countryRates
	if s.countryRateSvc != nil {
		rates = s.countryRateSvc.RatesFor(country)
	} else {
		rates = ratesFor(country)
	}
	return defaultPlanConfigFromRates(tenantID, scenarioID, country, rates)
}

// countryForPlan returns the country code from the plan's default scenario config.
// Falls back to "BE" if nothing can be found.
func (s *PlanService) countryForPlan(tenantID, planID uuid.UUID) string {
	scenarios, err := s.deps.GetScenario().ListByPlan(tenantID, planID)
	if err != nil || len(scenarios) == 0 {
		return "BE"
	}
	// Prefer the scenario flagged as default; otherwise take the first one.
	ref := scenarios[0]
	for _, sc := range scenarios {
		if sc.IsDefault {
			ref = sc
			break
		}
	}
	cfg, err := s.settingsRepo.GetConfig(tenantID, ref.ID)
	if err != nil || cfg == nil || cfg.Country == "" {
		return "BE"
	}
	return cfg.Country
}

// planLimitFor returns the maximum number of business plans allowed for a given
// commercial plan tier, consulting the FeaturePolicyService when available and
// falling back to hardcoded defaults for tests that don't inject the service.
func (s *PlanService) planLimitFor(userPlan string) int64 {
	if s.featurePolicySvc != nil {
		n := s.featurePolicySvc.NumericLimit(model.FeatureMaxPlans, userPlan)
		return int64(n)
	}
	// Hardcoded fallback (used by unit tests that don't inject featurePolicySvc).
	return planLimit(userPlan)
}

// planLimit is the hardcoded fallback used by tests.
func planLimit(userPlan string) int64 {
	switch userPlan {
	case "freemium":
		return 1
	case "pro":
		return 3
	default:
		return -1
	}
}

// CreatePlan creates a new business plan with default scenario and configuration.
// Plan count limits are read from the feature_policies table when available.
func (s *PlanService) CreatePlan(ctx context.Context, tenantID, createdBy uuid.UUID, name, description, country string) (*model.BusinessPlan, error) {
	if limit := s.planLimitFor(ctxutil.GetUserPlan(ctx)); limit > 0 {
		existing, err := s.planRepo.CountByTenant(tenantID)
		if err != nil {
			s.logger.WithError(err).Error("failed to count plans for tier check")
			return nil, apierror.Internal("failed to count plans")
		}
		if existing >= limit {
			return nil, apierror.Forbidden(
				fmt.Sprintf("your plan is limited to %d business plan(s) — upgrade to create more", limit),
			)
		}
	}

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

	if err := s.deps.GetScenario().Create(scenario); err != nil {
		s.logger.WithError(err).Error("failed to create default scenario")
		return nil, apierror.Internal("failed to create default scenario")
	}

	// Create default PlanConfig
	config := s.defaultPlanConfig(tenantID, scenario.ID, country)
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
		UserID:     ctxutil.GetUserID(ctx),
		EntityType: "plan",
		EntityID:   planID,
		Action:     event.ActionUpdate,
	})
	return nil
}

// DeletePlan removes a plan and all associated data.
func (s *PlanService) DeletePlan(ctx context.Context, tenantID, planID uuid.UUID) error {
	plan, err := s.GetPlan(ctx, tenantID, planID)
	if err != nil {
		return err
	}
	if plan.IsDemo {
		return apierror.Forbidden("demo plans cannot be deleted")
	}

	if err := s.planRepo.Delete(tenantID, planID); err != nil {
		s.logger.WithError(err).Error("failed to delete plan")
		return apierror.Internal("failed to delete plan")
	}

	s.logger.WithField("plan_id", planID).Info("plan deleted")
	s.emitter.Publish(event.Event{
		Type:       event.PlanDeleted,
		TenantID:   tenantID,
		UserID:     ctxutil.GetUserID(ctx),
		EntityType: "plan",
		EntityID:   planID,
		Action:     event.ActionDelete,
	})
	return nil
}

// ── Plan Status Machine ────────────────────────────────────────────────────────

// planStatusTransitions defines the allowed state-machine edges.
// key = current status, value = set of reachable statuses.
var planStatusTransitions = map[string]map[string]bool{
	"draft":    {"review": true},
	"review":   {"draft": true, "approved": true, "archived": true},
	"approved": {"review": true, "archived": true},
	"archived": {}, // terminal — cannot transition out
}

// ownerOnlyTransitions requires the caller to be "owner" (or "admin").
var ownerOnlyTransitions = map[string]bool{
	"approved": true, // locking requires owner
	"archived": true, // archiving requires owner
}

// TransitionPlanStatus moves a plan between lifecycle states with state-machine guards.
// callerRole is the tenant role of the caller ("owner", "admin", "user").
func (s *PlanService) TransitionPlanStatus(ctx context.Context, tenantID, planID uuid.UUID, newStatus, callerRole string) error {
	plan, err := s.GetPlan(ctx, tenantID, planID)
	if err != nil {
		return err
	}

	allowed, ok := planStatusTransitions[plan.Status]
	if !ok {
		return apierror.Conflict("plan is in an unknown status: " + plan.Status)
	}
	if !allowed[newStatus] {
		return apierror.Conflict("cannot transition plan from '" + plan.Status + "' to '" + newStatus + "'")
	}

	// Owner-only transitions
	if ownerOnlyTransitions[newStatus] && callerRole != "owner" && callerRole != "admin" {
		return apierror.Forbidden("only the plan owner can lock or archive a plan")
	}

	plan.Status = newStatus
	if err := s.planRepo.Update(plan); err != nil {
		s.logger.WithError(err).Error("failed to update plan status")
		return apierror.Internal("failed to update plan status")
	}

	s.logger.WithFields(logrus.Fields{
		"plan_id": planID, "new_status": newStatus,
	}).Info("plan status transitioned")

	s.emitter.Publish(event.Event{
		Type:       event.DataChanged,
		TenantID:   tenantID,
		UserID:     ctxutil.GetUserID(ctx),
		EntityType: "plan",
		EntityID:   planID,
		Action:     event.ActionUpdate,
	})
	return nil
}

// PlanImpact holds the dependency counts used by the safe-delete preview.
type PlanImpact struct {
	PlanID        string `json:"planId"`
	PlanName      string `json:"planName"`
	PlanStatus    string `json:"planStatus"`
	ScenarioCount int    `json:"scenarioCount"`
	IsDemo        bool   `json:"isDemo"`
	CanDelete     bool   `json:"canDelete"`
	BlockedReason string `json:"blockedReason,omitempty"`
}

// GetPlanImpact computes the dependency counts for a plan deletion preview.
func (s *PlanService) GetPlanImpact(ctx context.Context, tenantID, planID uuid.UUID) (*PlanImpact, error) {
	plan, err := s.GetPlan(ctx, tenantID, planID)
	if err != nil {
		return nil, err
	}

	scenarios, err := s.deps.GetScenario().ListByPlan(tenantID, planID)
	if err != nil {
		scenarios = nil
	}

	impact := &PlanImpact{
		PlanID:        planID.String(),
		PlanName:      plan.Name,
		PlanStatus:    plan.Status,
		ScenarioCount: len(scenarios),
		IsDemo:        plan.IsDemo,
		CanDelete:     true,
	}

	if plan.IsDemo {
		impact.CanDelete = false
		impact.BlockedReason = "Demo plans cannot be deleted."
	} else if plan.Status == "approved" {
		impact.CanDelete = false
		impact.BlockedReason = "Locked plans cannot be deleted. Unlock the plan first."
	}

	return impact, nil
}

// ScenarioImpact holds counts for the scenario deletion preview.
type ScenarioImpact struct {
	ScenarioID    string `json:"scenarioId"`
	ScenarioName  string `json:"scenarioName"`
	IsDefault     bool   `json:"isDefault"`
	IsLastInPlan  bool   `json:"isLastInPlan"`
	CanDelete     bool   `json:"canDelete"`
	BlockedReason string `json:"blockedReason,omitempty"`
}

// GetScenarioImpact computes deletion dependencies for a scenario.
func (s *PlanService) GetScenarioImpact(ctx context.Context, tenantID, planID, scenarioID uuid.UUID) (*ScenarioImpact, error) {
	scenario, err := s.deps.GetScenario().GetByID(tenantID, scenarioID)
	if err != nil || scenario == nil {
		return nil, apierror.NotFound("scenario", scenarioID.String())
	}

	siblings, err := s.deps.GetScenario().ListByPlan(tenantID, planID)
	if err != nil {
		siblings = nil
	}

	impact := &ScenarioImpact{
		ScenarioID:   scenarioID.String(),
		ScenarioName: scenario.Name,
		IsDefault:    scenario.IsDefault,
		IsLastInPlan: len(siblings) <= 1,
		CanDelete:    true,
	}

	if len(siblings) <= 1 {
		impact.CanDelete = false
		impact.BlockedReason = "Cannot delete the last scenario in a plan."
	}

	return impact, nil
}

// GetScenario retrieves a scenario by ID.
func (s *PlanService) GetScenario(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.Scenario, error) {
	scenario, err := s.deps.GetScenario().GetByID(tenantID, scenarioID)
	if err != nil || scenario == nil || scenario.TenantID != tenantID {
		s.logger.WithError(err).Warn("scenario not found")
		return nil, apierror.NotFound("scenario", scenarioID.String())
	}

	return scenario, nil
}

// ListScenarios lists all scenarios for a plan.
func (s *PlanService) ListScenarios(ctx context.Context, tenantID, planID uuid.UUID) ([]model.Scenario, error) {
	ptrScenarios, err := s.deps.GetScenario().ListByPlan(tenantID, planID)
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

// scenarioLimitFor returns the maximum number of scenarios allowed per plan,
// consulting the FeaturePolicyService when available.
func (s *PlanService) scenarioLimitFor(userPlan string) int {
	if s.featurePolicySvc != nil {
		return s.featurePolicySvc.NumericLimit(model.FeatureMaxScenarios, userPlan)
	}
	return scenarioLimit(userPlan)
}

// scenarioLimit is the hardcoded fallback used by tests.
func scenarioLimit(userPlan string) int {
	switch userPlan {
	case "freemium":
		return 1
	case "pro":
		return 3
	default:
		return -1
	}
}

// CreateScenario creates a new scenario for a plan.
// Scenario count limits are read from the feature_policies table when available.
func (s *PlanService) CreateScenario(ctx context.Context, tenantID, planID uuid.UUID, name, description string) (*model.Scenario, error) {
	// Verify plan exists
	if _, err := s.GetPlan(ctx, tenantID, planID); err != nil {
		return nil, err
	}

	if limit := s.scenarioLimitFor(ctxutil.GetUserPlan(ctx)); limit > 0 {
		existing, err := s.deps.GetScenario().ListByPlan(tenantID, planID)
		if err != nil {
			s.logger.WithError(err).Error("failed to list scenarios for tier check")
			return nil, apierror.Internal("failed to count scenarios")
		}
		if len(existing) >= limit {
			return nil, apierror.Forbidden(
				"your plan is limited to " + fmt.Sprintf("%d", limit) + " scenario(s) per plan — upgrade to add more",
			)
		}
	}

	scenario := &model.Scenario{
		TenantScoped: model.TenantScoped{ID: uuid.New()},
		PlanID:       planID,
		Name:         name,
		Description:  description,
		IsDefault:    false,
	}
	scenario.TenantID = tenantID

	if err := s.deps.GetScenario().Create(scenario); err != nil {
		s.logger.WithError(err).Error("failed to create scenario")
		return nil, apierror.Internal("failed to create scenario")
	}

	// Create default PlanConfig for new scenario — inherit country from the plan's existing config.
	config := s.defaultPlanConfig(tenantID, scenario.ID, s.countryForPlan(tenantID, planID))
	if err := s.settingsRepo.UpsertConfig(config); err != nil {
		s.logger.WithError(err).Error("failed to create config for new scenario")
		return nil, apierror.Internal("failed to create config for new scenario")
	}

	s.logger.WithField("scenario_id", scenario.ID).WithField("plan_id", planID).Info("scenario created")
	s.emitter.Publish(event.Event{
		Type:       event.DataChanged,
		TenantID:   tenantID,
		UserID:     ctxutil.GetUserID(ctx),
		ScenarioID: scenario.ID,
		EntityType: "scenario",
		EntityID:   scenario.ID,
		Action:     event.ActionCreate,
	})
	return scenario, nil
}

// UpdateScenario updates a scenario's name and/or description.
func (s *PlanService) UpdateScenario(ctx context.Context, tenantID, scenarioID uuid.UUID, name, description string) error {
	scenario, err := s.GetScenario(ctx, tenantID, scenarioID)
	if err != nil {
		return err
	}

	if name != "" {
		scenario.Name = name
	}
	if description != "" {
		scenario.Description = description
	}

	if err := s.deps.GetScenario().Update(scenario); err != nil {
		s.logger.WithError(err).Error("failed to update scenario")
		return apierror.Internal("failed to update scenario")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("scenario updated")
	s.emitter.Publish(event.Event{
		Type:       event.DataChanged,
		TenantID:   tenantID,
		UserID:     ctxutil.GetUserID(ctx),
		ScenarioID: scenarioID,
		EntityType: "scenario",
		EntityID:   scenarioID,
		Action:     event.ActionUpdate,
	})
	return nil
}

// CloneScenario clones a scenario with all its data to a new scenario.
// The scenario row and every child table copy run in a single database
// transaction, through the same sections a snapshot uses (settings,
// products, staff, capex, opex, P&L, financing plan, P&L-to-cash, WCR, cash
// and budget overrides). The first failed read or write rolls the whole
// clone back and is reported to the caller; no half-copied scenario is left
// behind (audit §3.1).
func (s *PlanService) CloneScenario(ctx context.Context, tenantID, sourceScenarioID uuid.UUID, newName string) (*model.Scenario, error) {
	sourceScenario, err := s.deps.GetScenario().GetByID(tenantID, sourceScenarioID)
	if err != nil || sourceScenario == nil || sourceScenario.TenantID != tenantID {
		s.logger.WithError(err).Warn("source scenario not found")
		return nil, apierror.NotFound("scenario", sourceScenarioID.String())
	}

	newScenario := &model.Scenario{
		TenantScoped: model.TenantScoped{ID: uuid.New()},
		PlanID:       sourceScenario.PlanID,
		Name:         newName,
		IsDefault:    false,
	}
	newScenario.TenantID = tenantID

	txErr := s.deps.GetDB().Transaction(func(tx *gorm.DB) error {
		txRepos := repo.NewRepoBundle(tx)
		if err := txRepos.Scenario.Create(newScenario); err != nil {
			return fmt.Errorf("create scenario: %w", err)
		}
		return copyScenarioData(registerSections(txRepos), tenantID, sourceScenarioID, newScenario.ID)
	})
	if txErr != nil {
		s.logger.WithError(txErr).WithField("source_scenario_id", sourceScenarioID).Error("failed to clone scenario (rolled back)")
		return nil, apierror.Internal("failed to clone scenario")
	}

	s.logger.WithField("source_scenario_id", sourceScenarioID).WithField("new_scenario_id", newScenario.ID).Info("scenario cloned")
	s.emitter.Publish(event.Event{
		Type:       event.DataChanged,
		TenantID:   tenantID,
		UserID:     ctxutil.GetUserID(ctx),
		ScenarioID: newScenario.ID,
		EntityType: "scenario",
		EntityID:   newScenario.ID,
		Action:     event.ActionCreate,
	})
	return newScenario, nil
}

// DeleteScenario removes a scenario and all associated data.
func (s *PlanService) DeleteScenario(ctx context.Context, tenantID, scenarioID uuid.UUID) error {
	scenario, err := s.deps.GetScenario().GetByID(tenantID, scenarioID)
	if err != nil || scenario == nil || scenario.TenantID != tenantID {
		s.logger.WithError(err).Warn("scenario not found")
		return apierror.NotFound("scenario", scenarioID.String())
	}

	if err := s.deps.GetScenario().Delete(tenantID, scenarioID); err != nil {
		s.logger.WithError(err).Error("failed to delete scenario")
		return apierror.Internal("failed to delete scenario")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("scenario deleted")
	s.emitter.Publish(event.Event{
		Type:       event.DataChanged,
		TenantID:   tenantID,
		UserID:     ctxutil.GetUserID(ctx),
		ScenarioID: scenarioID,
		EntityType: "scenario",
		EntityID:   scenarioID,
		Action:     event.ActionDelete,
	})
	return nil
}
