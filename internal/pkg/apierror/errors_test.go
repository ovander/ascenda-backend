package apierror

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotFound(t *testing.T) {
	err := NotFound("User", "user-123")
	assert.Equal(t, "not_found", err.Code)
	assert.Equal(t, http.StatusNotFound, err.StatusCode)
	assert.Equal(t, "User not found: user-123", err.Message)
}

func TestValidationError(t *testing.T) {
	details := map[string]string{"field": "email", "reason": "invalid format"}
	err := ValidationError("Validation failed", details)

	assert.Equal(t, "validation_error", err.Code)
	assert.Equal(t, http.StatusUnprocessableEntity, err.StatusCode)
	assert.Equal(t, "Validation failed", err.Message)
	assert.Equal(t, details, err.Details)
}

func TestBadRequest(t *testing.T) {
	err := BadRequest("Missing required field")
	assert.Equal(t, "bad_request", err.Code)
	assert.Equal(t, http.StatusBadRequest, err.StatusCode)
	assert.Equal(t, "Missing required field", err.Message)
}

func TestUnauthorized(t *testing.T) {
	err := Unauthorized("Invalid token")
	assert.Equal(t, "unauthorized", err.Code)
	assert.Equal(t, http.StatusUnauthorized, err.StatusCode)
	assert.Equal(t, "Invalid token", err.Message)
}

func TestForbidden(t *testing.T) {
	err := Forbidden("Insufficient permissions")
	assert.Equal(t, "forbidden", err.Code)
	assert.Equal(t, http.StatusForbidden, err.StatusCode)
	assert.Equal(t, "Insufficient permissions", err.Message)
}

func TestConflict(t *testing.T) {
	err := Conflict("Resource already exists")
	assert.Equal(t, "conflict", err.Code)
	assert.Equal(t, http.StatusConflict, err.StatusCode)
	assert.Equal(t, "Resource already exists", err.Message)
}

func TestInternal(t *testing.T) {
	err := Internal("Database connection failed")
	assert.Equal(t, "internal_error", err.Code)
	assert.Equal(t, http.StatusInternalServerError, err.StatusCode)
	assert.Equal(t, "Database connection failed", err.Message)
}

func TestAppErrorError(t *testing.T) {
	err := BadRequest("Test error")
	assert.Equal(t, "Test error", err.Error())
}

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name       string
		err        *AppError
		statusCode int
		hasDetails bool
	}{
		{
			name:       "not found error",
			err:        NotFound("Plan", "plan-456"),
			statusCode: http.StatusNotFound,
			hasDetails: false,
		},
		{
			name: "validation error with details",
			err: ValidationError("Invalid input", map[string]string{
				"field": "email",
				"error": "invalid format",
			}),
			statusCode: http.StatusUnprocessableEntity,
			hasDetails: true,
		},
		{
			name:       "bad request",
			err:        BadRequest("Invalid request body"),
			statusCode: http.StatusBadRequest,
			hasDetails: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tt.err.WriteJSON(w)

			assert.Equal(t, tt.statusCode, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			assert.NoError(t, err)

			assert.Equal(t, tt.err.Code, response.Error.Code)
			assert.Equal(t, tt.err.Message, response.Error.Message)

			if tt.hasDetails {
				assert.NotNil(t, response.Error.Details)
			}
		})
	}
}

func TestWriteJSONWithoutDetails(t *testing.T) {
	err := BadRequest("Test message")
	w := httptest.NewRecorder()

	err.WriteJSON(w)

	var response ErrorResponse
	json.NewDecoder(w.Body).Decode(&response)

	assert.Equal(t, "bad_request", response.Error.Code)
	assert.Equal(t, "Test message", response.Error.Message)
	assert.Nil(t, response.Error.Details)
}

func TestWriteJSONResponseStructure(t *testing.T) {
	err := ValidationError("Test validation", map[string]interface{}{
		"field1": "value1",
		"field2": 123,
	})

	w := httptest.NewRecorder()
	err.WriteJSON(w)

	body := w.Body.String()
	assert.Contains(t, body, "error")
	assert.Contains(t, body, "code")
	assert.Contains(t, body, "message")
	assert.Contains(t, body, "validation_error")
	assert.Contains(t, body, "Test validation")
}

func TestWriteJSONContentType(t *testing.T) {
	err := Internal("Server error")
	w := httptest.NewRecorder()

	err.WriteJSON(w)

	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestWriteJSONHTTPStatusCodes(t *testing.T) {
	tests := []struct {
		errFunc    func() *AppError
		expectedCode int
	}{
		{func() *AppError { return NotFound("x", "y") }, http.StatusNotFound},
		{func() *AppError { return BadRequest("msg") }, http.StatusBadRequest},
		{func() *AppError { return Unauthorized("msg") }, http.StatusUnauthorized},
		{func() *AppError { return Forbidden("msg") }, http.StatusForbidden},
		{func() *AppError { return Conflict("msg") }, http.StatusConflict},
		{func() *AppError { return Internal("msg") }, http.StatusInternalServerError},
		{func() *AppError { return ValidationError("msg", nil) }, http.StatusUnprocessableEntity},
	}

	for _, tt := range tests {
		w := httptest.NewRecorder()
		err := tt.errFunc()
		err.WriteJSON(w)

		assert.Equal(t, tt.expectedCode, w.Code)
	}
}

func TestWriteJSONValid(t *testing.T) {
	err := BadRequest("Invalid parameter")
	w := httptest.NewRecorder()
	err.WriteJSON(w)

	var result map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(w.Body.Bytes()))
	assert.NoError(t, decoder.Decode(&result))

	assert.Contains(t, result, "error")
}
