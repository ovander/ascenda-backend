package event

import (
	"encoding/json"

	"ascenda/internal/model"
	"ascenda/internal/repo"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// AuditSubscriber writes audit log entries to the database for every domain event.
type AuditSubscriber struct {
	auditRepo repo.AuditRepository
	logger    *logrus.Entry
}

// NewAuditSubscriber creates an AuditSubscriber and returns its handler function
// ready to be passed to Emitter.Subscribe.
func NewAuditSubscriber(auditRepo repo.AuditRepository, logger *logrus.Entry) Subscriber {
	s := &AuditSubscriber{
		auditRepo: auditRepo,
		logger:    logger.WithField("component", "audit_subscriber"),
	}
	return s.Handle
}

// Handle processes an event by persisting an audit log entry.
//
// entity_id is set to the scenario UUID when the event carries one — this makes
// every scenario-scoped mutation (products, staff, capex …) show up in the
// scenario audit trail downloaded by DownloadByScenario.  The original
// EntityID (e.g. the product or snapshot UUID) is injected into the Changes
// payload under the "_entityId" key so callers can still cross-reference it.
func (s *AuditSubscriber) Handle(evt Event) {
	// Determine which UUID to use as the audit row's entity_id.
	entityID := evt.EntityID
	if evt.ScenarioID != uuid.Nil {
		entityID = evt.ScenarioID
	}

	// Enrich changes with the original entity UUID when it differs from the
	// scope key (scenario) so downstream readers can identify the exact record.
	changes := evt.Changes
	if evt.EntityID != uuid.Nil && evt.EntityID != entityID {
		changes = injectEntityID(changes, evt.EntityID)
	}

	entry := &model.AuditLog{
		ID:         uuid.New(),
		TenantID:   evt.TenantID,
		UserID:     evt.UserID,
		EntityType: evt.EntityType,
		EntityID:   entityID,
		Action:     string(evt.Action),
		Changes:    changes,
	}

	if err := s.auditRepo.Create(entry); err != nil {
		s.logger.WithError(err).
			WithField("entity_type", evt.EntityType).
			WithField("action", evt.Action).
			Error("failed to write audit log")
	}
}

// injectEntityID merges {"_entityId": id} into an existing JSON object.
// If changes is nil or invalid JSON the result is a fresh object with just
// the _entityId key.
func injectEntityID(changes json.RawMessage, id uuid.UUID) json.RawMessage {
	m := make(map[string]interface{})
	if len(changes) > 0 {
		_ = json.Unmarshal(changes, &m)
	}
	m["_entityId"] = id.String()
	b, err := json.Marshal(m)
	if err != nil {
		return changes
	}
	return b
}
