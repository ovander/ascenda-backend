package service

// copyScenarioData drives CloneScenario: every section is captured from the
// source and written to the destination, and the first error stops the copy
// so the surrounding transaction rolls back.

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubSection struct {
	name       string
	captureErr error
	restoreErr error
	captured   []uuid.UUID
	restored   []uuid.UUID
	seen       map[string]json.RawMessage
}

func (st *stubSection) section() SnapshotSection {
	return SnapshotSection{
		Name: st.name,
		Capture: func(_ uuid.UUID, scenarioID uuid.UUID) (map[string]json.RawMessage, error) {
			st.captured = append(st.captured, scenarioID)
			if st.captureErr != nil {
				return nil, st.captureErr
			}
			return map[string]json.RawMessage{st.name + "Rows": json.RawMessage(`[1]`)}, nil
		},
		Clear: func(uuid.UUID, uuid.UUID) error { return nil },
		Restore: func(_ uuid.UUID, scenarioID uuid.UUID, data map[string]json.RawMessage) error {
			st.restored = append(st.restored, scenarioID)
			st.seen = data
			return st.restoreErr
		},
	}
}

func TestCopyScenarioData_CapturesSourceAndWritesDestination(t *testing.T) {
	src, dst := uuid.New(), uuid.New()
	a, b := &stubSection{name: "a"}, &stubSection{name: "b"}

	require.NoError(t, copyScenarioData([]SnapshotSection{a.section(), b.section()}, uuid.New(), src, dst))

	assert.Equal(t, []uuid.UUID{src}, a.captured)
	assert.Equal(t, []uuid.UUID{dst}, a.restored)
	assert.Equal(t, []uuid.UUID{dst}, b.restored)
	// Every section sees the merged data of all sections, as in a restore.
	assert.Contains(t, b.seen, "aRows")
	assert.Contains(t, b.seen, "bRows")
}

func TestCopyScenarioData_ReadErrorStopsBeforeAnyWrite(t *testing.T) {
	a, b := &stubSection{name: "a"}, &stubSection{name: "b", captureErr: errors.New("db down")}
	c := &stubSection{name: "c"}

	err := copyScenarioData([]SnapshotSection{a.section(), b.section(), c.section()}, uuid.New(), uuid.New(), uuid.New())
	require.ErrorContains(t, err, "capture b: db down")
	assert.Empty(t, a.restored, "nothing is written when a read fails")
	assert.Empty(t, c.captured, "later sections are not read")
}

func TestCopyScenarioData_WriteErrorIsReturned(t *testing.T) {
	a := &stubSection{name: "a", restoreErr: errors.New("unique violation")}
	b := &stubSection{name: "b"}

	err := copyScenarioData([]SnapshotSection{a.section(), b.section()}, uuid.New(), uuid.New(), uuid.New())
	require.ErrorContains(t, err, "copy a: unique violation")
	assert.Empty(t, b.restored, "the copy stops at the first failed write")
}
