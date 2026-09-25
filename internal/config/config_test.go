package config

import (
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
		Socrate: SocrateConfig{
			JWKSURL: "https://idp/jwks", ClientID: "id", ClientSecret: "secret", RedirectURL: "https://app/cb",
		},
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
