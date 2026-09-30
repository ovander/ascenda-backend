package config

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowDefaultTenantFallback_DefaultsByEnvironment(t *testing.T) {
	cases := map[string]bool{"development": true, "staging": false, "production": false, "": true}
	for env, want := range cases {
		t.Run("env="+env, func(t *testing.T) {
			if env != "" {
				t.Setenv("APP_ENV", env)
			} else {
				t.Setenv("APP_ENV", "") // envOrDefault treats "" as unset → development
			}
			t.Setenv("TENANT_DEFAULT_FALLBACK", "")
			assert.Equal(t, want, Load().AllowDefaultTenantFallback)
		})
	}
}

func TestAllowDefaultTenantFallback_ExplicitOverride(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("TENANT_DEFAULT_FALLBACK", "false")
	assert.False(t, Load().AllowDefaultTenantFallback)

	t.Setenv("APP_ENV", "staging")
	t.Setenv("TENANT_DEFAULT_FALLBACK", "true")
	assert.True(t, Load().AllowDefaultTenantFallback)
}

func TestValidate_RefusesDefaultTenantFallbackInProduction(t *testing.T) {
	cfg := &Config{
		Env:                        "production",
		DatabaseURL:                "postgres://x",
		AllowDefaultTenantFallback: true,
		Socrate:                    validProdSocrate(),
		BFF:                        validProdBFF(),
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "TENANT_DEFAULT_FALLBACK"), err.Error())

	cfg.AllowDefaultTenantFallback = false
	assert.NoError(t, cfg.Validate())
}

func TestTrustedProxyCIDRs(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "")
	assert.Equal(t, []string{"127.0.0.1/32", "::1/128"}, Load().TrustedProxyCIDRs, "default trusts loopback only")

	t.Setenv("TRUSTED_PROXY_CIDRS", " 10.0.0.0/8 , 192.168.1.5 ,")
	assert.Equal(t, []string{"10.0.0.0/8", "192.168.1.5"}, Load().TrustedProxyCIDRs)
}

// validProdSocrate is a complete production Socrate configuration.
func validProdSocrate() SocrateConfig {
	return SocrateConfig{
		BaseURL:      "https://socrate.vandermoten.eu",
		AdminBaseURL: "http://127.0.0.1:18082",
		JWKSURL:      "https://socrate.vandermoten.eu/.well-known/jwks.json",
		ClientID:     "id",
		ClientSecret: "secret",
	}
}

// validProdBFF is a complete production BFF configuration.
func validProdBFF() BFFConfig {
	return BFFConfig{
		RedirectURL: "https://ascenda.vandermoten.eu/bff/callback",
		CookieName:  "ascenda_session",
		IdleTTL:     30 * time.Minute,
		AbsoluteTTL: 8 * time.Hour,
	}
}

func TestBFFConfig_Defaults(t *testing.T) {
	for _, k := range []string{"BFF_REDIRECT_URL", "BFF_COOKIE_NAME", "BFF_SESSION_IDLE_TTL", "BFF_SESSION_ABSOLUTE_TTL", "BFF_INSECURE_COOKIE"} {
		t.Setenv(k, "")
	}
	b := Load().BFF
	assert.Equal(t, "ascenda_session", b.CookieName)
	assert.Equal(t, 30*time.Minute, b.IdleTTL)
	assert.Equal(t, 8*time.Hour, b.AbsoluteTTL)
	assert.False(t, b.InsecureCookie, "Secure unless explicitly turned off")
	assert.False(t, b.Enabled())

	t.Setenv("BFF_SESSION_IDLE_TTL", "thirty minutes")
	assert.Zero(t, Load().BFF.IdleTTL, "an unparsable duration is 0, which Validate rejects, not the default")
}

