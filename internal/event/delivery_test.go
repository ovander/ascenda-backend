package event

// Delivery guarantees of the async path (audit §3.1: events were dropped with
// a warning when the queue was full, and Publish after Close panicked).

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ascenda/internal/model"
	"ascenda/internal/repo"
)

// recorder is an async subscriber that records entity types and can block
// on events whose EntityType is "block" until release is closed.
type recorder struct {
	mu      sync.Mutex
	seen    []string
	started chan struct{}
	release chan struct{}
}

func newRecorder() *recorder {
	return &recorder{started: make(chan struct{}, 16), release: make(chan struct{})}
}

func (r *recorder) handle(e Event) {
	if e.EntityType == "block" {
		r.started <- struct{}{}
		<-r.release
	}
	r.mu.Lock()
	r.seen = append(r.seen, e.EntityType)
	r.mu.Unlock()
}

func (r *recorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.seen...)
}

func TestEmitter_QueueFullDeliversInPublisherInsteadOfDropping(t *testing.T) {
	e := NewEmitter(testLogger(), WithBufferSize(1), WithWorkers(1))
	rec := newRecorder()
	e.SubscribeAsync(rec.handle)
	before := testutil.ToFloat64(inlineDispatchTotal.WithLabelValues("queue_full"))

	e.Publish(Event{EntityType: "block"}) // taken by the only worker, which blocks
	<-rec.started
	e.Publish(Event{EntityType: "queued"}) // fills the one-slot queue
	e.Publish(Event{EntityType: "inline"}) // queue full: must not be dropped

	// The overflow event was handled synchronously by Publish itself.
	assert.Equal(t, []string{"inline"}, rec.snapshot())
	assert.Equal(t, before+1, testutil.ToFloat64(inlineDispatchTotal.WithLabelValues("queue_full")))

	close(rec.release)
	e.Close()
	assert.ElementsMatch(t, []string{"block", "queued", "inline"}, rec.snapshot(), "every event is delivered exactly once")
}

func TestEmitter_PublishAfterCloseIsDeliveredNotPanicking(t *testing.T) {
	e := NewEmitter(testLogger(), WithBufferSize(4), WithWorkers(1))
	rec := newRecorder()
	e.SubscribeAsync(rec.handle)
	before := testutil.ToFloat64(inlineDispatchTotal.WithLabelValues("closed"))

	e.Close()
	require.NotPanics(t, func() { e.Publish(Event{EntityType: "late"}) })

	assert.Equal(t, []string{"late"}, rec.snapshot())
	assert.Equal(t, before+1, testutil.ToFloat64(inlineDispatchTotal.WithLabelValues("closed")))
}

func TestEmitter_CloseIsIdempotent(t *testing.T) {
	e := NewEmitter(testLogger())
	e.SubscribeAsync(func(Event) {})
	e.Close()
	assert.NotPanics(t, e.Close)
}

func TestEmitter_ConcurrentPublishAndCloseLoseNothing(t *testing.T) {
	e := NewEmitter(testLogger(), WithBufferSize(8), WithWorkers(2))
	var delivered int64
	e.SubscribeAsync(func(Event) {
		time.Sleep(50 * time.Microsecond) // slow subscriber: the queue fills up
		atomic.AddInt64(&delivered, 1)
	})

	const publishers, perPublisher = 8, 50
	var wg sync.WaitGroup
	for p := 0; p < publishers; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perPublisher; i++ {
				e.Publish(Event{Type: DataChanged})
			}
		}()
	}
	time.Sleep(2 * time.Millisecond)
	e.Close() // races with the publishers on purpose
	wg.Wait()

	assert.Equal(t, int64(publishers*perPublisher), atomic.LoadInt64(&delivered))
}

