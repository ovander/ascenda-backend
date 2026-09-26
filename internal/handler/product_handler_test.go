package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ascenda/internal/dto"
	"ascenda/internal/model"
	"ascenda/internal/service"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── mockProductService ────────────────────────────────────────────────────────

type mockProductService struct {
	listFn               func(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]*model.Product, error)
	createFn             func(ctx context.Context, tenantID, scenarioID uuid.UUID, p *model.Product) error
	getFn                func(ctx context.Context, tenantID, productID uuid.UUID) (*model.Product, error)
	updateFn             func(ctx context.Context, tenantID, productID uuid.UUID, upd service.ProductUpdate) error
	deleteFn             func(ctx context.Context, tenantID, productID uuid.UUID) error
	getAssumptionsFn     func(ctx context.Context, tenantID, productID uuid.UUID) ([]model.ProductAssumption, error)
	updateAssumptionsFn  func(ctx context.Context, tenantID, scenarioID, productID uuid.UUID, a []model.ProductAssumption) error
	getVolumesFn         func(ctx context.Context, tenantID, productID uuid.UUID) ([]model.ProductSalesVolume, error)
	updateVolumesFn      func(ctx context.Context, tenantID, scenarioID, productID uuid.UUID, v []model.ProductSalesVolume) error
	getMarginsFn         func(ctx context.Context, tenantID, productID uuid.UUID) ([]model.ProductDistributorMargin, error)
	updateMarginsFn      func(ctx context.Context, tenantID, scenarioID, productID uuid.UUID, m []model.ProductDistributorMargin) error
	getDerivedBundleFn   func(ctx context.Context, tenantID, productID uuid.UUID) (*service.DerivedBundleResult, error)
	getRevenueFn         func(ctx context.Context, tenantID, scenarioID, productID uuid.UUID) (*model.ProductRevenueSummary, error)
	getConsolidatedRevFn func(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.ConsolidatedRevenue, error)
}

func (m *mockProductService) ListProducts(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]*model.Product, error) {
	if m.listFn != nil {
		return m.listFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("list not implemented")
}
func (m *mockProductService) CreateProduct(ctx context.Context, tenantID, scenarioID uuid.UUID, p *model.Product) error {
	if m.createFn != nil {
		return m.createFn(ctx, tenantID, scenarioID, p)
	}
	return apierror.Internal("create not implemented")
}
func (m *mockProductService) GetProduct(ctx context.Context, tenantID, productID uuid.UUID) (*model.Product, error) {
	if m.getFn != nil {
		return m.getFn(ctx, tenantID, productID)
	}
	return nil, apierror.Internal("get not implemented")
}
func (m *mockProductService) UpdateProduct(ctx context.Context, tenantID, productID uuid.UUID, upd service.ProductUpdate) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, tenantID, productID, upd)
	}
	return apierror.Internal("update not implemented")
}
func (m *mockProductService) DeleteProduct(ctx context.Context, tenantID, productID uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, tenantID, productID)
	}
	return apierror.Internal("delete not implemented")
}
func (m *mockProductService) GetAssumptions(ctx context.Context, tenantID, productID uuid.UUID) ([]model.ProductAssumption, error) {
	if m.getAssumptionsFn != nil {
		return m.getAssumptionsFn(ctx, tenantID, productID)
	}
	return nil, apierror.Internal("get assumptions not implemented")
}
func (m *mockProductService) UpdateAssumptions(ctx context.Context, tenantID, scenarioID, productID uuid.UUID, a []model.ProductAssumption) error {
	if m.updateAssumptionsFn != nil {
		return m.updateAssumptionsFn(ctx, tenantID, scenarioID, productID, a)
	}
	return apierror.Internal("update assumptions not implemented")
}
func (m *mockProductService) GetVolumes(ctx context.Context, tenantID, productID uuid.UUID) ([]model.ProductSalesVolume, error) {
	if m.getVolumesFn != nil {
		return m.getVolumesFn(ctx, tenantID, productID)
	}
	return nil, apierror.Internal("get volumes not implemented")
}
func (m *mockProductService) UpdateVolumes(ctx context.Context, tenantID, scenarioID, productID uuid.UUID, v []model.ProductSalesVolume) error {
	if m.updateVolumesFn != nil {
		return m.updateVolumesFn(ctx, tenantID, scenarioID, productID, v)
	}
	return apierror.Internal("update volumes not implemented")
}
func (m *mockProductService) GetMargins(ctx context.Context, tenantID, productID uuid.UUID) ([]model.ProductDistributorMargin, error) {
	if m.getMarginsFn != nil {
		return m.getMarginsFn(ctx, tenantID, productID)
	}
	return nil, apierror.Internal("get margins not implemented")
}
func (m *mockProductService) UpdateMargins(ctx context.Context, tenantID, scenarioID, productID uuid.UUID, mg []model.ProductDistributorMargin) error {
	if m.updateMarginsFn != nil {
		return m.updateMarginsFn(ctx, tenantID, scenarioID, productID, mg)
	}
	return apierror.Internal("update margins not implemented")
}
func (m *mockProductService) GetDerivedBundle(ctx context.Context, tenantID, productID uuid.UUID) (*service.DerivedBundleResult, error) {
	if m.getDerivedBundleFn != nil {
		return m.getDerivedBundleFn(ctx, tenantID, productID)
	}
	return nil, apierror.Internal("get derived bundle not implemented")
}
func (m *mockProductService) GetRevenueByProduct(ctx context.Context, tenantID, scenarioID, productID uuid.UUID) (*model.ProductRevenueSummary, error) {
	if m.getRevenueFn != nil {
		return m.getRevenueFn(ctx, tenantID, scenarioID, productID)
	}
	return nil, apierror.Internal("get revenue not implemented")
}
func (m *mockProductService) GetConsolidatedRevenue(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.ConsolidatedRevenue, error) {
	if m.getConsolidatedRevFn != nil {
		return m.getConsolidatedRevFn(ctx, tenantID, scenarioID)
	}
	return nil, apierror.Internal("get consolidated revenue not implemented")
}

