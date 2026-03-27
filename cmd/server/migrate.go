package main

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
	"ascenda/internal/config"
	"ascenda/internal/model"
)

// runMigrations conditionally applies GORM AutoMigrate (controlled by
// cfg.AutoMigrate / DB_AUTO_MIGRATE env var) and then runs idempotent
// post-migration SQL fixes for schema drift accumulated across iterations.
//
// Production guidance
// ───────────────────
// Set DB_AUTO_MIGRATE=false in production and manage schema changes with
// the versioned SQL files in db/migrations/ via `make migrate-up`.
// AutoMigrate is only safe on a clean development database.
//
// All ALTER TABLE / DELETE statements use IF EXISTS / DISTINCT ON so they are
// safe to run on every startup against both fresh and existing databases.
func runMigrations(db *gorm.DB, cfg *config.Config, log *logrus.Entry) error {
	start := time.Now()
	// Use a silent GORM session for migrations to avoid flooding the logs
	// with dozens of individual ALTER TABLE / CREATE INDEX statements.
	quiet := db.Session(&gorm.Session{Logger: db.Logger.LogMode(glogger.Silent)})

	// ── Pre-AutoMigrate fixes ─────────────────────────────────────────────────
	// Must run BEFORE AutoMigrate so that new unique indexes can be created
	// without failing on pre-existing duplicate rows.
	preMigrationFixes := []string{
		// Deduplicate fiplan_entries before adding uix_fiplan_entries.
		`DELETE FROM fiplan_entries
		 WHERE id NOT IN (
		     SELECT DISTINCT ON (scenario_id, line_id, year) id
		     FROM fiplan_entries
		     ORDER BY scenario_id, line_id, year, updated_at DESC
		 )`,
		// Deduplicate pnl_cash_entries before adding uix_pnl_cash_entries.
		`DELETE FROM pnl_cash_entries
		 WHERE id NOT IN (
		     SELECT DISTINCT ON (scenario_id, line_id, year) id
		     FROM pnl_cash_entries
		     ORDER BY scenario_id, line_id, year, updated_at DESC
		 )`,
	}
	for _, sql := range preMigrationFixes {
		if err := quiet.Exec(sql).Error; err != nil {
			return err
		}
	}

	// ── AutoMigrate all models ────────────────────────────────────────────────
	models := []interface{}{
		// Tenant and user models
		&model.Tenant{},
		&model.User{},
		&model.PlanMember{},
		&model.BusinessPlan{},
		&model.Scenario{},

		// Settings models
		&model.PlanConfig{},
		&model.OpeningBalance{},
		&model.WorkingCapitalConfig{},
		&model.OpexPerHire{},
		&model.CapexPerHire{},
		&model.MultiYearAdjustment{},

		// Product models
		&model.Product{},
		&model.ProductAssumption{},
		&model.ProductSalesVolume{},
		&model.ProductDistributorMargin{},

		// Staff models
		&model.StaffHeadcount{},
		&model.StaffSalary{},
		&model.StaffIncentive{},

		// Capex and Opex models
		&model.CapexEntry{},
		&model.OpexManualEntry{},

		// P&L and Financial models
		&model.PnlManualEntry{},
		&model.FiplanEntry{},
		&model.PnlCashEntry{},
		&model.WCREntry{},
		&model.CashMonthlyOverride{},
		&model.BudgetMonthlyOverride{},

		// Report, Snapshot and Audit models
		&model.Report{},
		&model.PlanSnapshot{},
		&model.AuditLog{},

		// AI usage control models
		&model.AIUsagePolicy{},
		&model.AIUsageRecord{},

		// BEP module (Pro tier and above)
		&model.BEPSnapshot{},
		&model.FixedCostLine{},
		&model.VariableCostLine{},
		&model.SensitivityConfig{},
		&model.OptimisationPlan{},
		&model.FixedCostSaving{},
		&model.VariableCostSaving{},
		&model.PCGReviewItem{},

		// Plan-level cap table (Pro tier and above)
		&model.PlanShareholder{},

		// Magic-link sign-in tokens
		&model.MagicLinkToken{},

		// Cap Table module (Enterprise tier)
		&model.CapTableCompany{},
		&model.CapTableShareClass{},
		&model.CapTableShareholder{},
		&model.CapTableRound{},
		&model.CapTablePosition{},
		&model.StockOptionPlan{},
		&model.OptionGrant{},
		&model.ValuationScenario{},
		&model.CapTableScenarioBranch{},
	}

	// ── AutoMigrate gate ──────────────────────────────────────────────────────
	// AutoMigrate is disabled by default in production (DB_AUTO_MIGRATE=false).
	// In production, run `make migrate-up` to apply versioned SQL migrations.
	if cfg.AutoMigrate {
		log.Info("running GORM AutoMigrate (DB_AUTO_MIGRATE=true)")
		if err := quiet.AutoMigrate(models...); err != nil {
			return fmt.Errorf("AutoMigrate failed: %w", err)
		}
		log.WithField("model_count", len(models)).Info("AutoMigrate completed")
	} else {
		log.Info("skipping GORM AutoMigrate (DB_AUTO_MIGRATE=false) — using versioned SQL migrations")
	}
	_ = models // suppress unused-variable error when AutoMigrate is skipped

	// ── Post-AutoMigrate column fixes ─────────────────────────────────────────
	// Orphaned columns left by GORM when field names / tags were corrected across
	// iterations. All statements use DROP COLUMN IF EXISTS so they are idempotent.

	postFixes := []string{
		// staff_* tables: YearIndex was mapped to year_index, now mapped to year.
		`ALTER TABLE staff_headcounts  DROP COLUMN IF EXISTS year_index`,
		`ALTER TABLE staff_salaries    DROP COLUMN IF EXISTS year_index`,
		`ALTER TABLE staff_incentives  DROP COLUMN IF EXISTS year_index`,

		// capex_entries: multiple stale columns from earlier iterations.
		`ALTER TABLE capex_entries DROP COLUMN IF EXISTS category_index`,
		`ALTER TABLE capex_entries DROP COLUMN IF EXISTS year`,
		`ALTER TABLE capex_entries DROP COLUMN IF EXISTS useful_life`,

		// opex_manual_entries: Year int field replaced by YearIndex → year_index.
		`ALTER TABLE opex_manual_entries DROP COLUMN IF EXISTS year`,

		// fiplan_entries / pnl_cash_entries: year_index added by GORM before
		// the gorm:"column:year" tag was added to the YearIndex field.
		`ALTER TABLE fiplan_entries   DROP COLUMN IF EXISTS year_index`,
		`ALTER TABLE pnl_cash_entries DROP COLUMN IF EXISTS year_index`,

		// working_capital_configs: scalar-day columns replaced by pct distribution.
		`ALTER TABLE working_capital_configs DROP COLUMN IF EXISTS customer_payment_days`,
		`ALTER TABLE working_capital_configs DROP COLUMN IF EXISTS supplier_payment_days`,
		`ALTER TABLE working_capital_configs DROP COLUMN IF EXISTS inventory_days`,

		// product child tables: purge orphaned rows left by the pre-cascade-fix bug
		// (child rows whose parent product was already deleted), then add ON DELETE CASCADE.
		// Both steps are idempotent across restarts.
		`DELETE FROM product_assumptions         WHERE product_id NOT IN (SELECT id FROM products)`,
		`DELETE FROM product_sales_volumes       WHERE product_id NOT IN (SELECT id FROM products)`,
		`DELETE FROM product_distributor_margins WHERE product_id NOT IN (SELECT id FROM products)`,
		`ALTER TABLE product_assumptions DROP CONSTRAINT IF EXISTS fk_product_assumptions_product`,
		`ALTER TABLE product_assumptions ADD CONSTRAINT fk_product_assumptions_product FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE`,
		`ALTER TABLE product_sales_volumes DROP CONSTRAINT IF EXISTS fk_product_sales_volumes_product`,
		`ALTER TABLE product_sales_volumes ADD CONSTRAINT fk_product_sales_volumes_product FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE`,
		`ALTER TABLE product_distributor_margins DROP CONSTRAINT IF EXISTS fk_product_distributor_margins_product`,
		`ALTER TABLE product_distributor_margins ADD CONSTRAINT fk_product_distributor_margins_product FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE`,

		// cap_table_rounds: FiPlan sync columns (bidirectional link).
		// AutoMigrate will also add these, but explicit statements ensure idempotency.
		`ALTER TABLE cap_table_rounds ADD COLUMN IF NOT EXISTS fiscal_year_index integer`,
		`ALTER TABLE cap_table_rounds ADD COLUMN IF NOT EXISTS fiplan_synced boolean NOT NULL DEFAULT false`,
		// fiplan_synced_amount_k: snapshot of AmountRaisedK at time of last sync.
		// Divergence = fiplan_synced=true AND fiplan_synced_amount_k != amount_raised_k.
		`ALTER TABLE cap_table_rounds ADD COLUMN IF NOT EXISTS fiplan_synced_amount_k numeric(15,2)`,

		// fiplan_entries: cap table round back-reference for capital_increase rows.
		`ALTER TABLE fiplan_entries ADD COLUMN IF NOT EXISTS cap_table_round_id uuid`,
		`ALTER TABLE fiplan_entries ADD COLUMN IF NOT EXISTS cap_table_round_label varchar(100) NOT NULL DEFAULT ''`,
	}

	for _, sql := range postFixes {
		if err := quiet.Exec(sql).Error; err != nil {
			return err
		}
	}

	log.WithFields(logrus.Fields{
		"models":     len(models),
		"post_fixes": len(postFixes),
		"elapsed":    fmt.Sprintf("%dms", time.Since(start).Milliseconds()),
	}).Info("database migrations completed")
	return nil
}
