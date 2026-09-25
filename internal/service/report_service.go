package service

import (
	"context"

	"ascenda/internal/compute"
	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/sirupsen/logrus"
)

// ReportService orchestrates full plan computation with optional caching.
type ReportService struct {
	orchestrator *PlanComputeOrchestrator
	cache        *ReportCache
	logger       *logrus.Entry
}

// NewReportService creates a new ReportService.
// It delegates all data-loading and compute work to the shared
// PlanComputeOrchestrator so that the loading logic lives in one place.
func NewReportService(orchestrator *PlanComputeOrchestrator, logger *logrus.Entry) *ReportService {
	return &ReportService{
		orchestrator: orchestrator,
		logger:       logger,
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

	input, err := s.orchestrator.LoadInputs(tenantID, scenarioID)
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

// GetPlanConfig loads the PlanConfig for a scenario, used by the analysis
// layer to access settings such as Language without re-running the compute engine.
// Uses the same Settings repository as the orchestrator's LoadInputs.
func (s *ReportService) GetPlanConfig(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.PlanConfig, error) {
	_ = ctx // reserved for future tracing / tenant propagation
	return s.orchestrator.repos.Settings.GetConfig(tenantID, scenarioID)
}
