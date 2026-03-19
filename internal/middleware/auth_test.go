package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestAuthMiddlewareMissingToken(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	auth := NewAuthMiddleware("http://example.com/.well-known/jwks.json", "test-secret", logger)

	handler := auth.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddlewareInvalidTokenFormat(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	auth := NewAuthMiddleware("http://example.com/.well-known/jwks.json", "test-secret", logger)

	handler := auth.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name              string
		authorizationHeader string
	}{
		{
			name:              "missing Bearer prefix",
			authorizationHeader: "InvalidToken123",
		},
		{
			name:              "only Bearer keyword",
			authorizationHeader: "Bearer",
		},
		{
			name:              "wrong prefix",
			authorizationHeader: "Basic token123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/test", nil)
			req.Header.Set("Authorization", tt.authorizationHeader)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestAuthMiddlewareExtractBearerToken(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	auth := NewAuthMiddleware("http://example.com/.well-known/jwks.json", "test-secret", logger)

	tests := []struct {
		name                  string
		authorizationHeader   string
		expectedError         bool
	}{
		{
			name:                  "valid Bearer token",
			authorizationHeader:   "Bearer valid.jwt.token",
			expectedError:         false,
		},
		{
			name:                  "empty Authorization header",
			authorizationHeader:   "",
			expectedError:         true,
		},
		{
			name:                  "invalid format",
			authorizationHeader:   "InvalidFormat",
			expectedError:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/test", nil)
			if tt.authorizationHeader != "" {
				req.Header.Set("Authorization", tt.authorizationHeader)
			}

			token, err := auth.extractBearerToken(req)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, token)
			}
		})
	}
}

func TestAuthMiddlewareInvalidToken(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	auth := NewAuthMiddleware("http://example.com/.well-known/jwks.json", "test-secret", logger)

	handler := auth.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.jwt.signature")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
