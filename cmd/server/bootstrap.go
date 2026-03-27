package main

import (
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	glogger "gorm.io/gorm/logger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"ascenda/internal/config"
	"ascenda/internal/event"
	"ascenda/internal/handler"
	"ascenda/internal/middleware"
	"ascenda/internal/pkg/logger"
	"ascenda/internal/repo"
	"ascenda/internal/router"
	"ascenda/internal/service"
)

// AppResources holds all resources created during bootstrap and needed for
// graceful shutdown. Pattern ported from GPWA cmd/server/bootstrap.go.
type AppResources struct {
	Server      *http.Server
	Logger      *logrus.Entry
	DB          *gorm.DB                      // closed after all requests drain
	Emitter     *event.Emitter                // drained after HTTP server stops accepting
	AIAccessMW  *middleware.AIAccessMiddleware // drained alongside emitter (async usage goroutines)
}

// Bootstrap initialises the entire application in dependency order and returns
// the resources required for graceful shutdown.
//
// Initialisation order:
//  1. Logger
//  2. Configuration
//  3. Auth (Socrate OAuth2)
//  4. Database
//  5. Migrations
//  6. Repositories
//  7. Services
//  8. Handlers
//  9. Middleware
// 10. Router
// 11. HTTP Server
func Bootstrap(cfg *config.Config) (*AppResources, error) {
	bootstrapStart := time.Now()

	// =========================================================================
	// 1. Logger
	// =========================================================================
	log := setupLogger(cfg)
	log.WithFields(logrus.Fields{
		"phase":   "startup",
		"version": cfg.Version,
		"commit":  cfg.GitCommit,
		"go":      runtime.Version(),
	}).Info("starting Ascenda backend")

	// =========================================================================
	// 2. Configuration
	// =========================================================================
	phaseLog := log.WithField("phase", "config")
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	logStartupConfig(cfg, phaseLog)
	phaseLog.Info("configuration validated")

	// =========================================================================
	// 3. Auth — Socrate OAuth2 connectivity
	// =========================================================================
	if err := checkSocrateConnectivity(cfg, log.WithField("phase", "auth")); err != nil {
		return nil, fmt.Errorf("Socrate OAuth2 server is not reachable: %w", err)
	}

	// =========================================================================
	// 4. Database
	// =========================================================================
	db, err := connectDatabase(cfg, log.WithField("phase", "database"))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// =========================================================================
	// 5. Migrations
	// =========================================================================
	if err := runMigrations(db, cfg, log.WithField("phase", "migrations")); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// =========================================================================
	// 6. Repositories
	// =========================================================================
	repos := repo.NewRepoBundle(db)
	log.WithField("phase", "repositories").Info("repository layer initialised")

	// =========================================================================
	// 7. Services
	// =========================================================================
	services := service.NewServiceBundle(repos, cfg, log.WithField("phase", "services"))

	// =========================================================================
	// 8. Handlers
	// =========================================================================
	handlers := handler.NewHandlerBundle(services, repos, db, cfg, log)
	log.WithField("phase", "handlers").Info("handler layer initialised")

	// =========================================================================
	// 9. Middleware
	// =========================================================================
	mwLog := log.WithField("phase", "middleware")
	authMW := middleware.NewAuthMiddleware(cfg.Socrate.JWKSURL, cfg.Socrate.BaseURL, log)
	tenantMW := middleware.NewTenantMiddleware(db, repos.User, log)
	rbacMW := middleware.NewRBACMiddleware(log)
	planAccessMW := middleware.NewPlanAccessMiddleware(repos.PlanMember, log)
	tierGateMW := middleware.NewTierGateMiddleware(repos.Tenant, log)
	aiAccessMW := middleware.NewAIAccessMiddleware(services.AIUsagePolicy, log)
	loggerMW := middleware.NewLoggerMiddleware(log.Logger)
	recoverMW := middleware.NewRecoverMiddleware(log.Logger)
	requestIDMW := middleware.NewRequestIDMiddleware()
	securityMW := middleware.NewSecurityHeadersMiddleware()
	mwLog.WithField("stack", "requestID → security → recover → logger → auth → tenant → rbac → planAccess → tierGate → aiAccess").
		Info("middleware stack initialised")

	// =========================================================================
	// 10. Router
	// =========================================================================
	r := router.NewRouter(
		handlers, authMW, tenantMW, rbacMW, planAccessMW, tierGateMW, aiAccessMW,
		loggerMW, recoverMW, requestIDMW, securityMW, log,
		cfg.AllowedOrigins,
		cfg.MaxRequestBodyBytes,
		cfg.MetricsEnabled,
	)
	_ = r // logged inside NewRouter

	// =========================================================================
	// 11. HTTP Server — timeouts set explicitly (same values as GPWA)
	// =========================================================================
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	log.WithFields(logrus.Fields{
		"phase":         "server",
		"addr":          fmt.Sprintf(":%d", cfg.Port),
		"read_timeout":  "30s",
		"write_timeout": "30s",
		"idle_timeout":  "120s",
	}).Info("HTTP server configured")

	// ── Bootstrap summary ─────────────────────────────────────────────────────
	log.WithFields(logrus.Fields{
		"phase":    "startup",
		"elapsed":  fmt.Sprintf("%dms", time.Since(bootstrapStart).Milliseconds()),
	}).Info("bootstrap completed")

	return &AppResources{
		Server:     server,
		Logger:     log,
		DB:         db,
		Emitter:    services.Emitter,
		AIAccessMW: aiAccessMW,
	}, nil
}

