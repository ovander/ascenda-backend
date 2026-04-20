// Package migrations embeds all versioned SQL migration files so that the
// compiled binary is self-contained — no external migration directory required
// at runtime.
//
// Usage (in cmd/server/migrate_runner.go):
//
//	src, err := iofs.New(migrations.FS, ".")
package migrations

import "embed"

// FS contains all versioned SQL migration files (*.up.sql and *.down.sql).
// Consumed by the golang-migrate iofs source in the server binary.
//
//go:embed *.sql
var FS embed.FS
