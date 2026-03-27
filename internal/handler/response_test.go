package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"ascenda/internal/pkg/apierror"
	"ascenda/internal/pkg/ctxutil"
)

// testRequest returns a minimal *http.Request with a no-op logrus logger in context,
// satisfying the handleError signature without requiring a real HTTP server.
func testRequest() *http.Request {
	logger := logrus.NewEntry(logrus.New())
	ctx := ctxutil.WithLogger(context.Background(), logger)
	r, _ := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	return r
}

// withChiParams adds URL parameters from chi router context to an *http.Request.
// This helper is used across multiple handler test files to inject route parameters.
func withChiParams(r *http.Request, params map[string]string) *http.Request {
	chiCtx := chi.NewRouteContext()
	for k, v := range params {
		chiCtx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, chiCtx))
}

func TestRespondJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"status": "ok"}

	respondJSON(w, http.StatusOK, data)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, "ok", result["status"])
}

func TestRespondNoContent(t *testing.T) {
	w := httptest.NewRecorder()

	respondNoContent(w)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestHandleErrorWithAppError(t *testing.T) {
	w := httptest.NewRecorder()
	appErr := apierror.NotFound("Plan", "123")

	handleError(w, testRequest(), appErr)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var errResp apierror.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(t, err)
	assert.Equal(t, "not_found", errResp.Error.Code)
}

func TestHandleErrorWithGenericError(t *testing.T) {
	w := httptest.NewRecorder()
	genericErr := bytes.NewBufferString("some error").String()
	err := io.EOF

	handleError(w, testRequest(), err)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var errResp apierror.ErrorResponse
	unmarshalErr := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(t, unmarshalErr)
	assert.Equal(t, "internal_error", errResp.Error.Code)
	_ = genericErr // avoid unused variable warning
}

func TestDecodeJSON(t *testing.T) {
	t.Run("valid json", func(t *testing.T) {
		body := io.NopCloser(bytes.NewBufferString(`{"name": "test"}`))
		r := &http.Request{Body: body}

		var target map[string]string
		err := decodeJSON(r, &target)

		assert.NoError(t, err)
		assert.Equal(t, "test", target["name"])
	})

	t.Run("invalid json", func(t *testing.T) {
		body := io.NopCloser(bytes.NewBufferString(`{invalid json}`))
		r := &http.Request{Body: body}

		var target map[string]string
		err := decodeJSON(r, &target)

		assert.Error(t, err)
	})
}

func TestDecodeAndValidate(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		reqBody := `{"email": "test@example.com", "role": "admin"}`
		body := io.NopCloser(bytes.NewBufferString(reqBody))
		r := httptest.NewRequest("POST", "/", body)

		var req InviteRequest
		err := decodeAndValidate(r, &req)

		assert.NoError(t, err)
		assert.Equal(t, "test@example.com", req.Email)
		assert.Equal(t, "admin", req.Role)
	})

	t.Run("invalid json", func(t *testing.T) {
		body := io.NopCloser(bytes.NewBufferString(`{invalid}`))
		r := httptest.NewRequest("POST", "/", body)

		var req InviteRequest
		err := decodeAndValidate(r, &req)

		assert.Error(t, err)
		appErr, ok := err.(*apierror.AppError)
		assert.True(t, ok)
		assert.Equal(t, "bad_request", appErr.Code)
	})

	t.Run("validation failure - missing required field", func(t *testing.T) {
		reqBody := `{"role": "admin"}`
		body := io.NopCloser(bytes.NewBufferString(reqBody))
		r := httptest.NewRequest("POST", "/", body)

		var req InviteRequest
		err := decodeAndValidate(r, &req)

		assert.Error(t, err)
		appErr, ok := err.(*apierror.AppError)
		assert.True(t, ok)
		assert.Equal(t, "validation_error", appErr.Code)
	})

	t.Run("validation failure - invalid email", func(t *testing.T) {
		reqBody := `{"email": "notanemail", "role": "admin"}`
		body := io.NopCloser(bytes.NewBufferString(reqBody))
		r := httptest.NewRequest("POST", "/", body)

		var req InviteRequest
		err := decodeAndValidate(r, &req)

		assert.Error(t, err)
		appErr, ok := err.(*apierror.AppError)
		assert.True(t, ok)
		assert.Equal(t, "validation_error", appErr.Code)
	})
}

func TestParseUUID(t *testing.T) {
	t.Run("valid uuid", func(t *testing.T) {
		validUUID := "550e8400-e29b-41d4-a716-446655440000"
		id, err := parseUUID(validUUID)

		assert.NoError(t, err)
		assert.Equal(t, validUUID, id.String())
	})

	t.Run("invalid uuid", func(t *testing.T) {
		invalidUUID := "not-a-valid-uuid"
		_, err := parseUUID(invalidUUID)

		assert.Error(t, err)
		appErr, ok := err.(*apierror.AppError)
		assert.True(t, ok)
		assert.Equal(t, "bad_request", appErr.Code)
	})

	t.Run("empty uuid", func(t *testing.T) {
		_, err := parseUUID("")

		assert.Error(t, err)
		appErr, ok := err.(*apierror.AppError)
		assert.True(t, ok)
		assert.Equal(t, "bad_request", appErr.Code)
	})
}

func TestParseUUIDParam(t *testing.T) {
	t.Run("valid uuid param", func(t *testing.T) {
		validUUID := "550e8400-e29b-41d4-a716-446655440000"
		id, err := parseUUIDParam(validUUID)

		assert.NoError(t, err)
		assert.Equal(t, validUUID, id.String())
	})

	t.Run("missing uuid param", func(t *testing.T) {
		_, err := parseUUIDParam("")

		assert.Error(t, err)
		appErr, ok := err.(*apierror.AppError)
		assert.True(t, ok)
		assert.Equal(t, "bad_request", appErr.Code)
		assert.Contains(t, appErr.Message, "missing id parameter")
	})

	t.Run("invalid uuid param", func(t *testing.T) {
		_, err := parseUUIDParam("invalid")

		assert.Error(t, err)
		appErr, ok := err.(*apierror.AppError)
		assert.True(t, ok)
		assert.Equal(t, "bad_request", appErr.Code)
	})
}
