// Package service — Break-Even Point (BEP) service.
// Orchestrates the three BEP sub-modules: Calculator, Sensitivity Analyser,
// and Optimisation Planner.
package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"ascenda/internal/compute"
	"ascenda/internal/event"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"ascenda/internal/repo"
)

// planReporter is the minimal surface of ReportService that ImportFromPlan requires.
// Using an interface decouples BEP from the full report service and makes unit
// testing possible without a live database.
type planReporter interface {
	GetFullReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.FullPlanOutput, error)
}

// BEPService orchestrates all BEP operations.
type BEPService struct {
	bepRepo   repo.BEPRepository
	reportSvc planReporter
	emitter   *event.Emitter
	logger    *logrus.Entry
}

// NewBEPService creates a new BEPService.
// reportSvc may be nil; ImportFromPlan returns an error when it is.
func NewBEPService(bepRepo repo.BEPRepository, reportSvc planReporter, emitter *event.Emitter, logger *logrus.Entry) *BEPService {
	return &BEPService{
		bepRepo:   bepRepo,
		reportSvc: reportSvc,
		emitter:   emitter,
		logger:    logger,
	}
}

// ─── BEPSnapshot CRUD ─────────────────────────────────────────────────────────

// ListSnapshots returns all BEP snapshots for a scenario.
func (s *BEPService) ListSnapshots(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]*model.BEPSnapshot, error) {
	rows, err := s.bepRepo.ListSnapshots(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("bep: failed to list snapshots")
		return nil, apierror.Internal("failed to list BEP snapshots")
	}
	return rows, nil
}

// CreateSnapshot creates a new BEP snapshot and auto-provisions the three
// default SensitivityConfig records.
func (s *BEPService) CreateSnapshot(ctx context.Context, snap *model.BEPSnapshot) error {
	if err := validateSnapshotInputs(snap); err != nil {
		return err
	}
	if err := s.bepRepo.CreateSnapshot(snap); err != nil {
		s.logger.WithError(err).Error("bep: failed to create snapshot")
		return apierror.Internal("failed to create BEP snapshot")
	}

	// Auto-provision default sensitivity configs
	defaults := model.DefaultSensitivityConfigs(snap.TenantID, snap.ID)
	for i := range defaults {
		if err := s.bepRepo.UpsertSensitivityConfig(&defaults[i]); err != nil {
			// Non-fatal: log and continue
			s.logger.WithError(err).Warn("bep: failed to create default sensitivity config")
		}
	}

	s.emitter.Publish(event.Event{
		Type: event.DataChanged, TenantID: snap.TenantID, UserID: ctxutil.GetUserID(ctx), ScenarioID: snap.ScenarioID,
		EntityType: "bep_snapshot", Action: event.ActionCreate,
		Changes: marshalChanges(map[string]any{
			"label":                 snap.Label,
			"fixedCostsTotal":       snap.FixedCostsTotal,
			"contributionMarginPct": snap.ContributionMarginPct,
		}),
	})
	return nil
}

// GetSnapshot returns a single snapshot by ID.
func (s *BEPService) GetSnapshot(ctx context.Context, tenantID, id uuid.UUID) (*model.BEPSnapshot, error) {
	snap, err := s.bepRepo.GetSnapshot(tenantID, id)
	if err != nil {
		return nil, apierror.NotFound("BEP snapshot", id.String())
	}
	return snap, nil
}

// UpdateSnapshot updates a snapshot's inputs and metadata.
func (s *BEPService) UpdateSnapshot(ctx context.Context, snap *model.BEPSnapshot) error {
	if err := validateSnapshotInputs(snap); err != nil {
		return err
	}
	// Confirm ownership before update
	if _, err := s.bepRepo.GetSnapshot(snap.TenantID, snap.ID); err != nil {
		return apierror.NotFound("BEP snapshot", snap.ID.String())
	}
	if err := s.bepRepo.UpdateSnapshot(snap); err != nil {
		s.logger.WithError(err).Error("bep: failed to update snapshot")
		return apierror.Internal("failed to update BEP snapshot")
	}
	s.emitter.Publish(event.Event{
		Type: event.DataChanged, TenantID: snap.TenantID, UserID: ctxutil.GetUserID(ctx), ScenarioID: snap.ScenarioID,
		EntityType: "bep_snapshot", Action: event.ActionUpdate,
		Changes: marshalChanges(map[string]any{
			"label":                 snap.Label,
			"fixedCostsTotal":       snap.FixedCostsTotal,
			"contributionMarginPct": snap.ContributionMarginPct,
		}),
	})
	return nil
}

