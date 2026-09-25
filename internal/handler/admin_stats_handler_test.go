package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ascenda/internal/service"
	"github.com/ovander/backendkit/apierror"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── mockAdminService ──────────────────────────────────────────────────────

type mockAdminService struct {
	getStatsFn   func(ctx context.Context) (*service.AdminStats, error)
	getAIUsageFn func(start, end time.Time) (*service.AdminAIUsageStats, error)
}

func (m *mockAdminService) GetStats(ctx context.Context) (*service.AdminStats, error) {
	if m.getStatsFn != nil {
		return m.getStatsFn(ctx)
	}
	return nil, apierror.Internal("get stats not implemented")
}

func (m *mockAdminService) GetAIUsageStats(start, end time.Time) (*service.AdminAIUsageStats, error) {
	if m.getAIUsageFn != nil {
		return m.getAIUsageFn(start, end)
	}
	return nil, apierror.Internal("get ai usage not implemented")
}

// ── fixtures ──────────────────────────────────────────────────────────────

func newAdminStatsHandler(svc *mockAdminService) *AdminStatsHandler {
	return NewAdminStatsHandler(svc, logrus.NewEntry(logrus.New()))
}

func mockAIUsageStats() *service.AdminAIUsageStats {
	now := time.Now().UTC()
	return &service.AdminAIUsageStats{
		PeriodStart: now.AddDate(0, 0, -30).Format(time.RFC3339),
		PeriodEnd:   now.Format(time.RFC3339),
		CallsToday:  12,
		CallsWeek:   84,
		CallsMonth:  310,
		ByFeature: []service.AIUsageFeatureItem{
			{
				Feature:            "AI_PLAN_NARRATION",
				TotalCalls:         200,
				SuccessfulCalls:    195,
				TotalTokens:        40000,
				EstimatedCostCents: 120.50,
			},
			{
				Feature:            "AI_VARIANCE_ANALYSIS",
				TotalCalls:         110,
				SuccessfulCalls:    108,
				TotalTokens:        22000,
				EstimatedCostCents: 66.00,
			},
		},
		ByTenant: []service.AIUsageTenantItem{
			{
				TenantID:           "tenant-uuid-1",
				TotalCalls:         180,
				SuccessfulCalls:    175,
				TotalTokens:        36000,
				EstimatedCostCents: 108.00,
			},
		},
	}
}

// ── GetStats ──────────────────────────────────────────────────────────────

