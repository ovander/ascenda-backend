// Package handler — Break-Even Point HTTP handler.
package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"ascenda/internal/service"
)

// BEPServicer defines the service interface the BEPHandler depends on.
// Using an interface enables full mock-based testing without a database.
type BEPServicer interface {
	// Snapshots
	ListSnapshots(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]*model.BEPSnapshot, error)
	CreateSnapshot(ctx context.Context, snap *model.BEPSnapshot) error
	GetSnapshot(ctx context.Context, tenantID, id uuid.UUID) (*model.BEPSnapshot, error)
	UpdateSnapshot(ctx context.Context, snap *model.BEPSnapshot) error
	DeleteSnapshot(ctx context.Context, tenantID, scenarioID, id uuid.UUID) error

	// Fixed / variable cost lines
	ListFixedCostLines(ctx context.Context, tenantID, snapshotID uuid.UUID) ([]*model.FixedCostLine, error)
	UpsertFixedCostLines(ctx context.Context, tenantID, snapshotID uuid.UUID, lines []model.FixedCostLine) error
	ListVariableCostLines(ctx context.Context, tenantID, snapshotID uuid.UUID) ([]*model.VariableCostLine, error)
	UpsertVariableCostLines(ctx context.Context, tenantID, snapshotID uuid.UUID, lines []model.VariableCostLine) error

	// Sensitivity configs
	ListSensitivityConfigs(ctx context.Context, tenantID, snapshotID uuid.UUID) ([]*model.SensitivityConfig, error)
	UpsertSensitivityConfig(ctx context.Context, cfg *model.SensitivityConfig) error

	// Computed reports
	GetBEPReport(ctx context.Context, tenantID, snapshotID uuid.UUID) (*model.BEPReport, error)
	GetPCGAccounts() []model.PCGAccount

	// Optimisation plans
	ListOptimisationPlans(ctx context.Context, tenantID, snapshotID uuid.UUID) ([]*model.OptimisationPlan, error)
	CreateOptimisationPlan(ctx context.Context, plan *model.OptimisationPlan) error
	GetOptimisationPlan(ctx context.Context, tenantID, id uuid.UUID) (*model.OptimisationPlan, error)
	UpdateOptimisationPlan(ctx context.Context, plan *model.OptimisationPlan) error
	DeleteOptimisationPlan(ctx context.Context, tenantID, id uuid.UUID) error

	// Savings
	ListFixedCostSavings(ctx context.Context, tenantID, planID uuid.UUID) ([]*model.FixedCostSaving, error)
	UpsertFixedCostSavings(ctx context.Context, tenantID, planID uuid.UUID, savings []model.FixedCostSaving) error
	ListVariableCostSavings(ctx context.Context, tenantID, planID uuid.UUID) ([]*model.VariableCostSaving, error)
	UpsertVariableCostSavings(ctx context.Context, tenantID, planID uuid.UUID, savings []model.VariableCostSaving) error

	// PCG review
	ListPCGReviewItems(ctx context.Context, tenantID, planID uuid.UUID) ([]*model.PCGReviewItem, error)
	UpsertPCGReviewItems(ctx context.Context, tenantID, planID uuid.UUID, items []model.PCGReviewItem) error

	// Optimised report
	GetOptimisedBEPReport(ctx context.Context, tenantID, planID uuid.UUID) (*model.OptimisedBEPReport, error)

	// Plan import
	ImportFromPlan(ctx context.Context, tenantID, scenarioID, snapshotID uuid.UUID, yearIndex int) (*model.BEPSnapshot, error)

	// Plan preview (read-only — used to pre-populate the creation form)
	PreviewFromPlan(ctx context.Context, tenantID, scenarioID uuid.UUID, yearIndex int) (*service.BEPPlanPreview, error)

	// Multi-year BEP report (derived from plan, no snapshot required)
	GetMultiYearBEPReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.MultiYearBEPReport, error)
}

// Compile-time check that *service.BEPService satisfies the interface.
var _ BEPServicer = (*service.BEPService)(nil)

// BEPHandler handles all BEP HTTP endpoints.
type BEPHandler struct {
	svc    BEPServicer
	logger *logrus.Entry
}

// NewBEPHandler creates a new BEPHandler.
func NewBEPHandler(svc BEPServicer, logger *logrus.Entry) *BEPHandler {
	return &BEPHandler{svc: svc, logger: logger}
}

// ─── URL parameter helpers ────────────────────────────────────────────────────

