package service

import (
	"github.com/google/uuid"
	"kerplan/internal/model"
)

// MockSettingsRepo is a manual mock implementation of SettingsRepo for testing
type MockSettingsRepo struct {
	configs         map[string]*model.PlanConfig
	openingBalances map[string]*model.OpeningBalance
	wcConfigs       map[string]*model.WorkingCapitalConfig
}

// NewMockSettingsRepo creates a new mock settings repository
func NewMockSettingsRepo() *MockSettingsRepo {
	return &MockSettingsRepo{
		configs:         make(map[string]*model.PlanConfig),
		openingBalances: make(map[string]*model.OpeningBalance),
		wcConfigs:       make(map[string]*model.WorkingCapitalConfig),
	}
}

// GetConfig retrieves a plan config
func (m *MockSettingsRepo) GetConfig(tenantID, scenarioID uuid.UUID) (*model.PlanConfig, error) {
	key := tenantID.String() + ":" + scenarioID.String()
	return m.configs[key], nil
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
