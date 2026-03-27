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

// OpexService orchestrates operating expenses CRUD and computation.
type OpexService struct {
	opexRepo      repo.OpexRepository
	reportService *ReportService
	emitter       *event.Emitter
	logger        *logrus.Entry
}

// NewOpexService creates a new OpexService.
func NewOpexService(opexRepo repo.OpexRepository, reportService *ReportService, emitter *event.Emitter, logger *logrus.Entry) *OpexService {
	return &OpexService{
		opexRepo:      opexRepo,
		reportService: reportService,
		emitter:       emitter,
		logger:        logger,
	}
}

// ListEntries lists all opex entries for a scenario.
func (s *OpexService) ListEntries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.OpexManualEntry, error) {
	ptrEntries, err := s.opexRepo.ListByScenario(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list opex entries")
		return nil, apierror.Internal("failed to list opex entries")
	}

	entries := make([]model.OpexManualEntry, len(ptrEntries))
	for i, e := range ptrEntries {
		entries[i] = *e
	}

	return entries, nil
}

// UpdateEntries updates or creates opex entries.
func (s *OpexService) UpdateEntries(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.OpexManualEntry) error {
	for i := range entries {
		if entries[i].ID == uuid.Nil {
			entries[i].ID = uuid.New()
		}
		entries[i].TenantID = tenantID
		entries[i].ScenarioID = scenarioID
	}

	if err := s.opexRepo.BatchUpsert(tenantID, scenarioID, entries); err != nil {
		s.logger.WithError(err).Error("failed to update opex entries")
		return apierror.Internal("failed to update opex entries")
	}

	lines := countUnique(entries, func(e model.OpexManualEntry) string { return string(e.LineID) })
	s.logger.WithField("scenario_id", scenarioID).Info("opex entries updated")
	s.emitter.Publish(event.Event{
		Type: event.DataChanged, TenantID: tenantID, UserID: ctxutil.GetUserID(ctx), ScenarioID: scenarioID,
		EntityType: "opex", Action: event.ActionUpdate,
		Changes: marshalChanges(map[string]any{"rows": len(entries), "lines": lines}),
	})
	return nil
}

// GetSummary delegates to ReportService for opex aggregation.
func (s *OpexService) GetSummary(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.OpexSummary, error) {
	fullReport, err := s.reportService.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to compute opex summary")
		return nil, apierror.Internal("failed to compute opex summary")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("opex summary computed")
	return &fullReport.Opex, nil
}
