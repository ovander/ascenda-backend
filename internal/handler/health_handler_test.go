package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestHealthCheck(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	handler := NewHealthHandler(nil, logger)

	r := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.Check(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, "ok", result["status"])
}
