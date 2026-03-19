package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"kerplan/internal/model"
	"kerplan/internal/pkg/apierror"
	"kerplan/internal/repo"
)

// FiplanService orchestrates financial plan CRUD and reporting.
type FiplanService struct {
	fiplanRepo    repo.FiplanRepository
	reportService *ReportService
	logger        *logrus.Entry
}

// NewFiplanService creates a new FiplanService.
func NewFiplanService(fiplanRepo repo.FiplanRepository, reportService *ReportService, logger *logrus.Entry) *FiplanService {
	return &FiplanService{
		fiplanRepo:    fiplanRepo,
		reportService: reportService,
		logger:        logger,
	}
}

// ListEntries lists all fiplan entries for a scenario.
func (s *FiplanService) ListEntries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.FiplanEntry, error) {
	ptrEntries, err := s.fiplanRepo.ListByScenario(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list fiplan entries")
		return nil, apierror.Internal("failed to list fiplan entries")
	}

	entries := make([]model.FiplanEntry, len(ptrEntries))
	for i, e := range ptrEntries {
		entries[i] = *e
	}

	return entries, nil
}

// UpdateEntries updates or creates fiplan entries.
func (s *FiplanService) UpdateEntries(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.FiplanEntry) error {
	for i := range entries {
		if entries[i].ID == uuid.Nil {
			entries[i].ID = uuid.New()
		}
		entries[i].TenantID = tenantID
		entries[i].ScenarioID = scenarioID
	}

	if err := s.fiplanRepo.BatchUpsert(tenantID, scenarioID, entries); err != nil {
		s.logger.WithError(err).Error("failed to update fiplan entries")
		return apierror.Internal("failed to update fiplan entries")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("fiplan entries updated")
	return nil
}

// GetReport delegates to ReportService for fiplan report.
func (s *FiplanService) GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.FiplanReport, error) {
	fullReport, err := s.reportService.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to compute fiplan report")
		return nil, apierror.Internal("failed to compute fiplan report")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("fiplan report computed")
	return &fullReport.FiPlan, nil
}

// GetGrantsForPnL returns grants entries that should be included in P&L.
func (s *FiplanService) GetGrantsForPnL(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.FiplanEntry, error) {
	entries, err := s.ListEntries(ctx, tenantID, scenarioID)
	if err != nil {
		return nil, err
	}

	var grants []model.FiplanEntry
	for _, entry := range entries {
		if entry.LineID == model.FiplanSubsidies ||
			entry.LineID == model.FiplanOtherGrants ||
			entry.LineID == model.FiplanRepayableGrants {
			grants = append(grants, entry)
		}
	}

	return grants, nil
}