// DeleteSnapshot deletes a snapshot and all its children.
func (s *BEPService) DeleteSnapshot(ctx context.Context, tenantID, scenarioID, id uuid.UUID) error {
	if _, err := s.bepRepo.GetSnapshot(tenantID, id); err != nil {
		return apierror.NotFound("BEP snapshot", id.String())
	}
	if err := s.bepRepo.DeleteSnapshot(tenantID, id); err != nil {
		s.logger.WithError(err).Error("bep: failed to delete snapshot")
		return apierror.Internal("failed to delete BEP snapshot")
	}
	s.emitter.Publish(event.Event{
		Type: event.DataChanged, TenantID: tenantID, UserID: ctxutil.GetUserID(ctx), ScenarioID: scenarioID,
		EntityType: "bep_snapshot", Action: event.ActionDelete,
		Changes: marshalChanges(map[string]any{"snapshotId": id}),
	})
	return nil
}

// ─── Fixed cost lines ─────────────────────────────────────────────────────────

// ListFixedCostLines returns all fixed cost lines for a snapshot.
func (s *BEPService) ListFixedCostLines(ctx context.Context, tenantID, snapshotID uuid.UUID) ([]*model.FixedCostLine, error) {
	rows, err := s.bepRepo.ListFixedCostLines(tenantID, snapshotID)
	if err != nil {
		s.logger.WithError(err).Error("bep: failed to list fixed cost lines")
		return nil, apierror.Internal("failed to list fixed cost lines")
	}
	return rows, nil
}

// UpsertFixedCostLines batch-upserts fixed cost lines for a snapshot.
func (s *BEPService) UpsertFixedCostLines(ctx context.Context, tenantID, snapshotID uuid.UUID, lines []model.FixedCostLine) error {
	for _, l := range lines {
		if l.AmountAnnual.LessThan(decimal.Zero) {
			return apierror.ValidationError("fixed cost amounts must be non-negative", l.Label)
		}
	}
	if err := s.bepRepo.BatchUpsertFixedCostLines(tenantID, snapshotID, lines); err != nil {
		s.logger.WithError(err).Error("bep: failed to upsert fixed cost lines")
		return apierror.Internal("failed to save fixed cost lines")
	}
	s.publishSnapshotChange(ctx, tenantID, snapshotID)
	return nil
}

// ─── Variable cost lines ──────────────────────────────────────────────────────

// ListVariableCostLines returns all variable cost lines for a snapshot.
func (s *BEPService) ListVariableCostLines(ctx context.Context, tenantID, snapshotID uuid.UUID) ([]*model.VariableCostLine, error) {
	rows, err := s.bepRepo.ListVariableCostLines(tenantID, snapshotID)
	if err != nil {
		s.logger.WithError(err).Error("bep: failed to list variable cost lines")
		return nil, apierror.Internal("failed to list variable cost lines")
	}
	return rows, nil
}

// UpsertVariableCostLines batch-upserts variable cost lines for a snapshot.
func (s *BEPService) UpsertVariableCostLines(ctx context.Context, tenantID, snapshotID uuid.UUID, lines []model.VariableCostLine) error {
	snap, err := s.bepRepo.GetSnapshot(tenantID, snapshotID)
	if err != nil {
		return apierror.NotFound("BEP snapshot", snapshotID.String())
	}

	for _, l := range lines {
		if l.AmountPerUnit.LessThan(decimal.Zero) {
			return apierror.ValidationError("variable cost amounts must be non-negative", l.Label)
		}
		// Variable cost per unit must be less than avg order value (spec §9.1)
		if snap.AvgOrderValue != nil && !snap.AvgOrderValue.IsZero() {
			if l.AmountPerUnit.GreaterThanOrEqual(*snap.AvgOrderValue) {
				return apierror.ValidationError(
					"variable cost per unit must be less than the average order value — contribution margin would be zero or negative",
					l.Label,
				)
			}
		}
	}

	if err := s.bepRepo.BatchUpsertVariableCostLines(tenantID, snapshotID, lines); err != nil {
		s.logger.WithError(err).Error("bep: failed to upsert variable cost lines")
		return apierror.Internal("failed to save variable cost lines")
	}
	s.publishSnapshotChange(ctx, tenantID, snapshotID)
	return nil
}

