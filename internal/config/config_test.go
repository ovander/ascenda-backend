package config

import (
	"os"
	"strings"
	"testing"

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
		RedirectURL:  "https://ascenda.vandermoten.eu/callback",
	}
}

func TestValidate_ProductionSocrateURLs(t *testing.T) {
	prod := func(mutate func(*SocrateConfig)) error {
		s := validProdSocrate()
		mutate(&s)
		return (&Config{Env: "production", DatabaseURL: "postgres://x", Socrate: s}).Validate()
	}

	require.NoError(t, prod(func(*SocrateConfig) {}))
	require.NoError(t, prod(func(s *SocrateConfig) { s.InternalURL = "http://127.0.0.1:8080" }))

	cases := []struct {
		name   string
		mutate func(*SocrateConfig)
		want   string
	}{
		{"issuer missing", func(s *SocrateConfig) { s.BaseURL = "" }, "SOCRATE_BASE_URL"},
		{"admin URL missing (backendkit would guess :8081)", func(s *SocrateConfig) { s.AdminBaseURL = "" }, "SOCRATE_ADMIN_URL"},
		{"issuer with trailing slash", func(s *SocrateConfig) { s.BaseURL = "https://socrate.vandermoten.eu/" }, "SOCRATE_BASE_URL must not end with /"},
		{"internal URL with trailing slash", func(s *SocrateConfig) { s.InternalURL = "http://127.0.0.1:8080/" }, "SOCRATE_INTERNAL_URL must not end with /"},
		{"admin URL with trailing slash", func(s *SocrateConfig) { s.AdminBaseURL = "http://127.0.0.1:18082/" }, "SOCRATE_ADMIN_URL must not end with /"},
		{"malformed redirect URL", func(s *SocrateConfig) { s.RedirectURL = "http:httpd://ascenda.vandermoten.eu/callback" }, "SOCRATE_REDIRECT_URL must be an absolute http(s) URL"},
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

func TestSocrateConfig_OAuthCallURL(t *testing.T) {
	s := SocrateConfig{BaseURL: "https://socrate.vandermoten.eu"}
	assert.Equal(t, "https://socrate.vandermoten.eu", s.OAuthCallURL())
	s.InternalURL = "http://127.0.0.1:8080"
	assert.Equal(t, "http://127.0.0.1:8080", s.OAuthCallURL())
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

func TestLoad_SocrateInternalURL(t *testing.T) {
	t.Setenv("SOCRATE_BASE_URL", "https://socrate.vandermoten.eu")
	t.Setenv("SOCRATE_INTERNAL_URL", "http://127.0.0.1:8080")
	cfg := Load()
	assert.Equal(t, "https://socrate.vandermoten.eu", cfg.Socrate.BaseURL)
	assert.Equal(t, "http://127.0.0.1:8080", cfg.Socrate.OAuthCallURL())
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
