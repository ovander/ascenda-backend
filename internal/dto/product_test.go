package dto

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ascenda/internal/model"
)

// TestProductFromModel_Fields pins the product's API shape. The five cost
// variability fields were removed (migration 000018): nothing computed with
// them, so the API must not offer them again by accident.
func TestProductFromModel_Fields(t *testing.T) {
	p := model.Product{
		TenantScoped: model.TenantScoped{ID: uuid.New()},
		ScenarioID:   uuid.New(),
		Name:         "Consulting",
		ProductType:  model.ProductTypeService,
		SortOrder:    2,
		DriverType:   model.DriverConsulting,
		DriverParams: json.RawMessage(`{"workingDays":220}`),
	}
	raw, err := json.Marshal(ProductFromModel(p))
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	assert.ElementsMatch(t, []string{
		"id", "scenarioId", "name", "productType", "sortOrder",
		"driverType", "driverParams", "createdAt", "updatedAt",
	}, keys)
	assert.Equal(t, "Consulting", got["name"])
	assert.Equal(t, "consulting", got["driverType"])
}
