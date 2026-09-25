package model

import (
	"github.com/shopspring/decimal"
)

// CountryConfig holds country-specific tax and regulatory configurations.
type CountryConfig struct {
	Name                     string            `json:"name"`
	Code                     string            `json:"code"`              // ISO 3166-1 alpha-2
	CorporateTaxRates        []decimal.Decimal `json:"corporateTaxRates"` // By tax bracket
	EmployerChargeRate       decimal.Decimal   `json:"employerChargeRate"`
	EmployeeContributionRate decimal.Decimal   `json:"employeeContributionRate"`
	VATPrimaryRate           decimal.Decimal   `json:"vatPrimaryRate"`
	VATReducedRate           decimal.Decimal   `json:"vatReducedRate"`
	Currency                 string            `json:"currency"` // ISO 4217
	CurrencySymbol           string            `json:"currencySymbol"`
	DateFormat               string            `json:"dateFormat"`      // "DD/MM/YYYY" | "MM/DD/YYYY" | "YYYY-MM-DD"
	PrimaryLanguage          string            `json:"primaryLanguage"` // BCP-47 language tag
}

// GetCountryConfig returns the configuration for the given ISO country code.
// Falls back to Belgium if the code is not found.
func GetCountryConfig(code string) CountryConfig {
	if cfg, ok := CountryConfigs[code]; ok {
		return cfg
	}
	return CountryConfigs["BE"]
}

