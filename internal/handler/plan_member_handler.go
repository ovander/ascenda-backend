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
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// PlanMemberHandler handles plan membership operations.
type PlanMemberHandler struct {
	repo   repo.PlanMemberRepository
	logger *logrus.Entry
}

// NewPlanMemberHandler creates a new PlanMemberHandler.
func NewPlanMemberHandler(repo repo.PlanMemberRepository, logger *logrus.Entry) *PlanMemberHandler {
	return &PlanMemberHandler{repo: repo, logger: logger}
}

// PlanMemberDTO represents a plan member in API responses.
type PlanMemberDTO struct {
	ID        string `json:"id"`
	PlanID    string `json:"planId"`
	UserID    string `json:"userId"`
	Role      string `json:"role"`
	GrantedBy string `json:"grantedBy"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func memberToDTO(m *model.PlanMember) PlanMemberDTO {
	return PlanMemberDTO{
		ID:        m.ID.String(),
		PlanID:    m.PlanID.String(),
		UserID:    m.UserID.String(),
		Role:      m.Role,
		GrantedBy: m.GrantedBy.String(),
		CreatedAt: m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: m.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// List returns all members of a plan.
func (h *PlanMemberHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxutil.GetTenantID(r.Context())
	planIDStr := extractPlanIDFromRequest(r)
	planID, err := uuid.Parse(planIDStr)
	if err != nil {
		handleError(w, r, apierror.BadRequest("invalid plan ID"))
		return
	}

	members, err := h.repo.ListByPlan(tenantID, planID)
	if err != nil {
		handleError(w, r, apierror.Internal("failed to list plan members"))
		return
	}

	dtos := make([]PlanMemberDTO, len(members))
	for i, m := range members {
		dtos[i] = memberToDTO(m)
	}
	respondJSON(w, http.StatusOK, dtos)
}

// GrantRequest represents a request to grant plan access.
type GrantRequest struct {
	UserID string `json:"userId" validate:"required"`
	Role   string `json:"role" validate:"required"`
}

// Grant adds a user to a plan.
func (h *PlanMemberHandler) Grant(w http.ResponseWriter, r *http.Request) {
	var req GrantRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	if req.Role != "editor" && req.Role != "viewer" {
		handleError(w, r, apierror.BadRequest("role must be editor or viewer"))
		return
	}

	// Freemium users cannot share plans — only the plan owner has access.
	if ctxutil.GetUserPlan(r.Context()) == "freemium" {
		handleError(w, r, apierror.Forbidden("freemium plan does not support plan sharing — upgrade to invite collaborators"))
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	grantedBy := ctxutil.GetUserID(r.Context())
	planIDStr := extractPlanIDFromRequest(r)
	planID, err := uuid.Parse(planIDStr)
	if err != nil {
		handleError(w, r, apierror.BadRequest("invalid plan ID"))
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		handleError(w, r, apierror.BadRequest("invalid user ID"))
		return
	}

	// Check if already a member
	existing, _ := h.repo.GetByPlanAndUser(tenantID, planID, userID)
	if existing != nil {
		handleError(w, r, apierror.Conflict("user already has access to this plan"))
		return
	}

	member := &model.PlanMember{
		ID:        uuid.New(),
		TenantID:  tenantID,
		PlanID:    planID,
		UserID:    userID,
		Role:      req.Role,
		GrantedBy: grantedBy,
	}

	if err := h.repo.Create(member); err != nil {
		handleError(w, r, apierror.Internal("failed to grant plan access"))
		return
	}

	respondJSON(w, http.StatusCreated, memberToDTO(member))
}

// UpdateRoleRequest represents a request to change a member's plan role.
type UpdateMemberRoleRequest struct {
	Role string `json:"role" validate:"required"`
}

// UpdateRole changes a member's plan role.
func (h *PlanMemberHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	var req UpdateMemberRoleRequest
	if err := decodeAndValidate(r, &req); err != nil {
		handleError(w, r, err)
		return
	}

	if req.Role != "editor" && req.Role != "viewer" {
		handleError(w, r, apierror.BadRequest("role must be editor or viewer"))
		return
	}

	tenantID := ctxutil.GetTenantID(r.Context())
	planIDStr := extractPlanIDFromRequest(r)
	planID, err := uuid.Parse(planIDStr)
	if err != nil {
		handleError(w, r, apierror.BadRequest("invalid plan ID"))
		return
	}

	userID, err := parseUUIDParam(chi.URLParam(r, "userId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	member, err := h.repo.GetByPlanAndUser(tenantID, planID, userID)
	if err != nil || member == nil {
		handleError(w, r, apierror.NotFound("plan member", userID.String()))
		return
	}

	member.Role = req.Role
	if err := h.repo.Update(member); err != nil {
		handleError(w, r, apierror.Internal("failed to update role"))
		return
	}

	respondJSON(w, http.StatusOK, memberToDTO(member))
}

// Revoke removes a user from a plan.
func (h *PlanMemberHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxutil.GetTenantID(r.Context())
	planIDStr := extractPlanIDFromRequest(r)
	planID, err := uuid.Parse(planIDStr)
	if err != nil {
		handleError(w, r, apierror.BadRequest("invalid plan ID"))
		return
	}

	userID, err := parseUUIDParam(chi.URLParam(r, "userId"))
	if err != nil {
		handleError(w, r, err)
		return
	}

	if err := h.repo.Delete(tenantID, planID, userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			handleError(w, r, apierror.NotFound("plan member", userID.String()))
			return
		}
		handleError(w, r, apierror.Internal("failed to revoke plan access"))
		return
	}

	respondNoContent(w)
}

// extractPlanIDFromRequest extracts planId from chi params or URL path.
func extractPlanIDFromRequest(r *http.Request) string {
	if id := chi.URLParam(r, "planId"); id != "" {
		return id
	}
	if id := chi.URLParam(r, "id"); id != "" {
		return id
	}
	// Fallback: parse from URL path
	parts := splitPath(r.URL.Path)
	for i, p := range parts {
		if p == "plans" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func splitPath(path string) []string {
	var parts []string
	for _, p := range split(path, '/') {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func split(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			if i > start {
				parts = append(parts, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}
