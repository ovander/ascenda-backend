package service

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"ascenda/internal/repo"
)

// ScenarioRestorer replaces the data of a scenario with the content of a
// snapshot. The operation is all-or-nothing: on error the scenario is left
// exactly as it was.
type ScenarioRestorer interface {
	RestoreScenarioData(tenantID, scenarioID uuid.UUID, data map[string]json.RawMessage) error
}

// errNoDatabase is returned by a restorer built without a database handle.
var errNoDatabase = errors.New("snapshot restore: no database handle")

// dbScenarioRestorer restores through the snapshot sections inside a single
// database transaction. Every section first clears its tables for the
// scenario, then every section writes the snapshot's rows back, so a restore
// yields the snapshot's state rather than a merge with the current one. The
// first error rolls the whole transaction back (audit §3.1).
type dbScenarioRestorer struct {
	db *gorm.DB
}

func (r *dbScenarioRestorer) RestoreScenarioData(tenantID, scenarioID uuid.UUID, data map[string]json.RawMessage) error {
	if r.db == nil {
		return errNoDatabase
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		sections := registerSections(repo.NewRepoBundle(tx))
		for _, section := range sections {
			if err := section.Clear(tenantID, scenarioID); err != nil {
				return fmt.Errorf("clear %s: %w", section.Name, err)
			}
		}
		for _, section := range sections {
			if err := section.Restore(tenantID, scenarioID, data); err != nil {
				return fmt.Errorf("restore %s: %w", section.Name, err)
			}
		}
		return nil
	})
}
