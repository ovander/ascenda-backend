package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Env is development, staging or production. Env: APP_ENV, required: with
	// no default, a deploy that forgets it cannot run with the production
	// checks silently off (Validate refuses to start).
	Env  string
	Port int
	// BindAddress is the interface the HTTP server listens on. Env: BIND_ADDRESS.
	// Default: 127.0.0.1 in production, where Caddy on the same host proxies to
	// the API and nothing else may reach it; every interface elsewhere. A
	// container sets BIND_ADDRESS=0.0.0.0 (see the Dockerfile).
	BindAddress    string
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

	// TrustedProxyCIDRs lists the reverse proxies whose X-Forwarded-For /
	// X-Real-IP headers are trusted when deriving the client IP for rate
	// limiting. Requests from any other peer are keyed on the TCP peer address.
	// Env: TRUSTED_PROXY_CIDRS (comma-separated CIDRs or IPs).
	// Default: loopback only, matching a reverse proxy on the same host.
	TrustedProxyCIDRs []string

	// AllowDefaultTenantFallback lets the tenant middleware provision users
	// whose JWT carries no tenant_id and who have no user record into the
	// seeded default workspace tenant. This is a local-development convenience
	// only: in any other environment such requests are rejected with 403.
	// Env: TENANT_DEFAULT_FALLBACK. Default: true in development, false otherwise.
	// Validate() refuses to start in production when it is enabled.
	AllowDefaultTenantFallback bool

	Socrate SocrateConfig
	BFF     BFFConfig
	AI      AIConfig
	DBPool  DBPoolConfig
	Sentry  SentryConfig
}

// SocrateConfig holds OAuth2 server configuration.
type SocrateConfig struct {
	// BaseURL is Socrate's public URL. It is the issuer every access token must
	// carry (iss), compared as an exact string: no trailing slash.
	BaseURL string
	// AdminBaseURL is Socrate's admin API. It listens on loopback on the Socrate
	// VPS only; the apps VPS reaches it through the host's SSH tunnel
	// (socrate-admin-tunnel.service) at http://127.0.0.1:18082. Env:
	// SOCRATE_ADMIN_URL. It is never derived: backendkit would guess
	// <BaseURL host>:8081, a different service, so Validate requires it
	// whenever SOCRATE_BASE_URL is set. There is no fallback when the tunnel
	// is down: admin calls fail.
	AdminBaseURL string
	ClientID     string
	ClientSecret string
	AppID        string // numeric app ID from Socrate admin console (avoids admin API call)
	JWKSURL      string
	// VerifyAudience requires ClientID in the token's aud claim. Env:
	// SOCRATE_VERIFY_AUDIENCE (default true); turn off only for an IdP that
	// does not set aud.
	VerifyAudience bool
}

// BFFConfig configures the Backend-for-Frontend: the server-side sign-in flow
// (/bff/login, /bff/callback), the session cookie and the session store. The
// browser holds only the opaque session cookie; tokens stay on the server.
type BFFConfig struct {
	// RedirectURL is the BFF's OAuth redirect URI, registered at Socrate
	// exactly: https://ascenda.vandermoten.eu/bff/callback in production. Its
	// origin is the SPA's, where the session cookie lives. Env: BFF_REDIRECT_URL.
	// Empty outside production disables the BFF sign-in routes.
	RedirectURL string
	// CookieName is the session cookie's name; with Secure it is sent as
	// "__Host-" + CookieName. Env: BFF_COOKIE_NAME (default ascenda_session).
	CookieName string
	// IdleTTL ends a session unused for that long; AbsoluteTTL ends any session
	// that old. Env: BFF_SESSION_IDLE_TTL (default 30m), BFF_SESSION_ABSOLUTE_TTL
	// (default 8h).
	IdleTTL     time.Duration
	AbsoluteTTL time.Duration
	// InsecureCookie drops Secure and the __Host- prefix, for local development
	// over plain http only. Env: BFF_INSECURE_COOKIE (default false). Validate
	// refuses it in production and with an https redirect URL.
	InsecureCookie bool
}

// Enabled reports whether the BFF sign-in routes are configured.
func (b BFFConfig) Enabled() bool { return b.RedirectURL != "" }

// defaultBindAddress is loopback in production, every interface otherwise.
func defaultBindAddress(env string) string {
	if env == "production" {
		return "127.0.0.1"
	}
	return ""
}

// ListenAddr is the address the HTTP server listens on (BIND_ADDRESS:PORT).
func (c *Config) ListenAddr() string {
	return net.JoinHostPort(c.BindAddress, strconv.Itoa(c.Port))
}

// AIConfig holds AI service configuration.
type AIConfig struct {
	Provider      string // "openai" or "claude"
	APIKey        string
	Model         string
	AllowedModels []string // optional allow-list for Claude models
	MaxTokens     int
	Temperature   float64
	Timeout       int // HTTP timeout in seconds
	EnableCache   bool
}

