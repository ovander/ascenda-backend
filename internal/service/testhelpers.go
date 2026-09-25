package service

import (
	"context"
	"errors"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// MockSettingsRepo is a manual mock implementation of SettingsRepo for testing
type MockSettingsRepo struct {
	configs              map[string]*model.PlanConfig
	openingBalances      map[string]*model.OpeningBalance
	wcConfigs            map[string]*model.WorkingCapitalConfig
	opexPerHire          map[string]*model.OpexPerHire
	capexPerHire         map[string]*model.CapexPerHire
	multiYearAdjustments map[string][]*model.MultiYearAdjustment
}

// NewMockSettingsRepo creates a new mock settings repository
func NewMockSettingsRepo() *MockSettingsRepo {
	return &MockSettingsRepo{
		configs:              make(map[string]*model.PlanConfig),
		openingBalances:      make(map[string]*model.OpeningBalance),
		wcConfigs:            make(map[string]*model.WorkingCapitalConfig),
		opexPerHire:          make(map[string]*model.OpexPerHire),
		capexPerHire:         make(map[string]*model.CapexPerHire),
		multiYearAdjustments: make(map[string][]*model.MultiYearAdjustment),
	}
}

// GetConfig retrieves a plan config
func (m *MockSettingsRepo) GetConfig(tenantID, scenarioID uuid.UUID) (*model.PlanConfig, error) {
	key := tenantID.String() + ":" + scenarioID.String()
	c := m.configs[key]
	if c == nil {
		return nil, errors.New("not found")
	}
	return c, nil
}

// UpsertConfig inserts or updates a plan config
func (m *MockSettingsRepo) UpsertConfig(config *model.PlanConfig) error {
	key := config.TenantID.String() + ":" + config.ScenarioID.String()
	m.configs[key] = config
	return nil
}

// GetOpeningBalance retrieves opening balance
func (m *MockSettingsRepo) GetOpeningBalance(tenantID, scenarioID uuid.UUID) (*model.OpeningBalance, error) {
	key := tenantID.String() + ":" + scenarioID.String()
	return m.openingBalances[key], nil
}

// UpsertOpeningBalance inserts or updates opening balance
func (m *MockSettingsRepo) UpsertOpeningBalance(balance *model.OpeningBalance) error {
	key := balance.TenantID.String() + ":" + balance.ScenarioID.String()
	m.openingBalances[key] = balance
	return nil
}

// GetWCConfig retrieves working capital config
func (m *MockSettingsRepo) GetWCConfig(tenantID, scenarioID uuid.UUID) (*model.WorkingCapitalConfig, error) {
	key := tenantID.String() + ":" + scenarioID.String()
	return m.wcConfigs[key], nil
}

// UpsertWCConfig inserts or updates working capital config
func (m *MockSettingsRepo) UpsertWCConfig(wcConfig *model.WorkingCapitalConfig) error {
	key := wcConfig.TenantID.String() + ":" + wcConfig.ScenarioID.String()
	m.wcConfigs[key] = wcConfig
	return nil
}

// GetOpexPerHire retrieves opex-per-hire config
func (m *MockSettingsRepo) GetOpexPerHire(tenantID, scenarioID uuid.UUID) (*model.OpexPerHire, error) {
	return m.opexPerHire[tenantID.String()+":"+scenarioID.String()], nil
}

// UpsertOpexPerHire inserts or updates opex-per-hire config
func (m *MockSettingsRepo) UpsertOpexPerHire(o *model.OpexPerHire) error {
	m.opexPerHire[o.TenantID.String()+":"+o.ScenarioID.String()] = o
	return nil
}

// GetCapexPerHire retrieves capex-per-hire config
func (m *MockSettingsRepo) GetCapexPerHire(tenantID, scenarioID uuid.UUID) (*model.CapexPerHire, error) {
	return m.capexPerHire[tenantID.String()+":"+scenarioID.String()], nil
}

// UpsertCapexPerHire inserts or updates capex-per-hire config
func (m *MockSettingsRepo) UpsertCapexPerHire(c *model.CapexPerHire) error {
	m.capexPerHire[c.TenantID.String()+":"+c.ScenarioID.String()] = c
	return nil
}

// ListMultiYearAdjustments lists multi-year adjustments for a scenario
func (m *MockSettingsRepo) ListMultiYearAdjustments(tenantID, scenarioID uuid.UUID) ([]*model.MultiYearAdjustment, error) {
	return m.multiYearAdjustments[tenantID.String()+":"+scenarioID.String()], nil
}

// BatchUpsertMultiYearAdjustments replaces multi-year adjustments for a scenario
func (m *MockSettingsRepo) BatchUpsertMultiYearAdjustments(tenantID, scenarioID uuid.UUID, adjustments []model.MultiYearAdjustment) error {
	key := tenantID.String() + ":" + scenarioID.String()
	ptrs := make([]*model.MultiYearAdjustment, len(adjustments))
	for i := range adjustments {
		a := adjustments[i]
		ptrs[i] = &a
	}
	m.multiYearAdjustments[key] = ptrs
	return nil
}

// MockPlanRepo is a manual mock implementation of PlanRepo for testing
type MockPlanRepo struct {
	plans     map[string]*model.BusinessPlan
	scenarios map[string]*model.Scenario
}

// NewMockPlanRepo creates a new mock plan repository
func NewMockPlanRepo() *MockPlanRepo {
	return &MockPlanRepo{
		plans:     make(map[string]*model.BusinessPlan),
		scenarios: make(map[string]*model.Scenario),
	}
}

// CreatePlan creates a new business plan
func (m *MockPlanRepo) CreatePlan(plan *model.BusinessPlan) error {
	key := plan.TenantID.String() + ":" + plan.ID.String()
	m.plans[key] = plan
	return nil
}

// GetPlan retrieves a business plan
func (m *MockPlanRepo) GetPlan(tenantID, planID uuid.UUID) (*model.BusinessPlan, error) {
	key := tenantID.String() + ":" + planID.String()
	return m.plans[key], nil
}

// CreateScenario creates a new scenario
func (m *MockPlanRepo) CreateScenario(scenario *model.Scenario) error {
	key := scenario.TenantID.String() + ":" + scenario.ID.String()
	m.scenarios[key] = scenario
	return nil
}

// GetScenario retrieves a scenario
func (m *MockPlanRepo) GetScenario(tenantID, scenarioID uuid.UUID) (*model.Scenario, error) {
	key := tenantID.String() + ":" + scenarioID.String()
	return m.scenarios[key], nil
}

// ListScenariosByPlan lists all scenarios for a plan
func (m *MockPlanRepo) ListScenariosByPlan(tenantID, planID uuid.UUID) ([]model.Scenario, error) {
	var results []model.Scenario
	for _, scenario := range m.scenarios {
		if scenario.TenantID == tenantID && scenario.PlanID == planID {
			results = append(results, *scenario)
		}
	}
	return results, nil
}

// MockSnapshotRepo is a manual mock implementation of SnapshotRepo for testing
type MockSnapshotRepo struct {
	snapshots map[string]*model.PlanSnapshot
}

// NewMockSnapshotRepo creates a new mock snapshot repository
func NewMockSnapshotRepo() *MockSnapshotRepo {
	return &MockSnapshotRepo{
		snapshots: make(map[string]*model.PlanSnapshot),
	}
}

// CreateSnapshot creates a new snapshot
func (m *MockSnapshotRepo) CreateSnapshot(snapshot *model.PlanSnapshot) error {
	key := snapshot.TenantID.String() + ":" + snapshot.ID.String()
	m.snapshots[key] = snapshot
	return nil
}

// GetSnapshot retrieves a snapshot
func (m *MockSnapshotRepo) GetSnapshot(tenantID, snapshotID uuid.UUID) (*model.PlanSnapshot, error) {
	key := tenantID.String() + ":" + snapshotID.String()
	return m.snapshots[key], nil
}

// ListSnapshots lists snapshots for a scenario
func (m *MockSnapshotRepo) ListSnapshots(tenantID, scenarioID uuid.UUID) ([]model.PlanSnapshot, error) {
	var results []model.PlanSnapshot
	for _, snapshot := range m.snapshots {
		if snapshot.TenantID == tenantID && snapshot.ScenarioID == scenarioID {
			results = append(results, *snapshot)
		}
	}
	return results, nil
}

// MockReportRepo is a manual mock implementation of ReportRepo for testing
type MockReportRepo struct {
	reports map[string]*model.Report
}

// NewMockReportRepo creates a new mock report repository
func NewMockReportRepo() *MockReportRepo {
	return &MockReportRepo{
		reports: make(map[string]*model.Report),
	}
}

// SaveReport saves a report
func (m *MockReportRepo) SaveReport(report *model.Report) error {
	key := report.TenantID.String() + ":" + report.ID.String()
	m.reports[key] = report
	return nil
}

// GetLatestReport retrieves latest report for a scenario
func (m *MockReportRepo) GetLatestReport(tenantID, scenarioID uuid.UUID) (*model.Report, error) {
	var latest *model.Report
	for _, report := range m.reports {
		if report.TenantID == tenantID && report.ScenarioID == scenarioID {
			if latest == nil || report.CreatedAt.After(latest.CreatedAt) {
				latest = report
			}
		}
	}
	return latest, nil
}

// ── MockProductRepo ───────────────────────────────────────────────────────────

// MockProductRepo is an in-memory implementation of ProductRepository for tests.
// Its DeleteProduct mirrors the real repo: it removes assumptions, volumes and
// margins before deleting the parent product row.
type MockProductRepo struct {
	products    map[string]*model.Product
	assumptions map[string]*model.ProductAssumption        // key = tenantID:assumptionID
	volumes     map[string]*model.ProductSalesVolume       // key = tenantID:volumeID
	margins     map[string]*model.ProductDistributorMargin // key = tenantID:marginID
}

// NewMockProductRepo creates a new MockProductRepo.
func NewMockProductRepo() *MockProductRepo {
	return &MockProductRepo{
		products:    make(map[string]*model.Product),
		assumptions: make(map[string]*model.ProductAssumption),
		volumes:     make(map[string]*model.ProductSalesVolume),
		margins:     make(map[string]*model.ProductDistributorMargin),
	}
}

func (m *MockProductRepo) CreateProduct(product *model.Product) error {
	m.products[product.TenantID.String()+":"+product.ID.String()] = product
	return nil
}

func (m *MockProductRepo) GetByID(tenantID, productID uuid.UUID) (*model.Product, error) {
	p := m.products[tenantID.String()+":"+productID.String()]
	return p, nil
}

func (m *MockProductRepo) ListProductsByScenario(tenantID, scenarioID uuid.UUID) ([]*model.Product, error) {
	var out []*model.Product
	for _, p := range m.products {
		if p.TenantID == tenantID && p.ScenarioID == scenarioID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (m *MockProductRepo) UpdateProduct(product *model.Product) error {
	m.products[product.TenantID.String()+":"+product.ID.String()] = product
	return nil
}

// DeleteProduct cascades: removes assumptions, volumes and margins for the
// product before deleting the product itself — matching the real repo behaviour.
func (m *MockProductRepo) DeleteProduct(tenantID, productID uuid.UUID) error {
	for k, a := range m.assumptions {
		if a.TenantID == tenantID && a.ProductID == productID {
			delete(m.assumptions, k)
		}
	}
	for k, v := range m.volumes {
		if v.TenantID == tenantID && v.ProductID == productID {
			delete(m.volumes, k)
		}
	}
	for k, mg := range m.margins {
		if mg.TenantID == tenantID && mg.ProductID == productID {
			delete(m.margins, k)
		}
	}
	delete(m.products, tenantID.String()+":"+productID.String())
	return nil
}

func (m *MockProductRepo) BatchUpsertAssumptions(_ uuid.UUID, _ uuid.UUID, assumptions []model.ProductAssumption) error {
	// delete existing rows for this product first (same as real repo)
	if len(assumptions) > 0 {
		pid := assumptions[0].ProductID
		for k, a := range m.assumptions {
			if a.TenantID == assumptions[0].TenantID && a.ProductID == pid {
				delete(m.assumptions, k)
			}
		}
	}
	for i := range assumptions {
		a := assumptions[i]
		m.assumptions[a.TenantID.String()+":"+a.ID.String()] = &a
	}
	return nil
}

func (m *MockProductRepo) GetAssumptionsByProduct(tenantID, productID uuid.UUID) ([]*model.ProductAssumption, error) {
	var out []*model.ProductAssumption
	for _, a := range m.assumptions {
		if a.TenantID == tenantID && a.ProductID == productID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (m *MockProductRepo) BatchUpsertVolumes(_ uuid.UUID, _ uuid.UUID, volumes []model.ProductSalesVolume) error {
	if len(volumes) > 0 {
		pid := volumes[0].ProductID
		for k, v := range m.volumes {
			if v.TenantID == volumes[0].TenantID && v.ProductID == pid {
				delete(m.volumes, k)
			}
		}
	}
	for i := range volumes {
		v := volumes[i]
		m.volumes[v.TenantID.String()+":"+v.ID.String()] = &v
	}
	return nil
}

func (m *MockProductRepo) GetVolumesByProduct(tenantID, productID uuid.UUID) ([]*model.ProductSalesVolume, error) {
	var out []*model.ProductSalesVolume
	for _, v := range m.volumes {
		if v.TenantID == tenantID && v.ProductID == productID {
			out = append(out, v)
		}
	}
	return out, nil
}

func (m *MockProductRepo) BatchUpsertMargins(_ uuid.UUID, _ uuid.UUID, margins []model.ProductDistributorMargin) error {
	if len(margins) > 0 {
		pid := margins[0].ProductID
		for k, mg := range m.margins {
			if mg.TenantID == margins[0].TenantID && mg.ProductID == pid {
				delete(m.margins, k)
			}
		}
	}
	for i := range margins {
		mg := margins[i]
		m.margins[mg.TenantID.String()+":"+mg.ID.String()] = &mg
	}
	return nil
}

func (m *MockProductRepo) GetMarginsByProduct(tenantID, productID uuid.UUID) ([]*model.ProductDistributorMargin, error) {
	var out []*model.ProductDistributorMargin
	for _, mg := range m.margins {
		if mg.TenantID == tenantID && mg.ProductID == productID {
			out = append(out, mg)
		}
	}
	return out, nil
}

func (m *MockProductRepo) GetAssumptionsByScenario(tenantID, scenarioID uuid.UUID) ([]*model.ProductAssumption, error) {
	products, _ := m.ListProductsByScenario(tenantID, scenarioID)
	pid := make(map[uuid.UUID]bool)
	for _, p := range products {
		pid[p.ID] = true
	}
	var out []*model.ProductAssumption
	for _, a := range m.assumptions {
		if a.TenantID == tenantID && pid[a.ProductID] {
			out = append(out, a)
		}
	}
	return out, nil
}

func (m *MockProductRepo) GetVolumesByScenario(tenantID, scenarioID uuid.UUID) ([]*model.ProductSalesVolume, error) {
	products, _ := m.ListProductsByScenario(tenantID, scenarioID)
	pid := make(map[uuid.UUID]bool)
	for _, p := range products {
		pid[p.ID] = true
	}
	var out []*model.ProductSalesVolume
	for _, v := range m.volumes {
		if v.TenantID == tenantID && pid[v.ProductID] {
			out = append(out, v)
		}
	}
	return out, nil
}

func (m *MockProductRepo) GetMarginsByScenario(tenantID, scenarioID uuid.UUID) ([]*model.ProductDistributorMargin, error) {
	products, _ := m.ListProductsByScenario(tenantID, scenarioID)
	pid := make(map[uuid.UUID]bool)
	for _, p := range products {
		pid[p.ID] = true
	}
	var out []*model.ProductDistributorMargin
	for _, mg := range m.margins {
		if mg.TenantID == tenantID && pid[mg.ProductID] {
			out = append(out, mg)
		}
	}
	return out, nil
}

// ── Simple BatchUpsert mock helper ────────────────────────────────────────────
// The simpleScenarioStore is a reusable keyed map used by all the simple
// "list-by-scenario / batch-upsert" mock repos below.

// ── MockStaffRepo ─────────────────────────────────────────────────────────────

type MockStaffRepo struct {
	headcounts map[string][]*model.StaffHeadcount
	salaries   map[string][]*model.StaffSalary
	incentives map[string][]*model.StaffIncentive
}

func NewMockStaffRepo() *MockStaffRepo {
	return &MockStaffRepo{
		headcounts: make(map[string][]*model.StaffHeadcount),
		salaries:   make(map[string][]*model.StaffSalary),
		incentives: make(map[string][]*model.StaffIncentive),
	}
}

func (m *MockStaffRepo) ListHeadcountsByScenario(tenantID, scenarioID uuid.UUID) ([]*model.StaffHeadcount, error) {
	return m.headcounts[tenantID.String()+":"+scenarioID.String()], nil
}
func (m *MockStaffRepo) BatchUpsertHeadcounts(tenantID, scenarioID uuid.UUID, hcs []model.StaffHeadcount) error {
	key := tenantID.String() + ":" + scenarioID.String()
	ptrs := make([]*model.StaffHeadcount, len(hcs))
	for i := range hcs {
		h := hcs[i]
		ptrs[i] = &h
	}
	m.headcounts[key] = ptrs
	return nil
}
func (m *MockStaffRepo) ListSalariesByScenario(tenantID, scenarioID uuid.UUID) ([]*model.StaffSalary, error) {
	return m.salaries[tenantID.String()+":"+scenarioID.String()], nil
}
func (m *MockStaffRepo) BatchUpsertSalaries(tenantID, scenarioID uuid.UUID, sals []model.StaffSalary) error {
	key := tenantID.String() + ":" + scenarioID.String()
	ptrs := make([]*model.StaffSalary, len(sals))
	for i := range sals {
		s := sals[i]
		ptrs[i] = &s
	}
	m.salaries[key] = ptrs
	return nil
}
func (m *MockStaffRepo) ListIncentivesByScenario(tenantID, scenarioID uuid.UUID) ([]*model.StaffIncentive, error) {
	return m.incentives[tenantID.String()+":"+scenarioID.String()], nil
}
func (m *MockStaffRepo) BatchUpsertIncentives(tenantID, scenarioID uuid.UUID, incs []model.StaffIncentive) error {
	key := tenantID.String() + ":" + scenarioID.String()
	ptrs := make([]*model.StaffIncentive, len(incs))
	for i := range incs {
		inc := incs[i]
		ptrs[i] = &inc
	}
	m.incentives[key] = ptrs
	return nil
}

// ── MockCapexRepo ─────────────────────────────────────────────────────────────

type MockCapexRepo struct {
	entries map[string][]*model.CapexEntry
}

func NewMockCapexRepo() *MockCapexRepo {
	return &MockCapexRepo{entries: make(map[string][]*model.CapexEntry)}
}
func (m *MockCapexRepo) ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.CapexEntry, error) {
	return m.entries[tenantID.String()+":"+scenarioID.String()], nil
}
func (m *MockCapexRepo) BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.CapexEntry) error {
	key := tenantID.String() + ":" + scenarioID.String()
	ptrs := make([]*model.CapexEntry, len(entries))
	for i := range entries {
		e := entries[i]
		ptrs[i] = &e
	}
	m.entries[key] = ptrs
	return nil
}

// ── MockOpexRepo ──────────────────────────────────────────────────────────────

type MockOpexRepo struct {
	entries map[string][]*model.OpexManualEntry
}

func NewMockOpexRepo() *MockOpexRepo {
	return &MockOpexRepo{entries: make(map[string][]*model.OpexManualEntry)}
}
func (m *MockOpexRepo) ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.OpexManualEntry, error) {
	return m.entries[tenantID.String()+":"+scenarioID.String()], nil
}
func (m *MockOpexRepo) BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.OpexManualEntry) error {
	key := tenantID.String() + ":" + scenarioID.String()
	ptrs := make([]*model.OpexManualEntry, len(entries))
	for i := range entries {
		e := entries[i]
		ptrs[i] = &e
	}
	m.entries[key] = ptrs
	return nil
}

// ── MockFiplanRepo ────────────────────────────────────────────────────────────

type MockFiplanRepo struct {
	entries map[string][]*model.FiplanEntry
}

func NewMockFiplanRepo() *MockFiplanRepo {
	return &MockFiplanRepo{entries: make(map[string][]*model.FiplanEntry)}
}
func (m *MockFiplanRepo) ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.FiplanEntry, error) {
	return m.entries[tenantID.String()+":"+scenarioID.String()], nil
}
func (m *MockFiplanRepo) BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.FiplanEntry) error {
	key := tenantID.String() + ":" + scenarioID.String()
	ptrs := make([]*model.FiplanEntry, len(entries))
	for i := range entries {
		e := entries[i]
		ptrs[i] = &e
	}
	m.entries[key] = ptrs
	return nil
}
func (m *MockFiplanRepo) UpsertCapitalIncreaseEntry(tenantID, scenarioID uuid.UUID, yearIndex int, amount decimal.Decimal, roundID uuid.UUID, roundLabel string) error {
	// Mirror the real repo: convert 0-based fiscalYearIndex -> 1-based FiPlan yearIndex.
	e := &model.FiplanEntry{
		ScenarioID: scenarioID, LineID: model.FiplanCapitalIncrease,
		YearIndex: yearIndex + 1, Amount: amount,
		CapTableRoundID: &roundID, CapTableRoundLabel: roundLabel,
	}
	e.TenantID = tenantID
	key := tenantID.String() + ":" + scenarioID.String()
	// Upsert: replace existing entry for same (line_id, year) if present.
	for _, existing := range m.entries[key] {
		if existing.LineID == model.FiplanCapitalIncrease && existing.YearIndex == e.YearIndex {
			existing.Amount = amount
			existing.CapTableRoundID = &roundID
			existing.CapTableRoundLabel = roundLabel
			return nil
		}
	}
	m.entries[key] = append(m.entries[key], e)
	return nil
}
func (m *MockFiplanRepo) ClearCapTableLink(tenantID, scenarioID uuid.UUID, yearIndex int) error {
	// Mirror the real repo: convert 0-based fiscalYearIndex -> 1-based FiPlan yearIndex.
	key := tenantID.String() + ":" + scenarioID.String()
	for _, e := range m.entries[key] {
		if e.LineID == model.FiplanCapitalIncrease && e.YearIndex == yearIndex+1 {
			e.CapTableRoundID = nil
			e.CapTableRoundLabel = ""
		}
	}
	return nil
}

