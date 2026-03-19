package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"kerplan/internal/model"
	"kerplan/internal/pkg/apierror"
	"kerplan/internal/repo"
)

// PnlCashService orchestrates P&L + Cash CRUD and reporting.
type PnlCashService struct {
	pnlCashRepo   repo.PnlCashRepository
	reportService *ReportService
	logger        *logrus.Entry
}

// NewPnlCashService creates a new PnlCashService.
func NewPnlCashService(pnlCashRepo repo.PnlCashRepository, reportService *ReportService, logger *logrus.Entry) *PnlCashService {
	return &PnlCashService{
		pnlCashRepo:   pnlCashRepo,
		reportService: reportService,
		logger:        logger,
	}
}

// ListEntries lists all P&L + Cash entries for a scenario.
func (s *PnlCashService) ListEntries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.PnlCashEntry, error) {
	ptrEntries, err := s.pnlCashRepo.ListByScenario(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list pnl cash entries")
		return nil, apierror.Internal("failed to list pnl cash entries")
	}

	entries := make([]model.PnlCashEntry, len(ptrEntries))
	for i, e := range ptrEntries {
		entries[i] = *e
	}

	return entries, nil
}

// UpdateEntries updates or creates P&L + Cash entries.
func (s *PnlCashService) UpdateEntries(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.PnlCashEntry) error {
	for i := range entries {
		if entries[i].ID == uuid.Nil {
			entries[i].ID = uuid.New()
		}
		entries[i].TenantID = tenantID
		entries[i].ScenarioID = scenarioID
	}

	if err := s.pnlCashRepo.BatchUpsert(tenantID, scenarioID, entries); err != nil {
		s.logger.WithError(err).Error("failed to update pnl cash entries")
		return apierror.Internal("failed to update pnl cash entries")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("pnl cash entries updated")
	return nil
}

// GetReport delegates to ReportService for P&L + Cash report.
func (s *PnlCashService) GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.PnlCashReport, error) {
	fullReport, err := s.reportService.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to compute pnl cash report")
		return nil, apierror.Internal("failed to compute pnl cash report")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("pnl cash report computed")
	return &fullReport.PnlCash, nil
}

// GetChartData returns chart-formatted P&L + Cash data.
func (s *PnlCashService) GetChartData(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error) {
	report, err := s.GetReport(ctx, tenantID, scenarioID)
	if err != nil {
		return nil, err
	}

	chartData := map[string]interface{}{
		"years":     report.ChartData.Years,
		"chartData": report.ChartData,
	}

	return chartData, nil
}
