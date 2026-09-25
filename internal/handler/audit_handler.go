package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"ascenda/internal/dto"
	"ascenda/internal/model"
	"ascenda/internal/repo"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
)

// ScenarioCapturer serialises the full current state of a scenario (satisfied
// by *service.SnapshotService).
type ScenarioCapturer interface {
	CaptureScenarioData(tenantID, scenarioID uuid.UUID) (json.RawMessage, error)
}

// AuditAccessResolver decides what the caller may see (satisfied by
// *service.PlanAccessResolver).
type AuditAccessResolver interface {
	PlanRole(tenantID, userID uuid.UUID, tenantRole string, planID uuid.UUID) (string, bool)
	AccessibleEntityIDs(tenantID, userID uuid.UUID, tenantRole string) (ids []uuid.UUID, all bool, err error)
}

// ScenarioLookup resolves a scenario to its plan.
type ScenarioLookup interface {
	GetByID(tenantID, scenarioID uuid.UUID) (*model.Scenario, error)
}

// AuditHandler exposes the audit trail, scoped to the plans the caller can
// access: audit rows carry the scenario ID (or plan ID for plan events, or the
// tenant ID for tenant-level events) as entity_id, which is resolved to a plan
// and checked with the same rule as PlanAccessMiddleware.
type AuditHandler struct {
	auditRepo   repo.AuditRepository
	snapshotSvc ScenarioCapturer
	access      AuditAccessResolver
	scenarios   ScenarioLookup
	logger      *logrus.Entry
}

// NewAuditHandler creates a new AuditHandler.
func NewAuditHandler(auditRepo repo.AuditRepository, snapshotSvc ScenarioCapturer, access AuditAccessResolver, scenarios ScenarioLookup, logger *logrus.Entry) *AuditHandler {
	return &AuditHandler{auditRepo: auditRepo, snapshotSvc: snapshotSvc, access: access, scenarios: scenarios, logger: logger}
}

// entityTypePlan is the entity_type of plan lifecycle events, whose entity_id
// is the plan ID rather than a scenario ID.
const entityTypePlan = "plan"

// canSeeEntry reports whether the caller may read an audit entry, and the
// scenario ID to capture for the detail view (uuid.Nil when there is none:
// tenant-level and plan-level events, or a scenario that no longer exists).
func (h *AuditHandler) canSeeEntry(r *http.Request, entry *model.AuditLog) (scenarioID uuid.UUID, ok bool) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	userID := ctxutil.GetUserID(ctx)
	role := ctxutil.GetUserRole(ctx)

	// Tenant-level events (audit exports) are visible to every tenant user.
	if entry.EntityID == tenantID {
		return uuid.Nil, role != "admin"
	}

	// Plan lifecycle events: entity_id is the plan.
	if entry.EntityType == entityTypePlan {
		_, allowed := h.access.PlanRole(tenantID, userID, role, entry.EntityID)
		return uuid.Nil, allowed
	}

	// Everything else: entity_id is the scenario.
	scenario, err := h.scenarios.GetByID(tenantID, entry.EntityID)
	if err != nil || scenario == nil {
		// The scenario no longer exists (deleted): nothing to capture, and only
		// the owner can still read the historical entry.
		return uuid.Nil, role == "owner"
	}
	_, allowed := h.access.PlanRole(tenantID, userID, role, scenario.PlanID)
	return scenario.ID, allowed
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

	// Restrict the trail to plans the caller can access; owners see the whole tenant.
	entityIDs, all, err := h.access.AccessibleEntityIDs(tenantID, ctxutil.GetUserID(r.Context()), ctxutil.GetUserRole(r.Context()))
	if err != nil {
		h.logger.WithError(err).Error("failed to resolve accessible plans for audit trail")
		handleError(w, r, apierror.Internal("failed to retrieve audit trail"))
		return
	}

	var (
		total int64
		logs  []*model.AuditLog
	)
	if all {
		total, err = h.auditRepo.CountByTenant(tenantID)
		if err == nil {
			logs, err = h.auditRepo.ListByTenant(tenantID, offset, limit)
		}
	} else {
		total, err = h.auditRepo.CountByTenantAndEntities(tenantID, entityIDs)
		if err == nil {
			logs, err = h.auditRepo.ListByTenantAndEntities(tenantID, entityIDs, offset, limit)
		}
	}
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
	userID := ctxutil.GetUserID(r.Context())
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
	if err != nil || entry == nil {
		handleError(w, r, apierror.NotFound("audit entry", entryID.String()))
		return
	}

	// The entity_id on audit rows is the scenario UUID (see audit_subscriber.go),
	// the plan UUID for plan events, or the tenant UUID for tenant-level events.
	// The caller must have access to the plan behind it; an entry they may not
	// see is reported as not found so IDs are not confirmed.
	scenarioID, allowed := h.canSeeEntry(r, entry)
	if !allowed {
		h.logger.WithFields(logrus.Fields{
			"user_id":  ctxutil.GetUserID(r.Context()),
			"entry_id": entryID,
		}).Warn("audit detail denied — caller has no access to the entry's plan")
		handleError(w, r, apierror.NotFound("audit entry", entryID.String()))
		return
	}

	snapshotData := json.RawMessage(`{}`)
	if scenarioID != uuid.Nil {
		snapshotData, err = h.snapshotSvc.CaptureScenarioData(tenantID, scenarioID)
		if err != nil {
			h.logger.WithError(err).Warn("failed to capture scenario snapshot for audit detail")
			// Non-fatal: return the entry without snapshot rather than a 500.
			snapshotData = json.RawMessage(`{}`)
		}
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
