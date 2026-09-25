package handler

import (
	"context"
	"net/http"

	"ascenda/internal/model"
	"ascenda/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
)

// CapTableServicer defines the service interface the CapTableHandler depends on.
type CapTableServicer interface {
	GetCompany(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CapTableCompany, error)
	UpsertCompany(ctx context.Context, company *model.CapTableCompany) error
	GetCountryProfile(code string) model.CapTableCountryProfile

	ListShareClasses(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapTableShareClass, error)
	UpsertShareClass(ctx context.Context, sc *model.CapTableShareClass) error
	DeleteShareClass(ctx context.Context, tenantID, id uuid.UUID) error

	ListShareholders(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapTableShareholder, error)
	CreateShareholder(ctx context.Context, sh *model.CapTableShareholder) error
	UpdateShareholder(ctx context.Context, sh *model.CapTableShareholder) error
	DeleteShareholder(ctx context.Context, tenantID, id uuid.UUID) error

	ListRounds(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapTableRound, error)
	CreateRound(ctx context.Context, rnd *model.CapTableRound) error
	UpdateRound(ctx context.Context, rnd *model.CapTableRound) error
	DeleteRound(ctx context.Context, tenantID, id uuid.UUID) error

	ListPlans(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.StockOptionPlan, error)
	CreatePlan(ctx context.Context, plan *model.StockOptionPlan) error
	UpdatePlan(ctx context.Context, plan *model.StockOptionPlan) error
	DeletePlan(ctx context.Context, tenantID, id uuid.UUID) error

	ListGrants(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.OptionGrant, error)
	CreateGrant(ctx context.Context, grant *model.OptionGrant) error
	UpdateGrant(ctx context.Context, grant *model.OptionGrant) error
	DeleteGrant(ctx context.Context, tenantID, id uuid.UUID) error

	ListValuationScenarios(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.ValuationScenario, error)
	CreateValuationScenario(ctx context.Context, vs *model.ValuationScenario) error
	UpdateValuationScenario(ctx context.Context, vs *model.ValuationScenario) error
	DeleteValuationScenario(ctx context.Context, tenantID, id uuid.UUID) error
	ComputeValuationScenario(ctx context.Context, tenantID, id uuid.UUID) (*model.ValuationScenarioResult, error)

	ListBranches(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapTableScenarioBranch, error)
	CreateBranch(ctx context.Context, branch *model.CapTableScenarioBranch) error
	UpdateBranch(ctx context.Context, branch *model.CapTableScenarioBranch) error

	GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CapTableReport, error)

	// FiPlan sync
	SyncRoundToFiplan(ctx context.Context, tenantID uuid.UUID, roundID uuid.UUID, fiscalYearIndex int) (*model.CapTableRound, error)
	UnlinkFromFiplan(ctx context.Context, tenantID uuid.UUID, roundID uuid.UUID) (*model.CapTableRound, error)

	// Opening balance sync (founding capital)
	SyncRoundToOpeningBalance(ctx context.Context, tenantID, roundID uuid.UUID) (*model.CapTableRound, error)
	UnsyncRoundFromOpeningBalance(ctx context.Context, tenantID, roundID uuid.UUID) (*model.CapTableRound, error)
}

// Compile-time check that *service.CapTableService satisfies the interface.
var _ CapTableServicer = (*service.CapTableService)(nil)

// CapTableHandler handles all cap table HTTP endpoints.
type CapTableHandler struct {
	svc    CapTableServicer
	logger *logrus.Entry
}

// NewCapTableHandler creates a new CapTableHandler.
func NewCapTableHandler(svc CapTableServicer, logger *logrus.Entry) *CapTableHandler {
	return &CapTableHandler{svc: svc, logger: logger}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// scenarioID extracts and validates the {scenarioId} URL parameter.
func (h *CapTableHandler) scenarioID(r *http.Request) (interface{}, error) {
	return parseUUIDParam(chi.URLParam(r, "scenarioId"))
}

// ─── CapTableCompany ──────────────────────────────────────────────────────────

// GetCapTableCompany GET /captable/company
func (h *CapTableHandler) GetCapTableCompany(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	company, err := h.svc.GetCompany(r.Context(), tenantID, sid)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, company)
}

