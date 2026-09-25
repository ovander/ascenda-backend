package handler

import (
	"errors"
	"net/http"

	"ascenda/internal/model"
	"ascenda/internal/repo"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// PlanCapTableHandler handles the simplified plan-level cap table endpoints
// (Pro tier). This is distinct from the full scenario-level CapTableHandler.
type PlanCapTableHandler struct {
	repo   repo.PlanShareholderRepository
	logger *logrus.Entry
}

// NewPlanCapTableHandler creates a new PlanCapTableHandler.
func NewPlanCapTableHandler(repo repo.PlanShareholderRepository, logger *logrus.Entry) *PlanCapTableHandler {
	return &PlanCapTableHandler{repo: repo, logger: logger}
}

// planCapTableSummary is the response shape for GET /cap-table.
type planCapTableSummary struct {
	PlanID        string                   `json:"planId"`
	TotalShares   int64                    `json:"totalShares"`
	TotalInvested decimal.Decimal          `json:"totalInvested"`
	Shareholders  []*model.PlanShareholder `json:"shareholders"`
}

// GetSummary GET /plans/{planId}/cap-table
// Returns all shareholders and aggregate totals for a plan.
func (h *PlanCapTableHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	planID, err := parseUUIDParam(chi.URLParam(r, "planId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())

	shareholders, err := h.repo.ListByPlan(tenantID, planID)
	if err != nil {
		handleError(w, r, apierror.Internal("failed to load cap table"))
		return
	}

	var totalShares int64
	totalInvested := decimal.Zero
	for _, sh := range shareholders {
		totalShares += sh.Shares
		totalInvested = totalInvested.Add(sh.InvestedAmount)
	}

	respondJSON(w, http.StatusOK, planCapTableSummary{
		PlanID:        planID.String(),
		TotalShares:   totalShares,
		TotalInvested: totalInvested,
		Shareholders:  shareholders,
	})
}

// createShareholderRequest is the POST /cap-table/shareholders body.
type createShareholderRequest struct {
	Name           string                `json:"name"`
	Type           model.ShareholderType `json:"type"`
	Shares         int64                 `json:"shares"`
	OwnershipPct   decimal.Decimal       `json:"ownershipPct"`
	InvestedAmount decimal.Decimal       `json:"investedAmount"`
	Notes          string                `json:"notes,omitempty"`
}

// CreateShareholder POST /plans/{planId}/cap-table/shareholders
func (h *PlanCapTableHandler) CreateShareholder(w http.ResponseWriter, r *http.Request) {
	planID, err := parseUUIDParam(chi.URLParam(r, "planId"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	var req createShareholderRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())

	sh := &model.PlanShareholder{
		PlanID:         planID,
		Name:           req.Name,
		Type:           req.Type,
		Shares:         req.Shares,
		OwnershipPct:   req.OwnershipPct,
		InvestedAmount: req.InvestedAmount,
		Notes:          req.Notes,
	}
	sh.TenantID = tenantID

	if err := h.repo.Create(sh); err != nil {
		handleError(w, r, apierror.Internal("failed to create shareholder"))
		return
	}
	respondJSON(w, http.StatusCreated, sh)
}

// updateShareholderRequest is the PUT /cap-table/shareholders/{id} body.
type updateShareholderRequest struct {
	Name           string                `json:"name"`
	Type           model.ShareholderType `json:"type"`
	Shares         int64                 `json:"shares"`
	OwnershipPct   decimal.Decimal       `json:"ownershipPct"`
	InvestedAmount decimal.Decimal       `json:"investedAmount"`
	Notes          string                `json:"notes,omitempty"`
}

// UpdateShareholder PUT /plans/{planId}/cap-table/shareholders/{id}
func (h *PlanCapTableHandler) UpdateShareholder(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(chi.URLParam(r, "id"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	var req updateShareholderRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())

	sh, err := h.shareholderInPlan(r, tenantID, id)
	if err != nil {
		handleError(w, r, err)
		return
	}
	sh.Name = req.Name
	sh.Type = req.Type
	sh.Shares = req.Shares
	sh.OwnershipPct = req.OwnershipPct
	sh.InvestedAmount = req.InvestedAmount
	sh.Notes = req.Notes

	if err := h.repo.Update(sh); err != nil {
		handleError(w, r, apierror.Internal("failed to update shareholder"))
		return
	}
	respondNoContent(w)
}

// DeleteShareholder DELETE /plans/{planId}/cap-table/shareholders/{id}
func (h *PlanCapTableHandler) DeleteShareholder(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(chi.URLParam(r, "id"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	tenantID := ctxutil.GetTenantID(r.Context())
	if _, err := h.shareholderInPlan(r, tenantID, id); err != nil {
		handleError(w, r, err)
		return
	}
	if err := h.repo.Delete(tenantID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			handleError(w, r, apierror.NotFound("shareholder", id.String()))
			return
		}
		handleError(w, r, apierror.Internal("failed to delete shareholder"))
		return
	}
	respondNoContent(w)
}

// shareholderInPlan loads the shareholder and verifies it belongs to the
// {planId} in the URL; a shareholder of another plan is reported as not found.
func (h *PlanCapTableHandler) shareholderInPlan(r *http.Request, tenantID, id uuid.UUID) (*model.PlanShareholder, error) {
	planID, err := parseUUIDParam(chi.URLParam(r, "planId"))
	if err != nil {
		return nil, err
	}
	sh, err := h.repo.GetByID(tenantID, id)
	if err != nil || sh == nil || sh.PlanID != planID {
		return nil, apierror.NotFound("shareholder", id.String())
	}
	return sh, nil
}
