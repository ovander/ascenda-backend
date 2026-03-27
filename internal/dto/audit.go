package dto

import (
	"encoding/json"
	"time"

	"ascenda/internal/model"
)

// AuditLogResponse is the API representation of an audit log entry.
type AuditLogResponse struct {
	ID         string          `json:"id"`
	UserID     string          `json:"userId"`
	EntityType string          `json:"entityType"`
	EntityID   string          `json:"entityId"`
	Action     string          `json:"action"`
	Changes    json.RawMessage `json:"changes,omitempty"`
	CreatedAt  time.Time       `json:"createdAt"`
}

// AuditLogFromModel converts a model.AuditLog to an AuditLogResponse.
func AuditLogFromModel(a *model.AuditLog) AuditLogResponse {
	return AuditLogResponse{
		ID:         a.ID.String(),
		UserID:     a.UserID.String(),
		EntityType: a.EntityType,
		EntityID:   a.EntityID.String(),
		Action:     a.Action,
		Changes:    a.Changes,
		CreatedAt:  a.CreatedAt,
	}
}

// AuditLogsFromModels converts a slice of model.AuditLog pointers to AuditLogResponse DTOs.
func AuditLogsFromModels(logs []*model.AuditLog) []AuditLogResponse {
	out := make([]AuditLogResponse, len(logs))
	for i, l := range logs {
		out[i] = AuditLogFromModel(l)
	}
	return out
}