// UpsertCapTableCompany PUT /captable/company
func (h *CapTableHandler) UpsertCapTableCompany(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	var company model.CapTableCompany
	if err := decodeAndValidate(r, &company); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	company.TenantID = tenantID
	company.ScenarioID = sid
	if err := h.svc.UpsertCompany(r.Context(), &company); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// GetCapTableCountryProfile GET /captable/country-profile?code=FR
func (h *CapTableHandler) GetCapTableCountryProfile(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		code = "BE"
	}
	profile := h.svc.GetCountryProfile(code)
	respondJSON(w, http.StatusOK, profile)
}

// ─── CapTableShareClass ───────────────────────────────────────────────────────

// ListShareClasses GET /captable/classes
func (h *CapTableHandler) ListShareClasses(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	classes, err := h.svc.ListShareClasses(r.Context(), tenantID, sid)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, classes)
}

// UpsertShareClasses PUT /captable/classes
// Accepts a single CapTableShareClass and upserts it.
func (h *CapTableHandler) UpsertShareClasses(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	var sc model.CapTableShareClass
	if err := decodeAndValidate(r, &sc); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	sc.TenantID = tenantID
	sc.ScenarioID = sid
	if err := h.svc.UpsertShareClass(r.Context(), &sc); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// ─── CapTableShareholder ──────────────────────────────────────────────────────

// ListShareholders GET /captable/shareholders
func (h *CapTableHandler) ListShareholders(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	shareholders, err := h.svc.ListShareholders(r.Context(), tenantID, sid)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, shareholders)
}

// UpsertShareholders PUT /captable/shareholders
// Accepts a single CapTableShareholder; creates if ID is nil, updates otherwise.
func (h *CapTableHandler) UpsertShareholders(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	var sh model.CapTableShareholder
	if err := decodeAndValidate(r, &sh); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	sh.TenantID = tenantID
	sh.ScenarioID = sid

	if sh.ID.String() == "00000000-0000-0000-0000-000000000000" {
		if err := h.svc.CreateShareholder(r.Context(), &sh); err != nil {
			handleError(w, r, err)
			return
		}
		respondJSON(w, http.StatusCreated, sh)
	} else {
		if err := h.svc.UpdateShareholder(r.Context(), &sh); err != nil {
			handleError(w, r, err)
			return
		}
		respondNoContent(w)
	}
}

