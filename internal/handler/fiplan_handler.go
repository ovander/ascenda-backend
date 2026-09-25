package handler

import (
	"context"
	"net/http"
	"strconv"

	"ascenda/internal/dto"
	"ascenda/internal/model"
	"ascenda/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

// FiplanServicer interface for dependency injection.
type FiplanServicer interface {
	ListEntries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.FiplanEntry, error)
	UpdateEntries(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.FiplanEntry) error
	UpsertCapitalIncreaseEntry(ctx context.Context, tenantID, scenarioID uuid.UUID, yearIndex int, amount decimal.Decimal, roundID uuid.UUID, roundLabel string) error
	GetCapitalIncreaseEntry(ctx context.Context, tenantID, scenarioID uuid.UUID, fiscalYearIndex int) (*model.FiplanEntry, error)
	GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.FiplanReport, error)
	GetGrantsForPnL(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.FiplanEntry, error)
}

// capTableRoundLinker is the minimal cap table interface the FiPlan handler
// needs for Flow A (FiPlan → Cap Table round creation).
// Satisfied by *service.CapTableService.
type capTableRoundLinker interface {
	CreateRound(ctx context.Context, rnd *model.CapTableRound) error
}

// FiplanHandler handles financial plan operations.
type FiplanHandler struct {
	svc         FiplanServicer
	roundLinker capTableRoundLinker // nil when cap table feature is unavailable
	logger      *logrus.Entry
}

// NewFiplanHandler creates a new FiplanHandler.
// roundLinker may be nil; CreateRoundFromCapitalIncrease returns 501 in that case.
func NewFiplanHandler(svc FiplanServicer, roundLinker capTableRoundLinker, logger *logrus.Entry) *FiplanHandler {
	return &FiplanHandler{
		svc:         svc,
		roundLinker: roundLinker,
		logger:      logger,
	}
}

// ListEntries returns all fiplan entries for a scenario.
func (h *FiplanHandler) ListEntries(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	entries, err := h.svc.ListEntries(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.FiplanEntriesFromModels(entries))
}

// UpdateEntries updates fiplan entries.
func (h *FiplanHandler) UpdateEntries(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	var entries []model.FiplanEntry
	if err := decodeAndValidate(r, &entries); err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	h.logger.WithField("entry_count", len(entries)).Debug("fiplan UpdateEntries: decoded ok, calling service")
	if err := h.svc.UpdateEntries(r.Context(), tenantID, scenarioID, entries); err != nil {
		handleError(w, r, err)
		return
	}

	respondNoContent(w)
}

// GetReport returns the financing plan report.
func (h *FiplanHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	report, err := h.svc.GetReport(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, report)
}

// GetGrantsForPnL returns grants for P&L inclusion.
func (h *FiplanHandler) GetGrantsForPnL(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	grants, err := h.svc.GetGrantsForPnL(r.Context(), tenantID, scenarioID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, grants)
}

// ─── Flow A: FiPlan → Cap Table round creation ────────────────────────────────

// createRoundFromCapitalIncreaseRequest is the body for
// POST /fiplan/capital-increase/{yearIndex}/create-round
type createRoundFromCapitalIncreaseRequest struct {
	Label          string `json:"label"`          // e.g. "Series A"
	ShareClassType string `json:"shareClassType"` // e.g. "preferred_a"
	PhaseNumber    int    `json:"phaseNumber"`    // 1-based phase number
	SortOrder      int    `json:"sortOrder"`
}

// CreateRoundFromCapitalIncrease POST /fiplan/capital-increase/{yearIndex}/create-round
//
// Flow A: the planned capital increase in FiPlan (entered by the user) is the
// source of truth. This endpoint formalises it as a Cap Table round, linking both
// sides so future negotiation divergence can be detected and re-synced (Flow B).
//
// yearIndex in the URL is 0-based (0 = Year 1 … 4 = Year 5), matching the cap
// table fiscal year convention. The endpoint reads the existing FiPlan entry for
// that year, creates a linked Cap Table round, and writes the round back-reference
// onto the FiPlan entry.
func (h *FiplanHandler) CreateRoundFromCapitalIncrease(w http.ResponseWriter, r *http.Request) {
	if h.roundLinker == nil {
		handleError(w, r, apierror.Internal("cap table feature is not available"))
		return
	}

	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	rawYear := chi.URLParam(r, "yearIndex")
	yearIndex, convErr := strconv.Atoi(rawYear)
	if convErr != nil || yearIndex < 0 || yearIndex > 4 {
		handleError(w, r, apierror.BadRequest("yearIndex must be an integer between 0 and 4"))
		return
	}

	var body createRoundFromCapitalIncreaseRequest
	if err := decodeAndValidate(r, &body); err != nil {
		handleError(w, r, err)
		return
	}
	if body.Label == "" {
		handleError(w, r, apierror.BadRequest("label is required"))
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())

	// 1. Read the existing FiPlan entry to get the planned amount.
	entry, err := h.svc.GetCapitalIncreaseEntry(r.Context(), tenantID, scenarioID, yearIndex)
	if err != nil {
		handleError(w, r, err)
		return
	}
	if entry == nil || entry.Amount.IsZero() {
		handleError(w, r, apierror.BadRequest("no capital increase amount set for Year "+strconv.Itoa(yearIndex+1)))
		return
	}
	if entry.CapTableRoundID != nil {
		handleError(w, r, apierror.BadRequest("capital increase for Year "+strconv.Itoa(yearIndex+1)+" is already linked to a cap table round"))
		return
	}

	// 2. Build the Cap Table round — amount converts from full € → k€.
	amountRaisedK := entry.Amount.Div(decimal.NewFromInt(1000))
	rnd := &model.CapTableRound{
		ScenarioID:          scenarioID,
		Label:               body.Label,
		ShareClassType:      model.ShareClassType(body.ShareClassType),
		EventType:           model.RoundEventFunding,
		AmountRaisedK:       amountRaisedK,
		PhaseNumber:         body.PhaseNumber,
		SortOrder:           body.SortOrder,
		FiscalYearIndex:     &yearIndex,
		FiplanSynced:        true,
		FiplanSyncedAmountK: &amountRaisedK,
	}
	rnd.TenantID = tenantID

	if err := h.roundLinker.CreateRound(r.Context(), rnd); err != nil {
		handleError(w, r, err)
		return
	}

	// 3. Write the round back-reference onto the FiPlan entry so the badge appears.
	if err := h.svc.UpsertCapitalIncreaseEntry(
		r.Context(), tenantID, scenarioID, yearIndex,
		entry.Amount, rnd.ID, rnd.Label,
	); err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusCreated, rnd)
}

// compile-time check: *service.CapTableService satisfies capTableRoundLinker.
var _ capTableRoundLinker = (*service.CapTableService)(nil)

// compile-time sentinel so decimal import is used.
var _ = decimal.Zero