// DBPoolConfig holds database connection pool settings.
type DBPoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int // in seconds
	ConnMaxIdleTime int // in seconds
}

// SentryConfig holds Sentry error monitoring configuration.
// Set SENTRY_DSN to enable; leave empty to disable silently.
type SentryConfig struct {
	DSN string
}

// Load reads configuration from environment variables.
// Call once from main() and pass the result explicitly — do not call Load()
// from multiple packages to avoid hidden global state.
func Load() *Config {
	return load()
}

// load does the actual work.
func load() *Config {
	env := strings.TrimSpace(os.Getenv("APP_ENV")) // required, no default: see Validate
	isProd := env == "production"

	// DB_AUTO_MIGRATE: default true in dev/staging, false in production.
	autoMigrateDefault := "true"
	if isProd {
		autoMigrateDefault = "false"
	}
	autoMigrate := envOrDefault("DB_AUTO_MIGRATE", autoMigrateDefault) == "true"

	// TENANT_DEFAULT_FALLBACK: only defaults to true for local development.
	tenantFallbackDefault := "false"
	if env == "development" {
		tenantFallbackDefault = "true"
	}
	allowDefaultTenant := envOrDefault("TENANT_DEFAULT_FALLBACK", tenantFallbackDefault) == "true"

	// Parse AI_ALLOWED_MODELS into a slice.
	var allowedModels []string
	if raw := envOrDefault("AI_ALLOWED_MODELS", ""); raw != "" {
		for _, m := range strings.Split(raw, ",") {
			if t := strings.TrimSpace(m); t != "" {
				allowedModels = append(allowedModels, t)
			}
		}
	}

	// Parse TRUSTED_PROXY_CIDRS, trimming whitespace from each entry.
	var trustedProxies []string
	for _, c := range strings.Split(envOrDefault("TRUSTED_PROXY_CIDRS", "127.0.0.1/32,::1/128"), ",") {
		if t := strings.TrimSpace(c); t != "" {
			trustedProxies = append(trustedProxies, t)
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
		Env:                        env,
		Port:                       envOrDefaultInt("PORT", 8080),
		BindAddress:                envOrDefault("BIND_ADDRESS", defaultBindAddress(env)),
		LogLevel:                   envOrDefault("LOG_LEVEL", ""),
		DatabaseURL:                envOrDefault("DATABASE_URL", "postgres://ascenda:ascenda@localhost:5432/ascenda?sslmode=disable"),
		AllowedOrigins:             origins,
		AppBaseURL:                 envOrDefault("APP_BASE_URL", "http://localhost:8080"),
		AutoMigrate:                autoMigrate,
		MaxRequestBodyBytes:        int64(envOrDefaultInt("MAX_REQUEST_BODY_BYTES", 1<<20)), // 1 MiB
		MetricsEnabled:             envOrDefault("METRICS_ENABLED", "false") == "true",
		AllowDefaultTenantFallback: allowDefaultTenant,
		TrustedProxyCIDRs:          trustedProxies,

		Socrate: SocrateConfig{
			BaseURL:        envOrDefault("SOCRATE_BASE_URL", ""),
			AdminBaseURL:   envOrDefault("SOCRATE_ADMIN_URL", ""),
			ClientID:       envOrDefault("SOCRATE_CLIENT_ID", ""),
			ClientSecret:   envOrDefault("SOCRATE_CLIENT_SECRET", ""),
			AppID:          envOrDefault("SOCRATE_APP_ID", ""),
			JWKSURL:        envOrDefault("SOCRATE_JWKS_URL", ""),
			VerifyAudience: envOrDefault("SOCRATE_VERIFY_AUDIENCE", "true") != "false",
		},

		BFF: BFFConfig{
			RedirectURL:    envOrDefault("BFF_REDIRECT_URL", ""),
			CookieName:     envOrDefault("BFF_COOKIE_NAME", "ascenda_session"),
			IdleTTL:        envOrDefaultDuration("BFF_SESSION_IDLE_TTL", 30*time.Minute),
			AbsoluteTTL:    envOrDefaultDuration("BFF_SESSION_ABSOLUTE_TTL", 8*time.Hour),
			InsecureCookie: envOrDefault("BFF_INSECURE_COOKIE", "false") == "true",
		},

		AI: AIConfig{
			Provider:      envOrDefault("AI_PROVIDER", "claude"),
			APIKey:        envOrDefault("AI_API_KEY", ""),
			Model:         envOrDefault("AI_MODEL", "claude-sonnet-4-6"),
			AllowedModels: allowedModels,
			MaxTokens:     envOrDefaultInt("AI_MAX_TOKENS", 4096),
			Temperature:   envOrDefaultFloat("AI_TEMPERATURE", 0.3),
			Timeout:       envOrDefaultInt("AI_TIMEOUT_SECONDS", 90),
			EnableCache:   envOrDefault("AI_CACHE_ENABLED", "true") == "true",
		},

		DBPool: DBPoolConfig{
			MaxOpenConns:    envOrDefaultInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    envOrDefaultInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: envOrDefaultInt("DB_CONN_MAX_LIFETIME", 3600),
			ConnMaxIdleTime: envOrDefaultInt("DB_CONN_MAX_IDLE_TIME", 300),
		},

		Sentry: SentryConfig{
			DSN: envOrDefault("SENTRY_DSN", ""),
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
	switch c.Env {
	case "development", "staging", "production":
	default:
		errs = append(errs, "APP_ENV (development, staging or production; no default)")
	}
	if c.DatabaseURL == "" {
		errs = append(errs, "DATABASE_URL")
	}
	// In every environment: with Socrate configured, the admin URL must be
	// explicit, or backendkit would derive <base host>:8081 (a different service).
	if !c.IsProd() && c.Socrate.BaseURL != "" && c.Socrate.AdminBaseURL == "" {
		errs = append(errs, "SOCRATE_ADMIN_URL (required with SOCRATE_BASE_URL; never derived)")
	}
	errs = append(errs, c.BFF.errors(c.IsProd())...)
	if c.IsProd() {
		if c.Socrate.BaseURL == "" {
			errs = append(errs, "SOCRATE_BASE_URL")
		}
		if c.Socrate.AdminBaseURL == "" {
			errs = append(errs, "SOCRATE_ADMIN_URL")
		}
		if c.Socrate.JWKSURL == "" {
			errs = append(errs, "SOCRATE_JWKS_URL")
		}
		if c.Socrate.ClientID == "" {
			errs = append(errs, "SOCRATE_CLIENT_ID")
		}
		if c.Socrate.ClientSecret == "" {
			errs = append(errs, "SOCRATE_CLIENT_SECRET")
		}
		errs = append(errs, c.Socrate.urlErrors()...)
		if c.AllowDefaultTenantFallback {
			// Hard error: the fallback places unknown users into a shared tenant.
			errs = append(errs, "TENANT_DEFAULT_FALLBACK must not be enabled in production")
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

// urlErrors reports Socrate URLs that are set but malformed. Base URLs (the
// issuer and the admin address) must not end with a slash: the
// issuer is compared exactly, and backendkit appends paths to the others.
func (s SocrateConfig) urlErrors() []string {
	var errs []string
	check := func(name, value string, isBase bool) {
		if value == "" {
			return
		}
		u, err := url.Parse(value)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			errs = append(errs, name+" must be an absolute http(s) URL, got "+strconv.Quote(value))
			return
		}
		if isBase && strings.HasSuffix(value, "/") {
			errs = append(errs, name+" must not end with /")
		}
	}
	check("SOCRATE_BASE_URL", s.BaseURL, true)
	check("SOCRATE_ADMIN_URL", s.AdminBaseURL, true)
	check("SOCRATE_JWKS_URL", s.JWKSURL, false)
	return errs
}

// errors reports a BFF setting that is missing or unsafe. In production the
// redirect URL is required and must be https, and the insecure cookie is
// refused; elsewhere the insecure cookie needs an http redirect URL, so it is
// never used on an https origin.
func (b BFFConfig) errors(prod bool) []string {
	var errs []string
	if prod && b.RedirectURL == "" {
		errs = append(errs, "BFF_REDIRECT_URL")
	}
	var scheme string
	if b.RedirectURL != "" {
		u, err := url.Parse(b.RedirectURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			errs = append(errs, "BFF_REDIRECT_URL must be an absolute http(s) URL, got "+strconv.Quote(b.RedirectURL))
		} else {
			scheme = u.Scheme
		}
	}
	if prod && scheme == "http" {
		errs = append(errs, "BFF_REDIRECT_URL must be https in production")
	}
	if b.InsecureCookie {
		switch {
		case prod:
			errs = append(errs, "BFF_INSECURE_COOKIE must not be enabled in production")
		case scheme != "http":
			errs = append(errs, "BFF_INSECURE_COOKIE needs an http:// BFF_REDIRECT_URL (local development only)")
		}
	}
	if !b.Enabled() {
		return errs
	}
	if b.CookieName == "" || strings.ContainsAny(b.CookieName, " \t;,=\"") {
		errs = append(errs, "BFF_COOKIE_NAME must be a plain cookie name")
	}
	if b.IdleTTL <= 0 || b.AbsoluteTTL <= 0 {
		errs = append(errs, "BFF_SESSION_IDLE_TTL and BFF_SESSION_ABSOLUTE_TTL must be positive durations")
	} else if b.IdleTTL > b.AbsoluteTTL {
		errs = append(errs, "BFF_SESSION_IDLE_TTL must not exceed BFF_SESSION_ABSOLUTE_TTL")
	}
	return errs
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

// envOrDefaultDuration parses a Go duration ("30m", "8h"). A value that does
// not parse becomes 0, which Validate rejects, rather than silently the default.
func envOrDefaultDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0
	}
	return d
}

func envOrDefaultFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}