func (h *BEPHandler) snapshotID(r *http.Request) (uuid.UUID, error) {
	return parseUUIDParam(chi.URLParam(r, "snapshotId"))
}

// planIDParam reads the BEP optimisation plan ID. The route uses {optPlanId}
// so the financial plan's {planId} higher up the tree is never shadowed.
func (h *BEPHandler) planIDParam(r *http.Request) (uuid.UUID, error) {
	return parseUUIDParam(chi.URLParam(r, "optPlanId"))
}

// ─── Snapshots ────────────────────────────────────────────────────────────────

// ListSnapshots GET /bep/snapshots
func (h *BEPHandler) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	rows, err := h.svc.ListSnapshots(r.Context(), tenantID, sid)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, rows)
}

// CreateSnapshot POST /bep/snapshots
func (h *BEPHandler) CreateSnapshot(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	var snap model.BEPSnapshot
	if err := decodeAndValidate(r, &snap); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	snap.TenantID = tenantID
	snap.ScenarioID = sid
	if err := h.svc.CreateSnapshot(r.Context(), &snap); err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, snap)
}

// GetSnapshot GET /bep/snapshots/{snapshotId}
func (h *BEPHandler) GetSnapshot(w http.ResponseWriter, r *http.Request) {
	snapID, err := h.snapshotID(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	snap, err := h.svc.GetSnapshot(r.Context(), tenantID, snapID)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, snap)
}

