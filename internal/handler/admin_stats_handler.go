package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"ascenda/internal/service"
)

// AdminStatser interface for dependency injection.
type AdminStatser interface {
	GetStats(ctx context.Context) (*service.AdminStats, error)
	GetAIUsageStats(start, end time.Time) (*service.AdminAIUsageStats, error)
}

// AdminStatsHandler handles the admin dashboard stats endpoint.
type AdminStatsHandler struct {
	adminSvc AdminStatser
	logger   *logrus.Entry
}

// NewAdminStatsHandler creates a new AdminStatsHandler.
func NewAdminStatsHandler(adminSvc AdminStatser, logger *logrus.Entry) *AdminStatsHandler {
	return &AdminStatsHandler{adminSvc: adminSvc, logger: logger}
}

// GetStats handles GET /api/v1/admin/stats.
// Platform-wide — requires platform:admin permission. No tenant context needed.
func (h *AdminStatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.adminSvc.GetStats(r.Context())
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

// GetAIUsage handles GET /api/v1/admin/ai-usage.
// Returns platform-wide AI feature usage statistics.
// Optional query params: start, end (RFC3339). Defaults to last 30 days.
func (h *AdminStatsHandler) GetAIUsage(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	end := now
	start := now.AddDate(0, 0, -30)

	if s := r.URL.Query().Get("start"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			start = t.UTC()
		}
	}
	if e := r.URL.Query().Get("end"); e != "" {
		if t, err := time.Parse(time.RFC3339, e); err == nil {
			end = t.UTC()
		}
	}

	stats, err := h.adminSvc.GetAIUsageStats(start, end)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, stats)
}