func TestValidate_BFF(t *testing.T) {
	validate := func(env string, mutate func(*BFFConfig)) error {
		b := validProdBFF()
		mutate(&b)
		return (&Config{Env: env, DatabaseURL: "postgres://x", Socrate: validProdSocrate(), BFF: b}).Validate()
	}
	require.NoError(t, validate("production", func(*BFFConfig) {}))

	cases := []struct {
		name, env string
		mutate    func(*BFFConfig)
		want      string
	}{
		{"redirect URL missing in production", "production", func(b *BFFConfig) { b.RedirectURL = "" }, "BFF_REDIRECT_URL"},
		{"http redirect URL in production", "production", func(b *BFFConfig) { b.RedirectURL = "http://ascenda.vandermoten.eu/bff/callback" }, "must be https in production"},
		{"malformed redirect URL", "production", func(b *BFFConfig) { b.RedirectURL = "http:httpd://ascenda.vandermoten.eu/bff/callback" }, "BFF_REDIRECT_URL must be an absolute http(s) URL"},
		{"Secure:false in production", "production", func(b *BFFConfig) { b.InsecureCookie = true }, "BFF_INSECURE_COOKIE must not be enabled in production"},
		{"Secure:false on an https origin", "development", func(b *BFFConfig) { b.InsecureCookie = true }, "BFF_INSECURE_COOKIE needs an http://"},
		{"cookie name with a separator", "production", func(b *BFFConfig) { b.CookieName = "a;b" }, "BFF_COOKIE_NAME"},
		{"zero TTL", "production", func(b *BFFConfig) { b.IdleTTL = 0 }, "must be positive durations"},
		{"idle longer than absolute", "production", func(b *BFFConfig) { b.IdleTTL = 9 * time.Hour }, "must not exceed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validate(tc.env, tc.mutate)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}

	// Local development over plain http may drop Secure, and only there.
	require.NoError(t, validate("development", func(b *BFFConfig) {
		b.RedirectURL = "http://localhost:5173/bff/callback"
		b.InsecureCookie = true
	}))
	// Without a redirect URL outside production the BFF is off and its other settings are not checked.
	require.NoError(t, (&Config{Env: "development", DatabaseURL: "postgres://x"}).Validate())
}

func TestValidate_ProductionSocrateURLs(t *testing.T) {
	prod := func(mutate func(*SocrateConfig)) error {
		s := validProdSocrate()
		mutate(&s)
		return (&Config{Env: "production", DatabaseURL: "postgres://x", Socrate: s, BFF: validProdBFF()}).Validate()
	}

	require.NoError(t, prod(func(*SocrateConfig) {}))

	cases := []struct {
		name   string
		mutate func(*SocrateConfig)
		want   string
	}{
		{"issuer missing", func(s *SocrateConfig) { s.BaseURL = "" }, "SOCRATE_BASE_URL"},
		{"admin URL missing (backendkit would guess :8081)", func(s *SocrateConfig) { s.AdminBaseURL = "" }, "SOCRATE_ADMIN_URL"},
		{"issuer with trailing slash", func(s *SocrateConfig) { s.BaseURL = "https://socrate.vandermoten.eu/" }, "SOCRATE_BASE_URL must not end with /"},
		{"admin URL with trailing slash", func(s *SocrateConfig) { s.AdminBaseURL = "http://127.0.0.1:18082/" }, "SOCRATE_ADMIN_URL must not end with /"},
		{"relative JWKS URL", func(s *SocrateConfig) { s.JWKSURL = "/.well-known/jwks.json" }, "SOCRATE_JWKS_URL must be an absolute http(s) URL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := prod(tc.mutate)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestListenAddr(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("PORT", "8100")
	t.Setenv("BIND_ADDRESS", "")
	assert.Equal(t, ":8100", Load().ListenAddr(), "every interface by default outside production")

	t.Setenv("BIND_ADDRESS", "127.0.0.1")
	assert.Equal(t, "127.0.0.1:8100", Load().ListenAddr())

	t.Setenv("BIND_ADDRESS", "::1")
	assert.Equal(t, "[::1]:8100", Load().ListenAddr())
}

func TestValidate_AdminURLNeverDerivedOutsideProduction(t *testing.T) {
	dev := func(base, admin string) error {
		return (&Config{Env: "development", DatabaseURL: "postgres://x",
			Socrate: SocrateConfig{BaseURL: base, AdminBaseURL: admin}}).Validate()
	}

	// Without Socrate (local development) nothing is required.
	require.NoError(t, dev("", ""))
	// With Socrate, the admin URL must be explicit: backendkit would otherwise
	// derive <base host>:8081, a different service.
	err := dev("https://socrate.vandermoten.eu", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SOCRATE_ADMIN_URL")
	require.NoError(t, dev("https://socrate.vandermoten.eu", "http://127.0.0.1:18082"))
}

func TestListenAddr_LoopbackByDefaultInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("PORT", "")
	t.Setenv("BIND_ADDRESS", "")
	assert.Equal(t, "127.0.0.1:8080", Load().ListenAddr(), "Caddy on the same host proxies to loopback")

	t.Setenv("BIND_ADDRESS", "0.0.0.0")
	assert.Equal(t, "0.0.0.0:8080", Load().ListenAddr(), "explicit override, e.g. in a container")
}

// The image runs with APP_ENV=production; loopback inside a container would be
// unreachable through a published port, so the Dockerfile must listen on every
// interface explicitly.
func TestDockerfileListensOnEveryInterface(t *testing.T) {
	data, err := os.ReadFile("../../Dockerfile")
	require.NoError(t, err)
	df := string(data)
	require.Contains(t, df, "ENV APP_ENV=production")
	assert.Contains(t, df, "ENV BIND_ADDRESS=0.0.0.0")
}
