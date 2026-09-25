package service

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"ascenda/internal/event"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/pagination"
	"ascenda/internal/repo"
)

// SnapshotService orchestrates plan versioning and snapshots.
type SnapshotService struct {
	snapshotRepo repo.SnapshotRepository
	scenarioRepo repo.ScenarioRepository
	sections     []SnapshotSection
	emitter      *event.Emitter
	logger       *logrus.Entry
}

// NewSnapshotService creates a new SnapshotService.
func NewSnapshotService(snapshotRepo repo.SnapshotRepository, repos *repo.RepoBundle, emitter *event.Emitter, logger *logrus.Entry) *SnapshotService {
	return &SnapshotService{
		snapshotRepo: snapshotRepo,
		scenarioRepo: repos.Scenario,
		sections:     registerSections(repos),
		emitter:      emitter,
		logger:       logger,
	}
}

// getInScenario loads a snapshot and verifies it belongs to scenarioID.
// A snapshot from another scenario is reported as not found so that IDs are
// not confirmed across plans.
func (s *SnapshotService) getInScenario(tenantID, scenarioID, snapshotID uuid.UUID) (*model.PlanSnapshot, error) {
	snapshot, err := s.snapshotRepo.GetByID(tenantID, snapshotID)
	if err != nil || snapshot == nil || snapshot.TenantID != tenantID {
		s.logger.WithError(err).WithField("snapshot_id", snapshotID).Warn("snapshot not found")
		return nil, apierror.NotFound("snapshot", snapshotID.String())
	}
	if snapshot.ScenarioID != scenarioID {
		s.logger.WithFields(logrus.Fields{
			"snapshot_id":       snapshotID,
			"scenario_id":       scenarioID,
			"snapshot_scenario": snapshot.ScenarioID,
		}).Warn("snapshot does not belong to scenario in URL — access denied")
		return nil, apierror.NotFound("snapshot", snapshotID.String())
	}
	return snapshot, nil
}

// Create captures the current state of a scenario as a snapshot.
func (s *SnapshotService) Create(ctx context.Context, tenantID, scenarioID uuid.UUID, label, description string) (*model.PlanSnapshot, error) {
	userID := ctxutil.GetUserID(ctx)
	if userID == uuid.Nil {
		return nil, apierror.Unauthorized("missing user in context")
	}

	// Get next version number by listing existing snapshots
	existingSnapshots, err := s.snapshotRepo.ListByScenario(tenantID, scenarioID, 0, 1000)
	maxVersion := 0
	if err == nil {
		for _, snap := range existingSnapshots {
			if snap.Version > maxVersion {
				maxVersion = snap.Version
			}
		}
	}

	// Serialize all scenario data
	data, err := s.captureScenarioData(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to capture scenario data")
		return nil, apierror.Internal("failed to capture scenario data")
	}

	snapshot := &model.PlanSnapshot{
		TenantScoped: model.TenantScoped{ID: uuid.New()},
		ScenarioID:   scenarioID,
		Version:      maxVersion + 1,
		Label:        label,
		Description:  description,
		Data:         data,
		CreatedBy:    userID,
	}
	snapshot.TenantID = tenantID

	if err := s.snapshotRepo.Create(snapshot); err != nil {
		s.logger.WithError(err).Error("failed to save snapshot")
		return nil, apierror.Internal("failed to save snapshot")
	}

	s.logger.WithField("snapshot_id", snapshot.ID).WithField("scenario_id", scenarioID).Info("snapshot created")
	s.emitter.Publish(event.Event{
		Type:       event.SnapshotCreated,
		TenantID:   tenantID,
		UserID:     userID,
		ScenarioID: scenarioID,
		EntityType: "snapshot",
		EntityID:   snapshot.ID,
		Action:     event.ActionCreate,
	})
	return snapshot, nil
}

