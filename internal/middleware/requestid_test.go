package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"kerplan/internal/pkg/ctxutil"
)

func TestRequestIDMiddlewareGeneratesID(t *testing.T) {
	middleware := NewRequestIDMiddleware()

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := ctxutil.GetRequestID(r.Context())
		assert.NotEmpty(t, requestID)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}

func TestRequestIDMiddlewarePassesThroughExisting(t *testing.T) {
	middleware := NewRequestIDMiddleware()
	testRequestID := "existing-request-id-123"

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := ctxutil.GetRequestID(r.Context())
		assert.Equal(t, testRequestID, requestID)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Request-ID", testRequestID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, testRequestID, w.Header().Get("X-Request-ID"))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestIDMiddlewareResponseHeader(t *testing.T) {
	middleware := NewRequestIDMiddleware()

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	responseID := w.Header().Get("X-Request-ID")
	assert.NotEmpty(t, responseID)

	// Verify it's a valid UUID format
	_, err := uuid.Parse(responseID)
	assert.NoError(t, err)
}

func TestRequestIDMiddlewareConsistency(t *testing.T) {
	middleware := NewRequestIDMiddleware()
	capturedID := ""

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = ctxutil.GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Context ID should match response header
	assert.Equal(t, capturedID, w.Header().Get("X-Request-ID"))
}

func TestRequestIDMiddlewareEmptyHeaderGenerated(t *testing.T) {
	middleware := NewRequestIDMiddleware()

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	// Don't set X-Request-ID header
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	requestID := w.Header().Get("X-Request-ID")
	assert.NotEmpty(t, requestID)

	// Should be valid UUID
	_, err := uuid.Parse(requestID)
	assert.NoError(t, err)
}

func TestRequestIDMiddlewareMultipleRequests(t *testing.T) {
	middleware := NewRequestIDMiddleware()
	ids := make(map[string]bool)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make multiple requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		requestID := w.Header().Get("X-Request-ID")
		assert.NotEmpty(t, requestID)
		ids[requestID] = true
	}

	// All IDs should be unique
	assert.Equal(t, 5, len(ids))
}
