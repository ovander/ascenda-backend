package main

import (
	"testing"

	"ascenda/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// prodConfig is a production configuration that passes validation.
func prodConfig() *config.Config {
	return &config.Config{
		Env:         "production",
		Port:        8080,
		DatabaseURL: "postgres://ascenda@127.0.0.1:1/ascenda",
		Socrate: config.SocrateConfig{
			BaseURL:        "https://socrate.vandermoten.eu",
			AdminBaseURL:   "http://127.0.0.1:18082",
			JWKSURL:        "https://socrate.vandermoten.eu/.well-known/jwks.json",
			ClientID:       "VowmSxfnObxDKFvdk1Lucg",
			ClientSecret:   "test-secret",
			RedirectURL:    "https://ascenda.vandermoten.eu/callback",
			VerifyAudience: true,
		},
	}
}

// TestBootstrap_RefusesToStartInProduction checks that start-up itself stops,
// before any database or network call, when the Socrate configuration would
// be wrong in production.
func TestBootstrap_RefusesToStartInProduction(t *testing.T) {
	require.NoError(t, prodConfig().Validate(), "the baseline must be valid")

	cases := []struct {
		name   string
		mutate func(*config.Config)
		want   string
	}{
		{"SOCRATE_ADMIN_URL empty (never derived)", func(c *config.Config) { c.Socrate.AdminBaseURL = "" }, "SOCRATE_ADMIN_URL"},
		{"issuer with a trailing slash", func(c *config.Config) { c.Socrate.BaseURL = "https://socrate.vandermoten.eu/" }, "SOCRATE_BASE_URL must not end with /"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := prodConfig()
			tc.mutate(cfg)

			res, err := Bootstrap(cfg)

			require.Error(t, err)
			assert.Nil(t, res)
			assert.Contains(t, err.Error(), "configuration validation failed")
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