// DeleteShareholder DELETE /captable/shareholders/{id}
func (h *CapTableHandler) DeleteShareholder(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(chi.URLParam(r, "id"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.DeleteShareholder(r.Context(), tenantID, id); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// ─── CapTableRound ────────────────────────────────────────────────────────────

// ListCapTableRounds GET /captable/rounds
func (h *CapTableHandler) ListCapTableRounds(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	rounds, err := h.svc.ListRounds(r.Context(), tenantID, sid)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, rounds)
}

// UpsertCapTableRounds PUT /captable/rounds
func (h *CapTableHandler) UpsertCapTableRounds(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	var rnd model.CapTableRound
	if err := decodeAndValidate(r, &rnd); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	rnd.TenantID = tenantID
	rnd.ScenarioID = sid

	if rnd.ID.String() == "00000000-0000-0000-0000-000000000000" {
		if err := h.svc.CreateRound(r.Context(), &rnd); err != nil {
			handleError(w, r, err)
			return
		}
		respondJSON(w, http.StatusCreated, rnd)
	} else {
		if err := h.svc.UpdateRound(r.Context(), &rnd); err != nil {
			handleError(w, r, err)
			return
		}
		respondNoContent(w)
	}
}

// DeleteCapTableRound DELETE /captable/rounds/{id}
func (h *CapTableHandler) DeleteCapTableRound(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(chi.URLParam(r, "id"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.DeleteRound(r.Context(), tenantID, id); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// ─── StockOptionPlan ─────────────────────────────────────────────────────────

// ListOptionPlans GET /captable/option-plans
func (h *CapTableHandler) ListOptionPlans(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	plans, err := h.svc.ListPlans(r.Context(), tenantID, sid)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, plans)
}

// UpsertOptionPlans PUT /captable/option-plans
func (h *CapTableHandler) UpsertOptionPlans(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	var plan model.StockOptionPlan
	if err := decodeAndValidate(r, &plan); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	plan.TenantID = tenantID
	plan.ScenarioID = sid

	if plan.ID.String() == "00000000-0000-0000-0000-000000000000" {
		if err := h.svc.CreatePlan(r.Context(), &plan); err != nil {
			handleError(w, r, err)
			return
		}
		respondJSON(w, http.StatusCreated, plan)
	} else {
		if err := h.svc.UpdatePlan(r.Context(), &plan); err != nil {
			handleError(w, r, err)
			return
		}
		respondNoContent(w)
	}
}

// DeleteOptionPlan DELETE /captable/option-plans/{id}
func (h *CapTableHandler) DeleteOptionPlan(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(chi.URLParam(r, "id"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.DeletePlan(r.Context(), tenantID, id); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// ─── OptionGrant ─────────────────────────────────────────────────────────────

// ListOptionGrants GET /captable/option-grants
func (h *CapTableHandler) ListOptionGrants(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	grants, err := h.svc.ListGrants(r.Context(), tenantID, sid)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, grants)
}

// UpsertOptionGrants PUT /captable/option-grants
func (h *CapTableHandler) UpsertOptionGrants(w http.ResponseWriter, r *http.Request) {
	var grant model.OptionGrant
	if err := decodeAndValidate(r, &grant); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	grant.TenantID = tenantID

	if grant.ID.String() == "00000000-0000-0000-0000-000000000000" {
		if err := h.svc.CreateGrant(r.Context(), &grant); err != nil {
			handleError(w, r, err)
			return
		}
		respondJSON(w, http.StatusCreated, grant)
	} else {
		if err := h.svc.UpdateGrant(r.Context(), &grant); err != nil {
			handleError(w, r, err)
			return
		}
		respondNoContent(w)
	}
}

// DeleteOptionGrant DELETE /captable/option-grants/{id}
func (h *CapTableHandler) DeleteOptionGrant(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(chi.URLParam(r, "id"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.DeleteGrant(r.Context(), tenantID, id); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// ─── ValuationScenario ────────────────────────────────────────────────────────

// ListValuationScenarios GET /captable/valuation
func (h *CapTableHandler) ListValuationScenarios(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	scenarios, err := h.svc.ListValuationScenarios(r.Context(), tenantID, sid)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, scenarios)
}

// CreateValuationScenario POST /captable/valuation
func (h *CapTableHandler) CreateValuationScenario(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	var vs model.ValuationScenario
	if err := decodeAndValidate(r, &vs); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	vs.TenantID = tenantID
	vs.ScenarioID = sid
	if err := h.svc.CreateValuationScenario(r.Context(), &vs); err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, vs)
}

// UpdateValuationScenario PUT /captable/valuation/{id}
func (h *CapTableHandler) UpdateValuationScenario(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(chi.URLParam(r, "id"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	var vs model.ValuationScenario
	if err := decodeAndValidate(r, &vs); err != nil {
		handleError(w, r, err)
		return
	}
	vs.ID = id
	vs.TenantID = ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateValuationScenario(r.Context(), &vs); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// DeleteValuationScenario DELETE /captable/valuation/{id}
func (h *CapTableHandler) DeleteValuationScenario(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(chi.URLParam(r, "id"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.DeleteValuationScenario(r.Context(), tenantID, id); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// ComputeValuationScenario POST /captable/valuation/{id}/compute
func (h *CapTableHandler) ComputeValuationScenario(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(chi.URLParam(r, "id"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	result, err := h.svc.ComputeValuationScenario(r.Context(), tenantID, id)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, result)
}

// ─── CapTableScenarioBranch ───────────────────────────────────────────────────

// ListCapTableBranches GET /captable/branches
func (h *CapTableHandler) ListCapTableBranches(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	branches, err := h.svc.ListBranches(r.Context(), tenantID, sid)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, branches)
}

// CreateCapTableBranch POST /captable/branches
func (h *CapTableHandler) CreateCapTableBranch(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	var branch model.CapTableScenarioBranch
	if err := decodeAndValidate(r, &branch); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	branch.TenantID = tenantID
	branch.ScenarioID = sid
	branch.CreatedByUserID = ctxutil.GetUserID(r.Context())
	if err := h.svc.CreateBranch(r.Context(), &branch); err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, branch)
}

// UpdateCapTableBranch PUT /captable/branches/{id}
func (h *CapTableHandler) UpdateCapTableBranch(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(chi.URLParam(r, "id"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	var branch model.CapTableScenarioBranch
	if err := decodeAndValidate(r, &branch); err != nil {
		handleError(w, r, err)
		return
	}
	branch.ID = id
	branch.TenantID = ctxutil.GetTenantID(r.Context())
	if err := h.svc.UpdateBranch(r.Context(), &branch); err != nil {
		handleError(w, r, err)
		return
	}
	respondNoContent(w)
}

// ─── Computed reports ─────────────────────────────────────────────────────────

// GetCapTableReport GET /captable/report
func (h *CapTableHandler) GetCapTableReport(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	report, err := h.svc.GetReport(r.Context(), tenantID, sid)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, report)
}

// GetIngeFiReport GET /captable/report/ingefie
func (h *CapTableHandler) GetIngeFiReport(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	report, err := h.svc.GetReport(r.Context(), tenantID, sid)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, report.IngeFi)
}

// GetStockOptionReport GET /captable/report/stock-options
func (h *CapTableHandler) GetStockOptionReport(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	report, err := h.svc.GetReport(r.Context(), tenantID, sid)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, report.StockOption)
}

// GetFastValoReport GET /captable/report/valuation
func (h *CapTableHandler) GetFastValoReport(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	report, err := h.svc.GetReport(r.Context(), tenantID, sid)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, report.FastValo)
}

// GetDilutionWaterfall GET /captable/report/waterfall
func (h *CapTableHandler) GetDilutionWaterfall(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	report, err := h.svc.GetReport(r.Context(), tenantID, sid)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, report.IngeFi.Waterfall)
}

// GetCapTableMatrix GET /captable/report/matrix
func (h *CapTableHandler) GetCapTableMatrix(w http.ResponseWriter, r *http.Request) {
	sid, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	report, err := h.svc.GetReport(r.Context(), tenantID, sid)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, report.IngeFi.Matrix)
}

// ─── FiPlan sync ──────────────────────────────────────────────────────────────

// syncRoundToFiplanRequest is the request body for POST .../rounds/{roundId}/sync-to-fiplan.
type syncRoundToFiplanRequest struct {
	FiscalYearIndex int `json:"fiscalYearIndex"` // 0-based (0 = Year 1 … 4 = Year 5)
}

// SyncRoundToFiplan POST /captable/rounds/{roundId}/sync-to-fiplan
// Pushes the round's AmountRaisedK to the capital_increase FiPlan entry for the
// specified fiscal year. Pro+ only (enforced at router level via TierPro gate).
func (h *CapTableHandler) SyncRoundToFiplan(w http.ResponseWriter, r *http.Request) {
	roundID, err := parseUUIDParam(chi.URLParam(r, "roundId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	var body syncRoundToFiplanRequest
	if err := decodeAndValidate(r, &body); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	round, err := h.svc.SyncRoundToFiplan(r.Context(), tenantID, roundID, body.FiscalYearIndex)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, round)
}

// UnlinkFromFiplan DELETE /captable/rounds/{roundId}/sync-to-fiplan
// Removes the cap table link from the FiPlan capital_increase entry.
// The amount in FiPlan is preserved.
func (h *CapTableHandler) UnlinkFromFiplan(w http.ResponseWriter, r *http.Request) {
	roundID, err := parseUUIDParam(chi.URLParam(r, "roundId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	round, err := h.svc.UnlinkFromFiplan(r.Context(), tenantID, roundID)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, round)
}

// SyncRoundToOpeningBalance POST /captable/rounds/{roundId}/sync-to-opening-balance
// Records a founding-capital round in the opening balance (share_capital + cash).
// Mutually exclusive with FiplanSynced — returns 400 if the round is already
// synced to FiPlan.
func (h *CapTableHandler) SyncRoundToOpeningBalance(w http.ResponseWriter, r *http.Request) {
	roundID, err := parseUUIDParam(chi.URLParam(r, "roundId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	round, err := h.svc.SyncRoundToOpeningBalance(r.Context(), tenantID, roundID)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, round)
}

// UnsyncRoundFromOpeningBalance DELETE /captable/rounds/{roundId}/sync-to-opening-balance
// Removes the opening-balance link and subtracts the round's contribution from
// opening_balance.share_capital and cash_and_securities.
func (h *CapTableHandler) UnsyncRoundFromOpeningBalance(w http.ResponseWriter, r *http.Request) {
	roundID, err := parseUUIDParam(chi.URLParam(r, "roundId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	round, err := h.svc.UnsyncRoundFromOpeningBalance(r.Context(), tenantID, roundID)
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, round)
}