// Restore restores a scenario from one of its own snapshots.
func (s *SnapshotService) Restore(ctx context.Context, tenantID, scenarioID, snapshotID uuid.UUID) error {
	snapshot, err := s.getInScenario(tenantID, scenarioID, snapshotID)
	if err != nil {
		return err
	}

	// Deserialize and restore data
	if err := s.restoreScenarioData(tenantID, snapshot.ScenarioID, snapshot.Data); err != nil {
		s.logger.WithError(err).Error("failed to restore snapshot")
		return apierror.Internal("failed to restore snapshot")
	}

	s.logger.WithField("snapshot_id", snapshotID).WithField("scenario_id", snapshot.ScenarioID).Info("snapshot restored")
	s.emitter.Publish(event.Event{
		Type:       event.SnapshotRestored,
		TenantID:   tenantID,
		UserID:     ctxutil.GetUserID(ctx),
		ScenarioID: snapshot.ScenarioID,
		EntityType: "snapshot",
		EntityID:   snapshotID,
		Action:     event.ActionRestore,
	})
	return nil
}

// CloneToScenario writes a snapshot's data into newScenarioID. The snapshot
// must belong to scenarioID (the scenario in the URL) and the target scenario
// must exist in the same plan — the caller's edit rights were checked against
// that plan, so they carry over to the target.
func (s *SnapshotService) CloneToScenario(ctx context.Context, tenantID, scenarioID, snapshotID, newScenarioID uuid.UUID) error {
	snapshot, err := s.getInScenario(tenantID, scenarioID, snapshotID)
	if err != nil {
		return err
	}

	source, err := s.scenarioRepo.GetByID(tenantID, scenarioID)
	if err != nil || source == nil {
		return apierror.NotFound("scenario", scenarioID.String())
	}
	target, err := s.scenarioRepo.GetByID(tenantID, newScenarioID)
	if err != nil || target == nil {
		return apierror.NotFound("scenario", newScenarioID.String())
	}
	if target.PlanID != source.PlanID {
		s.logger.WithFields(logrus.Fields{
			"snapshot_id":     snapshotID,
			"source_plan_id":  source.PlanID,
			"target_scenario": newScenarioID,
			"target_plan_id":  target.PlanID,
		}).Warn("snapshot clone target belongs to another plan — access denied")
		return apierror.NotFound("scenario", newScenarioID.String())
	}

	// Restore to new scenario
	if err := s.restoreScenarioData(tenantID, newScenarioID, snapshot.Data); err != nil {
		s.logger.WithError(err).Error("failed to clone snapshot")
		return apierror.Internal("failed to clone snapshot")
	}

	s.logger.WithField("snapshot_id", snapshotID).WithField("new_scenario_id", newScenarioID).Info("snapshot cloned to scenario")
	return nil
}

// Diff compares two snapshots of the same scenario by returning their raw data maps.
func (s *SnapshotService) Diff(ctx context.Context, tenantID, scenarioID, snapshot1ID, snapshot2ID uuid.UUID) (map[string]interface{}, error) {
	snap1, err := s.getInScenario(tenantID, scenarioID, snapshot1ID)
	if err != nil {
		return nil, err
	}
	data1Raw := snap1.Data

	snap2, err := s.getInScenario(tenantID, scenarioID, snapshot2ID)
	if err != nil {
		return nil, err
	}
	data2Raw := snap2.Data

	// Parse both into generic maps for comparison
	var data1, data2 map[string]json.RawMessage
	if err := json.Unmarshal(data1Raw, &data1); err != nil {
		return nil, apierror.Internal("failed to parse snapshot1 data")
	}
	if err := json.Unmarshal(data2Raw, &data2); err != nil {
		return nil, apierror.Internal("failed to parse snapshot2 data")
	}

	// Identify changed sections
	diff := make(map[string]interface{})
	allKeys := make(map[string]bool)
	for k := range data1 {
		allKeys[k] = true
	}
	for k := range data2 {
		allKeys[k] = true
	}

	for key := range allKeys {
		raw1, ok1 := data1[key]
		raw2, ok2 := data2[key]

		if !ok1 {
			diff[key] = map[string]interface{}{"status": "added", "after": json.RawMessage(raw2)}
		} else if !ok2 {
			diff[key] = map[string]interface{}{"status": "removed", "before": json.RawMessage(raw1)}
		} else if string(raw1) != string(raw2) {
			diff[key] = map[string]interface{}{"status": "changed", "before": json.RawMessage(raw1), "after": json.RawMessage(raw2)}
		}
	}

	return diff, nil
}

