package event

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"ascenda/internal/model"
	"ascenda/internal/repo"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// auditWriteAttempts is how many times an audit entry is written before it is
// counted as lost; the delay before attempt n+1 is n × auditRetryDelay.
const (
	auditWriteAttempts = 3
	auditRetryDelay    = 100 * time.Millisecond
)

// AuditSubscriber writes audit log entries to the database for every domain event.
type AuditSubscriber struct {
	auditRepo  repo.AuditRepository
	logger     *logrus.Entry
	retryDelay time.Duration
}

// NewAuditSubscriber creates an AuditSubscriber and returns its handler function
// ready to be passed to Emitter.Subscribe.
func NewAuditSubscriber(auditRepo repo.AuditRepository, logger *logrus.Entry) Subscriber {
	return newAuditSubscriber(auditRepo, logger, auditRetryDelay).Handle
}

func newAuditSubscriber(auditRepo repo.AuditRepository, logger *logrus.Entry, retryDelay time.Duration) *AuditSubscriber {
	return &AuditSubscriber{
		auditRepo:  auditRepo,
		logger:     logger.WithField("component", "audit_subscriber"),
		retryDelay: retryDelay,
	}
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

	// Transient database errors (failover, pool exhaustion) are retried with
	// a short linear backoff. The entry keeps its ID across attempts, so a
	// retry after an ambiguous failure (committed, but the reply was lost)
	// hits the primary key instead of creating a second row, and that
	// duplicate means the first attempt did land.
	var err error
	for attempt := 1; attempt <= auditWriteAttempts; attempt++ {
		err = s.auditRepo.Create(entry)
		if err == nil || (attempt > 1 && isUniqueViolation(err)) {
			return
		}
		if attempt < auditWriteAttempts {
			time.Sleep(time.Duration(attempt) * s.retryDelay)
		}
	}

	auditWriteFailuresTotal.Inc()
	s.logger.WithError(err).WithFields(logrus.Fields{
		"audit_id":    entry.ID,
		"tenant_id":   entry.TenantID,
		"user_id":     entry.UserID,
		"entity_type": entry.EntityType,
		"entity_id":   entry.EntityID,
		"action":      entry.Action,
		"attempts":    auditWriteAttempts,
	}).Error("audit log entry lost: write failed after all attempts")
}

// isUniqueViolation reports whether err is PostgreSQL's unique_violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
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