// ── Logger setup ──────────────────────────────────────────────────────────────

// setupLogger configures logrus, respecting:
//   - ENV: production → JSON formatter + Info level; else → Text formatter + Debug level
//   - LOG_LEVEL config overrides the default level (e.g. LOG_LEVEL=warn in dev)
//   - SetReportCaller(true) so every log line shows file:line — same as GPWA
func setupLogger(cfg *config.Config) *logrus.Entry {
	log := logrus.New()
	log.SetReportCaller(true)

	if cfg.Env == "production" {
		log.SetFormatter(&logrus.JSONFormatter{
			CallerPrettyfier: shortCaller,
		})
		log.SetLevel(logrus.InfoLevel)
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:    true,
			TimestampFormat:  "2006-01-02 15:04:05",
			CallerPrettyfier: shortCaller,
		})
		log.SetLevel(logrus.DebugLevel)
	}

	// Allow runtime override via LOG_LEVEL (e.g. LOG_LEVEL=warn)
	if lvlStr := strings.ToLower(cfg.LogLevel); lvlStr != "" {
		if lvl, err := logrus.ParseLevel(lvlStr); err == nil {
			log.SetLevel(lvl)
		} else {
			log.Warnf("invalid LOG_LEVEL %q — keeping default", lvlStr)
		}
	}

	entry := log.WithFields(logrus.Fields{
		"service": "ascenda-backend",
		"env":     cfg.Env,
	})
	entry.WithFields(logrus.Fields{
		"phase": "logger",
		"level": log.GetLevel().String(),
	}).Info("logger initialised")
	return entry
}

// shortCaller trims the full file path to just "pkg/file.go:line" for readability.
func shortCaller(f *runtime.Frame) (string, string) {
	file := f.File
	if idx := strings.LastIndex(file, "/"); idx >= 0 {
		if prev := strings.LastIndex(file[:idx], "/"); prev >= 0 {
			file = file[prev+1:]
		}
	}
	return "", fmt.Sprintf("%s:%d", file, f.Line)
}

// ── Database ──────────────────────────────────────────────────────────────────

// connectDatabase establishes a connection to the PostgreSQL database.
// GORM SQL output is routed through logrus via GormLogger (ported from GPWA).
func connectDatabase(cfg *config.Config, log *logrus.Entry) (*gorm.DB, error) {
	// In production log only warnings/errors; in dev trace all queries at Debug.
	gormLevel := glogger.Info
	slowThreshold := 200 * time.Millisecond
	if cfg.IsProd() {
		gormLevel = glogger.Warn
		slowThreshold = 500 * time.Millisecond
	}

	gormLog := logger.NewGormLogger(log, gormLevel, slowThreshold, true /* ignoreNotFound */)

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: gormLog,
	})
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

	log.WithFields(logrus.Fields{
		"max_open":          cfg.DBPool.MaxOpenConns,
		"max_idle":          cfg.DBPool.MaxIdleConns,
		"max_lifetime_s":    cfg.DBPool.ConnMaxLifetime,
		"max_idle_time_s":   cfg.DBPool.ConnMaxIdleTime,
	}).Info("database connection established")
	return db, nil
}

