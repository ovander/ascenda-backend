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
	DatabaseURL    string
	AllowedOrigins []string

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
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float64
	EnableCache bool
}

// DBPoolConfig holds database connection pool settings.
type DBPoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int // in seconds
	ConnMaxIdleTime int // in seconds
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Env:            envOrDefault("APP_ENV", "development"),
		Version:        envOrDefault("APP_VERSION", "0.1.0"),
		Port:           envOrDefaultInt("PORT", 8080),
		DatabaseURL:    envOrDefault("DATABASE_URL", "postgres://kerplan:kerplan@localhost:5432/kerplan?sslmode=disable"),
		AllowedOrigins: strings.Split(envOrDefault("CORS_ORIGINS", "http://localhost:5173"), ","),

		Socrate: SocrateConfig{
			BaseURL:      envOrDefault("SOCRATE_BASE_URL", ""),
			ClientID:     envOrDefault("SOCRATE_CLIENT_ID", ""),
			ClientSecret: envOrDefault("SOCRATE_CLIENT_SECRET", ""),
			JWKSURL:      envOrDefault("SOCRATE_JWKS_URL", ""),
			RedirectURL:  envOrDefault("SOCRATE_REDIRECT_URL", ""),
		},

		AI: AIConfig{
			APIKey:      envOrDefault("ANTHROPIC_API_KEY", ""),
			Model:       envOrDefault("AI_MODEL", "claude-sonnet-4-20250514"),
			MaxTokens:   envOrDefaultInt("AI_MAX_TOKENS", 4096),
			Temperature: envOrDefaultFloat("AI_TEMPERATURE", 0.3),
			EnableCache: envOrDefault("AI_CACHE_ENABLED", "true") == "true",
		},

		DBPool: DBPoolConfig{
			MaxOpenConns:    envOrDefaultInt("DB_MAX_OPEN_CONNS", 100),
			MaxIdleConns:    envOrDefaultInt("DB_MAX_IDLE_CONNS", 10),
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
	var missing []string
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.IsProd() {
		if c.Socrate.JWKSURL == "" {
			missing = append(missing, "SOCRATE_JWKS_URL")
		}
		if c.Socrate.ClientID == "" {
			missing = append(missing, "SOCRATE_CLIENT_ID")
		}
		if c.Socrate.ClientSecret == "" {
			missing = append(missing, "SOCRATE_CLIENT_SECRET")
		}
		if c.Socrate.RedirectURL == "" {
			missing = append(missing, "SOCRATE_REDIRECT_URL")
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
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
