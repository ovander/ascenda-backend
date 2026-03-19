package model

import (
	"github.com/shopspring/decimal"
)

// CountryConfig holds country-specific tax and regulatory configurations
type CountryConfig struct {
	Name                    string            `json:"name"`
	Code                    string            `json:"code"` // ISO 3166-1 alpha-2
	CorporateTaxRates       []decimal.Decimal `json:"corporateTaxRates"` // By tax bracket
	EmployerChargeRate      decimal.Decimal   `json:"employerChargeRate"`
	EmployeeContributionRate decimal.Decimal  `json:"employeeContributionRate"`
	VATPrimaryRate          decimal.Decimal   `json:"vatPrimaryRate"`
	VATReducedRate          decimal.Decimal   `json:"vatReducedRate"`
	Currency                string            `json:"currency"`
}

// CountryConfigs defines standard country configurations
var CountryConfigs = map[string]CountryConfig{
	"BE": {
		Name:                  "Belgium",
		Code:                  "BE",
		CorporateTaxRates:     []decimal.Decimal{decimal.RequireFromString("0.25")},
		EmployerChargeRate:    decimal.RequireFromString("0.3325"),
		EmployeeContributionRate: decimal.RequireFromString("0.1345"),
		VATPrimaryRate:        decimal.RequireFromString("0.21"),
		VATReducedRate:        decimal.RequireFromString("0.06"),
		Currency:              "EUR",
	},
	"NL": {
		Name:                  "Netherlands",
		Code:                  "NL",
		CorporateTaxRates:     []decimal.Decimal{decimal.RequireFromString("0.195")},
		EmployerChargeRate:    decimal.RequireFromString("0.248"),
		EmployeeContributionRate: decimal.RequireFromString("0.1495"),
		VATPrimaryRate:        decimal.RequireFromString("0.21"),
		VATReducedRate:        decimal.RequireFromString("0.09"),
		Currency:              "EUR",
	},
	"FR": {
		Name:                  "France",
		Code:                  "FR",
		CorporateTaxRates:     []decimal.Decimal{decimal.RequireFromString("0.25")},
		EmployerChargeRate:    decimal.RequireFromString("0.42"),
		EmployeeContributionRate: decimal.RequireFromString("0.0775"),
		VATPrimaryRate:        decimal.RequireFromString("0.20"),
		VATReducedRate:        decimal.RequireFromString("0.055"),
		Currency:              "EUR",
	},
	"DE": {
		Name:                  "Germany",
		Code:                  "DE",
		CorporateTaxRates:     []decimal.Decimal{decimal.RequireFromString("0.30")},
		EmployerChargeRate:    decimal.RequireFromString("0.1475"),
		EmployeeContributionRate: decimal.RequireFromString("0.1785"),
		VATPrimaryRate:        decimal.RequireFromString("0.19"),
		VATReducedRate:        decimal.RequireFromString("0.07"),
		Currency:              "EUR",
	},
}
