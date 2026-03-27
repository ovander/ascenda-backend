package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"ascenda/internal/event"
	"ascenda/internal/model"
	"ascenda/internal/pkg/apierror"
	"ascenda/internal/pkg/ctxutil"
	"ascenda/internal/repo"
)

// BudgetService orchestrates budget CRUD and reporting.
type BudgetService struct {
	budgetRepo    repo.BudgetRepository
	reportService *ReportService
	emitter       *event.Emitter
	logger        *logrus.Entry
}

// NewBudgetService creates a new BudgetService.
func NewBudgetService(budgetRepo repo.BudgetRepository, reportService *ReportService, emitter *event.Emitter, logger *logrus.Entry) *BudgetService {
	return &BudgetService{
		budgetRepo:    budgetRepo,
		reportService: reportService,
		emitter:       emitter,
		logger:        logger,
	}
}

// ListOverrides lists all budget overrides for a scenario and year.
func (s *BudgetService) ListOverrides(ctx context.Context, tenantID, scenarioID uuid.UUID, year int) ([]model.BudgetMonthlyOverride, error) {
	ptrOverrides, err := s.budgetRepo.ListByScenario(tenantID, scenarioID, year)
	if err != nil {
		s.logger.WithError(err).Error("failed to list budget overrides")
		return nil, apierror.Internal("failed to list budget overrides")
	}

	overrides := make([]model.BudgetMonthlyOverride, len(ptrOverrides))
	for i, o := range ptrOverrides {
		overrides[i] = *o
	}

	return overrides, nil
}

// UpdateOverrides updates or creates budget overrides.
func (s *BudgetService) UpdateOverrides(ctx context.Context, tenantID, scenarioID uuid.UUID, year int, overrides []model.BudgetMonthlyOverride) error {
	for i := range overrides {
		if overrides[i].ID == uuid.Nil {
			overrides[i].ID = uuid.New()
		}
		overrides[i].TenantID = tenantID
		overrides[i].ScenarioID = scenarioID
	}

	if err := s.budgetRepo.BatchUpsert(tenantID, scenarioID, overrides); err != nil {
		s.logger.WithError(err).Error("failed to update budget overrides")
		return apierror.Internal("failed to update budget overrides")
	}

	lines := countUnique(overrides, func(o model.BudgetMonthlyOverride) string {
		return string(o.LineID)
	})
	s.logger.WithField("scenario_id", scenarioID).WithField("year", year).Info("budget overrides updated")
	if s.emitter != nil {
		s.emitter.Publish(event.Event{
			Type:       event.DataChanged,
			TenantID:   tenantID,
			UserID:     ctxutil.GetUserID(ctx),
			ScenarioID: scenarioID,
			EntityType: "budget",
			Action:     event.ActionUpdate,
			Changes:    marshalChanges(map[string]any{"rows": len(overrides), "year": year, "lines": lines}),
		})
	}
	return nil
}

// GetBudget1Report returns year 1 budget analysis.
func (s *BudgetService) GetBudget1Report(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.Budget1Report, error) {
	fullReport, err := s.reportService.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to compute budget1 report")
		return nil, apierror.Internal("failed to compute budget1 report")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("budget1 report computed")
	return &fullReport.Budget1, nil
}

// GetBudget2Report returns year 2 budget analysis.
func (s *BudgetService) GetBudget2Report(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.Budget2Report, error) {
	fullReport, err := s.reportService.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to compute budget2 report")
		return nil, apierror.Internal("failed to compute budget2 report")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("budget2 report computed")
	return &fullReport.Budget2, nil
}