// ── MockFiplanSyncer ──────────────────────────────────────────────────────────
// Implements the fiplanSyncer interface for testing CapTableService sync methods.

type MockFiplanSyncer struct {
	UpsertedEntries []struct {
		TenantID, ScenarioID uuid.UUID
		YearIndex            int
		Amount               decimal.Decimal
		RoundID              uuid.UUID
		RoundLabel           string
	}
	ClearedLinks []struct {
		TenantID, ScenarioID uuid.UUID
		YearIndex            int
	}
	UpsertErr error
	ClearErr  error
}

func (m *MockFiplanSyncer) UpsertCapitalIncreaseEntry(ctx context.Context, tenantID, scenarioID uuid.UUID, yearIndex int, amount decimal.Decimal, roundID uuid.UUID, roundLabel string) error {
	if m.UpsertErr != nil {
		return m.UpsertErr
	}
	m.UpsertedEntries = append(m.UpsertedEntries, struct {
		TenantID, ScenarioID uuid.UUID
		YearIndex            int
		Amount               decimal.Decimal
		RoundID              uuid.UUID
		RoundLabel           string
	}{tenantID, scenarioID, yearIndex, amount, roundID, roundLabel})
	return nil
}
func (m *MockFiplanSyncer) ClearCapTableLink(ctx context.Context, tenantID, scenarioID uuid.UUID, yearIndex int) error {
	if m.ClearErr != nil {
		return m.ClearErr
	}
	m.ClearedLinks = append(m.ClearedLinks, struct {
		TenantID, ScenarioID uuid.UUID
		YearIndex            int
	}{tenantID, scenarioID, yearIndex})
	return nil
}