// ── Startup diagnostics ───────────────────────────────────────────────────────

// logStartupConfig logs a clear summary of the active configuration at startup.
// The log entry already carries phase=config.
func logStartupConfig(cfg *config.Config, log *logrus.Entry) {
	// Mask the database password for logging
	dbDisplay := cfg.DatabaseURL
	if u, err := url.Parse(cfg.DatabaseURL); err == nil && u.User != nil {
		masked := url.UserPassword(u.User.Username(), "****")
		u.User = masked
		dbDisplay = u.String()
	}

	log.WithFields(logrus.Fields{
		"port":     cfg.Port,
		"database": dbDisplay,
		"cors":     strings.Join(cfg.AllowedOrigins, ", "),
	}).Debug("core settings")

	log.WithFields(logrus.Fields{
		"max_open_conns":       cfg.DBPool.MaxOpenConns,
		"max_idle_conns":       cfg.DBPool.MaxIdleConns,
		"conn_max_lifetime_s":  cfg.DBPool.ConnMaxLifetime,
		"conn_max_idle_time_s": cfg.DBPool.ConnMaxIdleTime,
	}).Debug("database pool settings")

	log.WithFields(logrus.Fields{
		"provider":    cfg.AI.Provider,
		"model":       cfg.AI.Model,
		"max_tokens":  cfg.AI.MaxTokens,
		"timeout_s":   cfg.AI.Timeout,
		"cache":       cfg.AI.EnableCache,
	}).Debug("AI settings")
}

// checkSocrateConnectivity attempts an HTTP GET to the Socrate base URL.
// In development mode with no base URL configured, it logs a warning and continues.
// The log entry already carries phase=auth.
func checkSocrateConnectivity(cfg *config.Config, log *logrus.Entry) error {
	if cfg.Socrate.BaseURL == "" {
		if cfg.IsProd() {
			return fmt.Errorf("SOCRATE_BASE_URL is required in production")
		}
		log.Warn("Socrate OAuth2 not configured — auth endpoints will not work")
		return nil
	}

	log.WithFields(logrus.Fields{
		"base_url":     cfg.Socrate.BaseURL,
		"client_id":    cfg.Socrate.ClientID,
		"redirect_url": cfg.Socrate.RedirectURL,
		"jwks_url":     cfg.Socrate.JWKSURL,
	}).Debug("Socrate OAuth2 configuration")

	client := &http.Client{Timeout: 5 * time.Second}

	// Try the well-known OpenID configuration endpoint first, fall back to base URL.
	checkURL := cfg.Socrate.BaseURL + "/.well-known/openid-configuration"
	resp, err := client.Get(checkURL)
	if err != nil {
		resp, err = client.Get(cfg.Socrate.BaseURL)
		if err != nil {
			return fmt.Errorf("cannot reach Socrate at %s: %w", cfg.Socrate.BaseURL, err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("Socrate returned HTTP %d at %s", resp.StatusCode, checkURL)
	}

	if cfg.Socrate.JWKSURL != "" {
		jwksResp, err := client.Get(cfg.Socrate.JWKSURL)
		if err != nil {
			log.WithError(err).Warn("JWKS endpoint is not reachable")
		} else {
			jwksResp.Body.Close()
			if jwksResp.StatusCode != http.StatusOK {
				log.WithFields(logrus.Fields{
					"jwks_url": cfg.Socrate.JWKSURL,
					"status":   jwksResp.StatusCode,
				}).Warn("JWKS endpoint returned unexpected status")
			}
		}
	}

	log.WithField("base_url", cfg.Socrate.BaseURL).Info("Socrate OAuth2 verified")
	return nil
}
