package migrations

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // postgres driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Logger receives progress messages from golang-migrate.
type Logger interface {
	Printf(format string, v ...interface{})
}

// Apply runs every pending versioned SQL migration embedded in FS against the
// PostgreSQL database at databaseURL and returns the resulting version.
//
// It is the single code path for provisioning a schema: the server's
// `migrate` sub-command, the startup migration phase and the repository
// integration tests all call it, so a fresh database always ends up with the
// schema that the migrations describe — starting from 000000_baseline.
//
// The migrations table is pinned to public.schema_migrations and the search
// path to public, matching the deployed databases.
func Apply(databaseURL string, log Logger) (version uint, dirty bool, err error) {
	src, err := iofs.New(FS, ".")
	if err != nil {
		return 0, false, fmt.Errorf("load migration sources: %w", err)
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
		return 0, false, fmt.Errorf("initialise migrator: %w", err)
	}
	defer func() {
		// Close releases the source and the database connection; a failure
		// here cannot undo an applied migration, so it is only logged.
		if srcErr, dbErr := m.Close(); (srcErr != nil || dbErr != nil) && log != nil {
			log.Printf("migrations: close: source=%v database=%v", srcErr, dbErr)
		}
	}()

	if log != nil {
		m.Log = &migrateLogger{log}
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return 0, false, fmt.Errorf("apply migrations: %w", err)
	}

	version, dirty, err = m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, fmt.Errorf("read migration version: %w", err)
	}
	return version, dirty, nil
}

// migrateLogger adapts Logger to golang-migrate's Logger interface.
type migrateLogger struct{ log Logger }

func (l *migrateLogger) Printf(format string, v ...interface{}) { l.log.Printf(format, v...) }
func (l *migrateLogger) Verbose() bool                          { return false }
