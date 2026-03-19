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
type Emitter struct {
	mu        sync.RWMutex
	syncSubs  []Subscriber
	asyncSubs []Subscriber
	asyncCh   chan Event
	logger    *logrus.Entry
	wg        sync.WaitGroup
	closed    chan struct{}
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
		closed:  make(chan struct{}),
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

// Publish dispatches an event to all sync subscribers immediately, then
// enqueues it for async subscribers. A panic in one sync subscriber is
// recovered so that remaining subscribers still run.
func (e *Emitter) Publish(evt Event) {
	e.mu.RLock()
	syncSubs := make([]Subscriber, len(e.syncSubs))
	copy(syncSubs, e.syncSubs)
	hasAsync := len(e.asyncSubs) > 0
	e.mu.RUnlock()

	// Dispatch sync subscribers inline
	for _, sub := range syncSubs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					e.logger.WithField("panic", r).Error("sync subscriber panicked while handling event")
				}
			}()
			sub(evt)
		}()
	}

	// Enqueue for async subscribers (non-blocking drop if channel full)
	if hasAsync {
		select {
		case e.asyncCh <- evt:
		default:
			e.logger.Warn("async event channel full, dropping event")
		}
	}
}

// asyncWorker reads events from the async channel and dispatches to all async subscribers.
func (e *Emitter) asyncWorker() {
	defer e.wg.Done()
	for {
		select {
		case evt, ok := <-e.asyncCh:
			if !ok {
				return
			}
			e.mu.RLock()
			subs := make([]Subscriber, len(e.asyncSubs))
			copy(subs, e.asyncSubs)
			e.mu.RUnlock()

			for _, sub := range subs {
				func() {
					defer func() {
						if r := recover(); r != nil {
							e.logger.WithField("panic", r).Error("async subscriber panicked while handling event")
						}
					}()
					sub(evt)
				}()
			}
		case <-e.closed:
			return
		}
	}
}

// Close shuts down the async workers gracefully.
// It closes the async channel and waits for workers to drain.
func (e *Emitter) Close() {
	close(e.asyncCh)
	e.wg.Wait()
}

// SubscriberCount returns the total number of registered subscribers (sync + async).
func (e *Emitter) SubscriberCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.syncSubs) + len(e.asyncSubs)
}
