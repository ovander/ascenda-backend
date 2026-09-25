package service

// Tests for the scenario/plan binding of snapshot operations (audit finding
// S-H4): every snapshot lookup must be scoped to the scenario in the URL, and
// CloneToScenario must refuse targets outside the source scenario's plan.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"ascenda/internal/event"
	"ascenda/internal/model"
	"ascenda/internal/repo"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSnapshotRepo satisfies repo.SnapshotRepository with an in-memory map.
type fakeSnapshotRepo struct {
	snapshots map[uuid.UUID]*model.PlanSnapshot
	deleted   []uuid.UUID
}

func newFakeSnapshotRepo() *fakeSnapshotRepo {
	return &fakeSnapshotRepo{snapshots: make(map[uuid.UUID]*model.PlanSnapshot)}
}

func (f *fakeSnapshotRepo) Create(s *model.PlanSnapshot) error { f.snapshots[s.ID] = s; return nil }

func (f *fakeSnapshotRepo) GetByID(tenantID, snapshotID uuid.UUID) (*model.PlanSnapshot, error) {
	if s, ok := f.snapshots[snapshotID]; ok && s.TenantID == tenantID {
		return s, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeSnapshotRepo) ListByScenario(tenantID, scenarioID uuid.UUID, offset, limit int) ([]*model.PlanSnapshot, error) {
	return nil, nil
}

func (f *fakeSnapshotRepo) ListByPlan(tenantID, planID uuid.UUID, offset, limit int) ([]*model.PlanSnapshot, error) {
	return nil, nil
}

func (f *fakeSnapshotRepo) Delete(tenantID, snapshotID uuid.UUID) error {
	f.deleted = append(f.deleted, snapshotID)
	delete(f.snapshots, snapshotID)
	return nil
}

var _ repo.SnapshotRepository = (*fakeSnapshotRepo)(nil)

// recordingRestorer stands in for the transactional restorer: it records the
// scenario each restore targets and can be told to fail.
type recordingRestorer struct {
	calls []uuid.UUID
	data  []map[string]json.RawMessage
	err   error
}

func (r *recordingRestorer) RestoreScenarioData(_ uuid.UUID, scenarioID uuid.UUID, data map[string]json.RawMessage) error {
	r.calls = append(r.calls, scenarioID)
	r.data = append(r.data, data)
	return r.err
}

type snapshotFixture struct {
	svc        *SnapshotService
	snapRepo   *fakeSnapshotRepo
	restorer   *recordingRestorer
	events     []event.Event
	tenantID   uuid.UUID
	planA      uuid.UUID
	planB      uuid.UUID
	scenA1     uuid.UUID // plan A
	scenA2     uuid.UUID // plan A
	scenB1     uuid.UUID // plan B
	snapOfA1   uuid.UUID // snapshot taken on scenA1
	snapOfB1   uuid.UUID // snapshot taken on scenB1
	snapOfA1v2 uuid.UUID // second snapshot on scenA1
}

func newSnapshotFixture(t *testing.T) *snapshotFixture {
	t.Helper()
	logger := logrus.NewEntry(logrus.New())
	logger.Logger.SetLevel(logrus.PanicLevel)

	f := &snapshotFixture{
		snapRepo: newFakeSnapshotRepo(),
		restorer: &recordingRestorer{},
		tenantID: uuid.New(),
		planA:    uuid.New(),
		planB:    uuid.New(),
	}
	scenRepo := newInMemScenarioRepo()
	mk := func(planID uuid.UUID) uuid.UUID {
		s := &model.Scenario{TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: f.tenantID}, PlanID: planID}
		require.NoError(t, scenRepo.Create(s))
		return s.ID
	}
	f.scenA1, f.scenA2, f.scenB1 = mk(f.planA), mk(f.planA), mk(f.planB)

	snap := func(scenarioID uuid.UUID) uuid.UUID {
		s := &model.PlanSnapshot{
			TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: f.tenantID},
			ScenarioID:   scenarioID,
			Data:         json.RawMessage(`{"capexEntries":[]}`),
		}
		require.NoError(t, f.snapRepo.Create(s))
		return s.ID
	}
	f.snapOfA1, f.snapOfA1v2, f.snapOfB1 = snap(f.scenA1), snap(f.scenA1), snap(f.scenB1)

	emitter := event.NewEmitter(logger)
	emitter.Subscribe(func(e event.Event) { f.events = append(f.events, e) })
	t.Cleanup(emitter.Close)

	// Only the scenario repo is needed: writes go through the recording
	// restorer instead of the database-backed one.
	f.svc = NewSnapshotService(f.snapRepo, &repo.RepoBundle{Scenario: scenRepo}, emitter, logger)
	f.svc.restorer = f.restorer
	return f
}

func assertNotFound(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var appErr *apierror.AppError
	require.True(t, errors.As(err, &appErr), "expected *apierror.AppError, got %T", err)
	assert.Equal(t, http.StatusNotFound, appErr.StatusCode)
}

func TestSnapshotBinding_GetScopedToScenario(t *testing.T) {
	f := newSnapshotFixture(t)
	ctx := context.Background()

	got, err := f.svc.Get(ctx, f.tenantID, f.scenA1, f.snapOfA1)
	require.NoError(t, err)
	assert.Equal(t, f.snapOfA1, got.ID)

	// Same tenant, snapshot of another scenario/plan → not found.
	_, err = f.svc.Get(ctx, f.tenantID, f.scenA1, f.snapOfB1)
	assertNotFound(t, err)

	// Sibling scenario in the same plan is still a different scenario.
	_, err = f.svc.Get(ctx, f.tenantID, f.scenA2, f.snapOfA1)
	assertNotFound(t, err)

	// Foreign tenant.
	_, err = f.svc.Get(ctx, uuid.New(), f.scenA1, f.snapOfA1)
	assertNotFound(t, err)
}