// CountryConfigs defines standard country configurations.
var CountryConfigs = map[string]CountryConfig{
	// ── Belgium ──────────────────────────────────────────────────────────────
	"BE": {
		Name:                     "Belgium",
		Code:                     "BE",
		CorporateTaxRates:        []decimal.Decimal{decimal.RequireFromString("0.25")},
		EmployerChargeRate:       decimal.RequireFromString("0.3325"),
		EmployeeContributionRate: decimal.RequireFromString("0.1345"),
		VATPrimaryRate:           decimal.RequireFromString("0.21"),
		VATReducedRate:           decimal.RequireFromString("0.06"),
		Currency:                 "EUR",
		CurrencySymbol:           "€",
		DateFormat:               "DD/MM/YYYY",
		PrimaryLanguage:          "fr",
	},
	// ── Netherlands ───────────────────────────────────────────────────────────
	"NL": {
		Name:                     "Netherlands",
		Code:                     "NL",
		CorporateTaxRates:        []decimal.Decimal{decimal.RequireFromString("0.195")},
		EmployerChargeRate:       decimal.RequireFromString("0.248"),
		EmployeeContributionRate: decimal.RequireFromString("0.1495"),
		VATPrimaryRate:           decimal.RequireFromString("0.21"),
		VATReducedRate:           decimal.RequireFromString("0.09"),
		Currency:                 "EUR",
		CurrencySymbol:           "€",
		DateFormat:               "DD/MM/YYYY",
		PrimaryLanguage:          "nl",
	},
	// ── France ────────────────────────────────────────────────────────────────
	"FR": {
		Name:                     "France",
		Code:                     "FR",
		CorporateTaxRates:        []decimal.Decimal{decimal.RequireFromString("0.25")},
		EmployerChargeRate:       decimal.RequireFromString("0.42"),
		EmployeeContributionRate: decimal.RequireFromString("0.0775"),
		VATPrimaryRate:           decimal.RequireFromString("0.20"),
		VATReducedRate:           decimal.RequireFromString("0.055"),
		Currency:                 "EUR",
		CurrencySymbol:           "€",
		DateFormat:               "DD/MM/YYYY",
		PrimaryLanguage:          "fr",
	},
	// ── Germany ───────────────────────────────────────────────────────────────
	"DE": {
		Name:                     "Germany",
		Code:                     "DE",
		CorporateTaxRates:        []decimal.Decimal{decimal.RequireFromString("0.30")},
		EmployerChargeRate:       decimal.RequireFromString("0.1475"),
		EmployeeContributionRate: decimal.RequireFromString("0.1785"),
		VATPrimaryRate:           decimal.RequireFromString("0.19"),
		VATReducedRate:           decimal.RequireFromString("0.07"),
		Currency:                 "EUR",
		CurrencySymbol:           "€",
		DateFormat:               "DD/MM/YYYY",
		PrimaryLanguage:          "de",
	},
	// ── United Kingdom ────────────────────────────────────────────────────────
	"GB": {
		Name:                     "United Kingdom",
		Code:                     "GB",
		CorporateTaxRates:        []decimal.Decimal{decimal.RequireFromString("0.25")},
		EmployerChargeRate:       decimal.RequireFromString("0.138"),
		EmployeeContributionRate: decimal.RequireFromString("0.12"),
		VATPrimaryRate:           decimal.RequireFromString("0.20"),
		VATReducedRate:           decimal.RequireFromString("0.05"),
		Currency:                 "GBP",
		CurrencySymbol:           "£",
		DateFormat:               "DD/MM/YYYY",
		PrimaryLanguage:          "en",
	},
	// ── United States ─────────────────────────────────────────────────────────
	"US": {
		Name:                     "United States",
		Code:                     "US",
		CorporateTaxRates:        []decimal.Decimal{decimal.RequireFromString("0.21")},
		EmployerChargeRate:       decimal.RequireFromString("0.0765"), // FICA employer share
		EmployeeContributionRate: decimal.RequireFromString("0.0765"), // FICA employee share
		VATPrimaryRate:           decimal.RequireFromString("0.00"),   // No federal VAT; state sales tax varies
		VATReducedRate:           decimal.RequireFromString("0.00"),
		Currency:                 "USD",
		CurrencySymbol:           "$",
		DateFormat:               "MM/DD/YYYY",
		PrimaryLanguage:          "en",
	},
	// ── Switzerland ───────────────────────────────────────────────────────────
	"CH": {
		Name:                     "Switzerland",
		Code:                     "CH",
		CorporateTaxRates:        []decimal.Decimal{decimal.RequireFromString("0.185")}, // effective combined rate
		EmployerChargeRate:       decimal.RequireFromString("0.0625"),
		EmployeeContributionRate: decimal.RequireFromString("0.0625"),
		VATPrimaryRate:           decimal.RequireFromString("0.081"),
		VATReducedRate:           decimal.RequireFromString("0.026"),
		Currency:                 "CHF",
		CurrencySymbol:           "CHF",
		DateFormat:               "DD.MM.YYYY",
		PrimaryLanguage:          "fr",
	},
	// ── Spain ─────────────────────────────────────────────────────────────────
	"ES": {
		Name:                     "Spain",
		Code:                     "ES",
		CorporateTaxRates:        []decimal.Decimal{decimal.RequireFromString("0.25")},
		EmployerChargeRate:       decimal.RequireFromString("0.296"),
		EmployeeContributionRate: decimal.RequireFromString("0.064"),
		VATPrimaryRate:           decimal.RequireFromString("0.21"),
		VATReducedRate:           decimal.RequireFromString("0.10"),
		Currency:                 "EUR",
		CurrencySymbol:           "€",
		DateFormat:               "DD/MM/YYYY",
		PrimaryLanguage:          "es",
	},
	// ── Italy ─────────────────────────────────────────────────────────────────
	"IT": {
		Name:                     "Italy",
		Code:                     "IT",
		CorporateTaxRates:        []decimal.Decimal{decimal.RequireFromString("0.24")},
		EmployerChargeRate:       decimal.RequireFromString("0.30"),
		EmployeeContributionRate: decimal.RequireFromString("0.0919"),
		VATPrimaryRate:           decimal.RequireFromString("0.22"),
		VATReducedRate:           decimal.RequireFromString("0.10"),
		Currency:                 "EUR",
		CurrencySymbol:           "€",
		DateFormat:               "DD/MM/YYYY",
		PrimaryLanguage:          "it",
	},
	// ── Luxembourg ────────────────────────────────────────────────────────────
	"LU": {
		Name:                     "Luxembourg",
		Code:                     "LU",
		CorporateTaxRates:        []decimal.Decimal{decimal.RequireFromString("0.248")},
		EmployerChargeRate:       decimal.RequireFromString("0.12"),
		EmployeeContributionRate: decimal.RequireFromString("0.12"),
		VATPrimaryRate:           decimal.RequireFromString("0.17"),
		VATReducedRate:           decimal.RequireFromString("0.08"),
		Currency:                 "EUR",
		CurrencySymbol:           "€",
		DateFormat:               "DD/MM/YYYY",
		PrimaryLanguage:          "fr",
	},
	// ── Sweden ────────────────────────────────────────────────────────────────
	"SE": {
		Name:                     "Sweden",
		Code:                     "SE",
		CorporateTaxRates:        []decimal.Decimal{decimal.RequireFromString("0.206")},
		EmployerChargeRate:       decimal.RequireFromString("0.3142"),
		EmployeeContributionRate: decimal.RequireFromString("0.07"),
		VATPrimaryRate:           decimal.RequireFromString("0.25"),
		VATReducedRate:           decimal.RequireFromString("0.12"),
		Currency:                 "SEK",
		CurrencySymbol:           "kr",
		DateFormat:               "YYYY-MM-DD",
		PrimaryLanguage:          "sv",
	},
	// ── Norway ────────────────────────────────────────────────────────────────
	"NO": {
		Name:                     "Norway",
		Code:                     "NO",
		CorporateTaxRates:        []decimal.Decimal{decimal.RequireFromString("0.22")},
		EmployerChargeRate:       decimal.RequireFromString("0.141"),
		EmployeeContributionRate: decimal.RequireFromString("0.082"),
		VATPrimaryRate:           decimal.RequireFromString("0.25"),
		VATReducedRate:           decimal.RequireFromString("0.12"),
		Currency:                 "NOK",
		CurrencySymbol:           "kr",
		DateFormat:               "DD.MM.YYYY",
		PrimaryLanguage:          "no",
	},
	// ── Canada ────────────────────────────────────────────────────────────────
	"CA": {
		Name:                     "Canada",
		Code:                     "CA",
		CorporateTaxRates:        []decimal.Decimal{decimal.RequireFromString("0.265")}, // federal 15% + avg provincial
		EmployerChargeRate:       decimal.RequireFromString("0.0595"),                   // EI + CPP employer
		EmployeeContributionRate: decimal.RequireFromString("0.0595"),
		VATPrimaryRate:           decimal.RequireFromString("0.05"), // federal GST only
		VATReducedRate:           decimal.RequireFromString("0.00"),
		Currency:                 "CAD",
		CurrencySymbol:           "CA$",
		DateFormat:               "DD/MM/YYYY",
		PrimaryLanguage:          "en",
	},
}