// ─── Sensitivity configs ──────────────────────────────────────────────────────

// ListSensitivityConfigs returns all sensitivity configs for a snapshot.
func (s *BEPService) ListSensitivityConfigs(ctx context.Context, tenantID, snapshotID uuid.UUID) ([]*model.SensitivityConfig, error) {
	rows, err := s.bepRepo.ListSensitivityConfigs(tenantID, snapshotID)
	if err != nil {
		return nil, apierror.Internal("failed to list sensitivity configs")
	}
	return rows, nil
}

// UpsertSensitivityConfig creates or updates a sensitivity config.
func (s *BEPService) UpsertSensitivityConfig(ctx context.Context, cfg *model.SensitivityConfig) error {
	if cfg.StepSizePct.LessThanOrEqual(decimal.Zero) {
		return apierror.ValidationError("step size must be greater than zero", nil)
	}
	if cfg.RangePct.LessThanOrEqual(decimal.Zero) {
		return apierror.ValidationError("range must be greater than zero", nil)
	}
	cfg.IsDefault = false
	if err := s.bepRepo.UpsertSensitivityConfig(cfg); err != nil {
		s.logger.WithError(err).Error("bep: failed to upsert sensitivity config")
		return apierror.Internal("failed to save sensitivity config")
	}
	return nil
}

// ─── Computed reports ─────────────────────────────────────────────────────────

// GetBEPReport computes and returns the full BEP report for a snapshot.
func (s *BEPService) GetBEPReport(ctx context.Context, tenantID, snapshotID uuid.UUID) (*model.BEPReport, error) {
	snap, err := s.bepRepo.GetSnapshot(tenantID, snapshotID)
	if err != nil {
		return nil, apierror.NotFound("BEP snapshot", snapshotID.String())
	}

	configs, err := s.bepRepo.ListSensitivityConfigs(tenantID, snapshotID)
	if err != nil {
		s.logger.WithError(err).Warn("bep: failed to load sensitivity configs, using defaults")
		configs = nil
	}

	// Dereference to value slice
	cfgVals := make([]model.SensitivityConfig, len(configs))
	for i, c := range configs {
		cfgVals[i] = *c
	}

	report := compute.ComputeBEPReport(*snap, cfgVals)
	return &report, nil
}

// GetPCGAccounts returns the static PCG account reference list.
func (s *BEPService) GetPCGAccounts() []model.PCGAccount {
	return model.PCGAccounts
}

// ─── Optimisation Plans ───────────────────────────────────────────────────────

// ListOptimisationPlans returns all optimisation plans for a snapshot.
func (s *BEPService) ListOptimisationPlans(ctx context.Context, tenantID, snapshotID uuid.UUID) ([]*model.OptimisationPlan, error) {
	rows, err := s.bepRepo.ListOptimisationPlans(tenantID, snapshotID)
	if err != nil {
		return nil, apierror.Internal("failed to list optimisation plans")
	}
	return rows, nil
}

// CreateOptimisationPlan creates a new optimisation plan linked to a snapshot.
func (s *BEPService) CreateOptimisationPlan(ctx context.Context, plan *model.OptimisationPlan) error {
	if plan.Name == "" {
		return apierror.ValidationError("plan name is required", nil)
	}
	if _, err := s.bepRepo.GetSnapshot(plan.TenantID, plan.SnapshotID); err != nil {
		return apierror.NotFound("BEP snapshot", plan.SnapshotID.String())
	}
	if err := s.bepRepo.CreateOptimisationPlan(plan); err != nil {
		s.logger.WithError(err).Error("bep: failed to create optimisation plan")
		return apierror.Internal("failed to create optimisation plan")
	}
	return nil
}

