package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"kerplan/internal/model"
	"kerplan/internal/pkg/apierror"
	"kerplan/internal/repo"
)

// WCRService orchestrates working capital requirements CRUD and reporting.
type WCRService struct {
	wcrRepo       repo.WCRRepository
	reportService *ReportService
	logger        *logrus.Entry
}

// NewWCRService creates a new WCRService.
func NewWCRService(wcrRepo repo.WCRRepository, reportService *ReportService, logger *logrus.Entry) *WCRService {
	return &WCRService{
		wcrRepo:       wcrRepo,
		reportService: reportService,
		logger:        logger,
	}
}

// ListEntries lists all WCR entries for a scenario.
func (s *WCRService) ListEntries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.WCREntry, error) {
	ptrEntries, err := s.wcrRepo.ListByScenario(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list wcr entries")
		return nil, apierror.Internal("failed to list wcr entries")
	}
	entries := make([]model.WCREntry, len(ptrEntries))
	for i, e := range ptrEntries { entries[i] = *e }
	return entries, nil
}

// UpdateEntries updates or creates WCR entries.
func (s *WCRService) UpdateEntries(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.WCREntry) error {
	for i := range entries {
		if entries[i].ID == uuid.Nil {
			entries[i].ID = uuid.New()
		}
		entries[i].TenantID = tenantID
		entries[i].ScenarioID = scenarioID
	}

	if err := s.wcrRepo.BatchUpsert(tenantID, scenarioID, entries); err != nil {
		s.logger.WithError(err).Error("failed to update wcr entries")
		return apierror.Internal("failed to update wcr entries")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("wcr entries updated")
	return nil
}

// GetReport delegates to ReportService for WCR analysis.
func (s *WCRService) GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.WCRReport, error) {
	fullReport, err := s.reportService.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to compute wcr report")
		return nil, apierror.Internal("failed to compute wcr report")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("wcr report computed")
	return &fullReport.WCR, nil
}

// GetChartData returns chart-formatted WCR data.
func (s *WCRService) GetChartData(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error) {
	report, err := s.GetReport(ctx, tenantID, scenarioID)
	if err != nil {
		return nil, err
	}

	chartData := map[string]interface{}{
		"charts":  report.Charts,
		"summary": report.Summary,
	}

	return chartData, nil
}
