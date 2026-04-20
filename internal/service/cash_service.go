package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"ascenda/internal/event"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"ascenda/internal/repo"
)

// CashService orchestrates monthly cash flow CRUD and reporting.
type CashService struct {
	cashRepo      repo.CashRepository
	reportService *ReportService
	emitter       *event.Emitter
	logger        *logrus.Entry
}

// NewCashService creates a new CashService.
func NewCashService(cashRepo repo.CashRepository, reportService *ReportService, emitter *event.Emitter, logger *logrus.Entry) *CashService {
	return &CashService{
		cashRepo:      cashRepo,
		reportService: reportService,
		emitter:       emitter,
		logger:        logger,
	}
}

// ListOverrides lists all cash flow overrides for a scenario.
func (s *CashService) ListOverrides(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CashMonthlyOverride, error) {
	ptrOverrides, err := s.cashRepo.ListByScenario(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list cash overrides")
		return nil, apierror.Internal("failed to list cash overrides")
	}

	overrides := make([]model.CashMonthlyOverride, len(ptrOverrides))
	for i, o := range ptrOverrides {
		overrides[i] = *o
	}

	return overrides, nil
}

// UpdateOverrides updates or creates cash flow overrides.
func (s *CashService) UpdateOverrides(ctx context.Context, tenantID, scenarioID uuid.UUID, overrides []model.CashMonthlyOverride) error {
	for i := range overrides {
		if overrides[i].ID == uuid.Nil {
			overrides[i].ID = uuid.New()
		}
		overrides[i].TenantID = tenantID
		overrides[i].ScenarioID = scenarioID
	}

	if err := s.cashRepo.BatchUpsert(tenantID, scenarioID, overrides); err != nil {
		s.logger.WithError(err).Error("failed to update cash overrides")
		return apierror.Internal("failed to update cash overrides")
	}

	lines := countUnique(overrides, func(o model.CashMonthlyOverride) string {
		return string(o.LineID)
	})
	s.logger.WithField("scenario_id", scenarioID).Info("cash overrides updated")
	if s.emitter != nil {
		s.emitter.Publish(event.Event{
			Type:       event.DataChanged,
			TenantID:   tenantID,
			UserID:     ctxutil.GetUserID(ctx),
			ScenarioID: scenarioID,
			EntityType: "cash",
			Action:     event.ActionUpdate,
			Changes:    marshalChanges(map[string]any{"rows": len(overrides), "lines": lines}),
		})
	}
	return nil
}

// GetReport delegates to ReportService for cash flow analysis.
func (s *CashService) GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CashReport, error) {
	fullReport, err := s.reportService.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to compute cash report")
		return nil, apierror.Internal("failed to compute cash report")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("cash report computed")
	return &fullReport.Cash, nil
}
