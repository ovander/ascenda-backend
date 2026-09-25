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

// StaffService orchestrates staff CRUD and computation.
type StaffService struct {
	staffRepo     repo.StaffRepository
	reportService *ReportService
	emitter       *event.Emitter
	logger        *logrus.Entry
}

// NewStaffService creates a new StaffService.
func NewStaffService(staffRepo repo.StaffRepository, reportService *ReportService, emitter *event.Emitter, logger *logrus.Entry) *StaffService {
	return &StaffService{
		staffRepo:     staffRepo,
		reportService: reportService,
		emitter:       emitter,
		logger:        logger,
	}
}

// ListHeadcounts lists all staff headcount records for a scenario.
func (s *StaffService) ListHeadcounts(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.StaffHeadcount, error) {
	ptrHC, err := s.staffRepo.ListHeadcountsByScenario(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list headcounts")
		return nil, apierror.Internal("failed to list headcounts")
	}
	headcounts := make([]model.StaffHeadcount, len(ptrHC))
	for i, h := range ptrHC {
		headcounts[i] = *h
	}
	return headcounts, nil
}

// ListSalaries lists all salary records for a scenario.
func (s *StaffService) ListSalaries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.StaffSalary, error) {
	ptrSal, err := s.staffRepo.ListSalariesByScenario(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list salaries")
		return nil, apierror.Internal("failed to list salaries")
	}
	salaries := make([]model.StaffSalary, len(ptrSal))
	for i, s := range ptrSal {
		salaries[i] = *s
	}
	return salaries, nil
}

// ListIncentives lists all incentive records for a scenario.
func (s *StaffService) ListIncentives(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.StaffIncentive, error) {
	ptrInc, err := s.staffRepo.ListIncentivesByScenario(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list incentives")
		return nil, apierror.Internal("failed to list incentives")
	}
	incentives := make([]model.StaffIncentive, len(ptrInc))
	for i, inc := range ptrInc {
		incentives[i] = *inc
	}
	return incentives, nil
}

// UpdateHeadcounts updates or creates headcount records.
func (s *StaffService) UpdateHeadcounts(ctx context.Context, tenantID, scenarioID uuid.UUID, headcounts []model.StaffHeadcount) error {
	for i := range headcounts {
		if headcounts[i].ID == uuid.Nil {
			headcounts[i].ID = uuid.New()
		}
		headcounts[i].TenantID = tenantID
		headcounts[i].ScenarioID = scenarioID
	}

	if err := s.staffRepo.BatchUpsertHeadcounts(tenantID, scenarioID, headcounts); err != nil {
		s.logger.WithError(err).Error("failed to update headcounts")
		return apierror.Internal("failed to update headcounts")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("headcounts updated")
	categories := countUnique(headcounts, func(h model.StaffHeadcount) string { return string(h.Category) })
	s.emitter.Publish(event.Event{
		Type: event.DataChanged, TenantID: tenantID, UserID: ctxutil.GetUserID(ctx), ScenarioID: scenarioID,
		EntityType: "staff_headcounts", Action: event.ActionUpdate,
		Changes: marshalChanges(map[string]any{"rows": len(headcounts), "categories": categories}),
	})
	return nil
}

// UpdateSalaries updates or creates salary records.
func (s *StaffService) UpdateSalaries(ctx context.Context, tenantID, scenarioID uuid.UUID, salaries []model.StaffSalary) error {
	for i := range salaries {
		if salaries[i].ID == uuid.Nil {
			salaries[i].ID = uuid.New()
		}
		salaries[i].TenantID = tenantID
		salaries[i].ScenarioID = scenarioID
	}

	if err := s.staffRepo.BatchUpsertSalaries(tenantID, scenarioID, salaries); err != nil {
		s.logger.WithError(err).Error("failed to update salaries")
		return apierror.Internal("failed to update salaries")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("salaries updated")
	salaryCategories := countUnique(salaries, func(s model.StaffSalary) string { return string(s.Category) })
	s.emitter.Publish(event.Event{
		Type: event.DataChanged, TenantID: tenantID, UserID: ctxutil.GetUserID(ctx), ScenarioID: scenarioID,
		EntityType: "staff_salaries", Action: event.ActionUpdate,
		Changes: marshalChanges(map[string]any{"rows": len(salaries), "categories": salaryCategories}),
	})
	return nil
}

// UpdateIncentives updates or creates incentive records.
func (s *StaffService) UpdateIncentives(ctx context.Context, tenantID, scenarioID uuid.UUID, incentives []model.StaffIncentive) error {
	for i := range incentives {
		if incentives[i].ID == uuid.Nil {
			incentives[i].ID = uuid.New()
		}
		incentives[i].TenantID = tenantID
		incentives[i].ScenarioID = scenarioID
	}

	if err := s.staffRepo.BatchUpsertIncentives(tenantID, scenarioID, incentives); err != nil {
		s.logger.WithError(err).Error("failed to update incentives")
		return apierror.Internal("failed to update incentives")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("incentives updated")
	s.emitter.Publish(event.Event{
		Type: event.DataChanged, TenantID: tenantID, UserID: ctxutil.GetUserID(ctx), ScenarioID: scenarioID,
		EntityType: "staff_incentives", Action: event.ActionUpdate,
		Changes: marshalChanges(map[string]any{"rows": len(incentives)}),
	})
	return nil
}

// GetPayrollSummary delegates to ReportService for aggregated payroll metrics.
func (s *StaffService) GetPayrollSummary(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.StaffPayrollSummary, error) {
	fullReport, err := s.reportService.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to compute payroll summary")
		return nil, apierror.Internal("failed to compute payroll summary")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("payroll summary computed")
	return &fullReport.Payroll, nil
}
