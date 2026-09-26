package service

import (
	"context"
	"testing"

	"ascenda/internal/event"
	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestProductService wires up a ProductService backed by the in-memory mock.
func newTestProductService() (*ProductService, *MockProductRepo) {
	repo := NewMockProductRepo()
	logger := logrus.NewEntry(logrus.New())
	emitter := event.NewEmitter(logger)
	svc := NewProductService(repo, nil, emitter, logger)
	return svc, repo
}

// seedProduct inserts a product and returns it.
func seedProduct(repo *MockProductRepo, tenantID, scenarioID uuid.UUID, name string) *model.Product {
	p := &model.Product{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:   scenarioID,
		Name:         name,
		DriverType:   model.DriverGeneric,
	}
	_ = repo.CreateProduct(p)
	return p
}

// ── DeleteProduct ─────────────────────────────────────────────────────────────

func TestDeleteProduct_RemovesProductFromRepo(t *testing.T) {
	svc, repo := newTestProductService()
	tenantID := uuid.New()
	scenarioID := uuid.New()
	ctx := context.Background()

	p := seedProduct(repo, tenantID, scenarioID, "Widget")

	err := svc.DeleteProduct(ctx, tenantID, p.ID)
	require.NoError(t, err)

	got, _ := repo.GetByID(tenantID, p.ID)
	assert.Nil(t, got, "product should be removed from the repo")
}

func TestDeleteProduct_CascadesAssumptions(t *testing.T) {
	svc, repo := newTestProductService()
	tenantID := uuid.New()
	scenarioID := uuid.New()
	ctx := context.Background()

	p := seedProduct(repo, tenantID, scenarioID, "Widget")

	// Seed child rows
	_ = repo.BatchUpsertAssumptions(tenantID, scenarioID, []model.ProductAssumption{
		{TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID}, ProductID: p.ID, YearIndex: 1},
		{TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID}, ProductID: p.ID, YearIndex: 2},
	})

	// Pre-condition: child rows exist
	before, _ := repo.GetAssumptionsByProduct(tenantID, p.ID)
	assert.Len(t, before, 2)

	err := svc.DeleteProduct(ctx, tenantID, p.ID)
	require.NoError(t, err)

	after, _ := repo.GetAssumptionsByProduct(tenantID, p.ID)
	assert.Empty(t, after, "assumptions should be removed when product is deleted")
}

func TestDeleteProduct_CascadesVolumes(t *testing.T) {
	svc, repo := newTestProductService()
	tenantID := uuid.New()
	scenarioID := uuid.New()
	ctx := context.Background()

	p := seedProduct(repo, tenantID, scenarioID, "Widget")

	_ = repo.BatchUpsertVolumes(tenantID, scenarioID, []model.ProductSalesVolume{
		{TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID}, ProductID: p.ID, YearIndex: 1, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 100},
		{TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID}, ProductID: p.ID, YearIndex: 1, Zone: model.ZoneEurope, Channel: model.ChannelIndirect, UnitsSold: 50},
	})

	before, _ := repo.GetVolumesByProduct(tenantID, p.ID)
	assert.Len(t, before, 2)

	require.NoError(t, svc.DeleteProduct(ctx, tenantID, p.ID))

	after, _ := repo.GetVolumesByProduct(tenantID, p.ID)
	assert.Empty(t, after, "sales volumes should be removed when product is deleted")
}

func TestDeleteProduct_CascadesMargins(t *testing.T) {
	svc, repo := newTestProductService()
	tenantID := uuid.New()
	scenarioID := uuid.New()
	ctx := context.Background()

	p := seedProduct(repo, tenantID, scenarioID, "Widget")

	_ = repo.BatchUpsertMargins(tenantID, scenarioID, []model.ProductDistributorMargin{
		{TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID}, ProductID: p.ID, YearIndex: 1, Zone: model.ZoneEurope, MarginPercent: decimal.NewFromFloat(0.20)},
	})

	before, _ := repo.GetMarginsByProduct(tenantID, p.ID)
	assert.Len(t, before, 1)

	require.NoError(t, svc.DeleteProduct(ctx, tenantID, p.ID))

	after, _ := repo.GetMarginsByProduct(tenantID, p.ID)
	assert.Empty(t, after, "distributor margins should be removed when product is deleted")
}

func TestDeleteProduct_DoesNotAffectSiblingProductChildren(t *testing.T) {
	svc, repo := newTestProductService()
	tenantID := uuid.New()
	scenarioID := uuid.New()
	ctx := context.Background()

	// Two products in the same scenario
	p1 := seedProduct(repo, tenantID, scenarioID, "Widget")
	p2 := seedProduct(repo, tenantID, scenarioID, "Gadget")

	// Each has a volume row
	_ = repo.BatchUpsertVolumes(tenantID, scenarioID, []model.ProductSalesVolume{
		{TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID}, ProductID: p1.ID, YearIndex: 1, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 100},
	})
	_ = repo.BatchUpsertVolumes(tenantID, scenarioID, []model.ProductSalesVolume{
		{TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID}, ProductID: p2.ID, YearIndex: 1, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 200},
	})

	// Delete only p1
	require.NoError(t, svc.DeleteProduct(ctx, tenantID, p1.ID))

	// p2's volume should be untouched
	p2Volumes, _ := repo.GetVolumesByProduct(tenantID, p2.ID)
	assert.Len(t, p2Volumes, 1, "sibling product volumes should not be affected")
	assert.Equal(t, int64(200), p2Volumes[0].UnitsSold)
}

