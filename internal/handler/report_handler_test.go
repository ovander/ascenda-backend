package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
)

// ── mockReportService ─────────────────────────────────────────────────────

type mockReportService struct {
	getFullReportFn func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.FullPlanOutput, error)
}

func (m *mockReportService) GetFullReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.FullPlanOutput, error) {
	if m.getFullReportFn != nil {
		return m.getFullReportFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get full report not implemented")
}

// ── fixtures ──────────────────────────────────────────────────────────────

func newReportHandler(svc *mockReportService) *ReportHandler {
	return NewReportHandler(svc, logrus.NewEntry(logrus.New()))
}

// ── GetFullReport ─────────────────────────────────────────────────────────

func TestReportHandler_GetFullReport_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	// Minimal FullPlanOutput for testing
	report := &model.FullPlanOutput{
		// This is a simplified structure; adjust based on actual model.FullPlanOutput fields
	}

	svc := &mockReportService{
		getFullReportFn: func(_ context.Context, tid, sid uuid.UUID) (*model.FullPlanOutput, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return report, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newReportHandler(svc).GetFullReport(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotNil(t, got)
}

func TestReportHandler_GetFullReport_BadScenarioUUID(t *testing.T) {
	svc := &mockReportService{}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "not-a-uuid"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newReportHandler(svc).GetFullReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReportHandler_GetFullReport_EmptyScenarioId(t *testing.T) {
	svc := &mockReportService{}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": ""})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newReportHandler(svc).GetFullReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReportHandler_GetFullReport_ServiceNotFound(t *testing.T) {
	scenarioID := uuid.New()

	svc := &mockReportService{
		getFullReportFn: func(_ context.Context, _, _ uuid.UUID) (*model.FullPlanOutput, error) {
			return nil, apierror.NotFound("scenario", scenarioID.String())
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newReportHandler(svc).GetFullReport(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestReportHandler_GetFullReport_ServiceError(t *testing.T) {
	scenarioID := uuid.New()

	svc := &mockReportService{
		getFullReportFn: func(_ context.Context, _, _ uuid.UUID) (*model.FullPlanOutput, error) {
			return nil, apierror.Internal("failed to compute plan")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newReportHandler(svc).GetFullReport(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestReportHandler_GetFullReport_TenantIDFromContext(t *testing.T) {
	// Verify that tenantID is properly extracted from context
	tenantID := uuid.New()
	scenarioID := uuid.New()
	var capturedTenantID uuid.UUID

	svc := &mockReportService{
		getFullReportFn: func(_ context.Context, tid, _ uuid.UUID) (*model.FullPlanOutput, error) {
			capturedTenantID = tid
			return &model.FullPlanOutput{}, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newReportHandler(svc).GetFullReport(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, tenantID, capturedTenantID)
}

func TestReportHandler_GetFullReport_InvalidUUIDFormat(t *testing.T) {
	// Test various invalid UUID formats
	invalidUUIDs := []string{
		"12345",
		"xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
		"not-uuid-at-all",
		"00000000-0000-0000-0000-00000000000",  // too short
		"00000000-0000-0000-0000-000000000000-", // trailing hyphen
	}

	for _, invalidID := range invalidUUIDs {
		t.Run("invalid_"+invalidID, func(t *testing.T) {
			svc := &mockReportService{}

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r = withChiParams(r, map[string]string{"scenarioId": invalidID})
			r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
			w := httptest.NewRecorder()

			newReportHandler(svc).GetFullReport(w, r)

			assert.Equal(t, http.StatusBadRequest, w.Code, "should return 400 for invalid UUID: "+invalidID)
		})
	}
}
