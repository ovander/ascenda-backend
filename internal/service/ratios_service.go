package service

import (
	"context"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/sirupsen/logrus"
)

// RatiosService provides read-only financial ratios reporting.
type RatiosService struct {
	reportService *ReportService
	logger        *logrus.Entry
}

// NewRatiosService creates a new RatiosService.
func NewRatiosService(reportService *ReportService, logger *logrus.Entry) *RatiosService {
	return &RatiosService{
		reportService: reportService,
		logger:        logger,
	}
}

// GetReport delegates to ReportService for financial ratios.
func (s *RatiosService) GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.RatiosReport, error) {
	fullReport, err := s.reportService.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to compute ratios report")
		return nil, apierror.Internal("failed to compute ratios report")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("ratios report computed")
	return &fullReport.Ratios, nil
}

// GetChartData returns chart-formatted ratios data.
func (s *RatiosService) GetChartData(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error) {
	report, err := s.GetReport(ctx, tenantID, scenarioID)
	if err != nil {
		return nil, err
	}

	chartData := map[string]interface{}{
		"sales":          report.Sales,
		"operational":    report.Operational,
		"profitability":  report.Profitability,
		"equityLeverage": report.EquityLeverage,
		"valuation":      report.Valuation,
		"charts":         report.Charts,
	}

	return chartData, nil
}
