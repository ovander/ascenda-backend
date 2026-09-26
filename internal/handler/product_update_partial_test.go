package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ascenda/internal/event"
	"ascenda/internal/model"
	"ascenda/internal/service"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PUT /products/{id} is a partial update: the frontend sends only what it
// changes — { name } on a rename, { driverType, driverParams } when the
// driver configuration is saved. Fields left out of the body must keep
// their stored values. These tests run the real handler and product service
// over the in-memory repository, with the bodies the frontend sends.

type partialUpdateFixture struct {
	repo     *service.MockProductRepo
	handler  *ProductHandler
	tenantID uuid.UUID
	first    *model.Product
	second   *model.Product
}

const consultingParams = `{"headcount":["2","3","4","5","6"],"workingDays":220,"utilizationRate":["0.8","0.8","0.8","0.8","0.8"],"monthlyGross":["5000","5000","5000","5000","5000"],"employerCharges":"1.45"}`

func newPartialUpdateFixture(t *testing.T) *partialUpdateFixture {
	t.Helper()
	logger := logrus.NewEntry(logrus.New())
	repo := service.NewMockProductRepo()
	svc := service.NewProductService(repo, nil, event.NewEmitter(logger), logger)

	tenantID, scenarioID := uuid.New(), uuid.New()
	first := &model.Product{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:   scenarioID, Name: "Licences", ProductType: model.ProductTypeProduct,
		SortOrder: 0, DriverType: model.DriverGeneric,
	}
	second := &model.Product{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:   scenarioID, Name: "Consulting", ProductType: model.ProductTypeService,
		SortOrder: 1, DriverType: model.DriverConsulting, DriverParams: json.RawMessage(consultingParams),
	}
	require.NoError(t, repo.CreateProduct(first))
	require.NoError(t, repo.CreateProduct(second))
	return &partialUpdateFixture{repo: repo, handler: NewProductHandler(svc, logger), tenantID: tenantID, first: first, second: second}
}

func (f *partialUpdateFixture) put(t *testing.T, id uuid.UUID, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"productId": id.String()})
	r = r.WithContext(ctxutil.WithTenantID(r.Context(), f.tenantID))
	w := httptest.NewRecorder()
	f.handler.Update(w, r)
	return w
}

func (f *partialUpdateFixture) stored(t *testing.T, id uuid.UUID) *model.Product {
	t.Helper()
	p, err := f.repo.GetByID(f.tenantID, id)
	require.NoError(t, err)
	require.NotNil(t, p)
	return p
}

func TestProductUpdate_RenameChangesOnlyTheName(t *testing.T) {
	f := newPartialUpdateFixture(t)

	w := f.put(t, f.second.ID, `{"name":"Advisory"}`)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

	got := f.stored(t, f.second.ID)
	assert.Equal(t, "Advisory", got.Name)
	assert.Equal(t, 1, got.SortOrder, "a rename must not move the product")
	assert.Equal(t, model.DriverConsulting, got.DriverType)
	assert.JSONEq(t, consultingParams, string(got.DriverParams), "a rename must not touch the driver parameters")
	assert.Equal(t, model.ProductTypeService, got.ProductType)

	other := f.stored(t, f.first.ID)
	assert.Equal(t, "Licences", other.Name)
	assert.Equal(t, 0, other.SortOrder)
}

func TestProductUpdate_DriverSaveKeepsNameAndPosition(t *testing.T) {
	f := newPartialUpdateFixture(t)
	params := strings.Replace(consultingParams, `"workingDays":220`, `"workingDays":210`, 1)

	w := f.put(t, f.second.ID, `{"driverType":"consulting","driverParams":`+params+`}`)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

	got := f.stored(t, f.second.ID)
	assert.Equal(t, "Consulting", got.Name, "saving the driver configuration must not blank the name")
	assert.Equal(t, 1, got.SortOrder)
	assert.JSONEq(t, params, string(got.DriverParams))
}

func TestProductUpdate_EmptyBodyChangesNothing(t *testing.T) {
	f := newPartialUpdateFixture(t)

	w := f.put(t, f.second.ID, `{}`)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

	got := f.stored(t, f.second.ID)
	assert.Equal(t, "Consulting", got.Name)
	assert.Equal(t, 1, got.SortOrder)
	assert.Equal(t, model.DriverConsulting, got.DriverType)
	assert.JSONEq(t, consultingParams, string(got.DriverParams))
}

func TestProductUpdate_BlankNameIsRejected(t *testing.T) {
	f := newPartialUpdateFixture(t)

	w := f.put(t, f.second.ID, `{"name":"   "}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "Consulting", f.stored(t, f.second.ID).Name)
}

func TestProductUpdate_NullDriverParamsClearThem(t *testing.T) {
	f := newPartialUpdateFixture(t)

	// Switching back to the generic driver sends explicit nulls.
	w := f.put(t, f.second.ID, `{"driverType":"generic","driverParams":null}`)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

	got := f.stored(t, f.second.ID)
	assert.Equal(t, model.DriverGeneric, got.DriverType)
	assert.JSONEq(t, `null`, string(got.DriverParams))
	assert.Equal(t, "Consulting", got.Name)
	assert.Equal(t, 1, got.SortOrder)
}