// GetOptimisationPlan returns a single optimisation plan.
func (s *BEPService) GetOptimisationPlan(ctx context.Context, tenantID, id uuid.UUID) (*model.OptimisationPlan, error) {
	plan, err := s.bepRepo.GetOptimisationPlan(tenantID, id)
	if err != nil {
		return nil, apierror.NotFound("optimisation plan", id.String())
	}
	return plan, nil
}

// UpdateOptimisationPlan updates an optimisation plan.
func (s *BEPService) UpdateOptimisationPlan(ctx context.Context, plan *model.OptimisationPlan) error {
	if _, err := s.bepRepo.GetOptimisationPlan(plan.TenantID, plan.ID); err != nil {
		return apierror.NotFound("optimisation plan", plan.ID.String())
	}
	if err := s.bepRepo.UpdateOptimisationPlan(plan); err != nil {
		s.logger.WithError(err).Error("bep: failed to update optimisation plan")
		return apierror.Internal("failed to update optimisation plan")
	}
	return nil
}

// DeleteOptimisationPlan deletes an optimisation plan and all its savings records.
func (s *BEPService) DeleteOptimisationPlan(ctx context.Context, tenantID, id uuid.UUID) error {
	if _, err := s.bepRepo.GetOptimisationPlan(tenantID, id); err != nil {
		return apierror.NotFound("optimisation plan", id.String())
	}
	if err := s.bepRepo.DeleteOptimisationPlan(tenantID, id); err != nil {
		s.logger.WithError(err).Error("bep: failed to delete optimisation plan")
		return apierror.Internal("failed to delete optimisation plan")
	}
	return nil
}

// ─── Savings & PCG ────────────────────────────────────────────────────────────

// ListFixedCostSavings returns all fixed cost savings for a plan.
func (s *BEPService) ListFixedCostSavings(ctx context.Context, tenantID, planID uuid.UUID) ([]*model.FixedCostSaving, error) {
	rows, err := s.bepRepo.ListFixedCostSavings(tenantID, planID)
	if err != nil {
		return nil, apierror.Internal("failed to list fixed cost savings")
	}
	return rows, nil
}

// UpsertFixedCostSavings batch-upserts fixed cost savings for a plan.
// Validates that saving ≤ current cost for each line.
func (s *BEPService) UpsertFixedCostSavings(ctx context.Context, tenantID, planID uuid.UUID, savings []model.FixedCostSaving) error {
	if _, err := s.bepRepo.GetOptimisationPlan(tenantID, planID); err != nil {
		return apierror.NotFound("optimisation plan", planID.String())
	}
	for _, sv := range savings {
		if sv.SavingAmount.LessThan(decimal.Zero) {
			return apierror.ValidationError("saving amount must be non-negative", sv.FixedCostLineID.String())
		}
		if sv.SavingAmount.GreaterThan(sv.NewAmount.Add(sv.SavingAmount)) {
			// NewAmount should equal CurrentCost − SavingAmount → saving cannot exceed current
			return apierror.ValidationError("saving amount cannot exceed the current cost", sv.FixedCostLineID.String())
		}
	}
	if err := s.bepRepo.BatchUpsertFixedCostSavings(tenantID, planID, savings); err != nil {
		s.logger.WithError(err).Error("bep: failed to upsert fixed cost savings")
		return apierror.Internal("failed to save fixed cost savings")
	}
	return nil
}

// ListVariableCostSavings returns all variable cost savings for a plan.
func (s *BEPService) ListVariableCostSavings(ctx context.Context, tenantID, planID uuid.UUID) ([]*model.VariableCostSaving, error) {
	rows, err := s.bepRepo.ListVariableCostSavings(tenantID, planID)
	if err != nil {
		return nil, apierror.Internal("failed to list variable cost savings")
	}
	return rows, nil
}

