//go:build integration

// Package testdb provisions the PostgreSQL database shared by the integration
// test suites (build tag `integration`). Every test package that needs a real
// database calls Connect from its TestMain and wraps each test in Tx.
package testdb

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"

	dbmigrations "ascenda/migrations"
)

// Connect opens a migrated PostgreSQL database for a test binary.
//
// TEST_DATABASE_URL points the suite at an existing database (local
// development without Docker). Otherwise a throwaway postgres:16 container is
// started with testcontainers. Either way the schema is provisioned from the
// embedded SQL migrations — the same path production uses — so the tests fail
// if a model drifts from the migrations. The returned cleanup terminates the
// container, if one was started.
func Connect(ctx context.Context) (db *gorm.DB, cleanup func(), err error) {
	cleanup = func() {}
	connStr := os.Getenv("TEST_DATABASE_URL")
	if connStr == "" {
		pgContainer, err := tcpostgres.Run(ctx,
			"postgres:16-alpine",
			tcpostgres.WithDatabase("ascenda_test"),
			tcpostgres.WithUsername("testuser"),
			tcpostgres.WithPassword("testpass"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2),
			),
		)
		if err != nil {
			return nil, cleanup, fmt.Errorf("start postgres container: %w", err)
		}
		cleanup = func() { _ = pgContainer.Terminate(ctx) }

		connStr, err = pgContainer.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			return nil, cleanup, fmt.Errorf("container connection string: %w", err)
		}
	}

	if _, _, err := dbmigrations.Apply(connStr, nil); err != nil {
		return nil, cleanup, fmt.Errorf("apply SQL migrations: %w", err)
	}

	db, err = gorm.Open(postgres.Open(connStr), &gorm.Config{
		Logger: glogger.Default.LogMode(glogger.Silent),
	})
	if err != nil {
		return nil, cleanup, fmt.Errorf("open database: %w", err)
	}
	return db, cleanup, nil
}

// Tx returns a *gorm.DB that wraps db in a database transaction, rolled back
// when the test ends. Each test gets a clean, isolated view of the database
// without truncating tables between runs.
//
// GORM's nested-transaction support (SAVEPOINTs) means code under test can
// still call db.Transaction(…) normally — those inner transactions become
// savepoints on the outer test transaction and are rolled back with it.
func Tx(t *testing.T, db *gorm.DB) *gorm.DB {
	t.Helper()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("testdb: begin transaction: %v", tx.Error)
	}
	t.Cleanup(func() {
		if err := tx.Rollback().Error; err != nil && err != gorm.ErrInvalidTransaction {
			// ErrInvalidTransaction is expected if the test already committed.
			t.Logf("testdb cleanup: rollback: %v", err)
		}
	})
	return tx
}
