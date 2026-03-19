package handler

import (
	"net/http"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// HealthHandler provides health check endpoints.
type HealthHandler struct {
	db     *gorm.DB
	logger *logrus.Entry
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(db *gorm.DB, logger *logrus.Entry) *HealthHandler {
	return &HealthHandler{
		db:     db,
		logger: logger,
	}
}

// Check returns a simple health status.
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready checks if the service is ready (database connectivity).
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	// Ping database
	if err := h.db.WithContext(r.Context()).Raw("SELECT 1").Error; err != nil {
		h.logger.WithError(err).Error("database ping failed")
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not ready",
			"reason": "database unavailable",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