// ── MockPnlCashRepo ───────────────────────────────────────────────────────────

type MockPnlCashRepo struct {
	entries map[string][]*model.PnlCashEntry
}

func NewMockPnlCashRepo() *MockPnlCashRepo {
	return &MockPnlCashRepo{entries: make(map[string][]*model.PnlCashEntry)}
}
func (m *MockPnlCashRepo) ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.PnlCashEntry, error) {
	return m.entries[tenantID.String()+":"+scenarioID.String()], nil
}
func (m *MockPnlCashRepo) BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.PnlCashEntry) error {
	key := tenantID.String() + ":" + scenarioID.String()
	ptrs := make([]*model.PnlCashEntry, len(entries))
	for i := range entries {
		e := entries[i]
		ptrs[i] = &e
	}
	m.entries[key] = ptrs
	return nil
}

// ── MockWcrRepo ───────────────────────────────────────────────────────────────

type MockWcrRepo struct {
	entries map[string][]*model.WCREntry
}

func NewMockWcrRepo() *MockWcrRepo {
	return &MockWcrRepo{entries: make(map[string][]*model.WCREntry)}
}
func (m *MockWcrRepo) ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.WCREntry, error) {
	return m.entries[tenantID.String()+":"+scenarioID.String()], nil
}
func (m *MockWcrRepo) BatchUpsert(tenantID, scenarioID uuid.UUID, entries []model.WCREntry) error {
	key := tenantID.String() + ":" + scenarioID.String()
	ptrs := make([]*model.WCREntry, len(entries))
	for i := range entries {
		e := entries[i]
		ptrs[i] = &e
	}
	m.entries[key] = ptrs
	return nil
}

