package event

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func testLogger() *logrus.Entry {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel) // silence during tests
	return log.WithField("test", true)
}

func TestEmitterPublishCallsSubscribers(t *testing.T) {
	emitter := NewEmitter(testLogger())

	var called1, called2 bool
	emitter.Subscribe(func(e Event) { called1 = true })
	emitter.Subscribe(func(e Event) { called2 = true })

	emitter.Publish(Event{Type: DataChanged})

	assert.True(t, called1, "subscriber 1 should be called")
	assert.True(t, called2, "subscriber 2 should be called")
}

func TestEmitterSubscriberReceivesCorrectEvent(t *testing.T) {
	emitter := NewEmitter(testLogger())

	tenantID := uuid.New()
	scenarioID := uuid.New()

	var received Event
	emitter.Subscribe(func(e Event) { received = e })

	emitter.Publish(Event{
		Type:       DataChanged,
		TenantID:   tenantID,
		ScenarioID: scenarioID,
		EntityType: "product",
		Action:     ActionCreate,
	})

	assert.Equal(t, DataChanged, received.Type)
	assert.Equal(t, tenantID, received.TenantID)
	assert.Equal(t, scenarioID, received.ScenarioID)
	assert.Equal(t, "product", received.EntityType)
	assert.Equal(t, ActionCreate, received.Action)
}

func TestEmitterSubscriberPanicDoesNotCrash(t *testing.T) {
	emitter := NewEmitter(testLogger())

	var calledAfterPanic bool
	emitter.Subscribe(func(e Event) { panic("test panic") })
	emitter.Subscribe(func(e Event) { calledAfterPanic = true })

	// Should not panic
	emitter.Publish(Event{Type: DataChanged})

	assert.True(t, calledAfterPanic, "second subscriber should still be called after first panics")
}

func TestEmitterNoSubscribers(t *testing.T) {
	emitter := NewEmitter(testLogger())

	// Should not panic
	emitter.Publish(Event{Type: DataChanged})
	assert.Equal(t, 0, emitter.SubscriberCount())
}

func TestEmitterSubscriberCount(t *testing.T) {
	emitter := NewEmitter(testLogger())
	assert.Equal(t, 0, emitter.SubscriberCount())

	emitter.Subscribe(func(e Event) {})
	assert.Equal(t, 1, emitter.SubscriberCount())

	emitter.Subscribe(func(e Event) {})
	assert.Equal(t, 2, emitter.SubscriberCount())
}

func TestEmitterConcurrentPublish(t *testing.T) {
	emitter := NewEmitter(testLogger())

	var count int64
	emitter.Subscribe(func(e Event) {
		atomic.AddInt64(&count, 1)
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			emitter.Publish(Event{Type: DataChanged})
		}()
	}

	wg.Wait()
	assert.Equal(t, int64(100), atomic.LoadInt64(&count))
}

func TestReportInvalidator(t *testing.T) {
	inv, handler := NewReportInvalidator(testLogger())

	scenarioID := uuid.New()

	// Initially not stale
	assert.False(t, inv.IsStale(scenarioID))

	// Publish a DataChanged event
	handler(Event{
		Type:       DataChanged,
		ScenarioID: scenarioID,
	})

	assert.True(t, inv.IsStale(scenarioID))

	// Mark fresh
	inv.MarkFresh(scenarioID)
	assert.False(t, inv.IsStale(scenarioID))
}

func TestReportInvalidatorIgnoresNonDataChangedEvents(t *testing.T) {
	inv, handler := NewReportInvalidator(testLogger())

	scenarioID := uuid.New()
	handler(Event{
		Type:       PlanCreated,
		ScenarioID: scenarioID,
	})

	assert.False(t, inv.IsStale(scenarioID), "non-DataChanged events should not invalidate")
}

func TestReportInvalidatorIgnoresNilScenarioID(t *testing.T) {
	inv, handler := NewReportInvalidator(testLogger())

	handler(Event{
		Type:       DataChanged,
		ScenarioID: uuid.Nil,
	})

	assert.Empty(t, inv.StaleScenarios())
}

