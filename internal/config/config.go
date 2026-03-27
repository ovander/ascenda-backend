package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Env            string
	Version        string
	BuildTime      string
	GitCommit      string
	Port           int
	LogLevel       string
	DatabaseURL    string
	AllowedOrigins []string
	AppBaseURL     string // public base URL of the API, used for building email links

	// AutoMigrate controls whether GORM AutoMigrate runs at startup.
	// Default: true in development, false in production.
	// Set DB_AUTO_MIGRATE=true to force it on in production (not recommended).
	AutoMigrate bool

	// MaxRequestBodyBytes is the maximum size of an incoming HTTP request body.
	// Default: 1 MiB (1 048 576 bytes). Increase for endpoints that accept large
	// payloads (e.g. file uploads) by using the per-route override in the router.
	MaxRequestBodyBytes int64

	// MetricsEnabled exposes GET /metrics (Prometheus) when true.
	MetricsEnabled bool

	Socrate SocrateConfig
	AI      AIConfig
	DBPool  DBPoolConfig
}

// SocrateConfig holds OAuth2 server configuration.
type SocrateConfig struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	JWKSURL      string
	RedirectURL  string
}

// AIConfig holds AI service configuration.
type AIConfig struct {
	Provider      string   // "openai" or "claude"
	APIKey        string
	Model         string
	AllowedModels []string // optional allow-list for Claude models
	MaxTokens     int
	Temperature   float64
	Timeout       int  // HTTP timeout in seconds
	EnableCache   bool
}

// DBPoolConfig holds database connection pool settings.
type DBPoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int // in seconds
	ConnMaxIdleTime int // in seconds
}

// Load reads configuration from environment variables.
// Call once from main() and pass the result explicitly — do not call Load()
// from multiple packages to avoid hidden global state.
func Load() *Config {
	return load()
}

// load does the actual work.
func load() *Config {
	env := envOrDefault("APP_ENV", "development")
	isProd := env == "production"

	// DB_AUTO_MIGRATE: default true in dev/staging, false in production.
	autoMigrateDefault := "true"
	if isProd {
		autoMigrateDefault = "false"
	}
	autoMigrate := envOrDefault("DB_AUTO_MIGRATE", autoMigrateDefault) == "true"

	// Parse AI_ALLOWED_MODELS into a slice.
	var allowedModels []string
	if raw := envOrDefault("AI_ALLOWED_MODELS", ""); raw != "" {
		for _, m := range strings.Split(raw, ",") {
			if t := strings.TrimSpace(m); t != "" {
				allowedModels = append(allowedModels, t)
			}
		}
	}

	// Parse CORS_ORIGINS, trimming whitespace from each entry.
	var origins []string
	for _, o := range strings.Split(envOrDefault("CORS_ORIGINS", "http://localhost:5173"), ",") {
		if t := strings.TrimSpace(o); t != "" {
			origins = append(origins, t)
		}
	}

	return &Config{
		Env:                 env,
		Version:             envOrDefault("APP_VERSION", "0.1.0"),
		Port:                envOrDefaultInt("PORT", 8080),
		LogLevel:            envOrDefault("LOG_LEVEL", ""),
		DatabaseURL:         envOrDefault("DATABASE_URL", "postgres://kerplan:kerplan@localhost:5432/kerplan?sslmode=disable"),
		AllowedOrigins:      origins,
		AppBaseURL:          envOrDefault("APP_BASE_URL", "http://localhost:8080"),
		AutoMigrate:         autoMigrate,
		MaxRequestBodyBytes: int64(envOrDefaultInt("MAX_REQUEST_BODY_BYTES", 1<<20)), // 1 MiB
		MetricsEnabled:      envOrDefault("METRICS_ENABLED", "false") == "true",

		Socrate: SocrateConfig{
			BaseURL:      envOrDefault("SOCRATE_BASE_URL", ""),
			ClientID:     envOrDefault("SOCRATE_CLIENT_ID", ""),
			ClientSecret: envOrDefault("SOCRATE_CLIENT_SECRET", ""),
			JWKSURL:      envOrDefault("SOCRATE_JWKS_URL", ""),
			RedirectURL:  envOrDefault("SOCRATE_REDIRECT_URL", ""),
		},

		AI: AIConfig{
			Provider:      envOrDefault("AI_PROVIDER", "claude"),
			APIKey:        envOrDefault("AI_API_KEY", ""),
			Model:         envOrDefault("AI_MODEL", "claude-sonnet-4-6"),
			AllowedModels: allowedModels,
			MaxTokens:     envOrDefaultInt("AI_MAX_TOKENS", 4096),
			Temperature:   envOrDefaultFloat("AI_TEMPERATURE", 0.3),
			Timeout:       envOrDefaultInt("AI_TIMEOUT_SECONDS", 30),
			EnableCache:   envOrDefault("AI_CACHE_ENABLED", "true") == "true",
		},

		DBPool: DBPoolConfig{
			MaxOpenConns:    envOrDefaultInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    envOrDefaultInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: envOrDefaultInt("DB_CONN_MAX_LIFETIME", 3600),
			ConnMaxIdleTime: envOrDefaultInt("DB_CONN_MAX_IDLE_TIME", 300),
		},
	}
}

// IsProd returns true if the environment is production.
func (c *Config) IsProd() bool {
	return c.Env == "production"
}

// Validate checks that all required configuration fields are set.
func (c *Config) Validate() error {
	var errs []string
	if c.DatabaseURL == "" {
		errs = append(errs, "DATABASE_URL")
	}
	if c.IsProd() {
		if c.Socrate.JWKSURL == "" {
			errs = append(errs, "SOCRATE_JWKS_URL")
		}
		if c.Socrate.ClientID == "" {
			errs = append(errs, "SOCRATE_CLIENT_ID")
		}
		if c.Socrate.ClientSecret == "" {
			errs = append(errs, "SOCRATE_CLIENT_SECRET")
		}
		if c.Socrate.RedirectURL == "" {
			errs = append(errs, "SOCRATE_REDIRECT_URL")
		}
		if c.AutoMigrate {
			// Warn — not a hard error, but operators should be aware.
			errs = append(errs, "WARNING: DB_AUTO_MIGRATE=true in production — set to false and use 'make migrate-up'")
		}
	}
	if len(errs) > 0 {
		// Check if it's only a warning
		if len(errs) == 1 && strings.HasPrefix(errs[0], "WARNING:") {
			return fmt.Errorf("%s", errs[0])
		}
		return fmt.Errorf("missing required configuration: %s", strings.Join(errs, ", "))
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func envOrDefaultFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}
