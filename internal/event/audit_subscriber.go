package event

import (
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"kerplan/internal/model"
	"kerplan/internal/repo"
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
func (s *AuditSubscriber) Handle(evt Event) {
	entry := &model.AuditLog{
		ID:         uuid.New(),
		TenantID:   evt.TenantID,
		UserID:     evt.UserID,
		EntityType: evt.EntityType,
		EntityID:   evt.EntityID,
		Action:     string(evt.Action),
		Changes:    evt.Changes,
	}

	if err := s.auditRepo.Create(entry); err != nil {
		s.logger.WithError(err).
			WithField("entity_type", evt.EntityType).
			WithField("action", evt.Action).
			Error("failed to write audit log")
	}
}