// ── MockCashRepo ──────────────────────────────────────────────────────────────

type MockCashRepo struct {
	overrides map[string][]*model.CashMonthlyOverride
}

func NewMockCashRepo() *MockCashRepo {
	return &MockCashRepo{overrides: make(map[string][]*model.CashMonthlyOverride)}
}
func (m *MockCashRepo) ListByScenario(tenantID, scenarioID uuid.UUID) ([]*model.CashMonthlyOverride, error) {
	return m.overrides[tenantID.String()+":"+scenarioID.String()], nil
}
func (m *MockCashRepo) BatchUpsert(tenantID, scenarioID uuid.UUID, overrides []model.CashMonthlyOverride) error {
	key := tenantID.String() + ":" + scenarioID.String()
	ptrs := make([]*model.CashMonthlyOverride, len(overrides))
	for i := range overrides {
		o := overrides[i]
		ptrs[i] = &o
	}
	m.overrides[key] = ptrs
	return nil
}

// ── MockBudgetRepo ────────────────────────────────────────────────────────────

type MockBudgetRepo struct {
	overrides map[string][]*model.BudgetMonthlyOverride
}

func NewMockBudgetRepo() *MockBudgetRepo {
	return &MockBudgetRepo{overrides: make(map[string][]*model.BudgetMonthlyOverride)}
}
func (m *MockBudgetRepo) ListByScenario(tenantID, scenarioID uuid.UUID, year int) ([]*model.BudgetMonthlyOverride, error) {
	key := tenantID.String() + ":" + scenarioID.String()
	var out []*model.BudgetMonthlyOverride
	for _, o := range m.overrides[key] {
		if year == 0 || o.YearIndex == year {
			out = append(out, o)
		}
	}
	return out, nil
}
func (m *MockBudgetRepo) ListAllByScenario(tenantID, scenarioID uuid.UUID) ([]*model.BudgetMonthlyOverride, error) {
	return m.overrides[tenantID.String()+":"+scenarioID.String()], nil
}
func (m *MockBudgetRepo) BatchUpsert(tenantID, scenarioID uuid.UUID, overrides []model.BudgetMonthlyOverride) error {
	key := tenantID.String() + ":" + scenarioID.String()
	ptrs := make([]*model.BudgetMonthlyOverride, len(overrides))
	for i := range overrides {
		o := overrides[i]
		ptrs[i] = &o
	}
	m.overrides[key] = ptrs
	return nil
}

