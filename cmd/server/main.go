package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"kerplan/internal/config"
	"kerplan/internal/handler"
	"kerplan/internal/middleware"
	"kerplan/internal/model"
	"kerplan/internal/repo"
	"kerplan/internal/router"
	"kerplan/internal/service"
)

// Build-time variables injected via -ldflags.
// Example: go build -ldflags "-X main.buildTime=2026-03-17T12:00:00Z -X main.gitCommit=abc1234"
var (
	buildTime string
	gitCommit string
)

func main() {
	// Load .env file if present (does not override existing env vars)
	if err := godotenv.Load(); err != nil {
		// Not fatal — .env is optional (e.g. in production with real env vars)
		fmt.Println("No .env file found, using environment variables")
	}

	// Load configuration
	cfg := config.Load()
	cfg.BuildTime = buildTime
	cfg.GitCommit = gitCommit

	// Setup logger
	logger := setupLogger(cfg)
	logger.WithField("version", cfg.Version).Info("starting KerPlan backend server")

	// Log startup configuration summary
	logStartupConfig(cfg, logger)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logger.WithError(err).Fatal("configuration validation failed")
	}

	// Check Socrate OAuth2 connectivity
	if err := checkSocrateConnectivity(cfg, logger); err != nil {
		logger.WithError(err).Fatal("Socrate OAuth2 server is not reachable")
	}

	// Connect to database
	db, err := connectDatabase(cfg, logger)
	if err != nil {
		logger.WithError(err).Fatal("failed to connect to database")
	}

	// Run migrations
	if err := runMigrations(db, logger); err != nil {
		logger.WithError(err).Fatal("failed to run migrations")
	}

	// Create repositories
	repos := repo.NewRepoBundle(db)

	// Create services
	services := service.NewServiceBundle(repos, cfg, logger)

	// Create handlers
	handlers := handler.NewHandlerBundle(services, repos, db, cfg, logger)

	// Create middleware
	authMW := middleware.NewAuthMiddleware(cfg.Socrate.JWKSURL, cfg.Socrate.BaseURL, logger)
	tenantMW := middleware.NewTenantMiddleware(db, repos.User, logger)
	rbacMW := middleware.NewRBACMiddleware(logger)
	planAccessMW := middleware.NewPlanAccessMiddleware(repos.PlanMember, logger)
	loggerMW := middleware.NewLoggerMiddleware(logger.Logger)
	recoverMW := middleware.NewRecoverMiddleware(logger.Logger)
	requestIDMW := middleware.NewRequestIDMiddleware()
	securityMW := middleware.NewSecurityHeadersMiddleware()

	// Create router
	r := router.NewRouter(handlers, authMW, tenantMW, rbacMW, planAccessMW, loggerMW, recoverMW, requestIDMW, securityMW, logger)

	// Setup HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: r,
	}

	// Start server in background
	go func() {
		logger.WithField("port", cfg.Port).Info("listening for requests")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("server error")
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutting down server")
	if err := server.Close(); err != nil {
		logger.WithError(err).Error("error closing server")
	}
}

// setupLogger configures logrus logger.
func setupLogger(cfg *config.Config) *logrus.Entry {
	log := logrus.New()

	if cfg.Env == "production" {
		log.SetLevel(logrus.InfoLevel)
		log.SetFormatter(&logrus.JSONFormatter{})
	} else {
		log.SetLevel(logrus.DebugLevel)
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	return log.WithFields(logrus.Fields{
		"service": "kerplan-backend",
		"env":     cfg.Env,
	})
}

// connectDatabase establishes a connection to the PostgreSQL database.
func connectDatabase(cfg *config.Config, logger *logrus.Entry) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(cfg.DBPool.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.DBPool.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.DBPool.ConnMaxLifetime) * time.Second)
	sqlDB.SetConnMaxIdleTime(time.Duration(cfg.DBPool.ConnMaxIdleTime) * time.Second)

	logger.Info("database connection established")
	return db, nil
}

// runMigrations runs all database migrations.
func runMigrations(db *gorm.DB, logger *logrus.Entry) error {
	// List all models to migrate
	models := []interface{}{
		// Tenant and plan models
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
	}

	if err := db.AutoMigrate(models...); err != nil {
		return err
	}

	logger.Info("database migrations completed")
	return nil
}

