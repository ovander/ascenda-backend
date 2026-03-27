//go:build integration

package repo_test

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

	"ascenda/internal/model"
)

// globalDB is the single *gorm.DB instance shared across all repo tests.
// Each test wraps its operations in a transaction (see testDB) that is rolled
// back at cleanup, so tests are fully isolated without container restarts.
var globalDB *gorm.DB

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("kerplan_test"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to start postgres container: %v\n", err)
		os.Exit(1)
	}
	defer pgContainer.Terminate(ctx) //nolint:errcheck

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to get connection string: %v\n", err)
		os.Exit(1)
	}

	db, err := gorm.Open(postgres.Open(connStr), &gorm.Config{
		Logger: glogger.Default.LogMode(glogger.Silent),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to open db: %v\n", err)
		os.Exit(1)
	}

	if err := migrateAll(db); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: AutoMigrate failed: %v\n", err)
		os.Exit(1)
	}

	globalDB = db
	os.Exit(m.Run())
}

// migrateAll runs GORM AutoMigrate for every model used by repo tests.
// The set mirrors the list in cmd/server/migrate.go so tests exercise the
// real schema, not a hand-crafted subset.
func migrateAll(db *gorm.DB) error {
	return db.AutoMigrate(
		// Core tenant/user models
		&model.Tenant{},
		&model.User{},
		&model.PlanMember{},

		// Planning core
		&model.BusinessPlan{},
		&model.Scenario{},

		// Settings sub-tables
		&model.PlanConfig{},
		&model.OpeningBalance{},
		&model.WorkingCapitalConfig{},
		&model.OpexPerHire{},
		&model.CapexPerHire{},
		&model.MultiYearAdjustment{},

		// Products
		&model.Product{},
		&model.ProductAssumption{},
		&model.ProductSalesVolume{},
		&model.ProductDistributorMargin{},

		// Staff
		&model.StaffHeadcount{},
		&model.StaffSalary{},
		&model.StaffIncentive{},

		// Financial entries
		&model.CapexEntry{},
		&model.OpexManualEntry{},
		&model.PnlManualEntry{},
		&model.FiplanEntry{},
		&model.PnlCashEntry{},
		&model.WCREntry{},
		&model.CashMonthlyOverride{},
		&model.BudgetMonthlyOverride{},

		// Reporting / audit
		&model.Report{},
		&model.PlanSnapshot{},
		&model.AuditLog{},

		// Auth
		&model.MagicLinkToken{},

		// Cap table
		&model.CapTableCompany{},
		&model.CapTableShareClass{},
		&model.CapTableShareholder{},
		&model.CapTableRound{},
		&model.CapTablePosition{},
		&model.StockOptionPlan{},
		&model.OptionGrant{},
		&model.ValuationScenario{},
		&model.CapTableScenarioBranch{},

		// BEP
		&model.BEPSnapshot{},
		&model.FixedCostLine{},
		&model.VariableCostLine{},
		&model.SensitivityConfig{},
		&model.OptimisationPlan{},
		&model.FixedCostSaving{},
		&model.VariableCostSaving{},
		&model.PCGReviewItem{},

		// Pro-tier extras
		&model.PlanShareholder{},
		&model.AIUsagePolicy{},
		&model.AIUsageRecord{},
	)
}

// testDB returns a *gorm.DB that wraps the global connection in a database
// transaction.  The transaction is automatically rolled back when the test
// ends, giving each test a clean, isolated view of the database without
// truncating tables between runs.
//
// GORM's nested-transaction support (via SAVEPOINTs) means that code under
// test can still call db.Transaction(…) normally — those inner transactions
// become savepoints on the outer test transaction and are rolled back with it.
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	tx := globalDB.Begin()
	if tx.Error != nil {
		t.Fatalf("testDB: begin transaction: %v", tx.Error)
	}
	t.Cleanup(func() {
		if err := tx.Rollback().Error; err != nil && err != gorm.ErrInvalidTransaction {
			// ErrInvalidTransaction is expected if the test already committed.
			t.Logf("testDB cleanup: rollback: %v", err)
		}
	})
	return tx
}
