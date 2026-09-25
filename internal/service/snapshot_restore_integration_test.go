//go:build integration

package service

// Database-backed tests for the atomic snapshot restore (audit §3.1): a
// restore replaces the scenario's data with the snapshot's, and a failure in
// any section rolls the whole restore back.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"ascenda/internal/event"
	"ascenda/internal/model"
	"ascenda/internal/repo"
	"ascenda/internal/testdb"
)

var integrationDB *gorm.DB

func TestMain(m *testing.M) {
	db, cleanup, err := testdb.Connect(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		cleanup()
		os.Exit(1)
	}
	integrationDB = db
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// restoreFixture is one plan with one scenario holding a config, two products
// (one assumption each) and one capex entry, plus the service under test
// bound to the per-test transaction.
type restoreFixture struct {
	db       *gorm.DB
	repos    *repo.RepoBundle
	svc      *SnapshotService
	ctx      context.Context
	tenantID uuid.UUID
	planID   uuid.UUID
	scenID   uuid.UUID
}

func newRestoreFixture(t *testing.T) *restoreFixture {
	t.Helper()
	db := testdb.Tx(t, integrationDB)
	uid := uuid.New().String()[:8]

	tenant := &model.Tenant{ID: uuid.New(), Name: "Tenant-" + uid, Slug: "tenant-" + uid, IsActive: true}
	require.NoError(t, db.Create(tenant).Error)
	user := &model.User{TenantID: tenant.ID, Email: "u-" + uid + "@example.com", Name: "U", Role: "owner", IsActive: true}
	require.NoError(t, db.Create(user).Error)
	plan := &model.BusinessPlan{TenantScoped: model.TenantScoped{TenantID: tenant.ID}, Name: "Plan-" + uid, Status: "draft", CreatedBy: user.ID}
	require.NoError(t, db.Create(plan).Error)

	f := &restoreFixture{db: db, repos: repo.NewRepoBundle(db), tenantID: tenant.ID, planID: plan.ID}
	f.scenID = f.newScenario(t, "Base")
	f.ctx = ctxutil.WithUserID(context.Background(), user.ID)

	logger := logrus.NewEntry(logrus.New())
	logger.Logger.SetLevel(logrus.PanicLevel)
	emitter := event.NewEmitter(logger)
	t.Cleanup(emitter.Close)
	f.svc = NewSnapshotService(f.repos.Snapshot, f.repos, emitter, logger)

	f.seedConfig(t, f.scenID, "BE")
	f.seedProduct(t, f.scenID, "Widget", 10)
	f.seedProduct(t, f.scenID, "Gadget", 20)
	f.seedCapex(t, f.scenID, 1, 5000)
	return f
}

func (f *restoreFixture) newScenario(t *testing.T, name string) uuid.UUID {
	t.Helper()
	s := &model.Scenario{TenantScoped: model.TenantScoped{TenantID: f.tenantID}, PlanID: f.planID, Name: name}
	require.NoError(t, f.db.Create(s).Error)
	return s.ID
}

func (f *restoreFixture) seedConfig(t *testing.T, scenID uuid.UUID, country string) {
	t.Helper()
	cfg := &model.PlanConfig{TenantScoped: model.TenantScoped{TenantID: f.tenantID}, ScenarioID: scenID, ForecastStart: time.Now(), Country: country}
	require.NoError(t, f.db.Create(cfg).Error)
}

func (f *restoreFixture) seedProduct(t *testing.T, scenID uuid.UUID, name string, price int64) uuid.UUID {
	t.Helper()
	p := &model.Product{TenantScoped: model.TenantScoped{TenantID: f.tenantID}, ScenarioID: scenID, Name: name}
	require.NoError(t, f.db.Create(p).Error)
	a := &model.ProductAssumption{TenantScoped: model.TenantScoped{TenantID: f.tenantID}, ProductID: p.ID, YearIndex: 1, BaseUnitPrice: decimal.NewFromInt(price)}
	require.NoError(t, f.db.Create(a).Error)
	return p.ID
}

func (f *restoreFixture) seedCapex(t *testing.T, scenID uuid.UUID, year int, amount int64) {
	t.Helper()
	e := &model.CapexEntry{TenantScoped: model.TenantScoped{TenantID: f.tenantID}, ScenarioID: scenID, Category: model.AssetBuildings, YearIndex: year, Amount: decimal.NewFromInt(amount), DepreciationYears: 20}
	require.NoError(t, f.db.Create(e).Error)
}

// state is the observable content of a scenario the assertions compare.
type scenarioState struct {
	configs   int64
	country   string
	products  []string
	prices    []string
	capex     []string
	productID map[string]uuid.UUID
}

func (f *restoreFixture) state(t *testing.T, scenID uuid.UUID) scenarioState {
	t.Helper()
	var st scenarioState
	require.NoError(t, f.db.Model(&model.PlanConfig{}).Where("scenario_id = ?", scenID).Count(&st.configs).Error)
	if st.configs > 0 {
		cfg, err := f.repos.Settings.GetConfig(f.tenantID, scenID)
		require.NoError(t, err)
		st.country = cfg.Country
	}
	products, err := f.repos.Product.ListProductsByScenario(f.tenantID, scenID)
	require.NoError(t, err)
	st.productID = map[string]uuid.UUID{}
	for _, p := range products {
		st.products = append(st.products, p.Name)
		st.productID[p.Name] = p.ID
	}
	assumptions, err := f.repos.Product.GetAssumptionsByScenario(f.tenantID, scenID)
	require.NoError(t, err)
	for _, a := range assumptions {
		st.prices = append(st.prices, a.BaseUnitPrice.String())
	}
	capex, err := f.repos.Capex.ListByScenario(f.tenantID, scenID)
	require.NoError(t, err)
	for _, c := range capex {
		st.capex = append(st.capex, fmt.Sprintf("y%d=%s", c.YearIndex, c.Amount.String()))
	}
	return st
}

func TestSnapshotRestore_ReplacesScenarioWithSnapshotState(t *testing.T) {
	f := newRestoreFixture(t)
	before := f.state(t, f.scenID)

	snap, err := f.svc.Create(f.ctx, f.tenantID, f.scenID, "v1", "")
	require.NoError(t, err)

	// Drift away from the snapshot: new product, changed config, extra capex,
	// one assumption removed.
	f.seedProduct(t, f.scenID, "Gizmo", 30)
	require.NoError(t, f.db.Model(&model.PlanConfig{}).Where("scenario_id = ?", f.scenID).Update("country", "FR").Error)
	f.seedCapex(t, f.scenID, 2, 700)
	require.NoError(t, f.db.Where("product_id = ?", before.productID["Widget"]).Delete(&model.ProductAssumption{}).Error)
	drifted := f.state(t, f.scenID)
	require.ElementsMatch(t, []string{"Widget", "Gadget", "Gizmo"}, drifted.products)
	require.Equal(t, "FR", drifted.country)

	require.NoError(t, f.svc.Restore(f.ctx, f.tenantID, f.scenID, snap.ID))

	after := f.state(t, f.scenID)
	assert.Equal(t, int64(1), after.configs, "restore must not leave duplicate config rows")
	assert.Equal(t, "BE", after.country)
	assert.ElementsMatch(t, before.products, after.products, "products added since the snapshot are gone")
	assert.ElementsMatch(t, before.prices, after.prices, "assumptions follow their restored product")
	assert.ElementsMatch(t, before.capex, after.capex, "entries added since the snapshot are gone")

	// Restoring twice is idempotent: still no duplicates.
	require.NoError(t, f.svc.Restore(f.ctx, f.tenantID, f.scenID, snap.ID))
	again := f.state(t, f.scenID)
	assert.Equal(t, int64(1), again.configs)
	assert.ElementsMatch(t, before.products, again.products)
}

func TestSnapshotRestore_FailureRollsBackEverySection(t *testing.T) {
	f := newRestoreFixture(t)
	before := f.state(t, f.scenID)

	// Valid settings section followed by a corrupt products section: the
	// settings write succeeds before the products decode fails.
	cfg, err := f.repos.Settings.GetConfig(f.tenantID, f.scenID)
	require.NoError(t, err)
	cfg.Country = "DE"
	cfgJSON, err := json.Marshal(cfg)
	require.NoError(t, err)
	corrupt := &model.PlanSnapshot{
		TenantScoped: model.TenantScoped{TenantID: f.tenantID},
		ScenarioID:   f.scenID,
		Version:      1,
		Label:        "corrupt",
		CreatedBy:    uuid.New(),
		Data:         json.RawMessage(`{"config":` + string(cfgJSON) + `,"products":"not-an-array"}`),
	}
	require.NoError(t, f.repos.Snapshot.Create(corrupt))

	err = f.svc.Restore(f.ctx, f.tenantID, f.scenID, corrupt.ID)
	require.Error(t, err)

	after := f.state(t, f.scenID)
	assert.Equal(t, before, after, "a failed restore must leave the scenario exactly as it was")
}

func TestSnapshotRestore_CloneReplacesTargetAndKeepsSource(t *testing.T) {
	f := newRestoreFixture(t)
	target := f.newScenario(t, "Target")
	f.seedConfig(t, target, "NL")
	f.seedProduct(t, target, "Old", 1)

	snap, err := f.svc.Create(f.ctx, f.tenantID, f.scenID, "v1", "")
	require.NoError(t, err)
	source := f.state(t, f.scenID)

	require.NoError(t, f.svc.CloneToScenario(f.ctx, f.tenantID, f.scenID, snap.ID, target))

	got := f.state(t, target)
	assert.Equal(t, int64(1), got.configs)
	assert.Equal(t, "BE", got.country, "target config replaced by the snapshot's")
	assert.ElementsMatch(t, source.products, got.products, "target's own product is gone")
	assert.ElementsMatch(t, source.capex, got.capex)
	for name, id := range got.productID {
		assert.NotEqual(t, source.productID[name], id, "cloned product %s must get a fresh ID", name)
	}
	assert.Equal(t, source, f.state(t, f.scenID), "source scenario untouched")
}