// UpdateSnapshot PUT /bep/snapshots/{snapshotId}
func (h *BEPHandler) UpdateSnapshot(w http.ResponseWriter, r *http.Request) {
	snapID, err := h.snapshotID(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	var snap model.BEPSnapshot
	if err := decodeAndValidate(r, &snap); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	snap.TenantID = tenantID
	snap.ID = snapID
	snap.ScenarioID, _ = parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err := h.svc.UpdateSnapshot(r.Context(), &snap); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// DeleteSnapshot DELETE /bep/snapshots/{snapshotId}
func (h *BEPHandler) DeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	snapID, err := h.snapshotID(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	sid, _ := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.DeleteSnapshot(r.Context(), tenantID, sid, snapID); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// ─── Fixed cost lines ─────────────────────────────────────────────────────────

// ListFixedCostLines GET /bep/snapshots/{snapshotId}/fixed-costs
func (h *BEPHandler) ListFixedCostLines(w http.ResponseWriter, r *http.Request) {
	snapID, err := h.snapshotID(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	rows, err := h.svc.ListFixedCostLines(r.Context(), tenantID, snapID)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, rows)
}

// UpsertFixedCostLines PUT /bep/snapshots/{snapshotId}/fixed-costs
func (h *BEPHandler) UpsertFixedCostLines(w http.ResponseWriter, r *http.Request) {
	snapID, err := h.snapshotID(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	var lines []model.FixedCostLine
	if err := decodeJSON(r, &lines); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpsertFixedCostLines(r.Context(), tenantID, snapID, lines); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// ─── Variable cost lines ──────────────────────────────────────────────────────

// ListVariableCostLines GET /bep/snapshots/{snapshotId}/variable-costs
func (h *BEPHandler) ListVariableCostLines(w http.ResponseWriter, r *http.Request) {
	snapID, err := h.snapshotID(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	rows, err := h.svc.ListVariableCostLines(r.Context(), tenantID, snapID)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, rows)
}

// UpsertVariableCostLines PUT /bep/snapshots/{snapshotId}/variable-costs
func (h *BEPHandler) UpsertVariableCostLines(w http.ResponseWriter, r *http.Request) {
	snapID, err := h.snapshotID(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	var lines []model.VariableCostLine
	if err := decodeJSON(r, &lines); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpsertVariableCostLines(r.Context(), tenantID, snapID, lines); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// ─── Sensitivity configs ──────────────────────────────────────────────────────

// ListSensitivityConfigs GET /bep/snapshots/{snapshotId}/sensitivity-config
func (h *BEPHandler) ListSensitivityConfigs(w http.ResponseWriter, r *http.Request) {
	snapID, err := h.snapshotID(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	rows, err := h.svc.ListSensitivityConfigs(r.Context(), tenantID, snapID)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, rows)
}

// UpsertSensitivityConfig PUT /bep/snapshots/{snapshotId}/sensitivity-config/{analysisType}
func (h *BEPHandler) UpsertSensitivityConfig(w http.ResponseWriter, r *http.Request) {
	snapID, err := h.snapshotID(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	analysisType := model.BEPSensitivityType(chi.URLParam(r, "analysisType"))
	switch analysisType {
	case model.SensRevenue, model.SensMargin, model.SensFixedCost:
		// valid
	default:
		handleError(w, r, apierror.BadRequest("invalid analysis type: "+string(analysisType)+
			" — must be 'revenue', 'margin', or 'fixed_cost'"))
		return
	}
	var cfg model.SensitivityConfig
	if err := decodeAndValidate(r, &cfg); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	cfg.TenantID = tenantID
	cfg.SnapshotID = snapID
	cfg.AnalysisType = analysisType
	if err := h.svc.UpsertSensitivityConfig(r.Context(), &cfg); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// ─── Computed reports ─────────────────────────────────────────────────────────

// GetBEPReport GET /bep/snapshots/{snapshotId}/report
func (h *BEPHandler) GetBEPReport(w http.ResponseWriter, r *http.Request) {
	snapID, err := h.snapshotID(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	report, err := h.svc.GetBEPReport(r.Context(), tenantID, snapID)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, report)
}

// GetPCGAccounts GET /bep/pcg-accounts
func (h *BEPHandler) GetPCGAccounts(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, h.svc.GetPCGAccounts())
}

// ─── Optimisation plans ───────────────────────────────────────────────────────

// ListOptimisationPlans GET /bep/snapshots/{snapshotId}/plans
func (h *BEPHandler) ListOptimisationPlans(w http.ResponseWriter, r *http.Request) {
	snapID, err := h.snapshotID(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	rows, err := h.svc.ListOptimisationPlans(r.Context(), tenantID, snapID)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, rows)
}

// CreateOptimisationPlan POST /bep/snapshots/{snapshotId}/plans
func (h *BEPHandler) CreateOptimisationPlan(w http.ResponseWriter, r *http.Request) {
	snapID, err := h.snapshotID(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	var plan model.OptimisationPlan
	if err := decodeAndValidate(r, &plan); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	plan.TenantID = tenantID
	plan.SnapshotID = snapID
	plan.CreatedBy = ctxutil.GetUserID(r.Context())
	if err := h.svc.CreateOptimisationPlan(r.Context(), &plan); err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, plan)
}

// GetOptimisationPlan GET /bep/snapshots/{snapshotId}/plans/{planId}
func (h *BEPHandler) GetOptimisationPlan(w http.ResponseWriter, r *http.Request) {
	planID, err := h.planIDParam(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	plan, err := h.svc.GetOptimisationPlan(r.Context(), tenantID, planID)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, plan)
}

// UpdateOptimisationPlan PUT /bep/snapshots/{snapshotId}/plans/{planId}
func (h *BEPHandler) UpdateOptimisationPlan(w http.ResponseWriter, r *http.Request) {
	planID, err := h.planIDParam(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	var plan model.OptimisationPlan
	if err := decodeAndValidate(r, &plan); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	plan.TenantID = tenantID
	plan.ID = planID
	if err := h.svc.UpdateOptimisationPlan(r.Context(), &plan); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// DeleteOptimisationPlan DELETE /bep/snapshots/{snapshotId}/plans/{planId}
func (h *BEPHandler) DeleteOptimisationPlan(w http.ResponseWriter, r *http.Request) {
	planID, err := h.planIDParam(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.DeleteOptimisationPlan(r.Context(), tenantID, planID); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// ─── Optimised BEP report ─────────────────────────────────────────────────────

// GetOptimisedBEPReport GET /bep/snapshots/{snapshotId}/plans/{planId}/report
func (h *BEPHandler) GetOptimisedBEPReport(w http.ResponseWriter, r *http.Request) {
	planID, err := h.planIDParam(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	report, err := h.svc.GetOptimisedBEPReport(r.Context(), tenantID, planID)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, report)
}

// ─── Fixed cost savings ───────────────────────────────────────────────────────

// ListFixedCostSavings GET /bep/snapshots/{snapshotId}/plans/{planId}/savings/fixed
func (h *BEPHandler) ListFixedCostSavings(w http.ResponseWriter, r *http.Request) {
	planID, err := h.planIDParam(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	rows, err := h.svc.ListFixedCostSavings(r.Context(), tenantID, planID)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, rows)
}

// UpsertFixedCostSavings PUT /bep/snapshots/{snapshotId}/plans/{planId}/savings/fixed
func (h *BEPHandler) UpsertFixedCostSavings(w http.ResponseWriter, r *http.Request) {
	planID, err := h.planIDParam(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	var savings []model.FixedCostSaving
	if err := decodeJSON(r, &savings); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpsertFixedCostSavings(r.Context(), tenantID, planID, savings); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// ─── Variable cost savings ────────────────────────────────────────────────────

// ListVariableCostSavings GET /bep/snapshots/{snapshotId}/plans/{planId}/savings/variable
func (h *BEPHandler) ListVariableCostSavings(w http.ResponseWriter, r *http.Request) {
	planID, err := h.planIDParam(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	rows, err := h.svc.ListVariableCostSavings(r.Context(), tenantID, planID)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, rows)
}

// UpsertVariableCostSavings PUT /bep/snapshots/{snapshotId}/plans/{planId}/savings/variable
func (h *BEPHandler) UpsertVariableCostSavings(w http.ResponseWriter, r *http.Request) {
	planID, err := h.planIDParam(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	var savings []model.VariableCostSaving
	if err := decodeJSON(r, &savings); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpsertVariableCostSavings(r.Context(), tenantID, planID, savings); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// ─── PCG review ───────────────────────────────────────────────────────────────

// ListPCGReviewItems GET /bep/snapshots/{snapshotId}/plans/{planId}/pcg-review
func (h *BEPHandler) ListPCGReviewItems(w http.ResponseWriter, r *http.Request) {
	planID, err := h.planIDParam(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	rows, err := h.svc.ListPCGReviewItems(r.Context(), tenantID, planID)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, rows)
}

// UpsertPCGReviewItems PUT /bep/snapshots/{snapshotId}/plans/{planId}/pcg-review
func (h *BEPHandler) UpsertPCGReviewItems(w http.ResponseWriter, r *http.Request) {
	planID, err := h.planIDParam(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	var items []model.PCGReviewItem
	if err := decodeJSON(r, &items); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpsertPCGReviewItems(r.Context(), tenantID, planID, items); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// ─── Plan import ──────────────────────────────────────────────────────────────

// ImportFromPlan POST /bep/snapshots/{snapshotId}/import-from-plan?year=N
// Reads computed plan data for the requested year (1–5) and overwrites the
// snapshot's BEP inputs (fixed costs, margin %, avg order value) with the
// derived values.
func (h *BEPHandler) ImportFromPlan(w http.ResponseWriter, r *http.Request) {
	snapID, err := h.snapshotID(r)
	if err != nil {
		handleError(w, r, err)
		return
	}
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	// ?year=1..5 — convert to 0-based index
	yearParam := r.URL.Query().Get("year")
	if yearParam == "" {
		handleError(w, r, apierror.ValidationError("query parameter 'year' is required (1–5)", "year"))
		return
	}
	year, convErr := strconv.Atoi(yearParam)
	if convErr != nil || year < 1 || year > 5 {
		handleError(w, r, apierror.ValidationError("'year' must be an integer between 1 and 5", "year"))
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	snap, svcErr := h.svc.ImportFromPlan(r.Context(), tenantID, sid, snapID, year-1)
	if svcErr != nil {
		handleError(w, r, svcErr)
		return
	}
	respondJSON(w, http.StatusOK, snap)
}

// PreviewFromPlan GET /bep/plan-preview?year=N
// Returns the BEP inputs that would be derived from the plan for the given year,
// without creating or modifying any snapshot. Used to pre-populate the creation form.
func (h *BEPHandler) PreviewFromPlan(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	yearParam := r.URL.Query().Get("year")
	if yearParam == "" {
		handleError(w, r, apierror.ValidationError("query parameter 'year' is required (1–5)", "year"))
		return
	}
	year, convErr := strconv.Atoi(yearParam)
	if convErr != nil || year < 1 || year > 5 {
		handleError(w, r, apierror.ValidationError("'year' must be an integer between 1 and 5", "year"))
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	preview, svcErr := h.svc.PreviewFromPlan(r.Context(), tenantID, sid, year-1)
	if svcErr != nil {
		handleError(w, r, svcErr)
		return
	}
	respondJSON(w, http.StatusOK, preview)
}

// ─── Multi-year BEP report ────────────────────────────────────────────────────

// GetMultiYearBEPReport GET /bep/multi-year-report
// Derives the 5-year break-even report directly from the plan.
// No snapshot is required; the result is computed fresh each time.
func (h *BEPHandler) GetMultiYearBEPReport(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	report, svcErr := h.svc.GetMultiYearBEPReport(r.Context(), tenantID, sid)
	if svcErr != nil {
		handleError(w, r, svcErr)
		return
	}
	respondJSON(w, http.StatusOK, report)
}