// UpsertVariableCostSavings batch-upserts variable cost savings for a plan.
func (s *BEPService) UpsertVariableCostSavings(ctx context.Context, tenantID, planID uuid.UUID, savings []model.VariableCostSaving) error {
	if _, err := s.bepRepo.GetOptimisationPlan(tenantID, planID); err != nil {
		return apierror.NotFound("optimisation plan", planID.String())
	}
	for _, sv := range savings {
		if sv.SavingAmount.LessThan(decimal.Zero) {
			return apierror.ValidationError("saving amount must be non-negative", sv.VariableCostLineID.String())
		}
	}
	if err := s.bepRepo.BatchUpsertVariableCostSavings(tenantID, planID, savings); err != nil {
		s.logger.WithError(err).Error("bep: failed to upsert variable cost savings")
		return apierror.Internal("failed to save variable cost savings")
	}
	return nil
}

// ListPCGReviewItems returns all PCG review items for a plan.
func (s *BEPService) ListPCGReviewItems(ctx context.Context, tenantID, planID uuid.UUID) ([]*model.PCGReviewItem, error) {
	rows, err := s.bepRepo.ListPCGReviewItems(tenantID, planID)
	if err != nil {
		return nil, apierror.Internal("failed to list PCG review items")
	}
	return rows, nil
}

// UpsertPCGReviewItems batch-upserts PCG review items for a plan.
func (s *BEPService) UpsertPCGReviewItems(ctx context.Context, tenantID, planID uuid.UUID, items []model.PCGReviewItem) error {
	if _, err := s.bepRepo.GetOptimisationPlan(tenantID, planID); err != nil {
		return apierror.NotFound("optimisation plan", planID.String())
	}
	if err := s.bepRepo.BatchUpsertPCGReviewItems(tenantID, planID, items); err != nil {
		s.logger.WithError(err).Error("bep: failed to upsert PCG review items")
		return apierror.Internal("failed to save PCG review items")
	}
	return nil
}

// GetOptimisedBEPReport computes the optimised break-even for a given plan.
func (s *BEPService) GetOptimisedBEPReport(ctx context.Context, tenantID, planID uuid.UUID) (*model.OptimisedBEPReport, error) {
	plan, err := s.bepRepo.GetOptimisationPlan(tenantID, planID)
	if err != nil {
		return nil, apierror.NotFound("optimisation plan", planID.String())
	}
	snap, err := s.bepRepo.GetSnapshot(tenantID, plan.SnapshotID)
	if err != nil {
		return nil, apierror.NotFound("BEP snapshot", plan.SnapshotID.String())
	}

	// Sum fixed cost savings
	fixedSavings, err := s.bepRepo.ListFixedCostSavings(tenantID, planID)
	if err != nil {
		return nil, apierror.Internal("failed to load fixed cost savings")
	}
	totalFixedSavings := decimal.Zero
	for _, sv := range fixedSavings {
		totalFixedSavings = totalFixedSavings.Add(sv.SavingAmount)
	}

	// Sum new variable cost per unit (baseline lines minus savings)
	varLines, err := s.bepRepo.ListVariableCostLines(tenantID, snap.ID)
	if err != nil {
		return nil, apierror.Internal("failed to load variable cost lines")
	}
	varSavings, err := s.bepRepo.ListVariableCostSavings(tenantID, planID)
	if err != nil {
		return nil, apierror.Internal("failed to load variable cost savings")
	}

	// Build saving map: varCostLineID → saving amount
	savingByLine := make(map[uuid.UUID]decimal.Decimal, len(varSavings))
	for _, sv := range varSavings {
		savingByLine[sv.VariableCostLineID] = sv.SavingAmount
	}

	newTotalVarCostPerUnit := decimal.Zero
	for _, line := range varLines {
		saving := savingByLine[line.ID]
		net := line.AmountPerUnit.Sub(saving)
		if net.LessThan(decimal.Zero) {
			net = decimal.Zero
		}
		newTotalVarCostPerUnit = newTotalVarCostPerUnit.Add(net)
	}

	report := compute.ComputeOptimisedBEP(
		snap.FixedCostsTotal,
		snap.ContributionMarginPct,
		snap.AvgOrderValue,
		totalFixedSavings,
		newTotalVarCostPerUnit,
		len(varLines) > 0,
	)
	report.SnapshotID = snap.ID
	report.PlanID = planID

	return &report, nil
}

// ─── Plan import ──────────────────────────────────────────────────────────────