// ── MockCapTableRepo ──────────────────────────────────────────────────────────

type MockCapTableRepo struct {
	companies    map[string]*model.CapTableCompany
	shareClasses map[string]*model.CapTableShareClass
	shareholders map[string]*model.CapTableShareholder
	rounds       map[string]*model.CapTableRound
	positions    map[string][]*model.CapTablePosition // key = tenantID:roundID
	plans        map[string]*model.StockOptionPlan
	grants       map[string]*model.OptionGrant
	valuations   map[string]*model.ValuationScenario
	branches     map[string]*model.CapTableScenarioBranch
}

func NewMockCapTableRepo() *MockCapTableRepo {
	return &MockCapTableRepo{
		companies:    make(map[string]*model.CapTableCompany),
		shareClasses: make(map[string]*model.CapTableShareClass),
		shareholders: make(map[string]*model.CapTableShareholder),
		rounds:       make(map[string]*model.CapTableRound),
		positions:    make(map[string][]*model.CapTablePosition),
		plans:        make(map[string]*model.StockOptionPlan),
		grants:       make(map[string]*model.OptionGrant),
		valuations:   make(map[string]*model.ValuationScenario),
		branches:     make(map[string]*model.CapTableScenarioBranch),
	}
}

func (m *MockCapTableRepo) GetCompany(tenantID, scenarioID uuid.UUID) (*model.CapTableCompany, error) {
	c := m.companies[tenantID.String()+":"+scenarioID.String()]
	if c == nil {
		return nil, errors.New("not found")
	}
	return c, nil
}
func (m *MockCapTableRepo) UpsertCompany(company *model.CapTableCompany) error {
	m.companies[company.TenantID.String()+":"+company.ScenarioID.String()] = company
	return nil
}
func (m *MockCapTableRepo) ListShareClasses(tenantID, scenarioID uuid.UUID) ([]*model.CapTableShareClass, error) {
	var out []*model.CapTableShareClass
	for _, sc := range m.shareClasses {
		if sc.TenantID == tenantID && sc.ScenarioID == scenarioID {
			out = append(out, sc)
		}
	}
	return out, nil
}
func (m *MockCapTableRepo) UpsertShareClass(sc *model.CapTableShareClass) error {
	if sc.ID == uuid.Nil {
		sc.ID = uuid.New()
	}
	m.shareClasses[sc.TenantID.String()+":"+sc.ID.String()] = sc
	return nil
}
func (m *MockCapTableRepo) DeleteShareClass(tenantID, id uuid.UUID) error {
	delete(m.shareClasses, tenantID.String()+":"+id.String())
	return nil
}
func (m *MockCapTableRepo) ListShareholders(tenantID, scenarioID uuid.UUID) ([]*model.CapTableShareholder, error) {
	var out []*model.CapTableShareholder
	for _, sh := range m.shareholders {
		if sh.TenantID == tenantID && sh.ScenarioID == scenarioID {
			out = append(out, sh)
		}
	}
	return out, nil
}
func (m *MockCapTableRepo) GetShareholder(tenantID, id uuid.UUID) (*model.CapTableShareholder, error) {
	sh := m.shareholders[tenantID.String()+":"+id.String()]
	if sh == nil {
		return nil, errors.New("not found")
	}
	return sh, nil
}
func (m *MockCapTableRepo) CreateShareholder(sh *model.CapTableShareholder) error {
	m.shareholders[sh.TenantID.String()+":"+sh.ID.String()] = sh
	return nil
}
func (m *MockCapTableRepo) UpdateShareholder(sh *model.CapTableShareholder) error {
	m.shareholders[sh.TenantID.String()+":"+sh.ID.String()] = sh
	return nil
}
func (m *MockCapTableRepo) DeleteShareholder(tenantID, id uuid.UUID) error {
	delete(m.shareholders, tenantID.String()+":"+id.String())
	return nil
}
func (m *MockCapTableRepo) ListRounds(tenantID, scenarioID uuid.UUID) ([]*model.CapTableRound, error) {
	var out []*model.CapTableRound
	for _, r := range m.rounds {
		if r.TenantID == tenantID && r.ScenarioID == scenarioID {
			out = append(out, r)
		}
	}
	return out, nil
}
func (m *MockCapTableRepo) GetRound(tenantID, id uuid.UUID) (*model.CapTableRound, error) {
	r := m.rounds[tenantID.String()+":"+id.String()]
	if r == nil {
		return nil, errors.New("not found")
	}
	return r, nil
}
func (m *MockCapTableRepo) CreateRound(rnd *model.CapTableRound) error {
	m.rounds[rnd.TenantID.String()+":"+rnd.ID.String()] = rnd
	return nil
}
func (m *MockCapTableRepo) UpdateRound(rnd *model.CapTableRound) error {
	m.rounds[rnd.TenantID.String()+":"+rnd.ID.String()] = rnd
	return nil
}
func (m *MockCapTableRepo) DeleteRound(tenantID, id uuid.UUID) error {
	delete(m.rounds, tenantID.String()+":"+id.String())
	return nil
}
func (m *MockCapTableRepo) ListPositionsByRound(tenantID, roundID uuid.UUID) ([]*model.CapTablePosition, error) {
	return m.positions[tenantID.String()+":"+roundID.String()], nil
}
func (m *MockCapTableRepo) ListPositionsByScenario(tenantID, scenarioID uuid.UUID) ([]*model.CapTablePosition, error) {
	// Mirror the real repo's JOIN: collect round IDs that belong to this scenario first.
	roundIDs := map[uuid.UUID]bool{}
	for _, r := range m.rounds {
		if r.TenantID == tenantID && r.ScenarioID == scenarioID {
			roundIDs[r.ID] = true
		}
	}
	var out []*model.CapTablePosition
	for _, ps := range m.positions {
		for _, p := range ps {
			if p.TenantID == tenantID && roundIDs[p.RoundID] {
				out = append(out, p)
			}
		}
	}
	return out, nil
}
func (m *MockCapTableRepo) BatchUpsertPositions(tenantID, roundID uuid.UUID, positions []model.CapTablePosition) error {
	key := tenantID.String() + ":" + roundID.String()
	ptrs := make([]*model.CapTablePosition, len(positions))
	for i := range positions {
		p := positions[i]
		ptrs[i] = &p
	}
	m.positions[key] = ptrs
	return nil
}
func (m *MockCapTableRepo) ListPlans(tenantID, scenarioID uuid.UUID) ([]*model.StockOptionPlan, error) {
	var out []*model.StockOptionPlan
	for _, p := range m.plans {
		if p.TenantID == tenantID && p.ScenarioID == scenarioID {
			out = append(out, p)
		}
	}
	return out, nil
}
func (m *MockCapTableRepo) GetPlan(tenantID, id uuid.UUID) (*model.StockOptionPlan, error) {
	p := m.plans[tenantID.String()+":"+id.String()]
	if p == nil {
		return nil, errors.New("not found")
	}
	return p, nil
}
func (m *MockCapTableRepo) CreatePlan(plan *model.StockOptionPlan) error {
	m.plans[plan.TenantID.String()+":"+plan.ID.String()] = plan
	return nil
}
func (m *MockCapTableRepo) UpdatePlan(plan *model.StockOptionPlan) error {
	m.plans[plan.TenantID.String()+":"+plan.ID.String()] = plan
	return nil
}
func (m *MockCapTableRepo) DeletePlan(tenantID, id uuid.UUID) error {
	delete(m.plans, tenantID.String()+":"+id.String())
	return nil
}
func (m *MockCapTableRepo) ListGrantsByPlan(tenantID, planID uuid.UUID) ([]*model.OptionGrant, error) {
	var out []*model.OptionGrant
	for _, g := range m.grants {
		if g.TenantID == tenantID && g.PlanID == planID {
			out = append(out, g)
		}
	}
	return out, nil
}
func (m *MockCapTableRepo) ListGrantsByScenario(tenantID, scenarioID uuid.UUID) ([]*model.OptionGrant, error) {
	// Mirror the real repo's JOIN: find plans for this scenario, then return matching grants.
	planIDs := map[uuid.UUID]bool{}
	for _, p := range m.plans {
		if p.TenantID == tenantID && p.ScenarioID == scenarioID {
			planIDs[p.ID] = true
		}
	}
	var out []*model.OptionGrant
	for _, g := range m.grants {
		if g.TenantID == tenantID && planIDs[g.PlanID] {
			out = append(out, g)
		}
	}
	return out, nil
}
func (m *MockCapTableRepo) GetGrant(tenantID, id uuid.UUID) (*model.OptionGrant, error) {
	g := m.grants[tenantID.String()+":"+id.String()]
	if g == nil {
		return nil, errors.New("not found")
	}
	return g, nil
}
func (m *MockCapTableRepo) CreateGrant(grant *model.OptionGrant) error {
	m.grants[grant.TenantID.String()+":"+grant.ID.String()] = grant
	return nil
}
func (m *MockCapTableRepo) UpdateGrant(grant *model.OptionGrant) error {
	m.grants[grant.TenantID.String()+":"+grant.ID.String()] = grant
	return nil
}
func (m *MockCapTableRepo) DeleteGrant(tenantID, id uuid.UUID) error {
	delete(m.grants, tenantID.String()+":"+id.String())
	return nil
}
func (m *MockCapTableRepo) ListValuationScenarios(tenantID, scenarioID uuid.UUID) ([]*model.ValuationScenario, error) {
	var out []*model.ValuationScenario
	for _, v := range m.valuations {
		if v.TenantID == tenantID && v.ScenarioID == scenarioID {
			out = append(out, v)
		}
	}
	return out, nil
}
func (m *MockCapTableRepo) GetValuationScenario(tenantID, id uuid.UUID) (*model.ValuationScenario, error) {
	v := m.valuations[tenantID.String()+":"+id.String()]
	if v == nil {
		return nil, errors.New("not found")
	}
	return v, nil
}
func (m *MockCapTableRepo) CreateValuationScenario(vs *model.ValuationScenario) error {
	m.valuations[vs.TenantID.String()+":"+vs.ID.String()] = vs
	return nil
}
func (m *MockCapTableRepo) UpdateValuationScenario(vs *model.ValuationScenario) error {
	m.valuations[vs.TenantID.String()+":"+vs.ID.String()] = vs
	return nil
}
func (m *MockCapTableRepo) DeleteValuationScenario(tenantID, id uuid.UUID) error {
	delete(m.valuations, tenantID.String()+":"+id.String())
	return nil
}
func (m *MockCapTableRepo) ListBranches(tenantID, scenarioID uuid.UUID) ([]*model.CapTableScenarioBranch, error) {
	var out []*model.CapTableScenarioBranch
	for _, b := range m.branches {
		if b.TenantID == tenantID && b.ScenarioID == scenarioID {
			out = append(out, b)
		}
	}
	return out, nil
}
func (m *MockCapTableRepo) GetBranch(tenantID, id uuid.UUID) (*model.CapTableScenarioBranch, error) {
	b := m.branches[tenantID.String()+":"+id.String()]
	if b == nil {
		return nil, errors.New("not found")
	}
	return b, nil
}
func (m *MockCapTableRepo) CreateBranch(branch *model.CapTableScenarioBranch) error {
	m.branches[branch.TenantID.String()+":"+branch.ID.String()] = branch
	return nil
}
func (m *MockCapTableRepo) UpdateBranch(branch *model.CapTableScenarioBranch) error {
	m.branches[branch.TenantID.String()+":"+branch.ID.String()] = branch
	return nil
}
func (m *MockCapTableRepo) DeleteBranch(tenantID, id uuid.UUID) error {
	delete(m.branches, tenantID.String()+":"+id.String())
	return nil
}

