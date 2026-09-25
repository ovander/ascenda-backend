package service

import (
	"context"

	"ascenda/internal/event"
	"ascenda/internal/model"
	"ascenda/internal/repo"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
)

// CapexService orchestrates capital expenditure CRUD and computation.
type CapexService struct {
	capexRepo     repo.CapexRepository
	reportService *ReportService
	emitter       *event.Emitter
	logger        *logrus.Entry
}

// NewCapexService creates a new CapexService.
func NewCapexService(capexRepo repo.CapexRepository, reportService *ReportService, emitter *event.Emitter, logger *logrus.Entry) *CapexService {
	return &CapexService{
		capexRepo:     capexRepo,
		reportService: reportService,
		emitter:       emitter,
		logger:        logger,
	}
}

// ListEntries lists all capex entries for a scenario.
func (s *CapexService) ListEntries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapexEntry, error) {
	ptrEntries, err := s.capexRepo.ListByScenario(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list capex entries")
		return nil, apierror.Internal("failed to list capex entries")
	}

	entries := make([]model.CapexEntry, len(ptrEntries))
	for i, e := range ptrEntries {
		entries[i] = *e
	}

	return entries, nil
}

// UpdateEntries updates or creates capex entries.
func (s *CapexService) UpdateEntries(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.CapexEntry) error {
	for i := range entries {
		if entries[i].ID == uuid.Nil {
			entries[i].ID = uuid.New()
		}
		entries[i].TenantID = tenantID
		entries[i].ScenarioID = scenarioID
	}

	if err := s.capexRepo.BatchUpsert(tenantID, scenarioID, entries); err != nil {
		s.logger.WithError(err).Error("failed to update capex entries")
		return apierror.Internal("failed to update capex entries")
	}

	categories := countUnique(entries, func(e model.CapexEntry) string { return string(e.Category) })
	s.logger.WithField("scenario_id", scenarioID).Info("capex entries updated")
	s.emitter.Publish(event.Event{
		Type: event.DataChanged, TenantID: tenantID, UserID: ctxutil.GetUserID(ctx), ScenarioID: scenarioID,
		EntityType: "capex", Action: event.ActionUpdate,
		Changes: marshalChanges(map[string]any{"rows": len(entries), "categories": categories}),
	})
	return nil
}

// GetSummary delegates to ReportService for capex aggregation.
func (s *CapexService) GetSummary(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CapexSummary, error) {
	fullReport, err := s.reportService.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to compute capex summary")
		return nil, apierror.Internal("failed to compute capex summary")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("capex summary computed")
	return &fullReport.Capex, nil
}
