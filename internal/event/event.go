package event

import (
	"encoding/json"

	"github.com/google/uuid"
)

// Type classifies domain events.
type Type string

const (
	// DataChanged is emitted when any scenario entity is created, updated, or deleted.
	DataChanged Type = "data_changed"

	// PlanCreated is emitted when a new business plan is created.
	PlanCreated Type = "plan_created"

	// PlanDeleted is emitted when a business plan is deleted.
	PlanDeleted Type = "plan_deleted"

	// SnapshotCreated is emitted when a scenario snapshot is taken.
	SnapshotCreated Type = "snapshot_created"

	// SnapshotRestored is emitted when a snapshot is restored.
	SnapshotRestored Type = "snapshot_restored"
)

// Action classifies the mutation that triggered an event.
type Action string

const (
	ActionCreate  Action = "create"
	ActionUpdate  Action = "update"
	ActionDelete  Action = "delete"
	ActionRestore Action = "restore"
)

// Event represents a domain event emitted after a mutation.
type Event struct {
	Type       Type            `json:"type"`
	TenantID   uuid.UUID       `json:"tenantId"`
	UserID     uuid.UUID       `json:"userId"`
	ScenarioID uuid.UUID       `json:"scenarioId,omitempty"`
	EntityType string          `json:"entityType"`
	EntityID   uuid.UUID       `json:"entityId,omitempty"`
	Action     Action          `json:"action"`
	Changes    json.RawMessage `json:"changes,omitempty"`
}
