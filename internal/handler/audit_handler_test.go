package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/ovander/backendkit/ctxutil"
)

// ── TestAuditHandler_List_MissingTenantContext ───────────────────────────────

func TestAuditHandler_List_MissingTenantContext(t *testing.T) {
	// When tenant context is missing, handler returns 403 without calling repo.
	// Pass nil for dependencies since they won't be accessed.
	handler := NewAuditHandler(nil, nil, logrus.NewEntry(logrus.New()))

	r := httptest.NewRequest(http.MethodGet, "/?page=0&limit=50", nil)
	// No tenant context — defaults to zero UUID
	w := httptest.NewRecorder()

	handler.List(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ── TestAuditHandler_RecordExport_MissingTenantContext ──────────────────────

func TestAuditHandler_RecordExport_MissingTenantContext(t *testing.T) {
	// When tenant context is missing, handler returns 403 without creating entry.
	handler := NewAuditHandler(nil, nil, logrus.NewEntry(logrus.New()))

	body := map[string]interface{}{
		"totalExported": 100,
		"filters": map[string]interface{}{
			"entityType": "scenario",
		},
	}
	data, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, "/export", bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	// No tenant context — defaults to zero UUID
	w := httptest.NewRecorder()

	handler.RecordExport(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ── TestAuditHandler_RecordExport_InvalidJSON ────────────────────────────────

func TestAuditHandler_RecordExport_InvalidJSON(t *testing.T) {
	// When JSON body is invalid, handler returns 400.
	handler := NewAuditHandler(nil, nil, logrus.NewEntry(logrus.New()))

	tenantID := uuid.New()
	r := httptest.NewRequest(http.MethodPost, "/export", bytes.NewReader([]byte("invalid json")))
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	handler.RecordExport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── TestAuditHandler_GetDetail_MissingTenantContext ──────────────────────────

func TestAuditHandler_GetDetail_MissingTenantContext(t *testing.T) {
	// When tenant context is missing, handler returns 403 without reading repo.
	handler := NewAuditHandler(nil, nil, logrus.NewEntry(logrus.New()))

	entryID := uuid.New()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"entryId": entryID.String()})
	// No tenant context — defaults to zero UUID
	w := httptest.NewRecorder()

	handler.GetDetail(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ── TestAuditHandler_GetDetail_InvalidEntryUUID ──────────────────────────────

func TestAuditHandler_GetDetail_InvalidEntryUUID(t *testing.T) {
	// When entryId URL param is not a valid UUID, handler returns 400.
	handler := NewAuditHandler(nil, nil, logrus.NewEntry(logrus.New()))

	tenantID := uuid.New()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"entryId": "not-a-uuid"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	handler.GetDetail(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── TestAuditHandler_DownloadByScenario_MissingTenantContext ──────────────────

func TestAuditHandler_DownloadByScenario_MissingTenantContext(t *testing.T) {
	// When tenant context is missing, handler returns 403 without querying repo.
	handler := NewAuditHandler(nil, nil, logrus.NewEntry(logrus.New()))

	scenarioID := uuid.New()
	r := httptest.NewRequest(http.MethodGet, "/download", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	// No tenant context — defaults to zero UUID
	w := httptest.NewRecorder()

	handler.DownloadByScenario(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ── TestAuditHandler_DownloadByScenario_InvalidScenarioUUID ──────────────────

func TestAuditHandler_DownloadByScenario_InvalidScenarioUUID(t *testing.T) {
	// When scenarioId URL param is not a valid UUID, handler returns 400.
	handler := NewAuditHandler(nil, nil, logrus.NewEntry(logrus.New()))

	tenantID := uuid.New()
	r := httptest.NewRequest(http.MethodGet, "/download", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "invalid-uuid"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	handler.DownloadByScenario(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
