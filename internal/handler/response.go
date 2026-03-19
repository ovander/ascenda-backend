package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/go-playground/validator/v10"
	"kerplan/internal/pkg/apierror"
)

var validate = validator.New()

// respondJSON writes a JSON response with the given status code.
func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// respondNoContent returns a 204 No Content response.
func respondNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// handleError checks if an error is an AppError and writes appropriate JSON response.
func handleError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apierror.AppError); ok {
		appErr.WriteJSON(w)
		return
	}

	// Fallback for non-AppError errors
	internalErr := apierror.Internal(err.Error())
	internalErr.WriteJSON(w)
}

// decodeJSON reads and unmarshals the request body into a target struct.
func decodeJSON(r *http.Request, target interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

// decodeAndValidate reads, unmarshals, and validates the request body.
func decodeAndValidate(r *http.Request, target interface{}) error {
	if err := decodeJSON(r, target); err != nil {
		return apierror.BadRequest("invalid request body: " + err.Error())
	}

	if err := validate.Struct(target); err != nil {
		return apierror.ValidationError("validation failed", err.Error())
	}

	return nil
}

// parseUUID parses a string into a UUID.
func parseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, apierror.BadRequest("invalid uuid: " + s)
	}
	return id, nil
}

// parseUUIDParam extracts and parses a UUID from URL parameters.
// This is typically used with chi route parameters like chi.URLParam(r, "id").
func parseUUIDParam(param string) (uuid.UUID, error) {
	if param == "" {
		return uuid.Nil, apierror.BadRequest("missing id parameter")
	}
	return parseUUID(param)
}
