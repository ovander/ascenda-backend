package main

import (
	"os"

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
	version, dirty, err := dbmigrations.Apply(databaseURL, &migrateLogger{log})
	if err != nil {
		return err
	}
	log.WithFields(logrus.Fields{
		"version": version,
		"dirty":   dirty,
	}).Info("SQL migrations complete")
	return nil
}

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
