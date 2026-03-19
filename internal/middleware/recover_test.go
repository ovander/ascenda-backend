package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestRecoverMiddlewarePanicRecovery(t *testing.T) {
	logger := logrus.New()
	recover := NewRecoverMiddleware(logger)

	handler := recover.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal server error")
}

func TestRecoverMiddlewareNoPanic(t *testing.T) {
	logger := logrus.New()
	recover := NewRecoverMiddleware(logger)

	handler := recover.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())
}

func TestRecoverMiddlewarePanicWithString(t *testing.T) {
	logger := logrus.New()
	recover := NewRecoverMiddleware(logger)

	handler := recover.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("custom panic message")
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		handler.ServeHTTP(w, req)
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRecoverMiddlewarePanicWithError(t *testing.T) {
	logger := logrus.New()
	recover := NewRecoverMiddleware(logger)

	customErr := "test error"
	handler := recover.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(customErr)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		handler.ServeHTTP(w, req)
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRecoverMiddlewareResponseWritten(t *testing.T) {
	logger := logrus.New()
	recover := NewRecoverMiddleware(logger)

	handler := recover.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		panic("panic after headers")
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Once WriteHeader(200) has been called, the status code is already committed.
	// The recover middleware catches the panic but cannot override the status code
	// in httptest.Recorder — the first WriteHeader call wins.
	assert.Equal(t, http.StatusOK, w.Code)
}
