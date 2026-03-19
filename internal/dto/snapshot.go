package dto

import (
	"encoding/json"
	"time"

	"kerplan/internal/model"
)

// SnapshotResponse is the API representation of a plan snapshot (without data payload).
type SnapshotResponse struct {
	ID          string `json:"id"`
	ScenarioID  string `json:"scenarioId"`
	Version     int    `json:"version"`
	Label       string `json:"label"`
	Description string `json:"description"`
	CreatedBy   string `json:"createdBy"`
	Reason      string `json:"reason,omitempty"`
	ReportHash  string `json:"reportHash,omitempty"`
	Timestamps
}

// SnapshotFromModel converts a model.PlanSnapshot to a SnapshotResponse.
func SnapshotFromModel(s model.PlanSnapshot) SnapshotResponse {
	return SnapshotResponse{
		ID:          s.ID.String(),
		ScenarioID:  s.ScenarioID.String(),
		Version:     s.Version,
		Label:       s.Label,
		Description: s.Description,
		CreatedBy:   s.CreatedBy.String(),
		Reason:      s.Reason,
		ReportHash:  s.ReportHash,
		Timestamps: Timestamps{
			CreatedAt: s.CreatedAt,
			UpdatedAt: s.UpdatedAt,
		},
	}
}

// SnapshotsFromModels converts a slice of model.PlanSnapshot to SnapshotResponse DTOs.
func SnapshotsFromModels(snapshots []model.PlanSnapshot) []SnapshotResponse {
	out := make([]SnapshotResponse, len(snapshots))
	for i, s := range snapshots {
		out[i] = SnapshotFromModel(s)
	}
	return out
}

// SnapshotDataResponse wraps the snapshot metadata plus the raw data payload.
type SnapshotDataResponse struct {
	SnapshotResponse
	Data json.RawMessage `json:"data"`
}

// SnapshotDataFromModel converts a model.PlanSnapshot (with data) to a SnapshotDataResponse.
func SnapshotDataFromModel(s model.PlanSnapshot) SnapshotDataResponse {
	return SnapshotDataResponse{
		SnapshotResponse: SnapshotFromModel(s),
		Data:             s.Data,
	}
}

// SnapshotDiffResponse contains compared sections between two snapshots.
type SnapshotDiffResponse struct {
	Snapshot1 SnapshotMeta           `json:"snapshot1"`
	Snapshot2 SnapshotMeta           `json:"snapshot2"`
	Changes   map[string]interface{} `json:"changes"`
}

// SnapshotMeta is a lightweight identifier used in diff responses.
type SnapshotMeta struct {
	ID        string    `json:"id"`
	Version   int       `json:"version"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"createdAt"`
}
