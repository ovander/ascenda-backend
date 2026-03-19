package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"kerplan/internal/model"
	"kerplan/internal/pkg/apierror"
)

// BSheetService provides read-only balance sheet reporting.
type BSheetService struct {
	reportService *ReportService
	logger        *logrus.Entry
}

// NewBSheetService creates a new BSheetService.
func NewBSheetService(reportService *ReportService, logger *logrus.Entry) *BSheetService {
	return &BSheetService{
		reportService: reportService,
		logger:        logger,
	}
}

// GetReport delegates to ReportService for balance sheet data.
func (s *BSheetService) GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.BSheetReport, error) {
	fullReport, err := s.reportService.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to compute balance sheet report")
		return nil, apierror.Internal("failed to compute balance sheet report")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("balance sheet report computed")
	return &fullReport.BSheet, nil
}

// GetChartData returns chart-formatted balance sheet data.
func (s *BSheetService) GetChartData(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error) {
	report, err := s.GetReport(ctx, tenantID, scenarioID)
	if err != nil {
		return nil, err
	}

	chartData := map[string]interface{}{
		"detailed":  report.Detailed,
		"condensed": report.Condensed,
		"analysis":  report.Analysis,
		"charts":    report.Charts,
		"equity":    report.Equity,
	}

	return chartData, nil
}