func TestAdminStatsHandler_GetStats_Success(t *testing.T) {
	stats := &service.AdminStats{
		Tenants: 5,
		Users: service.AdminUserStats{
			Total:    20,
			Active:   18,
			Inactive: 2,
		},
		Plans: service.AdminPlanStats{
			Total: 10,
		},
		Scenarios: service.AdminScenarioStats{
			Total: 30,
		},
		RecentActivity: []service.ActivityItem{},
		TopUsers:       []service.TopUserItem{},
	}

	svc := &mockAdminService{
		getStatsFn: func(ctx context.Context) (*service.AdminStats, error) {
			return stats, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	newAdminStatsHandler(svc).GetStats(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got service.AdminStats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, int64(5), got.Tenants)
	assert.Equal(t, int64(20), got.Users.Total)
	assert.Equal(t, int64(10), got.Plans.Total)
}

func TestAdminStatsHandler_GetStats_ServiceError(t *testing.T) {
	svc := &mockAdminService{
		getStatsFn: func(ctx context.Context) (*service.AdminStats, error) {
			return nil, apierror.Internal("database query failed")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	newAdminStatsHandler(svc).GetStats(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAdminStatsHandler_GetStats_ContextCancellation(t *testing.T) {
	svc := &mockAdminService{
		getStatsFn: func(ctx context.Context) (*service.AdminStats, error) {
			return nil, apierror.Internal("context cancelled")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	newAdminStatsHandler(svc).GetStats(w, r)

	assert.True(t, w.Code >= 400)
}

// ── GetAIUsage ────────────────────────────────────────────────────────────

func TestAdminStatsHandler_GetAIUsage_Success(t *testing.T) {
	expected := mockAIUsageStats()

	svc := &mockAdminService{
		getAIUsageFn: func(start, end time.Time) (*service.AdminAIUsageStats, error) {
			return expected, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ai-usage", nil)
	w := httptest.NewRecorder()

	newAdminStatsHandler(svc).GetAIUsage(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got service.AdminAIUsageStats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, int64(12), got.CallsToday)
	assert.Equal(t, int64(84), got.CallsWeek)
	assert.Equal(t, int64(310), got.CallsMonth)
	require.Len(t, got.ByFeature, 2)
	assert.Equal(t, "AI_PLAN_NARRATION", got.ByFeature[0].Feature)
	assert.Equal(t, int64(200), got.ByFeature[0].TotalCalls)
	assert.Equal(t, int64(195), got.ByFeature[0].SuccessfulCalls)
	assert.InDelta(t, 120.50, got.ByFeature[0].EstimatedCostCents, 0.001)
	require.Len(t, got.ByTenant, 1)
	assert.Equal(t, "tenant-uuid-1", got.ByTenant[0].TenantID)
}

func TestAdminStatsHandler_GetAIUsage_DefaultsToThirtyDays(t *testing.T) {
	var capturedStart, capturedEnd time.Time

	svc := &mockAdminService{
		getAIUsageFn: func(start, end time.Time) (*service.AdminAIUsageStats, error) {
			capturedStart = start
			capturedEnd = end
			return mockAIUsageStats(), nil
		},
	}

	before := time.Now().UTC().Add(-time.Second)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ai-usage", nil)
	w := httptest.NewRecorder()

	newAdminStatsHandler(svc).GetAIUsage(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, capturedEnd.After(before), "end should be at or after test start")
	expectedStart := capturedEnd.AddDate(0, 0, -30)
	diff := capturedStart.Sub(expectedStart)
	assert.Less(t, diff.Abs().Seconds(), float64(2), "start should be 30 days before end")
}

func TestAdminStatsHandler_GetAIUsage_CustomDateRange(t *testing.T) {
	var capturedStart, capturedEnd time.Time

	svc := &mockAdminService{
		getAIUsageFn: func(start, end time.Time) (*service.AdminAIUsageStats, error) {
			capturedStart = start
			capturedEnd = end
			return mockAIUsageStats(), nil
		},
	}

	wantStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)

	url := "/api/v1/admin/ai-usage?start=" + wantStart.Format(time.RFC3339) + "&end=" + wantEnd.Format(time.RFC3339)
	r := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()

	newAdminStatsHandler(svc).GetAIUsage(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, wantStart, capturedStart)
	assert.Equal(t, wantEnd, capturedEnd)
}

func TestAdminStatsHandler_GetAIUsage_InvalidDateParamsFallsBackToDefault(t *testing.T) {
	var capturedStart time.Time

	svc := &mockAdminService{
		getAIUsageFn: func(start, end time.Time) (*service.AdminAIUsageStats, error) {
			capturedStart = start
			return mockAIUsageStats(), nil
		},
	}

	// malformed date — handler should ignore and use the 30-day default
	r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ai-usage?start=not-a-date", nil)
	w := httptest.NewRecorder()

	newAdminStatsHandler(svc).GetAIUsage(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, time.Since(capturedStart) > 29*24*time.Hour,
		"start should be ~30 days ago when param is invalid")
}

func TestAdminStatsHandler_GetAIUsage_ServiceError(t *testing.T) {
	svc := &mockAdminService{
		getAIUsageFn: func(start, end time.Time) (*service.AdminAIUsageStats, error) {
			return nil, apierror.Internal("db unavailable")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ai-usage", nil)
	w := httptest.NewRecorder()

	newAdminStatsHandler(svc).GetAIUsage(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAdminStatsHandler_GetAIUsage_EmptyResults(t *testing.T) {
	svc := &mockAdminService{
		getAIUsageFn: func(start, end time.Time) (*service.AdminAIUsageStats, error) {
			return &service.AdminAIUsageStats{
				CallsToday: 0,
				CallsWeek:  0,
				CallsMonth: 0,
				ByFeature:  []service.AIUsageFeatureItem{},
				ByTenant:   []service.AIUsageTenantItem{},
			}, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ai-usage", nil)
	w := httptest.NewRecorder()

	newAdminStatsHandler(svc).GetAIUsage(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got service.AdminAIUsageStats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, int64(0), got.CallsToday)
	assert.Empty(t, got.ByFeature)
	assert.Empty(t, got.ByTenant)
}
