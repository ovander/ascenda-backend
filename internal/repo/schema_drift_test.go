//go:build integration

package repo_test

// Guards against drift between the SQL migrations (what production runs) and
// the GORM models (what the code reads and writes, and what development
// AutoMigrate applies). The test database is provisioned from the migrations
// alone (see testdb.Connect); running AutoMigrate over every table model on
// top of it must be a no-op. Any DDL GORM wants to issue is drift: fix the
// model tags or add a migration, then rerun.

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"

	"ascenda/internal/model"
)

// ddlRecorder is a GORM logger that keeps every DDL statement it sees.
type ddlRecorder struct {
	glogger.Interface
	mu         sync.Mutex
	statements []string
}

var ddlStatement = regexp.MustCompile(`(?i)^\s*(ALTER|CREATE|DROP|COMMENT)\b`)

func (r *ddlRecorder) LogMode(glogger.LogLevel) glogger.Interface { return r }

func (r *ddlRecorder) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	if ddlStatement.MatchString(sql) {
		r.mu.Lock()
		r.statements = append(r.statements, sql)
		r.mu.Unlock()
	}
}

func TestSchema_MigrationsMatchModels(t *testing.T) {
	db := testDB(t) // rolled back at cleanup: any DDL below is undone

	rec := &ddlRecorder{Interface: glogger.Discard}
	err := db.Session(&gorm.Session{Logger: rec}).AutoMigrate(model.TableModels()...)

	require.NoError(t, err, "AutoMigrate failed on the migration-provisioned schema (drift it cannot even reconcile)")
	require.Empty(t, rec.statements,
		"the SQL migrations and the GORM models disagree; AutoMigrate would run:\n  %s\n"+
			"Fix the model tags or add a migration so that both describe the same schema.",
		strings.Join(rec.statements, ";\n  "))
}
