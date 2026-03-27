package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"ascenda/internal/event"
	"ascenda/internal/model"
	"ascenda/internal/pkg/apierror"
	"ascenda/internal/pkg/ctxutil"
	"ascenda/internal/repo"
)

// FiplanService orchestrates financial plan CRUD and reporting.
type FiplanService struct {
	fiplanRepo    repo.FiplanRepository
	reportService *ReportService
	emitter       *event.Emitter
	logger        *logrus.Entry
}

// NewFiplanService creates a new FiplanService.
func NewFiplanService(fiplanRepo repo.FiplanRepository, reportService *ReportService, emitter *event.Emitter, logger *logrus.Entry) *FiplanService {
	return &FiplanService{
		fiplanRepo:    fiplanRepo,
		reportService: reportService,
		emitter:       emitter,
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

// GetCapitalIncreaseEntry returns the capital_increase FiPlan entry for the given
// 0-based fiscalYearIndex, or nil if the user has not entered an amount yet.
// Used by Flow A (FiPlan → Cap Table) to read the planned injection amount.
func (s *FiplanService) GetCapitalIncreaseEntry(ctx context.Context, tenantID, scenarioID uuid.UUID, fiscalYearIndex int) (*model.FiplanEntry, error) {
	entries, err := s.ListEntries(ctx, tenantID, scenarioID)
	if err != nil {
		return nil, err
	}
	targetYear := fiscalYearIndex + 1 // FiPlan stores yearIndex 1-based
	for i := range entries {
		if entries[i].LineID == model.FiplanCapitalIncrease && entries[i].YearIndex == targetYear {
			return &entries[i], nil
		}
	}
	return nil, nil // no entry yet — caller decides whether this is an error
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

	lines := countUnique(entries, func(e model.FiplanEntry) string { return string(e.LineID) })
	s.logger.WithField("scenario_id", scenarioID).Info("fiplan entries updated")
	s.emitter.Publish(event.Event{
		Type: event.DataChanged, TenantID: tenantID, UserID: ctxutil.GetUserID(ctx), ScenarioID: scenarioID,
		EntityType: "fiplan", Action: event.ActionUpdate,
		Changes: marshalChanges(map[string]any{"rows": len(entries), "lines": lines}),
	})
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

// UpsertCapitalIncreaseEntry upserts a capital_increase FiPlan entry and records the
// cap table round link. Called by CapTableService when syncing a round to FiPlan.
// amount must be in full currency units (not k€).
func (s *FiplanService) UpsertCapitalIncreaseEntry(ctx context.Context, tenantID, scenarioID uuid.UUID, yearIndex int, amount decimal.Decimal, roundID uuid.UUID, roundLabel string) error {
	if err := s.fiplanRepo.UpsertCapitalIncreaseEntry(tenantID, scenarioID, yearIndex, amount, roundID, roundLabel); err != nil {
		s.logger.WithError(err).Error("failed to upsert capital increase entry from cap table sync")
		return apierror.Internal("failed to sync capital increase to financial plan")
	}
	s.emitter.Publish(event.Event{
		Type: event.DataChanged, TenantID: tenantID, UserID: ctxutil.GetUserID(ctx), ScenarioID: scenarioID,
		EntityType: "fiplan", Action: event.ActionUpdate,
		Changes: marshalChanges(map[string]any{
			"line": "capital_increase", "year": yearIndex, "source": "cap_table_sync",
		}),
	})
	return nil
}

// ClearCapTableLink removes the cap table link from a capital_increase FiPlan entry.
// Called by CapTableService when a round is unlinked from FiPlan.
func (s *FiplanService) ClearCapTableLink(ctx context.Context, tenantID, scenarioID uuid.UUID, yearIndex int) error {
	if err := s.fiplanRepo.ClearCapTableLink(tenantID, scenarioID, yearIndex); err != nil {
		s.logger.WithError(err).Error("failed to clear cap table link from fiplan entry")
		return apierror.Internal("failed to unlink from financial plan")
	}
	s.emitter.Publish(event.Event{
		Type: event.DataChanged, TenantID: tenantID, UserID: ctxutil.GetUserID(ctx), ScenarioID: scenarioID,
		EntityType: "fiplan", Action: event.ActionUpdate,
		Changes: marshalChanges(map[string]any{
			"line": "capital_increase", "year": yearIndex, "source": "cap_table_unlink",
		}),
	})
	return nil
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
