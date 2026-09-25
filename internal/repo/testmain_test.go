//go:build integration

package repo_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"gorm.io/gorm"

	"ascenda/internal/testdb"
)

// globalDB is the single *gorm.DB instance shared across all repo tests.
// Each test wraps its operations in a transaction (see testDB) that is rolled
// back at cleanup, so tests are fully isolated without container restarts.
var globalDB *gorm.DB

func TestMain(m *testing.M) {
	db, cleanup, err := testdb.Connect(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		cleanup()
		os.Exit(1)
	}
	globalDB = db
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// testDB returns a per-test transaction on the shared database; see testdb.Tx.
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testdb.Tx(t, globalDB)
}