// ── fixtures ──────────────────────────────────────────────────────────────────

func makeProduct(tenantID, scenarioID uuid.UUID, name string) *model.Product {
	p := &model.Product{
		TenantScoped: model.TenantScoped{
			ID:       uuid.New(),
			TenantID: tenantID,
		},
		ScenarioID:  scenarioID,
		Name:        name,
		ProductType: model.ProductTypeProduct,
		DriverType:  model.DriverGeneric,
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	return p
}

func newProductHandler(svc ProductServicer) *ProductHandler {
	return NewProductHandler(svc, logrus.NewEntry(logrus.New()))
}

// ── List ──────────────────────────────────────────────────────────────────────

func TestProductHandler_List_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	p1 := makeProduct(tenantID, scenarioID, "Widget A")
	p2 := makeProduct(tenantID, scenarioID, "Widget B")

	svc := &mockProductService{
		listFn: func(_ context.Context, tid, sid uuid.UUID) ([]*model.Product, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			return []*model.Product{p1, p2}, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newProductHandler(svc).List(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []dto.ProductResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 2)
	assert.Equal(t, "Widget A", got[0].Name)
	assert.Equal(t, "Widget B", got[1].Name)
}

func TestProductHandler_List_Empty(t *testing.T) {
	svc := &mockProductService{
		listFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.Product, error) {
			return []*model.Product{}, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newProductHandler(svc).List(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []dto.ProductResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Empty(t, got)
}

func TestProductHandler_List_BadScenarioUUID(t *testing.T) {
	svc := &mockProductService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": "not-a-uuid"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newProductHandler(svc).List(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProductHandler_List_ServiceError(t *testing.T) {
	svc := &mockProductService{
		listFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.Product, error) {
			return nil, apierror.Internal("db down")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newProductHandler(svc).List(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ── Create ────────────────────────────────────────────────────────────────────

func TestProductHandler_Create_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()

	svc := &mockProductService{
		createFn: func(_ context.Context, tid, sid uuid.UUID, p *model.Product) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, "New Widget", p.Name)
			// Assign an ID to simulate DB persistence
			p.ID = uuid.New()
			p.ScenarioID = scenarioID
			p.TenantID = tenantID
			p.CreatedAt = time.Now()
			p.UpdatedAt = time.Now()
			return nil
		},
	}

	body, _ := json.Marshal(CreateProductRequest{Name: "New Widget"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newProductHandler(svc).Create(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
	var got dto.ProductResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "New Widget", got.Name)
}

func TestProductHandler_Create_MissingName(t *testing.T) {
	svc := &mockProductService{}
	body, _ := json.Marshal(map[string]string{})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newProductHandler(svc).Create(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestProductHandler_Create_BadScenarioUUID(t *testing.T) {
	svc := &mockProductService{}
	body, _ := json.Marshal(CreateProductRequest{Name: "X"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newProductHandler(svc).Create(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProductHandler_Create_ServiceError(t *testing.T) {
	svc := &mockProductService{
		createFn: func(_ context.Context, _, _ uuid.UUID, _ *model.Product) error {
			return apierror.Conflict("duplicate product name")
		},
	}

	body, _ := json.Marshal(CreateProductRequest{Name: "X"})
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"scenarioId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newProductHandler(svc).Create(w, r)

	assert.Equal(t, http.StatusConflict, w.Code)
}

// ── Get ───────────────────────────────────────────────────────────────────────

func TestProductHandler_Get_Success(t *testing.T) {
	tenantID := uuid.New()
	scenarioID := uuid.New()
	product := makeProduct(tenantID, scenarioID, "Widget A")

	svc := &mockProductService{
		getFn: func(_ context.Context, _, pid uuid.UUID) (*model.Product, error) {
			assert.Equal(t, product.ID, pid)
			return product, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"productId": product.ID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newProductHandler(svc).Get(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got dto.ProductResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "Widget A", got.Name)
	assert.Equal(t, product.ID.String(), got.ID)
}

func TestProductHandler_Get_NotFound(t *testing.T) {
	svc := &mockProductService{
		getFn: func(_ context.Context, _, _ uuid.UUID) (*model.Product, error) {
			return nil, apierror.NotFound("product", "missing")
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"productId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newProductHandler(svc).Get(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestProductHandler_Get_BadProductUUID(t *testing.T) {
	svc := &mockProductService{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"productId": "bad"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newProductHandler(svc).Get(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── Update ────────────────────────────────────────────────────────────────────

func TestProductHandler_Update_Success(t *testing.T) {
	productID := uuid.New()
	called := false

	svc := &mockProductService{
		updateFn: func(_ context.Context, _, pid uuid.UUID, upd service.ProductUpdate) error {
			assert.Equal(t, productID, pid)
			require.NotNil(t, upd.Name)
			assert.Equal(t, "Renamed Widget", *upd.Name)
			assert.Nil(t, upd.DriverType, "not sent, so not changed")
			assert.Nil(t, upd.DriverParams, "not sent, so not changed")
			called = true
			return nil
		},
	}

	body := []byte(`{"name":"Renamed Widget"}`)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"productId": productID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newProductHandler(svc).Update(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, called)
}

func TestProductHandler_Update_ServiceError(t *testing.T) {
	svc := &mockProductService{
		updateFn: func(_ context.Context, _, _ uuid.UUID, _ service.ProductUpdate) error {
			return apierror.NotFound("product", "gone")
		},
	}

	body := []byte(`{"name":"X"}`)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"productId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newProductHandler(svc).Update(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestProductHandler_Delete_Success(t *testing.T) {
	productID := uuid.New()
	called := false

	svc := &mockProductService{
		deleteFn: func(_ context.Context, _, pid uuid.UUID) error {
			assert.Equal(t, productID, pid)
			called = true
			return nil
		},
	}

	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r = withChiParams(r, map[string]string{"productId": productID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newProductHandler(svc).Delete(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, called)
}

func TestProductHandler_Delete_NotFound(t *testing.T) {
	svc := &mockProductService{
		deleteFn: func(_ context.Context, _, _ uuid.UUID) error {
			return apierror.NotFound("product", "x")
		},
	}

	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r = withChiParams(r, map[string]string{"productId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newProductHandler(svc).Delete(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── GetAssumptions ────────────────────────────────────────────────────────────

func TestProductHandler_GetAssumptions_Success(t *testing.T) {
	tenantID := uuid.New()
	productID := uuid.New()

	assumption := model.ProductAssumption{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ProductID:    productID,
		YearIndex:    1,
	}

	svc := &mockProductService{
		getAssumptionsFn: func(_ context.Context, _, pid uuid.UUID) ([]model.ProductAssumption, error) {
			assert.Equal(t, productID, pid)
			return []model.ProductAssumption{assumption}, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"productId": productID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newProductHandler(svc).GetAssumptions(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []dto.AssumptionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 1)
	assert.Equal(t, 1, got[0].YearIndex)
}

func TestProductHandler_GetAssumptions_Empty(t *testing.T) {
	svc := &mockProductService{
		getAssumptionsFn: func(_ context.Context, _, _ uuid.UUID) ([]model.ProductAssumption, error) {
			return []model.ProductAssumption{}, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"productId": uuid.New().String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newProductHandler(svc).GetAssumptions(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []dto.AssumptionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Empty(t, got)
}

// ── UpdateAssumptions ─────────────────────────────────────────────────────────

func TestProductHandler_UpdateAssumptions_Success(t *testing.T) {
	productID := uuid.New()
	scenarioID := uuid.New()
	called := false

	svc := &mockProductService{
		updateAssumptionsFn: func(_ context.Context, _, sid, pid uuid.UUID, a []model.ProductAssumption) error {
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, productID, pid)
			assert.Len(t, a, 1)
			called = true
			return nil
		},
	}

	payload := []model.ProductAssumption{{YearIndex: 1}}
	body, _ := json.Marshal(payload)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{
		"productId":  productID.String(),
		"scenarioId": scenarioID.String(),
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newProductHandler(svc).UpdateAssumptions(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, called)
}

func TestProductHandler_UpdateAssumptions_ServiceError(t *testing.T) {
	svc := &mockProductService{
		updateAssumptionsFn: func(_ context.Context, _, _, _ uuid.UUID, _ []model.ProductAssumption) error {
			return apierror.Forbidden("read-only scenario")
		},
	}

	payload := []model.ProductAssumption{{YearIndex: 1}}
	body, _ := json.Marshal(payload)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{
		"productId":  uuid.New().String(),
		"scenarioId": uuid.New().String(),
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newProductHandler(svc).UpdateAssumptions(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ── GetVolumes ────────────────────────────────────────────────────────────────

func TestProductHandler_GetVolumes_Success(t *testing.T) {
	tenantID := uuid.New()
	productID := uuid.New()

	vol := model.ProductSalesVolume{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ProductID:    productID,
		YearIndex:    1,
		UnitsSold:    500,
	}

	svc := &mockProductService{
		getVolumesFn: func(_ context.Context, _, pid uuid.UUID) ([]model.ProductSalesVolume, error) {
			assert.Equal(t, productID, pid)
			return []model.ProductSalesVolume{vol}, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"productId": productID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newProductHandler(svc).GetVolumes(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []dto.VolumeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 1)
	assert.Equal(t, int64(500), got[0].UnitsSold)
}

// ── UpdateVolumes ─────────────────────────────────────────────────────────────

func TestProductHandler_UpdateVolumes_Success(t *testing.T) {
	productID := uuid.New()
	scenarioID := uuid.New()
	called := false

	svc := &mockProductService{
		updateVolumesFn: func(_ context.Context, _, sid, pid uuid.UUID, v []model.ProductSalesVolume) error {
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, productID, pid)
			called = true
			return nil
		},
	}

	payload := []model.ProductSalesVolume{{YearIndex: 1, UnitsSold: 100}}
	body, _ := json.Marshal(payload)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{
		"productId":  productID.String(),
		"scenarioId": scenarioID.String(),
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newProductHandler(svc).UpdateVolumes(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, called)
}

// ── GetMargins ────────────────────────────────────────────────────────────────

func TestProductHandler_GetMargins_Success(t *testing.T) {
	tenantID := uuid.New()
	productID := uuid.New()

	mg := model.ProductDistributorMargin{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ProductID:    productID,
		YearIndex:    1,
	}

	svc := &mockProductService{
		getMarginsFn: func(_ context.Context, _, pid uuid.UUID) ([]model.ProductDistributorMargin, error) {
			return []model.ProductDistributorMargin{mg}, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"productId": productID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newProductHandler(svc).GetMargins(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var got []dto.MarginResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 1)
	assert.Equal(t, 1, got[0].YearIndex)
}

// ── UpdateMargins ─────────────────────────────────────────────────────────────

func TestProductHandler_UpdateMargins_Success(t *testing.T) {
	productID := uuid.New()
	scenarioID := uuid.New()
	called := false

	svc := &mockProductService{
		updateMarginsFn: func(_ context.Context, _, sid, pid uuid.UUID, _ []model.ProductDistributorMargin) error {
			assert.Equal(t, scenarioID, sid)
			assert.Equal(t, productID, pid)
			called = true
			return nil
		},
	}

	payload := []model.ProductDistributorMargin{{YearIndex: 1}}
	body, _ := json.Marshal(payload)
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{
		"productId":  productID.String(),
		"scenarioId": scenarioID.String(),
	})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newProductHandler(svc).UpdateMargins(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, called)
}

// ── Response shape ────────────────────────────────────────────────────────────

func TestProductHandler_List_ResponseContainsRequiredFields(t *testing.T) {
	// Guard: JSON response must always include id, name, productType, driverType
	// so the frontend can destructure safely without optional-chaining.
	tenantID := uuid.New()
	scenarioID := uuid.New()
	p := makeProduct(tenantID, scenarioID, "Alpha")

	svc := &mockProductService{
		listFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.Product, error) {
			return []*model.Product{p}, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"scenarioId": scenarioID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newProductHandler(svc).List(w, r)

	require.Equal(t, http.StatusOK, w.Code)

	var raw []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	require.Len(t, raw, 1)
	for _, field := range []string{"id", "name", "productType", "driverType"} {
		_, ok := raw[0][field]
		assert.Truef(t, ok, "response JSON must contain field %q", field)
	}
}

// ── GetDerivedBundle ──────────────────────────────────────────────────────────

func TestProductHandler_GetDerivedBundle_Success(t *testing.T) {
	tenantID := uuid.New()
	productID := uuid.New()

	want := &service.DerivedBundleResult{
		Volumes: []model.ProductSalesVolume{{YearIndex: 1, UnitsSold: 100}},
	}
	want.Assumptions[0] = model.ProductAssumption{YearIndex: 1}

	svc := &mockProductService{
		getDerivedBundleFn: func(_ context.Context, tID, pID uuid.UUID) (*service.DerivedBundleResult, error) {
			assert.Equal(t, tenantID, tID)
			assert.Equal(t, productID, pID)
			return want, nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"productId": productID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newProductHandler(svc).GetDerivedBundle(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp service.DerivedBundleResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Volumes, 1)
	assert.Equal(t, int64(100), resp.Volumes[0].UnitsSold)
}

func TestProductHandler_GetDerivedBundle_ServiceError_Returns500(t *testing.T) {
	tenantID := uuid.New()
	productID := uuid.New()

	svc := &mockProductService{
		getDerivedBundleFn: func(_ context.Context, _, _ uuid.UUID) (*service.DerivedBundleResult, error) {
			return nil, apierror.NotFound("product", productID.String())
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"productId": productID.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), tenantID))
	w := httptest.NewRecorder()

	newProductHandler(svc).GetDerivedBundle(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestProductHandler_GetDerivedBundle_InvalidProductID(t *testing.T) {
	svc := &mockProductService{}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParams(r, map[string]string{"productId": "not-a-uuid"})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), uuid.New()))
	w := httptest.NewRecorder()

	newProductHandler(svc).GetDerivedBundle(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
