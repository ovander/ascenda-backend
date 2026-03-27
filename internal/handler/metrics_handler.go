package handler

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandler exposes Prometheus metrics at GET /metrics.
// It is only registered in the router when cfg.MetricsEnabled is true.
//
// Deployment note: in production, restrict /metrics to an internal network or
// add basic auth — it exposes internal process and request counters.
type MetricsHandler struct{}

// NewMetricsHandler creates a new MetricsHandler.
func NewMetricsHandler() *MetricsHandler {
	return &MetricsHandler{}
}

// ServeHTTP delegates to the default Prometheus HTTP handler which serialises
// all registered metrics in the Prometheus exposition format.
func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	promhttp.Handler().ServeHTTP(w, r)
}
