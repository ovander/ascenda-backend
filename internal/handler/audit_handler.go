package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"ascenda/internal/dto"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"ascenda/internal/repo"
	"ascenda/internal/service"
)

// AuditHandler exposes the tenant-scoped audit trail.
type AuditHandler struct {
	auditRepo   *repo.AuditRepo
	snapshotSvc *service.SnapshotService
	logger      *logrus.Entry
}

// NewAuditHandler creates a new AuditHandler.
func NewAuditHandler(auditRepo *repo.AuditRepo, snapshotSvc *service.SnapshotService, logger *logrus.Entry) *AuditHandler {
	return &AuditHandler{auditRepo: auditRepo, snapshotSvc: snapshotSvc, logger: logger}
}

// List returns a paginated list of audit log entries for the current tenant.
// Query params: page (0-based), limit (default 50, max 200).
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxutil.GetTenantID(r.Context())
	if tenantID.String() == "00000000-0000-0000-0000-000000000000" {
		handleError(w, r, apierror.Forbidden("missing tenant context"))
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := page * limit

	total, err := h.auditRepo.CountByTenant(tenantID)
	if err != nil {
		handleError(w, r, apierror.Internal("failed to count audit entries"))
		return
	}

	logs, err := h.auditRepo.ListByTenant(tenantID, offset, limit)
	if err != nil {
		handleError(w, r, apierror.Internal("failed to retrieve audit trail"))
		return
	}

	respondJSON(w, http.StatusOK, dto.NewPagedResponse(dto.AuditLogsFromModels(logs), total, page, limit))
}

// RecordExport persists a single audit entry that records a tenant-wide audit
// export performed by the current user.  Called by the frontend immediately
// after the downloaded file is assembled, so the export itself is traceable.
//
// Request body (JSON):
//
//	{ "totalExported": 123, "filters": { "entityType": "...", "action": "...", "search": "..." } }
func (h *AuditHandler) RecordExport(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxutil.GetTenantID(r.Context())
	userID   := ctxutil.GetUserID(r.Context())
	if tenantID.String() == "00000000-0000-0000-0000-000000000000" {
		handleError(w, r, apierror.Forbidden("missing tenant context"))
		return
	}

	var body struct {
		TotalExported int            `json:"totalExported"`
		Filters       map[string]any `json:"filters"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		handleError(w, r, apierror.BadRequest("invalid request body"))
		return
	}

	changes, _ := json.Marshal(map[string]any{
		"totalExported": body.TotalExported,
		"filters":       body.Filters,
	})

	entry := &model.AuditLog{
		ID:         uuid.New(),
		TenantID:   tenantID,
		UserID:     userID,
		EntityType: "audit_export",
		EntityID:   tenantID, // tenant-level event — no specific entity
		Action:     "export",
		Changes:    changes,
	}
	if err := h.auditRepo.Create(entry); err != nil {
		h.logger.WithError(err).Error("failed to record audit export event")
		handleError(w, r, apierror.Internal("failed to record export"))
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{"id": entry.ID.String()})
}

// GetDetail returns a single audit log entry enriched with the full current
// state of the related scenario (all inputs, outputs, and settings captured via
// the snapshot machinery).  The response structure is:
//
//	{
//	  "entry":            { ...audit log fields + changes },
//	  "scenarioSnapshot": { ...all section data (products, staff, fiplan, …) },
//	  "exportedAt":       "2006-01-02T15:04:05Z"
//	}
//
// URL param: {entryId} — the UUID of the audit log entry to detail.
func (h *AuditHandler) GetDetail(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxutil.GetTenantID(r.Context())
	if tenantID.String() == "00000000-0000-0000-0000-000000000000" {
		handleError(w, r, apierror.Forbidden("missing tenant context"))
		return
	}

	entryID, err := uuid.Parse(chi.URLParam(r, "entryId"))
	if err != nil {
		handleError(w, r, apierror.BadRequest("invalid entry id"))
		return
	}

	entry, err := h.auditRepo.GetByID(tenantID, entryID)
	if err != nil {
		handleError(w, r, apierror.NotFound("audit entry", entryID.String()))
		return
	}

	// The entity_id on audit rows is always the scenario UUID (see audit_subscriber.go).
	// For tenant-level events (e.g. audit_export) entity_id == tenantID — skip capture.
	var snapshotData json.RawMessage
	if entry.EntityID != tenantID {
		snapshotData, err = h.snapshotSvc.CaptureScenarioData(tenantID, entry.EntityID)
		if err != nil {
			h.logger.WithError(err).Warn("failed to capture scenario snapshot for audit detail")
			// Non-fatal: return the entry without snapshot rather than a 500.
			snapshotData = json.RawMessage(`{}`)
		}
	} else {
		snapshotData = json.RawMessage(`{}`)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entry":            dto.AuditLogFromModel(entry),
		"scenarioSnapshot": snapshotData,
		"exportedAt":       time.Now().UTC(),
	})
}

// DownloadByScenario streams the full audit trail for a single scenario as a
// JSON file attachment.  Intended for dev/debug use only — the frontend shows
// the button only in import.meta.env.DEV mode.
func (h *AuditHandler) DownloadByScenario(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxutil.GetTenantID(r.Context())
	if tenantID.String() == "00000000-0000-0000-0000-000000000000" {
		handleError(w, r, apierror.Forbidden("missing tenant context"))
		return
	}

	scenarioID, err := uuid.Parse(chi.URLParam(r, "scenarioId"))
	if err != nil {
		handleError(w, r, apierror.BadRequest("invalid scenario id"))
		return
	}

	logs, err := h.auditRepo.ListByEntityID(tenantID, scenarioID)
	if err != nil {
		h.logger.WithError(err).Error("failed to retrieve scenario audit trail")
		handleError(w, r, apierror.Internal("failed to retrieve scenario audit trail"))
		return
	}

	payload := dto.AuditLogsFromModels(logs)

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		handleError(w, r, apierror.Internal("failed to encode audit trail"))
		return
	}

	filename := fmt.Sprintf("audit-scenario-%s-%s.json",
		scenarioID.String()[:8],
		time.Now().UTC().Format("20060102-150405"),
	)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