// List lists all snapshots for a scenario (without data).
func (s *SnapshotService) List(ctx context.Context, tenantID, scenarioID uuid.UUID, params pagination.Params) ([]model.PlanSnapshot, int64, error) {
	ptrSnapshots, err := s.snapshotRepo.ListByScenario(tenantID, scenarioID, params.Offset, params.PerPage)
	if err != nil {
		s.logger.WithError(err).Error("failed to list snapshots")
		return nil, 0, apierror.Internal("failed to list snapshots")
	}
	snapshots := make([]model.PlanSnapshot, len(ptrSnapshots))
	for i, snap := range ptrSnapshots {
		snapshots[i] = *snap
	}
	return snapshots, int64(len(snapshots)), nil
}

// Get retrieves a snapshot by ID (without data).
func (s *SnapshotService) Get(ctx context.Context, tenantID, scenarioID, snapshotID uuid.UUID) (*model.PlanSnapshot, error) {
	return s.getInScenario(tenantID, scenarioID, snapshotID)
}

// GetData retrieves raw snapshot data.
func (s *SnapshotService) GetData(ctx context.Context, tenantID, scenarioID, snapshotID uuid.UUID) (json.RawMessage, error) {
	snapshot, err := s.getInScenario(tenantID, scenarioID, snapshotID)
	if err != nil {
		return nil, err
	}
	return snapshot.Data, nil
}

// Delete removes a snapshot.
func (s *SnapshotService) Delete(ctx context.Context, tenantID, scenarioID, snapshotID uuid.UUID) error {
	// Verify the snapshot belongs to the scenario in the URL.
	if _, err := s.getInScenario(tenantID, scenarioID, snapshotID); err != nil {
		return err
	}

	if err := s.snapshotRepo.Delete(tenantID, snapshotID); err != nil {
		s.logger.WithError(err).Error("failed to delete snapshot")
		return apierror.Internal("failed to delete snapshot")
	}

	s.logger.WithField("snapshot_id", snapshotID).Info("snapshot deleted")
	return nil
}

// -----------------------------------------------------------------------
// Internal helpers
// -----------------------------------------------------------------------

// CaptureScenarioData is the public variant of captureScenarioData.
// It serialises the current state of every entity section (settings, products,
// staff, capex, opex, pnl, fiplan, pnlCash, wcr, cash, budget) into a single
// JSON object.  Used by the audit detail endpoint to embed a full scenario
// snapshot alongside a single audit entry download.
func (s *SnapshotService) CaptureScenarioData(tenantID, scenarioID uuid.UUID) (json.RawMessage, error) {
	return s.captureScenarioData(tenantID, scenarioID)
}

// captureScenarioData iterates over registered sections to serialize all
// scenario entities into a single JSON map. The output is backward-compatible
// with the SnapshotData struct keys.
func (s *SnapshotService) captureScenarioData(tenantID, scenarioID uuid.UUID) (json.RawMessage, error) {
	combined := make(map[string]json.RawMessage)

	for _, section := range s.sections {
		pairs, err := section.Capture(tenantID, scenarioID)
		if err != nil {
			s.logger.WithError(err).WithField("section", section.Name).Warn("snapshot capture failed for section")
			continue
		}
		for k, v := range pairs {
			combined[k] = v
		}
	}

	return json.Marshal(combined)
}

// restoreScenarioData parses the snapshot JSON into a key→RawMessage map
// and delegates to each registered section for deserialization and repo writes.
func (s *SnapshotService) restoreScenarioData(tenantID, scenarioID uuid.UUID, dataJSON json.RawMessage) error {
	var data map[string]json.RawMessage
	if err := json.Unmarshal(dataJSON, &data); err != nil {
		return err
	}

	for _, section := range s.sections {
		if err := section.Restore(tenantID, scenarioID, data); err != nil {
			s.logger.WithError(err).WithField("section", section.Name).Warn("snapshot restore failed for section")
		}
	}

	s.logger.WithField("scenario_id", scenarioID).Info("scenario data restored from snapshot")
	return nil
}
