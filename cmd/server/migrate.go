package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"

	"ascenda/internal/config"
	"ascenda/internal/model"
)

// runMigrations executes database schema migrations in a deterministic,
// production-safe order:
//
// 1. Run versioned SQL migrations (source of truth)
// 2. Run optional data fixes (safe / idempotent)
// 3. Run GORM AutoMigrate (DEV ONLY)
//
// This ensures compatibility with:
// - empty databases
// - existing databases
// - CI/CD pipelines
func runMigrations(db *gorm.DB, cfg *config.Config, log *logrus.Entry) error {
	start := time.Now()

	quiet := db.Session(&gorm.Session{
		Logger: db.Logger.LogMode(glogger.Silent),
	})

	// =========================================================================
	// 1. SQL MIGRATIONS (SOURCE OF TRUTH)
	// =========================================================================
	log.Info("running SQL migrations (golang-migrate)")

	if err := runSQLMigrations(cfg.DatabaseURL, log); err != nil {
		return fmt.Errorf("SQL migrations failed: %w", err)
	}

	log.Info("SQL migrations completed")

	// =========================================================================
	// 2. OPTIONAL DATA FIXES (SAFE)
	// =========================================================================
	log.Info("running optional data fixes")

	if err := runDataFixes(quiet, log); err != nil {
		return fmt.Errorf("data fixes failed: %w", err)
	}

	// =========================================================================
	// 3. OPTIONAL GORM AUTOMIGRATE (DEV ONLY)
	// =========================================================================
	models := []interface{}{
		// Tenant and user models
		&model.Tenant{},
		&model.User{},
		&model.PlanMember{},
		&model.BusinessPlan{},
		&model.Scenario{},

		// Settings
		&model.PlanConfig{},
		&model.OpeningBalance{},
		&model.WorkingCapitalConfig{},
		&model.OpexPerHire{},
		&model.CapexPerHire{},
		&model.MultiYearAdjustment{},

		// Product
		&model.Product{},
		&model.ProductAssumption{},
		&model.ProductSalesVolume{},
		&model.ProductDistributorMargin{},

		// Staff
		&model.StaffHeadcount{},
		&model.StaffSalary{},
		&model.StaffIncentive{},

		// Financial
		&model.CapexEntry{},
		&model.OpexManualEntry{},
		&model.PnlManualEntry{},
		&model.FiplanEntry{},
		&model.PnlCashEntry{},
		&model.WCREntry{},
		&model.CashMonthlyOverride{},
		&model.BudgetMonthlyOverride{},

		// Reporting
		&model.Report{},
		&model.PlanSnapshot{},
		&model.AuditLog{},

		// Platform configs
		&model.CountryRateConfig{},

		// AI
		&model.AIUsagePolicy{},
		&model.AIUsageRecord{},

		// BEP
		&model.BEPSnapshot{},
		&model.FixedCostLine{},
		&model.VariableCostLine{},
		&model.SensitivityConfig{},
		&model.OptimisationPlan{},
		&model.FixedCostSaving{},
		&model.VariableCostSaving{},
		&model.PCGReviewItem{},

		// Cap table
		&model.PlanShareholder{},
		&model.CapTableCompany{},
		&model.CapTableShareClass{},
		&model.CapTableShareholder{},
		&model.CapTableRound{},
		&model.CapTablePosition{},
		&model.StockOptionPlan{},
		&model.OptionGrant{},
		&model.ValuationScenario{},
		&model.CapTableScenarioBranch{},

		// Auth
		&model.MagicLinkToken{},

		// Feature policies
		&model.FeaturePolicy{},
	}

	if cfg.AutoMigrate {
		log.Warn("running GORM AutoMigrate (DEV ONLY — not recommended in production)")

		if err := quiet.AutoMigrate(models...); err != nil {
			return fmt.Errorf("AutoMigrate failed: %w", err)
		}

		log.WithField("model_count", len(models)).Info("AutoMigrate completed")
	} else {
		log.Info("AutoMigrate disabled (production mode)")
	}

	// =========================================================================
	// DONE
	// =========================================================================
	log.WithFields(logrus.Fields{
		"models":  len(models),
		"elapsed": fmt.Sprintf("%dms", time.Since(start).Milliseconds()),
	}).Info("database migrations completed")

	return nil
}

// runDataFixes executes safe, idempotent data cleanup queries.
// All queries must tolerate missing tables.
func runDataFixes(db *gorm.DB, log *logrus.Entry) error {
	fixes := []string{
		// Deduplicate fiplan_entries
		`DELETE FROM fiplan_entries
		 WHERE id NOT IN (
		     SELECT DISTINCT ON (scenario_id, line_id, year) id
		     FROM fiplan_entries
		     ORDER BY scenario_id, line_id, year, updated_at DESC
		 )`,

		// Deduplicate pnl_cash_entries
		`DELETE FROM pnl_cash_entries
		 WHERE id NOT IN (
		     SELECT DISTINCT ON (scenario_id, line_id, year) id
		     FROM pnl_cash_entries
		     ORDER BY scenario_id, line_id, year, updated_at DESC
		 )`,

		// Cleanup orphan product rows
		`DELETE FROM product_assumptions
		 WHERE product_id NOT IN (SELECT id FROM products)`,

		`DELETE FROM product_sales_volumes
		 WHERE product_id NOT IN (SELECT id FROM products)`,

		`DELETE FROM product_distributor_margins
		 WHERE product_id NOT IN (SELECT id FROM products)`,
	}

	for _, sql := range fixes {
		if err := db.Exec(sql).Error; err != nil {
			// Ignore missing table errors (important for empty DB)
			if strings.Contains(err.Error(), "does not exist") {
				log.Debug("skipping fix (table does not exist)")
				continue
			}
			return err
		}
	}

	log.WithField("fix_count", len(fixes)).Info("data fixes completed")
	return nil
}