// ── MockBEPRepo ───────────────────────────────────────────────────────────────

type MockBEPRepo struct {
	snapshots       map[string]*model.BEPSnapshot
	fixedLines      map[string][]*model.FixedCostLine    // key = tenantID:snapshotID
	variableLines   map[string][]*model.VariableCostLine // key = tenantID:snapshotID
	sensitivityCfgs map[string]*model.SensitivityConfig
	optPlans        map[string]*model.OptimisationPlan
	fixedSavings    map[string][]*model.FixedCostSaving    // key = tenantID:planID
	varSavings      map[string][]*model.VariableCostSaving // key = tenantID:planID
	pcgItems        map[string][]*model.PCGReviewItem      // key = tenantID:planID
}

func NewMockBEPRepo() *MockBEPRepo {
	return &MockBEPRepo{
		snapshots:       make(map[string]*model.BEPSnapshot),
		fixedLines:      make(map[string][]*model.FixedCostLine),
		variableLines:   make(map[string][]*model.VariableCostLine),
		sensitivityCfgs: make(map[string]*model.SensitivityConfig),
		optPlans:        make(map[string]*model.OptimisationPlan),
		fixedSavings:    make(map[string][]*model.FixedCostSaving),
		varSavings:      make(map[string][]*model.VariableCostSaving),
		pcgItems:        make(map[string][]*model.PCGReviewItem),
	}
}

