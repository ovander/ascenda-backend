package handler

import (
	"encoding/json"
	"net/http"

	"ascenda/internal/model"
	"ascenda/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/ovander/backendkit/apierror"
	"github.com/sirupsen/logrus"
)

// FeaturePolicySvc is the interface the handler requires.
type FeaturePolicySvc interface {
	List() ([]*model.FeaturePolicy, error)
	UpdatePolicy(req service.UpdateFeaturePolicyRequest) (*model.FeaturePolicy, error)
}

// FeaturePolicyHandler exposes the feature-policy read and admin-write endpoints.
type FeaturePolicyHandler struct {
	svc    FeaturePolicySvc
	logger *logrus.Entry
}

func NewFeaturePolicyHandler(svc FeaturePolicySvc, logger *logrus.Entry) *FeaturePolicyHandler {
	return &FeaturePolicyHandler{svc: svc, logger: logger}
}

// List handles GET /api/v1/feature-policies
// Public (authenticated) — returns the full policy table so the frontend can
// evaluate limits client-side without hardcoding tier rules.
func (h *FeaturePolicyHandler) List(w http.ResponseWriter, r *http.Request) {
	policies, err := h.svc.List()
	if err != nil {
		handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, policies)
}

// Update handles PUT /api/v1/admin/feature-policies/{feature}
// Platform-admin only — moves a feature between tiers or changes its limits.
func (h *FeaturePolicyHandler) Update(w http.ResponseWriter, r *http.Request) {
	feature := chi.URLParam(r, "feature")
	if feature == "" {
		handleError(w, r, apierror.BadRequest("feature slug is required"))
		return
	}

	var req service.UpdateFeaturePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(w, r, apierror.BadRequest("invalid request body"))
		return
	}
	req.Feature = feature

	policy, err := h.svc.UpdatePolicy(req)
	if err != nil {
		handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, policy)
}