// ImportFromPlan reads the computed plan report for the given scenario and
// year, then overwrites the snapshot's BEP inputs with values derived from
// the plan data:
//
//   - FixedCostsTotal = Payroll[year] + Opex[year]
//   - ContributionMarginPct = Revenue.GrossMarginPct[year] × 100  (ratio → percentage)
//   - AvgOrderValue = TotalTurnover / TotalUnitSales              (omitted when 0 units)
//
// yearIndex is 0-based (0 = Year 1, 4 = Year 5).
func (s *BEPService) ImportFromPlan(
	ctx context.Context,
	tenantID, scenarioID, snapshotID uuid.UUID,
	yearIndex int,
) (*model.BEPSnapshot, error) {
	if yearIndex < 0 || yearIndex >= compute.MaxYears {
		return nil, apierror.ValidationError(
			fmt.Sprintf("year must be between 1 and %d", compute.MaxYears), "year",
		)
	}

	if s.reportSvc == nil {
		return nil, apierror.Internal("plan report service is not available")
	}

	// Load existing snapshot to confirm ownership and preserve metadata.
	snap, err := s.bepRepo.GetSnapshot(tenantID, snapshotID)
	if err != nil {
		return nil, apierror.NotFound("BEP snapshot", snapshotID.String())
	}

	// Fetch the full plan report (cached unless data changed).
	report, err := s.reportSvc.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("bep: failed to fetch plan report for import")
		return nil, apierror.Internal("failed to fetch plan report")
	}

	// ── Derive BEP inputs ────────────────────────────────────────────────────
	payroll := report.Payroll.Payroll[yearIndex].TotalPayroll
	opex := report.Opex.GrandTotal[yearIndex]
	snap.FixedCostsTotal = payroll.Add(opex)

	// GrossMarginPct is stored as a ratio (e.g. 0.43 for 43%).
	// ContributionMarginPct expects a percentage in the range [0, 100].
	snap.ContributionMarginPct = report.Revenue.Totals[yearIndex].GrossMarginPct.Mul(decimal.NewFromInt(100))

	totalUnits := report.Revenue.Totals[yearIndex].TotalUnitSales
	totalTurnover := report.Revenue.Totals[yearIndex].TotalTurnover
	if totalUnits > 0 && !totalTurnover.IsZero() {
		avgOrder := totalTurnover.Div(decimal.NewFromInt(totalUnits))
		snap.AvgOrderValue = &avgOrder
	}

	snap.Source = model.BEPSourceImported

	if err := s.bepRepo.UpdateSnapshot(snap); err != nil {
		s.logger.WithError(err).Error("bep: failed to persist snapshot after plan import")
		return nil, apierror.Internal("failed to save snapshot after import")
	}

	s.emitter.Publish(event.Event{
		Type: event.DataChanged, TenantID: tenantID, UserID: ctxutil.GetUserID(ctx), ScenarioID: scenarioID,
		EntityType: "bep_snapshot", Action: event.ActionUpdate,
		Changes: marshalChanges(map[string]any{"source": "imported", "yearIndex": yearIndex}),
	})

	return snap, nil
}

// BEPPlanPreview holds the BEP inputs derived from a plan year without
// committing them to any snapshot. Used to pre-populate the creation form.
type BEPPlanPreview struct {
	FixedCostsTotal       decimal.Decimal  `json:"fixedCostsTotal"`
	ContributionMarginPct decimal.Decimal  `json:"contributionMarginPct"`
	AvgOrderValue         *decimal.Decimal `json:"avgOrderValue,omitempty"`
	YearIndex             int              `json:"yearIndex"`
}