func (m *MockBEPRepo) ListSnapshots(tenantID, scenarioID uuid.UUID) ([]*model.BEPSnapshot, error) {
	var out []*model.BEPSnapshot
	for _, s := range m.snapshots {
		if s.TenantID == tenantID && s.ScenarioID == scenarioID {
			out = append(out, s)
		}
	}
	return out, nil
}
func (m *MockBEPRepo) CreateSnapshot(snap *model.BEPSnapshot) error {
	m.snapshots[snap.TenantID.String()+":"+snap.ID.String()] = snap
	return nil
}
func (m *MockBEPRepo) GetSnapshot(tenantID, id uuid.UUID) (*model.BEPSnapshot, error) {
	s := m.snapshots[tenantID.String()+":"+id.String()]
	if s == nil {
		return nil, errors.New("not found")
	}
	return s, nil
}
func (m *MockBEPRepo) UpdateSnapshot(snap *model.BEPSnapshot) error {
	m.snapshots[snap.TenantID.String()+":"+snap.ID.String()] = snap
	return nil
}
func (m *MockBEPRepo) DeleteSnapshot(tenantID, id uuid.UUID) error {
	delete(m.snapshots, tenantID.String()+":"+id.String())
	return nil
}
func (m *MockBEPRepo) ListFixedCostLines(tenantID, snapshotID uuid.UUID) ([]*model.FixedCostLine, error) {
	return m.fixedLines[tenantID.String()+":"+snapshotID.String()], nil
}
func (m *MockBEPRepo) BatchUpsertFixedCostLines(tenantID, snapshotID uuid.UUID, lines []model.FixedCostLine) error {
	key := tenantID.String() + ":" + snapshotID.String()
	ptrs := make([]*model.FixedCostLine, len(lines))
	for i := range lines {
		l := lines[i]
		ptrs[i] = &l
	}
	m.fixedLines[key] = ptrs
	return nil
}
func (m *MockBEPRepo) ListVariableCostLines(tenantID, snapshotID uuid.UUID) ([]*model.VariableCostLine, error) {
	return m.variableLines[tenantID.String()+":"+snapshotID.String()], nil
}
func (m *MockBEPRepo) BatchUpsertVariableCostLines(tenantID, snapshotID uuid.UUID, lines []model.VariableCostLine) error {
	key := tenantID.String() + ":" + snapshotID.String()
	ptrs := make([]*model.VariableCostLine, len(lines))
	for i := range lines {
		l := lines[i]
		ptrs[i] = &l
	}
	m.variableLines[key] = ptrs
	return nil
}
func (m *MockBEPRepo) ListSensitivityConfigs(tenantID, snapshotID uuid.UUID) ([]*model.SensitivityConfig, error) {
	var out []*model.SensitivityConfig
	for _, c := range m.sensitivityCfgs {
		if c.TenantID == tenantID && c.SnapshotID == snapshotID {
			out = append(out, c)
		}
	}
	return out, nil
}
func (m *MockBEPRepo) UpsertSensitivityConfig(cfg *model.SensitivityConfig) error {
	if cfg.ID == uuid.Nil {
		cfg.ID = uuid.New()
	}
	m.sensitivityCfgs[cfg.TenantID.String()+":"+cfg.ID.String()] = cfg
	return nil
}
func (m *MockBEPRepo) CreateOptimisationPlan(p *model.OptimisationPlan) error {
	m.optPlans[p.TenantID.String()+":"+p.ID.String()] = p
	return nil
}
func (m *MockBEPRepo) GetOptimisationPlan(tenantID, id uuid.UUID) (*model.OptimisationPlan, error) {
	p := m.optPlans[tenantID.String()+":"+id.String()]
	if p == nil {
		return nil, errors.New("not found")
	}
	return p, nil
}
func (m *MockBEPRepo) ListOptimisationPlans(tenantID, snapshotID uuid.UUID) ([]*model.OptimisationPlan, error) {
	var out []*model.OptimisationPlan
	for _, p := range m.optPlans {
		if p.TenantID == tenantID && p.SnapshotID == snapshotID {
			out = append(out, p)
		}
	}
	return out, nil
}
func (m *MockBEPRepo) UpdateOptimisationPlan(p *model.OptimisationPlan) error {
	m.optPlans[p.TenantID.String()+":"+p.ID.String()] = p
	return nil
}
func (m *MockBEPRepo) DeleteOptimisationPlan(tenantID, id uuid.UUID) error {
	delete(m.optPlans, tenantID.String()+":"+id.String())
	return nil
}
func (m *MockBEPRepo) ListFixedCostSavings(tenantID, planID uuid.UUID) ([]*model.FixedCostSaving, error) {
	return m.fixedSavings[tenantID.String()+":"+planID.String()], nil
}
func (m *MockBEPRepo) BatchUpsertFixedCostSavings(tenantID, planID uuid.UUID, savings []model.FixedCostSaving) error {
	key := tenantID.String() + ":" + planID.String()
	ptrs := make([]*model.FixedCostSaving, len(savings))
	for i := range savings {
		s := savings[i]
		ptrs[i] = &s
	}
	m.fixedSavings[key] = ptrs
	return nil
}
func (m *MockBEPRepo) ListVariableCostSavings(tenantID, planID uuid.UUID) ([]*model.VariableCostSaving, error) {
	return m.varSavings[tenantID.String()+":"+planID.String()], nil
}
func (m *MockBEPRepo) BatchUpsertVariableCostSavings(tenantID, planID uuid.UUID, savings []model.VariableCostSaving) error {
	key := tenantID.String() + ":" + planID.String()
	ptrs := make([]*model.VariableCostSaving, len(savings))
	for i := range savings {
		s := savings[i]
		ptrs[i] = &s
	}
	m.varSavings[key] = ptrs
	return nil
}
func (m *MockBEPRepo) ListPCGReviewItems(tenantID, planID uuid.UUID) ([]*model.PCGReviewItem, error) {
	return m.pcgItems[tenantID.String()+":"+planID.String()], nil
}
func (m *MockBEPRepo) BatchUpsertPCGReviewItems(tenantID, planID uuid.UUID, items []model.PCGReviewItem) error {
	key := tenantID.String() + ":" + planID.String()
	ptrs := make([]*model.PCGReviewItem, len(items))
	for i := range items {
		it := items[i]
		ptrs[i] = &it
	}
	m.pcgItems[key] = ptrs
	return nil
}
