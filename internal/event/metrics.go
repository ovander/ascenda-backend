package event

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus counters for the event bus, exposed on GET /metrics when
// METRICS_ENABLED=true. Any non-zero value is worth a look: the bus is sized
// so that none of them should move under normal load.
var (
	// inlineDispatchTotal counts events whose async subscribers ran in the
	// publisher's goroutine instead of a worker: the queue was full
	// (backpressure) or the emitter was already closed (shutdown).
	inlineDispatchTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ascenda_event_inline_dispatch_total",
		Help: "Events delivered to async subscribers in the publishing goroutine, by reason (queue_full, closed).",
	}, []string{"reason"})

	// subscriberPanicsTotal counts recovered subscriber panics.
	subscriberPanicsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ascenda_event_subscriber_panics_total",
		Help: "Panics recovered in event subscribers, by mode (sync, async).",
	}, []string{"mode"})

	// auditWriteFailuresTotal counts audit entries that could not be written
	// after every retry. Each one is also logged at error level with the
	// identifiers needed to reconstruct it.
	auditWriteFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ascenda_audit_write_failures_total",
		Help: "Audit log entries lost after all write attempts failed.",
	})
)
