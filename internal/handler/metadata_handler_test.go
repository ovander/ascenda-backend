package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestGetVersion_Success(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	handler := NewMetadataHandler("1.2.3", "2026-03-17T10:00:00Z", "abc1234", logger)

	r := httptest.NewRequest("GET", "/api/v1/version", nil)
	w := httptest.NewRecorder()

	handler.GetVersion(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp VersionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "1.2.3", resp.Version)
	assert.Equal(t, "2026-03-17T10:00:00Z", resp.BuildTime)
	assert.Equal(t, "abc1234", resp.GitCommit)
	assert.Equal(t, runtime.Version(), resp.GoVersion)
}

func TestGetVersion_EmptyFields(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	handler := NewMetadataHandler("0.1.0", "", "", logger)

	r := httptest.NewRequest("GET", "/api/v1/version", nil)
	w := httptest.NewRecorder()

	handler.GetVersion(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp VersionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "0.1.0", resp.Version)
	assert.Empty(t, resp.BuildTime)
	assert.Empty(t, resp.GitCommit)
}

func TestGetMetadata_Success(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	handler := NewMetadataHandler("1.0.0", "", "", logger)

	r := httptest.NewRequest("GET", "/api/v1/metadata", nil)
	w := httptest.NewRecorder()

	handler.GetMetadata(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp MetadataResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	// Verify all enum lists are populated
	assert.NotEmpty(t, resp.AssetCategories, "assetCategories should not be empty")
	assert.NotEmpty(t, resp.StaffCategories, "staffCategories should not be empty")
	assert.NotEmpty(t, resp.StaffFunctions, "staffFunctions should not be empty")
	assert.NotEmpty(t, resp.OpexLines, "opexLines should not be empty")
	assert.NotEmpty(t, resp.OpexSubcategories, "opexSubcategories should not be empty")
	assert.NotEmpty(t, resp.PnlLines, "pnlLines should not be empty")
	assert.NotEmpty(t, resp.FiplanLines, "fiplanLines should not be empty")
	assert.NotEmpty(t, resp.WcrLines, "wcrLines should not be empty")
	assert.NotEmpty(t, resp.PnlCashLines, "pnlCashLines should not be empty")
	assert.NotEmpty(t, resp.CashLines, "cashLines should not be empty")
	assert.NotEmpty(t, resp.BudgetLines, "budgetLines should not be empty")
	assert.NotEmpty(t, resp.DistributionRules, "distributionRules should not be empty")
	assert.NotEmpty(t, resp.SalesChannels, "salesChannels should not be empty")
	assert.NotEmpty(t, resp.GeoZones, "geoZones should not be empty")
}

func TestGetMetadata_StaffFunctions(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	handler := NewMetadataHandler("1.0.0", "", "", logger)

	r := httptest.NewRequest("GET", "/api/v1/metadata", nil)
	w := httptest.NewRecorder()

	handler.GetMetadata(w, r)

	var resp MetadataResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	// Verify the 4 expected staff functions
	assert.Len(t, resp.StaffFunctions, 4)
	assert.Contains(t, resp.StaffFunctions, "rnd")
	assert.Contains(t, resp.StaffFunctions, "production")
	assert.Contains(t, resp.StaffFunctions, "sales")
	assert.Contains(t, resp.StaffFunctions, "gna")
}

func TestGetMetadata_SalesChannelsAndGeoZones(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	handler := NewMetadataHandler("1.0.0", "", "", logger)

	r := httptest.NewRequest("GET", "/api/v1/metadata", nil)
	w := httptest.NewRecorder()

	handler.GetMetadata(w, r)

	var resp MetadataResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	assert.Len(t, resp.SalesChannels, 2)
	assert.Contains(t, resp.SalesChannels, "direct")
	assert.Contains(t, resp.SalesChannels, "indirect")

	assert.Len(t, resp.GeoZones, 3)
	assert.Contains(t, resp.GeoZones, "france")
	assert.Contains(t, resp.GeoZones, "europe")
	assert.Contains(t, resp.GeoZones, "export")
}

func TestGetMetadata_DistributionRules(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	handler := NewMetadataHandler("1.0.0", "", "", logger)

	r := httptest.NewRequest("GET", "/api/v1/metadata", nil)
	w := httptest.NewRecorder()

	handler.GetMetadata(w, r)

	var resp MetadataResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	assert.Len(t, resp.DistributionRules, 4)
	assert.Contains(t, resp.DistributionRules, "even_spread")
	assert.Contains(t, resp.DistributionRules, "lump_m1")
	assert.Contains(t, resp.DistributionRules, "from_schedule")
	assert.Contains(t, resp.DistributionRules, "manual_only")
}

func TestGetMetadata_CachedResponsesAreIdentical(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	handler := NewMetadataHandler("1.0.0", "2026-01-01", "def5678", logger)

	// Call GetMetadata twice and verify responses are identical (cached)
	w1 := httptest.NewRecorder()
	handler.GetMetadata(w1, httptest.NewRequest("GET", "/api/v1/metadata", nil))

	w2 := httptest.NewRecorder()
	handler.GetMetadata(w2, httptest.NewRequest("GET", "/api/v1/metadata", nil))

	assert.Equal(t, w1.Body.String(), w2.Body.String())
}

func TestGetMetadata_AssetCategoryFields(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	handler := NewMetadataHandler("1.0.0", "", "", logger)

	r := httptest.NewRequest("GET", "/api/v1/metadata", nil)
	w := httptest.NewRecorder()

	handler.GetMetadata(w, r)

	var resp MetadataResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	// Verify asset categories have required fields
	for _, cat := range resp.AssetCategories {
		assert.NotEmpty(t, cat.ID, "asset category ID should not be empty")
	}
}

func TestGetMetadata_OpexLineFields(t *testing.T) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	handler := NewMetadataHandler("1.0.0", "", "", logger)

	r := httptest.NewRequest("GET", "/api/v1/metadata", nil)
	w := httptest.NewRecorder()

	handler.GetMetadata(w, r)

	var resp MetadataResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	// Verify opex lines have required fields
	for _, line := range resp.OpexLines {
		assert.NotEmpty(t, line.ID, "opex line ID should not be empty")
		assert.NotEmpty(t, line.Subcategory, "opex line subcategory should not be empty")
	}
}
