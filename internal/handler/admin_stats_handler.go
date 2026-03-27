package handler

import (
	"context"
	"net/http"

	"github.com/sirupsen/logrus"
	"ascenda/internal/service"
)

// AdminStatser interface for dependency injection.
type AdminStatser interface {
	GetStats(ctx context.Context) (*service.AdminStats, error)
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