// logStartupConfig logs a clear summary of the active configuration at startup.
func logStartupConfig(cfg *config.Config, logger *logrus.Entry) {
	// Mask the database password for logging
	dbDisplay := cfg.DatabaseURL
	if u, err := url.Parse(cfg.DatabaseURL); err == nil && u.User != nil {
		masked := url.UserPassword(u.User.Username(), "****")
		u.User = masked
		dbDisplay = u.String()
	}

	logger.WithFields(logrus.Fields{
		"port":        cfg.Port,
		"database":    dbDisplay,
		"cors":        strings.Join(cfg.AllowedOrigins, ", "),
	}).Info("configuration loaded")

	logger.WithFields(logrus.Fields{
		"max_open_conns":     cfg.DBPool.MaxOpenConns,
		"max_idle_conns":     cfg.DBPool.MaxIdleConns,
		"conn_max_lifetime_s": cfg.DBPool.ConnMaxLifetime,
		"conn_max_idle_time_s": cfg.DBPool.ConnMaxIdleTime,
	}).Info("database pool settings")

	// Socrate OAuth2 config
	if cfg.Socrate.BaseURL == "" {
		logger.Warn("Socrate OAuth2 base URL is not configured (SOCRATE_BASE_URL)")
	} else {
		logger.WithFields(logrus.Fields{
			"base_url":     cfg.Socrate.BaseURL,
			"client_id":    cfg.Socrate.ClientID,
			"redirect_url": cfg.Socrate.RedirectURL,
			"jwks_url":     cfg.Socrate.JWKSURL,
			"token_alg":    "RS256",
		}).Info("Socrate OAuth2 configuration")
	}

	if cfg.Socrate.ClientID == "" {
		logger.Warn("Socrate client ID is not configured (SOCRATE_CLIENT_ID)")
	}
	if cfg.Socrate.ClientSecret == "" {
		logger.Warn("Socrate client secret is not configured (SOCRATE_CLIENT_SECRET)")
	}
	if cfg.Socrate.JWKSURL == "" {
		logger.Warn("Socrate JWKS URL is not configured (SOCRATE_JWKS_URL) — token validation will fail")
	}
}

// checkSocrateConnectivity attempts an HTTP GET to the Socrate base URL to verify reachability.
// In development mode with no base URL configured, it logs a warning and continues.
func checkSocrateConnectivity(cfg *config.Config, logger *logrus.Entry) error {
	if cfg.Socrate.BaseURL == "" {
		if cfg.IsProd() {
			return fmt.Errorf("SOCRATE_BASE_URL is required in production")
		}
		logger.Warn("Socrate OAuth2 not configured — auth endpoints will not work")
		return nil
	}

	client := &http.Client{Timeout: 5 * time.Second}

	// Try the well-known OpenID configuration endpoint first, fall back to base URL
	checkURL := cfg.Socrate.BaseURL + "/.well-known/openid-configuration"
	resp, err := client.Get(checkURL)
	if err != nil {
		// Try base URL directly
		resp, err = client.Get(cfg.Socrate.BaseURL)
		if err != nil {
			return fmt.Errorf("cannot reach Socrate at %s: %w", cfg.Socrate.BaseURL, err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("Socrate returned HTTP %d at %s", resp.StatusCode, checkURL)
	}

	logger.WithField("base_url", cfg.Socrate.BaseURL).Info("Socrate OAuth2 server is reachable")

	// If JWKS URL is configured, verify it too
	if cfg.Socrate.JWKSURL != "" {
		jwksResp, err := client.Get(cfg.Socrate.JWKSURL)
		if err != nil {
			logger.WithError(err).Warn("JWKS endpoint is not reachable")
		} else {
			jwksResp.Body.Close()
			if jwksResp.StatusCode == http.StatusOK {
				logger.WithField("jwks_url", cfg.Socrate.JWKSURL).Info("JWKS endpoint verified")
			} else {
				logger.WithFields(logrus.Fields{
					"jwks_url": cfg.Socrate.JWKSURL,
					"status":   jwksResp.StatusCode,
				}).Warn("JWKS endpoint returned unexpected status")
			}
		}
	}

	return nil
}