func TestEmitter_SubscriberPanicIsCounted(t *testing.T) {
	e := NewEmitter(testLogger())
	before := testutil.ToFloat64(subscriberPanicsTotal.WithLabelValues("sync"))
	e.Subscribe(func(Event) { panic("boom") })
	e.Publish(Event{})
	assert.Equal(t, before+1, testutil.ToFloat64(subscriberPanicsTotal.WithLabelValues("sync")))
}

// ── Audit subscriber ─────────────────────────────────────────────────────────

// flakyAuditRepo fails the first `failures` Create calls with err.
type flakyAuditRepo struct {
	repo.AuditRepository
	failures int
	err      error
	calls    int
	written  []*model.AuditLog
}

func (f *flakyAuditRepo) Create(l *model.AuditLog) error {
	f.calls++
	if f.calls <= f.failures {
		return f.err
	}
	f.written = append(f.written, l)
	return nil
}

func TestAuditSubscriber_RetriesTransientFailures(t *testing.T) {
	r := &flakyAuditRepo{failures: 2, err: errors.New("connection reset")}
	sub := newAuditSubscriber(r, testLogger(), time.Millisecond)

	sub.Handle(Event{TenantID: uuid.New(), EntityType: "product", Action: ActionUpdate})

	assert.Equal(t, 3, r.calls)
	require.Len(t, r.written, 1)
	assert.Equal(t, "product", r.written[0].EntityType)
}

func TestAuditSubscriber_CountsEntryLostAfterAllAttempts(t *testing.T) {
	r := &flakyAuditRepo{failures: 99, err: errors.New("database is down")}
	sub := newAuditSubscriber(r, testLogger(), time.Millisecond)
	before := testutil.ToFloat64(auditWriteFailuresTotal)

	sub.Handle(Event{TenantID: uuid.New(), EntityType: "product", Action: ActionDelete})

	assert.Equal(t, auditWriteAttempts, r.calls)
	assert.Empty(t, r.written)
	assert.Equal(t, before+1, testutil.ToFloat64(auditWriteFailuresTotal))
}

func TestAuditSubscriber_DuplicateOnRetryMeansFirstWriteLanded(t *testing.T) {
	// Attempt 1 fails ambiguously, attempt 2 hits the primary key: the row
	// exists, so the entry must not be counted as lost nor retried again.
	r := &sequenceAuditRepo{errs: []error{errors.New("unexpected EOF"), &pgconn.PgError{Code: "23505"}}}
	sub := newAuditSubscriber(r, testLogger(), time.Millisecond)
	before := testutil.ToFloat64(auditWriteFailuresTotal)

	sub.Handle(Event{TenantID: uuid.New(), EntityType: "plan", Action: ActionCreate})

	assert.Equal(t, 2, r.calls)
	assert.Equal(t, before, testutil.ToFloat64(auditWriteFailuresTotal))
}

func TestAuditSubscriber_DuplicateOnFirstAttemptIsRetried(t *testing.T) {
	// A duplicate on the very first attempt is not explained by an earlier
	// attempt of ours, so it is not taken as success outright: the write is
	// retried, and only the retry's duplicate proves the row exists.
	dup := &pgconn.PgError{Code: "23505"}
	r := &sequenceAuditRepo{errs: []error{dup, dup, dup}}
	sub := newAuditSubscriber(r, testLogger(), time.Millisecond)
	before := testutil.ToFloat64(auditWriteFailuresTotal)

	sub.Handle(Event{TenantID: uuid.New(), EntityType: "plan", Action: ActionCreate})

	// Attempt 2 sees a duplicate and treats it as the first write landing.
	assert.Equal(t, 2, r.calls)
	assert.Equal(t, before, testutil.ToFloat64(auditWriteFailuresTotal))
}

// sequenceAuditRepo returns errs[i] on call i (nil once the list runs out).
type sequenceAuditRepo struct {
	repo.AuditRepository
	errs  []error
	calls int
}

func (s *sequenceAuditRepo) Create(*model.AuditLog) error {
	s.calls++
	if s.calls <= len(s.errs) {
		return s.errs[s.calls-1]
	}
	return nil
}
