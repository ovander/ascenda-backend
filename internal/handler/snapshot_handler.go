package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"kerplan/internal/dto"
	"kerplan/internal/pkg/ctxutil"
	"kerplan/internal/pkg/pagination"
	"kerplan/internal/service"
)

// SnapshotHandler handles snapshot/versioning operations.
type SnapshotHandler struct {
	svc    *service.SnapshotService
	logger *logrus.Entry
}

// NewSnapshotHandler creates a new SnapshotHandler.
func NewSnapshotHandler(svc *service.SnapshotService, logger *logrus.Entry) *SnapshotHandler {
	return &SnapshotHandler{
		svc:    svc,
		logger: logger,
	}
}

// List returns all snapshots for a scenario.
func (h *SnapshotHandler) List(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 10
	}

	params := pagination.Params{
		Offset:  page * limit,
		PerPage: limit,
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	snapshots, total, err := h.svc.List(r.Context(), tenantID, scenarioID, params)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.NewPagedResponse(dto.SnapshotsFromModels(snapshots), total, page, limit))
}

// CreateRequest represents a snapshot creation request.
type CreateSnapshotRequest struct {
	Label       string `json:"label" validate:"required"`
	Description string `json:"description"`
}

// Create creates a new snapshot.
func (h *SnapshotHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateSnapshotRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, err)
		return
	}

	scenarioID, err := parseUUIDParam(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	snapshot, err := h.svc.Create(r.Context(), tenantID, scenarioID, req.Label, req.Description)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, dto.SnapshotFromModel(*snapshot))
}

// Get retrieves a snapshot by ID.
func (h *SnapshotHandler) Get(w http.ResponseWriter, r *http.Request) {
	snapshotID, err := parseUUIDParam(chi.URLParam(r, "snapshotId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	snapshot, err := h.svc.Get(r.Context(), tenantID, snapshotID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, dto.SnapshotFromModel(*snapshot))
}

// GetData retrieves snapshot with complete plan data.
func (h *SnapshotHandler) GetData(w http.ResponseWriter, r *http.Request) {
	snapshotID, err := parseUUIDParam(chi.URLParam(r, "snapshotId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	data, err := h.svc.GetData(r.Context(), tenantID, snapshotID)
	if err != nil {
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// Restore restores a scenario from a snapshot.
func (h *SnapshotHandler) Restore(w http.ResponseWriter, r *http.Request) {
	snapshotID, err := parseUUIDParam(chi.URLParam(r, "snapshotId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.Restore(r.Context(), tenantID, snapshotID); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}

// CloneRequest represents a snapshot clone request.
type CloneSnapshotRequest struct {
	NewScenarioID string `json:"newScenarioId" validate:"required"`
}

// Clone creates a new scenario from a snapshot.
func (h *SnapshotHandler) Clone(w http.ResponseWriter, r *http.Request) {
	var req CloneSnapshotRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, err)
		return
	}

	snapshotID, err := parseUUIDParam(chi.URLParam(r, "snapshotId"))
	if err != nil {
		handleError(w, err)
		return
	}

	newScenarioID, err := parseUUID(req.NewScenarioID)
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.CloneToScenario(r.Context(), tenantID, snapshotID, newScenarioID); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}

// Diff compares two snapshots.
func (h *SnapshotHandler) Diff(w http.ResponseWriter, r *http.Request) {
	snapshot1ID, err := parseUUIDParam(chi.URLParam(r, "snapshot1Id"))
	if err != nil {
		handleError(w, err)
		return
	}

	snapshot2ID, err := parseUUIDParam(chi.URLParam(r, "snapshot2Id"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	diff, err := h.svc.Diff(r.Context(), tenantID, snapshot1ID, snapshot2ID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"changes": diff})
}

// Delete removes a snapshot.
func (h *SnapshotHandler) Delete(w http.ResponseWriter, r *http.Request) {
	snapshotID, err := parseUUIDParam(chi.URLParam(r, "snapshotId"))
	if err != nil {
		handleError(w, err)
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	if err := h.svc.Delete(r.Context(), tenantID, snapshotID); err != nil {
		handleError(w, err)
		return
	}

	respondNoContent(w)
}
