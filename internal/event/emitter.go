package event

import (
	"sync"

	"github.com/sirupsen/logrus"
)

// Subscriber is a function that processes domain events.
type Subscriber func(Event)

// Emitter is an in-process event bus that dispatches domain events to
// registered subscribers. Sync subscribers run in the calling goroutine;
// async subscribers are dispatched to a background worker pool.
//
// Delivery to async subscribers is guaranteed for every published event: when
// the queue is full, or once the emitter is closed, Publish runs the async
// subscribers itself instead of dropping the event. A burst therefore slows
// the publishers down (backpressure) rather than losing audit entries.
type Emitter struct {
	mu        sync.RWMutex
	syncSubs  []Subscriber
	asyncSubs []Subscriber
	asyncCh   chan Event
	closed    bool // guarded by mu; set once by Close
	logger    *logrus.Entry
	wg        sync.WaitGroup
}

// NewEmitter creates a new Emitter with a buffered async channel.
// The bufferSize controls the async event channel capacity.
func NewEmitter(logger *logrus.Entry, opts ...EmitterOption) *Emitter {
	cfg := emitterConfig{
		bufferSize: 256,
		workers:    4,
	}
	for _, o := range opts {
		o(&cfg)
	}

	e := &Emitter{
		asyncCh: make(chan Event, cfg.bufferSize),
		logger:  logger.WithField("component", "event_emitter"),
	}

	// Start async worker goroutines
	for i := 0; i < cfg.workers; i++ {
		e.wg.Add(1)
		go e.asyncWorker()
	}

	return e
}

// emitterConfig holds configuration for the emitter.
type emitterConfig struct {
	bufferSize int
	workers    int
}

// EmitterOption configures the emitter.
type EmitterOption func(*emitterConfig)

// WithBufferSize sets the async channel buffer size.
func WithBufferSize(size int) EmitterOption {
	return func(c *emitterConfig) { c.bufferSize = size }
}

// WithWorkers sets the number of async worker goroutines.
func WithWorkers(n int) EmitterOption {
	return func(c *emitterConfig) { c.workers = n }
}

// Subscribe registers a synchronous subscriber that runs in the Publish goroutine.
func (e *Emitter) Subscribe(sub Subscriber) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.syncSubs = append(e.syncSubs, sub)
}

// SubscribeAsync registers an asynchronous subscriber that runs in a background worker.
func (e *Emitter) SubscribeAsync(sub Subscriber) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.asyncSubs = append(e.asyncSubs, sub)
}

// Publish dispatches an event to all sync subscribers immediately, then hands
// it to the async subscribers: through the worker queue when there is room,
// otherwise in the calling goroutine (queue full, or emitter closed). No event
// is ever dropped. A panic in one subscriber is recovered so that the
// remaining subscribers still run.
func (e *Emitter) Publish(evt Event) {
	e.mu.RLock()
	syncSubs := append([]Subscriber(nil), e.syncSubs...)
	e.mu.RUnlock()

	e.dispatch(syncSubs, evt, "sync")

	// The read lock is held across the non-blocking send so that Close cannot
	// close the channel between the closed check and the send.
	e.mu.RLock()
	asyncSubs := append([]Subscriber(nil), e.asyncSubs...)
	reason := ""
	if len(asyncSubs) > 0 {
		if e.closed {
			reason = "closed"
		} else {
			select {
			case e.asyncCh <- evt:
			default:
				reason = "queue_full"
			}
		}
	}
	e.mu.RUnlock()

	if reason != "" {
		inlineDispatchTotal.WithLabelValues(reason).Inc()
		e.logger.WithFields(logrus.Fields{
			"reason":      reason,
			"event_type":  evt.Type,
			"entity_type": evt.EntityType,
		}).Warn("async event queue unavailable, delivering event in the publishing goroutine")
		e.dispatch(asyncSubs, evt, "async")
	}
}

// dispatch runs every subscriber on evt, recovering panics individually.
func (e *Emitter) dispatch(subs []Subscriber, evt Event, mode string) {
	for _, sub := range subs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					subscriberPanicsTotal.WithLabelValues(mode).Inc()
					e.logger.WithFields(logrus.Fields{
						"panic":      r,
						"mode":       mode,
						"event_type": evt.Type,
					}).Error("event subscriber panicked while handling event")
				}
			}()
			sub(evt)
		}()
	}
}

// asyncWorker delivers queued events to the async subscribers until the queue
// is closed and drained.
func (e *Emitter) asyncWorker() {
	defer e.wg.Done()
	for evt := range e.asyncCh {
		e.mu.RLock()
		subs := append([]Subscriber(nil), e.asyncSubs...)
		e.mu.RUnlock()
		e.dispatch(subs, evt, "async")
	}
}

// Close stops accepting events into the queue, waits until the workers have
// delivered every queued event, and returns. Events published afterwards are
// delivered in the publishing goroutine. Close is safe to call more than once.
func (e *Emitter) Close() {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.closed = true
	close(e.asyncCh)
	e.mu.Unlock()
	e.wg.Wait()
}

// SubscriberCount returns the total number of registered subscribers (sync + async).
func (e *Emitter) SubscriberCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.syncSubs) + len(e.asyncSubs)
}
