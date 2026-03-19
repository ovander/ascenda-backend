package apierror

import (
	"encoding/json"
	"net/http"
)

// AppError is the standard error type returned by services.
type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
	Details    any    `json:"details,omitempty"`
}

func (e *AppError) Error() string { return e.Message }

// ErrorResponse wraps AppError for JSON API responses.
type ErrorResponse struct {
	Error AppError `json:"error"`
}

// --- Constructors ---

func NotFound(entity, id string) *AppError {
	return &AppError{
		Code:       "not_found",
		StatusCode: http.StatusNotFound,
		Message:    entity + " not found: " + id,
	}
}

func ValidationError(msg string, details any) *AppError {
	return &AppError{
		Code:       "validation_error",
		StatusCode: http.StatusUnprocessableEntity,
		Message:    msg,
		Details:    details,
	}
}

func BadRequest(msg string) *AppError {
	return &AppError{
		Code:       "bad_request",
		StatusCode: http.StatusBadRequest,
		Message:    msg,
	}
}

func Unauthorized(msg string) *AppError {
	return &AppError{
		Code:       "unauthorized",
		StatusCode: http.StatusUnauthorized,
		Message:    msg,
	}
}

func Forbidden(msg string) *AppError {
	return &AppError{
		Code:       "forbidden",
		StatusCode: http.StatusForbidden,
		Message:    msg,
	}
}

func Conflict(msg string) *AppError {
	return &AppError{
		Code:       "conflict",
		StatusCode: http.StatusConflict,
		Message:    msg,
	}
}

func Internal(msg string) *AppError {
	return &AppError{
		Code:       "internal_error",
		StatusCode: http.StatusInternalServerError,
		Message:    msg,
	}
}

// WriteJSON writes an AppError as a JSON response.
func (e *AppError) WriteJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.StatusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: *e})
}
