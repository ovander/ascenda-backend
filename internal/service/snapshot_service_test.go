package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"kerplan/internal/model"
)

func TestSnapshotRepoCreateAndGet(t *testing.T) {
	mockRepo := NewMockSnapshotRepo()

	tenantID := uuid.New()
	scenarioID := uuid.New()

	snapshot := &model.PlanSnapshot{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:   scenarioID,
		Version:      1,
		Label:        "Test Snapshot",
		Description:  "A test snapshot",
	}

	err := mockRepo.CreateSnapshot(snapshot)
	assert.NoError(t, err)

	retrieved, err := mockRepo.GetSnapshot(tenantID, snapshot.ID)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "Test Snapshot", retrieved.Label)
}

func TestSnapshotRepoListSnapshots(t *testing.T) {
	mockRepo := NewMockSnapshotRepo()

	tenantID := uuid.New()
	scenarioID := uuid.New()

	for i := 0; i < 3; i++ {
		snap := &model.PlanSnapshot{
			TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			ScenarioID:   scenarioID,
			Version:      i + 1,
			Label:        "Snapshot " + string(rune('A'+i)),
		}
		mockRepo.CreateSnapshot(snap)
	}

	retrieved, err := mockRepo.ListSnapshots(tenantID, scenarioID)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(retrieved))

	labels := make(map[string]bool)
	for _, snap := range retrieved {
		labels[snap.Label] = true
	}
	assert.True(t, labels["Snapshot A"])
	assert.True(t, labels["Snapshot B"])
	assert.True(t, labels["Snapshot C"])
}

func TestSnapshotRepoEmptyList(t *testing.T) {
	mockRepo := NewMockSnapshotRepo()

	tenantID := uuid.New()
	scenarioID := uuid.New()

	retrieved, err := mockRepo.ListSnapshots(tenantID, scenarioID)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(retrieved))
}

func TestSnapshotRepoGetByID(t *testing.T) {
	mockRepo := NewMockSnapshotRepo()

	tenantID := uuid.New()
	scenarioID := uuid.New()

	snapshot := &model.PlanSnapshot{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:   scenarioID,
		Version:      1,
		Label:        "Production Snapshot",
		Description:  "Captured for review",
	}

	mockRepo.CreateSnapshot(snapshot)

	retrieved, err := mockRepo.GetSnapshot(tenantID, snapshot.ID)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, snapshot.Label, retrieved.Label)
	assert.Equal(t, snapshot.Description, retrieved.Description)
}

func TestSnapshotRepoMultipleScenariosIsolation(t *testing.T) {
	mockRepo := NewMockSnapshotRepo()

	tenantID := uuid.New()
	scenario1 := uuid.New()
	scenario2 := uuid.New()

	snap1 := &model.PlanSnapshot{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:   scenario1,
		Version:      1,
		Label:        "Scenario 1 Snapshot",
	}

	snap2 := &model.PlanSnapshot{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:   scenario2,
		Version:      1,
		Label:        "Scenario 2 Snapshot",
	}

	mockRepo.CreateSnapshot(snap1)
	mockRepo.CreateSnapshot(snap2)

	list1, _ := mockRepo.ListSnapshots(tenantID, scenario1)
	assert.Equal(t, 1, len(list1))
	assert.Equal(t, "Scenario 1 Snapshot", list1[0].Label)

	list2, _ := mockRepo.ListSnapshots(tenantID, scenario2)
	assert.Equal(t, 1, len(list2))
	assert.Equal(t, "Scenario 2 Snapshot", list2[0].Label)
}