func TestDeleteProduct_NotFound(t *testing.T) {
	svc, _ := newTestProductService()
	ctx := context.Background()

	err := svc.DeleteProduct(ctx, uuid.New(), uuid.New())
	assert.Error(t, err, "deleting a non-existent product should return an error")
}

// ── CreateProduct ─────────────────────────────────────────────────────────────

func TestCreateProduct_AssignsIDsAndPersists(t *testing.T) {
	svc, repo := newTestProductService()
	tenantID := uuid.New()
	scenarioID := uuid.New()
	ctx := context.Background()

	p := &model.Product{
		Name:       "SaaS",
		DriverType: model.DriverSaaS,
	}

	err := svc.CreateProduct(ctx, tenantID, scenarioID, p)
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, p.ID, "product ID must be assigned")
	assert.Equal(t, tenantID, p.TenantID)
	assert.Equal(t, scenarioID, p.ScenarioID)

	stored, _ := repo.GetByID(tenantID, p.ID)
	require.NotNil(t, stored)
	assert.Equal(t, "SaaS", stored.Name)
}

// ── ListProducts ──────────────────────────────────────────────────────────────

func TestListProducts_ReturnsOnlyMatchingScenario(t *testing.T) {
	svc, repo := newTestProductService()
	tenantID := uuid.New()
	s1 := uuid.New()
	s2 := uuid.New()
	ctx := context.Background()

	seedProduct(repo, tenantID, s1, "A")
	seedProduct(repo, tenantID, s1, "B")
	seedProduct(repo, tenantID, s2, "C")

	products, err := svc.ListProducts(ctx, tenantID, s1)
	require.NoError(t, err)
	assert.Len(t, products, 2)

	products2, _ := svc.ListProducts(ctx, tenantID, s2)
	assert.Len(t, products2, 1)
}

// ── GetDerivedBundle ──────────────────────────────────────────────────────────

func TestGetDerivedBundle_Generic_ReturnsStoredBundle(t *testing.T) {
	svc, repo := newTestProductService()
	tenantID := uuid.New()
	scenarioID := uuid.New()
	ctx := context.Background()

	p := seedProduct(repo, tenantID, scenarioID, "Widget")

	// Seed one volume row and one assumption row.
	_ = repo.BatchUpsertVolumes(tenantID, scenarioID, []model.ProductSalesVolume{
		{TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID}, ProductID: p.ID, YearIndex: 1, UnitsSold: 500},
	})
	_ = repo.BatchUpsertAssumptions(tenantID, scenarioID, []model.ProductAssumption{
		{TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID}, ProductID: p.ID, YearIndex: 1,
			BaseUnitPrice: decimal.NewFromFloat(10)},
	})

	result, err := svc.GetDerivedBundle(ctx, tenantID, p.ID)
	require.NoError(t, err)

	require.Len(t, result.Volumes, 1)
	assert.Equal(t, int64(500), result.Volumes[0].UnitsSold)
	// Assumptions is a [MaxYears] array; check the Y1 slot (index 0).
	assert.True(t, result.Assumptions[0].BaseUnitPrice.Equal(decimal.NewFromFloat(10)))
}

func TestGetDerivedBundle_DriverComputeError_FallsBackToRawBundle(t *testing.T) {
	// A product with a non-generic driver but malformed/null params should NOT
	// return 500; it should return the raw stored bundle (new behaviour after fix).
	svc, repo := newTestProductService()
	tenantID := uuid.New()
	scenarioID := uuid.New()
	ctx := context.Background()

	// Create a consulting product whose DriverParams is intentionally invalid JSON.
	p := &model.Product{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:   scenarioID,
		Name:         "Bad Consulting",
		DriverType:   model.DriverConsulting,
		DriverParams: []byte(`{"headcount": "not-a-number"}`), // malformed for ConsultingParams
	}
	_ = repo.CreateProduct(p)

	// Seed a volume so the raw bundle is non-empty.
	_ = repo.BatchUpsertVolumes(tenantID, scenarioID, []model.ProductSalesVolume{
		{TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID}, ProductID: p.ID, YearIndex: 1, UnitsSold: 42},
	})

	result, err := svc.GetDerivedBundle(ctx, tenantID, p.ID)
	// Must not error — falls back to raw bundle.
	require.NoError(t, err)
	require.NotNil(t, result)
	// The raw volume row should be returned unchanged.
	require.Len(t, result.Volumes, 1)
	assert.Equal(t, int64(42), result.Volumes[0].UnitsSold)
}

func TestGetDerivedBundle_ProductNotFound_ReturnsError(t *testing.T) {
	svc, _ := newTestProductService()
	ctx := context.Background()

	_, err := svc.GetDerivedBundle(ctx, uuid.New(), uuid.New())
	require.Error(t, err)
}
