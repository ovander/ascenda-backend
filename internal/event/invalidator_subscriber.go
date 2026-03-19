package event

import (
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// ReportInvalidator tracks which scenarios have stale (invalidated)
// cached reports. Any DataChanged event marks the affected scenario as
// stale so that the next report request triggers a fresh computation.
type ReportInvalidator struct {
	mu    sync.RWMutex
	stale map[uuid.UUID]bool // scenarioID -> true if stale
	logger *logrus.Entry
}

// NewReportInvalidator creates a ReportInvalidator and returns its handler
// function ready to be passed to Emitter.Subscribe.
func NewReportInvalidator(logger *logrus.Entry) (*ReportInvalidator, Subscriber) {
	inv := &ReportInvalidator{
		stale:  make(map[uuid.UUID]bool),
		logger: logger.WithField("component", "report_invalidator"),
	}
	return inv, inv.Handle
}

// Handle processes an event by marking the scenario's cached report as stale.
func (inv *ReportInvalidator) Handle(evt Event) {
	if evt.Type != DataChanged {
		return
	}

	if evt.ScenarioID == uuid.Nil {
		return
	}

	inv.mu.Lock()
	defer inv.mu.Unlock()
	inv.stale[evt.ScenarioID] = true

	inv.logger.WithField("scenario_id", evt.ScenarioID).
		Debug("report cache invalidated")
}

// IsStale returns true if the scenario's cached report has been invalidated.
func (inv *ReportInvalidator) IsStale(scenarioID uuid.UUID) bool {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	return inv.stale[scenarioID]
}

// MarkFresh clears the stale flag for a scenario after a fresh report has
// been computed and cached.
func (inv *ReportInvalidator) MarkFresh(scenarioID uuid.UUID) {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	delete(inv.stale, scenarioID)
}

// StaleScenarios returns the set of scenario IDs that are currently stale.
// Useful for diagnostics and testing.
func (inv *ReportInvalidator) StaleScenarios() []uuid.UUID {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	ids := make([]uuid.UUID, 0, len(inv.stale))
	for id := range inv.stale {
		ids = append(ids, id)
	}
	return ids
}
