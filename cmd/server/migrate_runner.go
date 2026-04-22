package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/sirupsen/logrus"

	dbmigrations "ascenda/migrations"
)

// runSQLMigrations applies all pending versioned SQL migrations from the
// embedded migrations/ directory using golang-migrate.
//
// Applied versions are tracked in a schema_migrations table (created
// automatically on first run). The function is idempotent: already-applied
// migrations are skipped.
//
// On existing deployments that already ran the pre-numbered SQL files manually,
// run once to mark them as applied without re-executing:
//
//	make migrate-force version=12
func runSQLMigrations(databaseURL string, log *logrus.Entry) error {
	src, err := iofs.New(dbmigrations.FS, ".")
	if err != nil {
		return fmt.Errorf("load migration sources: %w", err)
	}

	url := databaseURL

	if !strings.Contains(url, "search_path=") {
		url += "&search_path=public"
	}

	if !strings.Contains(url, "x-migrations-table=") {
		url += "&x-migrations-table=public.schema_migrations"
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, url)

	if err != nil {
		return fmt.Errorf("initialise migrator: %w", err)
	}
	defer m.Close()

	// Route golang-migrate's internal logger through logrus.
	m.Log = &migrateLogger{log}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("read migration version: %w", err)
	}

	log.WithFields(logrus.Fields{
		"version": version,
		"dirty":   dirty,
	}).Info("SQL migrations complete")

	return nil
}

// runMigrateCommand is the entry point for the `migrate` sub-command.
// It applies all pending SQL migrations and exits — no HTTP server is started.
// Exit code 0 on success, 1 on any error (so `set -e` in deploy scripts works).
func runMigrateCommand(databaseURL string) {
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	log.SetLevel(logrus.InfoLevel)
	entry := log.WithField("cmd", "migrate")

	entry.Info("running SQL migrations (migrate-only mode)")

	if err := runSQLMigrations(databaseURL, entry); err != nil {
		entry.WithError(err).Fatal("migration failed")
		os.Exit(1)
	}

	entry.Info("migrations complete — exiting")
	os.Exit(0)
}

// migrateLogger adapts logrus to the migrate.Logger interface.
type migrateLogger struct{ log *logrus.Entry }

func (l *migrateLogger) Printf(format string, v ...interface{}) {
	l.log.Debugf(format, v...)
}
func (l *migrateLogger) Verbose() bool { return true }