// PreviewFromPlan computes the same BEP inputs as ImportFromPlan but returns
// them as a plain struct without writing to the database. Intended to
// pre-populate the "New Snapshot" form before the user commits.
func (s *BEPService) PreviewFromPlan(
	ctx context.Context,
	tenantID, scenarioID uuid.UUID,
	yearIndex int,
) (*BEPPlanPreview, error) {
	if yearIndex < 0 || yearIndex >= compute.MaxYears {
		return nil, apierror.ValidationError(
			fmt.Sprintf("year must be between 1 and %d", compute.MaxYears), "year",
		)
	}
	if s.reportSvc == nil {
		return nil, apierror.Internal("plan report service is not available")
	}

	report, err := s.reportSvc.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("bep: failed to fetch plan report for preview")
		return nil, apierror.Internal("failed to fetch plan report")
	}

	preview := &BEPPlanPreview{
		FixedCostsTotal: report.Payroll.Payroll[yearIndex].TotalPayroll.Add(report.Opex.GrandTotal[yearIndex]),
		// GrossMarginPct is a ratio (0–1); ContributionMarginPct must be a percentage (0–100).
		ContributionMarginPct: report.Revenue.Totals[yearIndex].GrossMarginPct.Mul(decimal.NewFromInt(100)),
		YearIndex:             yearIndex,
	}

	totalUnits := report.Revenue.Totals[yearIndex].TotalUnitSales
	totalTurnover := report.Revenue.Totals[yearIndex].TotalTurnover
	if totalUnits > 0 && !totalTurnover.IsZero() {
		avgOrder := totalTurnover.Div(decimal.NewFromInt(totalUnits))
		preview.AvgOrderValue = &avgOrder
	}

	return preview, nil
}

// ─── Multi-year BEP ───────────────────────────────────────────────────────────

// GetMultiYearBEPReport derives the 5-year break-even report directly from the
// plan — no snapshot is required.  The report is always computed fresh; there
// is no database round-trip beyond fetching the plan itself.
func (s *BEPService) GetMultiYearBEPReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.MultiYearBEPReport, error) {
	if s.reportSvc == nil {
		return nil, apierror.Internal("plan report service is not available")
	}

	fullPlan, err := s.reportSvc.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("bep: failed to fetch plan report for multi-year BEP")
		return nil, apierror.Internal("failed to fetch plan report")
	}

	result := compute.ComputeMultiYearBEP(fullPlan)
	return &result, nil
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

// validateSnapshotInputs enforces the BEP spec §9.1 input constraints.
func validateSnapshotInputs(snap *model.BEPSnapshot) error {
	if snap.Label == "" {
		return apierror.ValidationError("snapshot label is required", nil)
	}
	if snap.FixedCostsTotal.LessThan(decimal.Zero) {
		return apierror.ValidationError("fixed costs total must be non-negative", nil)
	}
	// Margin capped between 0.000001% and 100%
	minMargin := decimal.NewFromFloat(0.000001)
	maxMargin := decimal.NewFromInt(100)
	if snap.ContributionMarginPct.LessThan(decimal.Zero) || snap.ContributionMarginPct.GreaterThan(maxMargin) {
		return apierror.ValidationError("contribution margin must be between 0 and 100 %", nil)
	}
	// Allow 0 for the undefined-state case; only warn (via compute engine), do not reject.
	// But cap minimum positive margin to avoid precision issues.
	if snap.ContributionMarginPct.GreaterThan(decimal.Zero) && snap.ContributionMarginPct.LessThan(minMargin) {
		return apierror.ValidationError("contribution margin is too small (minimum 0.000001 %)", nil)
	}
	if snap.AvgOrderValue != nil && snap.AvgOrderValue.LessThanOrEqual(decimal.Zero) {
		return apierror.ValidationError("average order value must be positive when provided", nil)
	}
	return nil
}

// publish fires a DataChanged event on the emitter.
func (s *BEPService) publish(ctx context.Context, tenantID, scenarioID uuid.UUID, entityType string, action event.Action) {
	if s.emitter == nil {
		return
	}
	s.emitter.Publish(event.Event{
		Type:       event.DataChanged,
		TenantID:   tenantID,
		UserID:     ctxutil.GetUserID(ctx),
		ScenarioID: scenarioID,
		EntityType: entityType,
		Action:     action,
	})
}

// publishSnapshotChange emits a DataChanged event, resolving scenarioID from the snapshot.
func (s *BEPService) publishSnapshotChange(ctx context.Context, tenantID, snapshotID uuid.UUID) {
	snap, err := s.bepRepo.GetSnapshot(tenantID, snapshotID)
	if err != nil {
		return
	}
	s.publish(ctx, tenantID, snap.ScenarioID, "bep_snapshot", event.ActionUpdate)
}
