package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireBadRequest(t *testing.T, err error, contains string) {
	t.Helper()
	require.Error(t, err)
	var appErr *apierror.AppError
	require.True(t, errors.As(err, &appErr), "expected *apierror.AppError, got %T", err)
	assert.Equal(t, http.StatusBadRequest, appErr.StatusCode)
	assert.Contains(t, appErr.Error(), contains)
}

func TestCreateProduct_RejectsInvalidCompetitionResults(t *testing.T) {
	svc, repo := newTestProductService()
	tenantID, scenarioID := uuid.New(), uuid.New()

	p := &model.Product{
		Name:         "Prize money",
		DriverType:   model.DriverCompetition,
		DriverParams: json.RawMessage(`{"events":[10,0,0,0,0],"cuts":[12,0,0,0,0]}`),
	}
	err := svc.CreateProduct(context.Background(), tenantID, scenarioID, p)

	requireBadRequest(t, err, "cuts made (12) exceed events played (10)")
	list, _ := repo.ListProductsByScenario(tenantID, scenarioID)
	assert.Empty(t, list, "nothing is stored")
}

func TestUpdateProduct_ValidatesParamsAgainstTheStoredDriverType(t *testing.T) {
	svc, repo := newTestProductService()
	tenantID, scenarioID := uuid.New(), uuid.New()
	p := seedProduct(repo, tenantID, scenarioID, "Sponsoring")
	p.DriverType = model.DriverContract

	// The request changes only the params; the type comes from the stored product.
	update := &model.Product{Name: "Sponsoring", DriverParams: json.RawMessage(`{"contracts":[{"bonusPerWin":-5}]}`)}
	err := svc.UpdateProduct(context.Background(), tenantID, p.ID, update)

	requireBadRequest(t, err, "bonus per win must not be negative")
}

func TestGetDerivedBundle_ContractReadsWinsFromSiblingCompetitionProduct(t *testing.T) {
	svc, repo := newTestProductService()
	tenantID, scenarioID := uuid.New(), uuid.New()

	prize := seedProduct(repo, tenantID, scenarioID, "Prize money")
	prize.DriverType = model.DriverCompetition
	prize.DriverParams = json.RawMessage(`{"events":[20,0,0,0,0],"cuts":[10,0,0,0,0],"wins":[2,0,0,0,0]}`)

	sponsor := seedProduct(repo, tenantID, scenarioID, "Sponsoring")
	sponsor.DriverType = model.DriverContract
	sponsor.DriverParams = json.RawMessage(`{"contracts":[{"partner":"Club","amounts":[5000,0,0,0,0],"bonusPerWin":750}]}`)

	// A competition product in another scenario must not count.
	other := seedProduct(repo, tenantID, uuid.New(), "Other scenario")
	other.DriverType = model.DriverCompetition
	other.DriverParams = json.RawMessage(`{"events":[5,0,0,0,0],"cuts":[5,0,0,0,0],"wins":[5,0,0,0,0]}`)

	got, err := svc.GetDerivedBundle(context.Background(), tenantID, sponsor.ID)
	require.NoError(t, err)

	assert.True(t, got.Assumptions[0].BaseUnitPrice.Equal(decimal.NewFromInt(6500)),
		"5 000 + 2 wins × 750, got %s", got.Assumptions[0].BaseUnitPrice)
}