func TestReportInvalidatorStaleScenarios(t *testing.T) {
	inv, handler := NewReportInvalidator(testLogger())

	id1 := uuid.New()
	id2 := uuid.New()

	handler(Event{Type: DataChanged, ScenarioID: id1})
	handler(Event{Type: DataChanged, ScenarioID: id2})

	stale := inv.StaleScenarios()
	assert.Len(t, stale, 2)
	assert.Contains(t, stale, id1)
	assert.Contains(t, stale, id2)
}

func TestEmitterAsyncSubscriber(t *testing.T) {
	emitter := NewEmitter(testLogger(), WithBufferSize(10), WithWorkers(2))
	defer emitter.Close()

	var count int64
	done := make(chan struct{})
	emitter.SubscribeAsync(func(e Event) {
		atomic.AddInt64(&count, 1)
		if atomic.LoadInt64(&count) == 5 {
			close(done)
		}
	})

	for i := 0; i < 5; i++ {
		emitter.Publish(Event{Type: DataChanged})
	}

	// Wait for async processing (with timeout)
	select {
	case <-done:
		assert.Equal(t, int64(5), atomic.LoadInt64(&count))
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for async subscribers")
	}
}

func TestEmitterSyncAndAsyncMixed(t *testing.T) {
	emitter := NewEmitter(testLogger(), WithBufferSize(10), WithWorkers(2))
	defer emitter.Close()

	var syncCalled int64
	var asyncCalled int64
	done := make(chan struct{})

	emitter.Subscribe(func(e Event) {
		atomic.AddInt64(&syncCalled, 1)
	})
	emitter.SubscribeAsync(func(e Event) {
		atomic.AddInt64(&asyncCalled, 1)
		close(done)
	})

	emitter.Publish(Event{Type: DataChanged})

	// Sync should already be called
	assert.Equal(t, int64(1), atomic.LoadInt64(&syncCalled))

	select {
	case <-done:
		assert.Equal(t, int64(1), atomic.LoadInt64(&asyncCalled))
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for async subscriber")
	}
}

func TestEmitterAsyncPanicRecovery(t *testing.T) {
	emitter := NewEmitter(testLogger(), WithBufferSize(10), WithWorkers(1))
	defer emitter.Close()

	var calledAfterPanic int64
	done := make(chan struct{})

	emitter.SubscribeAsync(func(e Event) {
		if e.EntityType == "panic" {
			panic("async panic test")
		}
		atomic.AddInt64(&calledAfterPanic, 1)
		close(done)
	})

	// First event causes panic
	emitter.Publish(Event{Type: DataChanged, EntityType: "panic"})
	// Second event should still work
	emitter.Publish(Event{Type: DataChanged, EntityType: "normal"})

	select {
	case <-done:
		assert.Equal(t, int64(1), atomic.LoadInt64(&calledAfterPanic))
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: async worker didn't recover from panic")
	}
}

func TestEmitterClose(t *testing.T) {
	emitter := NewEmitter(testLogger(), WithBufferSize(10), WithWorkers(2))

	var count int64
	emitter.SubscribeAsync(func(e Event) {
		atomic.AddInt64(&count, 1)
	})

	for i := 0; i < 5; i++ {
		emitter.Publish(Event{Type: DataChanged})
	}
	emitter.Close()
	// Close waits for the workers to drain the queue.
	assert.Equal(t, int64(5), atomic.LoadInt64(&count))
}

func TestEmitterSubscriberCountMixed(t *testing.T) {
	emitter := NewEmitter(testLogger(), WithBufferSize(10), WithWorkers(1))
	defer emitter.Close()

	assert.Equal(t, 0, emitter.SubscriberCount())

	emitter.Subscribe(func(e Event) {})
	assert.Equal(t, 1, emitter.SubscriberCount())

	emitter.SubscribeAsync(func(e Event) {})
	assert.Equal(t, 2, emitter.SubscriberCount())
}
