package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// wrapTimeout returns a handler wrapped with TimeoutMiddleware(d).
func wrapTimeout(d time.Duration, h http.HandlerFunc) http.Handler {
	return TimeoutMiddleware(d)(h)
}

// ─────────────────────────────────────────────────────────────────────────────
// Basic behaviour
// ─────────────────────────────────────────────────────────────────────────────

func TestTimeoutMiddleware_CompletesWithinTimeout(t *testing.T) {
	var ctxErr error

	h := wrapTimeout(500*time.Millisecond, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond)
		ctxErr = r.Context().Err()
		w.WriteHeader(http.StatusOK)
	})

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	assert.NoError(t, ctxErr, "context must not be cancelled when handler finishes within the timeout")
}

func TestTimeoutMiddleware_ContextCancelledAfterExpiry(t *testing.T) {
	// The handler blocks on ctx.Done(); it must unblock once the deadline fires.
	var ctxErr error

	h := wrapTimeout(20*time.Millisecond, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		ctxErr = r.Context().Err()
		w.WriteHeader(http.StatusOK)
	})

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, context.DeadlineExceeded, ctxErr, "expired context must return DeadlineExceeded")
}

func TestTimeoutMiddleware_DeadlineIsApproximate(t *testing.T) {
	// Verify the deadline is set and roughly matches the configured duration.
	var d time.Time

	h := wrapTimeout(500*time.Millisecond, func(w http.ResponseWriter, r *http.Request) {
		d, _ = r.Context().Deadline()
		w.WriteHeader(http.StatusOK)
	})

	before := time.Now()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	require.False(t, d.IsZero(), "deadline must be set in the handler context")
	assert.WithinDuration(t, before.Add(500*time.Millisecond), d, 50*time.Millisecond,
		"deadline should be ≈500ms from when the middleware was entered")
}

// ─────────────────────────────────────────────────────────────────────────────
// Key regression: inner timeout overrides outer shorter deadline
// ─────────────────────────────────────────────────────────────────────────────

// TestTimeoutMiddleware_InnerOverridesOuterDeadline is the primary regression
// test for the bug where a global crudTimeout (5 s) caused every sub-group
// timeout (30 s, 60 s, 120 s) to be silently capped at 5 s.
//
// The fix (context.WithoutCancel) must make the innermost TimeoutMiddleware
// authoritative, regardless of any deadline already present on the parent.
func TestTimeoutMiddleware_InnerOverridesOuterDeadline(t *testing.T) {
	var ctxErr error

	h := wrapTimeout(200*time.Millisecond, func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the outer deadline but within the inner timeout.
		// Without the fix this sleep would expire the outer 10 ms deadline,
		// resulting in ctxErr == context.DeadlineExceeded.
		time.Sleep(30 * time.Millisecond)
		ctxErr = r.Context().Err()
		w.WriteHeader(http.StatusOK)
	})

	// Simulate the outer crudTimeout having been applied already (e.g. 10 ms).
	outerCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(outerCtx)
	h.ServeHTTP(httptest.NewRecorder(), req)

	assert.NoError(t, ctxErr,
		"handler slept 30ms which is beyond the 10ms outer deadline — "+
			"inner 200ms timeout must override the outer one")
}

// TestTimeoutMiddleware_NestedMiddlewaresInnermostWins verifies the chained
// middleware scenario: outer=10ms wraps inner=200ms wraps handler.
// The handler's effective deadline must come from the innermost middleware.
func TestTimeoutMiddleware_NestedMiddlewaresInnermostWins(t *testing.T) {
	var ctxErr error

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond) // > outer(10ms), < inner(200ms)
		ctxErr = r.Context().Err()
		w.WriteHeader(http.StatusOK)
	})

	// Innermost timeout is longest (mimics reportTimeout / aiTimeout).
	inner := TimeoutMiddleware(200 * time.Millisecond)(handler)
	// Outermost timeout is shortest (mimics the global crudTimeout).
	outer := TimeoutMiddleware(10 * time.Millisecond)(inner)

	outer.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	assert.NoError(t, ctxErr,
		"the innermost TimeoutMiddleware (200ms) must govern the handler, not the outermost (10ms)")
}

// TestTimeoutMiddleware_OuterTimeoutStillProtectsOwnLevel verifies that even
// though inner routes can escape the outer deadline, the outer deadline still
// fires for handlers that sit AT the outer level (i.e., don't apply their own
// longer timeout).
func TestTimeoutMiddleware_OuterDeadlineFiresForOuterHandlers(t *testing.T) {
	var ctxErr error

	// A handler that sits directly under the outer (10 ms) timeout — no inner
	// middleware to override it.
	h := wrapTimeout(10*time.Millisecond, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		ctxErr = r.Context().Err()
		w.WriteHeader(http.StatusOK)
	})

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, context.DeadlineExceeded, ctxErr,
		"handlers that don't override the timeout must still be governed by it")
}

// ─────────────────────────────────────────────────────────────────────────────
// Context value propagation
// ─────────────────────────────────────────────────────────────────────────────

func TestTimeoutMiddleware_ContextValuesPreserved(t *testing.T) {
	type ctxKey struct{}

	var gotVal any

	h := wrapTimeout(500*time.Millisecond, func(w http.ResponseWriter, r *http.Request) {
		gotVal = r.Context().Value(ctxKey{})
		w.WriteHeader(http.StatusOK)
	})

	// Inject a value into the parent context.
	ctx := context.WithValue(context.Background(), ctxKey{}, "expected")
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	h.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, "expected", gotVal,
		"context values must survive context.WithoutCancel + context.WithTimeout")
}

// ─────────────────────────────────────────────────────────────────────────────
// Parent cancellation isolation (documented trade-off of WithoutCancel)
// ─────────────────────────────────────────────────────────────────────────────

// TestTimeoutMiddleware_ParentCancellationNotPropagated documents the known
// trade-off of using context.WithoutCancel: if the parent context is cancelled
// (e.g., client disconnects at the TCP level before the middleware fires),
// that cancellation does NOT propagate into the handler's context.
//
// This is intentional for AI/report routes — we want the configured timeout
// to be authoritative and not be short-circuited by an inherited cancellation.
func TestTimeoutMiddleware_ParentCancellationNotPropagated(t *testing.T) {
	parentCtx, parentCancel := context.WithCancel(context.Background())

	var ctxErrAfterParentCancel error

	h := wrapTimeout(500*time.Millisecond, func(w http.ResponseWriter, r *http.Request) {
		// Cancel the parent from within the handler.
		parentCancel()

		// Give the runtime a moment to propagate (if it would).
		time.Sleep(5 * time.Millisecond)

		ctxErrAfterParentCancel = r.Context().Err()
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(parentCtx)
	h.ServeHTTP(httptest.NewRecorder(), req)

	assert.NoError(t, ctxErrAfterParentCancel,
		"parent cancellation must NOT propagate to the handler — "+
			"the inner timeout is the sole cancellation source (known trade-off of WithoutCancel)")
}