func TestSnapshotBinding_GetDataScopedToScenario(t *testing.T) {
	f := newSnapshotFixture(t)
	ctx := context.Background()

	data, err := f.svc.GetData(ctx, f.tenantID, f.scenA1, f.snapOfA1)
	require.NoError(t, err)
	assert.JSONEq(t, `{"capexEntries":[]}`, string(data))

	_, err = f.svc.GetData(ctx, f.tenantID, f.scenA1, f.snapOfB1)
	assertNotFound(t, err)
}

func TestSnapshotBinding_DeleteScopedToScenario(t *testing.T) {
	f := newSnapshotFixture(t)
	ctx := context.Background()

	err := f.svc.Delete(ctx, f.tenantID, f.scenA1, f.snapOfB1)
	assertNotFound(t, err)
	assert.Empty(t, f.snapRepo.deleted, "a snapshot of another scenario must not be deleted")

	require.NoError(t, f.svc.Delete(ctx, f.tenantID, f.scenA1, f.snapOfA1))
	assert.Equal(t, []uuid.UUID{f.snapOfA1}, f.snapRepo.deleted)
}

func TestSnapshotBinding_RestoreScopedToScenario(t *testing.T) {
	f := newSnapshotFixture(t)
	ctx := context.Background()

	// Restoring a snapshot that belongs to another scenario must be refused,
	// even though the snapshot itself would be found by (tenant, id).
	err := f.svc.Restore(ctx, f.tenantID, f.scenA1, f.snapOfB1)
	assertNotFound(t, err)
	assert.Empty(t, f.restorer.calls, "a refused restore must not touch the scenario")

	require.NoError(t, f.svc.Restore(ctx, f.tenantID, f.scenA1, f.snapOfA1))
	require.Equal(t, []uuid.UUID{f.scenA1}, f.restorer.calls)
	assert.Contains(t, f.restorer.data[0], "capexEntries", "restorer receives the parsed snapshot")
	require.Len(t, f.events, 1)
	assert.Equal(t, event.SnapshotRestored, f.events[0].Type)
	assert.Equal(t, f.scenA1, f.events[0].ScenarioID)
}

func TestSnapshotBinding_RestoreFailureIsReportedAndEmitsNothing(t *testing.T) {
	f := newSnapshotFixture(t)
	f.restorer.err = errors.New("write products: boom")

	err := f.svc.Restore(context.Background(), f.tenantID, f.scenA1, f.snapOfA1)
	require.Error(t, err)
	var appErr *apierror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, http.StatusInternalServerError, appErr.StatusCode)
	assert.Empty(t, f.events, "no SnapshotRestored event for a failed restore")
}

func TestSnapshotBinding_RestoreRejectsCorruptSnapshotData(t *testing.T) {
	f := newSnapshotFixture(t)
	f.snapRepo.snapshots[f.snapOfA1].Data = json.RawMessage(`{not json`)

	err := f.svc.Restore(context.Background(), f.tenantID, f.scenA1, f.snapOfA1)
	require.Error(t, err)
	assert.Empty(t, f.restorer.calls, "unparseable data must fail before any write")
	assert.Empty(t, f.events)
}

func TestSnapshotBinding_RestorerWithoutDatabaseFails(t *testing.T) {
	// The default restorer is built from RepoBundle.DB; a bundle without a
	// handle must fail loudly rather than panic or pretend to restore.
	r := &dbScenarioRestorer{}
	err := r.RestoreScenarioData(uuid.New(), uuid.New(), map[string]json.RawMessage{})
	assert.ErrorIs(t, err, errNoDatabase)
}

func TestSnapshotBinding_DiffRequiresBothInScenario(t *testing.T) {
	f := newSnapshotFixture(t)
	ctx := context.Background()

	diff, err := f.svc.Diff(ctx, f.tenantID, f.scenA1, f.snapOfA1, f.snapOfA1v2)
	require.NoError(t, err)
	assert.Empty(t, diff)

	_, err = f.svc.Diff(ctx, f.tenantID, f.scenA1, f.snapOfA1, f.snapOfB1)
	assertNotFound(t, err)

	_, err = f.svc.Diff(ctx, f.tenantID, f.scenA1, f.snapOfB1, f.snapOfA1)
	assertNotFound(t, err)
}

func TestSnapshotBinding_CloneTargetMustBeInSamePlan(t *testing.T) {
	f := newSnapshotFixture(t)
	ctx := context.Background()

	// Target in the same plan → allowed, and the target is what gets written.
	require.NoError(t, f.svc.CloneToScenario(ctx, f.tenantID, f.scenA1, f.snapOfA1, f.scenA2))
	require.Equal(t, []uuid.UUID{f.scenA2}, f.restorer.calls)

	// Target in another plan of the same tenant → refused.
	err := f.svc.CloneToScenario(ctx, f.tenantID, f.scenA1, f.snapOfA1, f.scenB1)
	assertNotFound(t, err)

	// Unknown target scenario → refused.
	err = f.svc.CloneToScenario(ctx, f.tenantID, f.scenA1, f.snapOfA1, uuid.New())
	assertNotFound(t, err)

	// Snapshot not in the URL scenario → refused before any target check.
	err = f.svc.CloneToScenario(ctx, f.tenantID, f.scenA1, f.snapOfB1, f.scenA2)
	assertNotFound(t, err)

	assert.Len(t, f.restorer.calls, 1, "refused clones never reach the restorer")
}
