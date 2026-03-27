package service

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"ascenda/internal/model"
)

// countryRates holds the statutory financial rates used to seed a new PlanConfig.
// All rates are fractions (0.25 = 25 %).
// Sources: official tax authority / social-security websites, 2024 figures.
type countryRates struct {
	CorporateTaxRate decimal.Decimal // standard corporate income-tax rate
	VATRate          decimal.Decimal // standard VAT / GST rate
	EmployerTaxRate  decimal.Decimal // employer social-security contribution rate
	MLTInterestRate  decimal.Decimal // indicative medium-term bank lending rate
	Language         string          // ISO 639-1 primary language code
	CurrencySymbol   string
}

// ratesByCountry maps ISO 3166-1 alpha-2 country codes to their default rates.
var ratesByCountry = map[string]countryRates{
	// ── Euro-zone ─────────────────────────────────────────────────────────────
	"BE": {
		CorporateTaxRate: decimal.NewFromFloat(0.25),   // 25 % standard rate
		VATRate:          decimal.NewFromFloat(0.21),   // 21 % standard rate
		EmployerTaxRate:  decimal.NewFromFloat(0.2767), // ~27.67 % social charges
		MLTInterestRate:  decimal.NewFromFloat(0.03),
		Language:         "fr",
		CurrencySymbol:   "€",
	},
	"FR": {
		CorporateTaxRate: decimal.NewFromFloat(0.25),   // 25 % standard rate
		VATRate:          decimal.NewFromFloat(0.20),   // 20 % standard rate
		EmployerTaxRate:  decimal.NewFromFloat(0.42),   // ~42 % all employer charges
		MLTInterestRate:  decimal.NewFromFloat(0.03),
		Language:         "fr",
		CurrencySymbol:   "€",
	},
	"LU": {
		CorporateTaxRate: decimal.NewFromFloat(0.17),   // 17 % federal rate
		VATRate:          decimal.NewFromFloat(0.17),   // 17 % standard rate
		EmployerTaxRate:  decimal.NewFromFloat(0.12),   // ~12 % employer contributions
		MLTInterestRate:  decimal.NewFromFloat(0.03),
		Language:         "fr",
		CurrencySymbol:   "€",
	},
	"NL": {
		CorporateTaxRate: decimal.NewFromFloat(0.258),  // 25.8 % above €200k
		VATRate:          decimal.NewFromFloat(0.21),   // 21 % standard rate
		EmployerTaxRate:  decimal.NewFromFloat(0.20),   // ~20 % social charges
		MLTInterestRate:  decimal.NewFromFloat(0.03),
		Language:         "nl",
		CurrencySymbol:   "€",
	},
	"DE": {
		CorporateTaxRate: decimal.NewFromFloat(0.299),  // ~30 % (Körperschaftsteuer + trade tax)
		VATRate:          decimal.NewFromFloat(0.19),   // 19 % standard rate
		EmployerTaxRate:  decimal.NewFromFloat(0.20),   // ~20 % social charges
		MLTInterestRate:  decimal.NewFromFloat(0.035),
		Language:         "de",
		CurrencySymbol:   "€",
	},
	"ES": {
		CorporateTaxRate: decimal.NewFromFloat(0.25),   // 25 % standard rate
		VATRate:          decimal.NewFromFloat(0.21),   // 21 % standard rate
		EmployerTaxRate:  decimal.NewFromFloat(0.30),   // ~30 % social charges
		MLTInterestRate:  decimal.NewFromFloat(0.035),
		Language:         "es",
		CurrencySymbol:   "€",
	},
	"IT": {
		CorporateTaxRate: decimal.NewFromFloat(0.279),  // 24 % IRES + ~3.9 % IRAP
		VATRate:          decimal.NewFromFloat(0.22),   // 22 % standard rate
		EmployerTaxRate:  decimal.NewFromFloat(0.30),   // ~30 % social charges
		MLTInterestRate:  decimal.NewFromFloat(0.035),
		Language:         "it",
		CurrencySymbol:   "€",
	},
	"PT": {
		CorporateTaxRate: decimal.NewFromFloat(0.21),   // 21 % standard rate
		VATRate:          decimal.NewFromFloat(0.23),   // 23 % standard rate
		EmployerTaxRate:  decimal.NewFromFloat(0.2375), // 23.75 % social charges
		MLTInterestRate:  decimal.NewFromFloat(0.035),
		Language:         "pt",
		CurrencySymbol:   "€",
	},
	"IE": {
		CorporateTaxRate: decimal.NewFromFloat(0.125),  // 12.5 % trading rate
		VATRate:          decimal.NewFromFloat(0.23),   // 23 % standard rate
		EmployerTaxRate:  decimal.NewFromFloat(0.1105), // 11.05 % PRSI
		MLTInterestRate:  decimal.NewFromFloat(0.03),
		Language:         "en",
		CurrencySymbol:   "€",
	},
	// ── Non-euro Europe ───────────────────────────────────────────────────────
	"CH": {
		CorporateTaxRate: decimal.NewFromFloat(0.18),   // ~18 % effective (federal + cantonal)
		VATRate:          decimal.NewFromFloat(0.081),  // 8.1 % standard rate (2024)
		EmployerTaxRate:  decimal.NewFromFloat(0.0635), // ~6.35 % AHV/IV/EO employer part
		MLTInterestRate:  decimal.NewFromFloat(0.02),
		Language:         "fr",
		CurrencySymbol:   "CHF",
	},
	"GB": {
		CorporateTaxRate: decimal.NewFromFloat(0.25),   // 25 % from April 2023
		VATRate:          decimal.NewFromFloat(0.20),   // 20 % standard rate
		EmployerTaxRate:  decimal.NewFromFloat(0.138),  // 13.8 % National Insurance
		MLTInterestRate:  decimal.NewFromFloat(0.045),
		Language:         "en",
		CurrencySymbol:   "£",
	},
	// ── Americas ──────────────────────────────────────────────────────────────
	"US": {
		CorporateTaxRate: decimal.NewFromFloat(0.21),   // 21 % federal rate
		VATRate:          decimal.NewFromFloat(0.00),   // no federal VAT (state sales tax varies)
		EmployerTaxRate:  decimal.NewFromFloat(0.0765), // 7.65 % FICA (Social Security + Medicare)
		MLTInterestRate:  decimal.NewFromFloat(0.05),
		Language:         "en",
		CurrencySymbol:   "$",
	},
	"CA": {
		CorporateTaxRate: decimal.NewFromFloat(0.265),  // 15 % federal + ~11.5 % provincial avg
		VATRate:          decimal.NewFromFloat(0.05),   // 5 % federal GST (+ provincial varies)
		EmployerTaxRate:  decimal.NewFromFloat(0.076),  // ~7.6 % CPP + EI employer share
		MLTInterestRate:  decimal.NewFromFloat(0.045),
		Language:         "fr",
		CurrencySymbol:   "CA$",
	},
}

// fallbackRates is used when the requested country code is not in ratesByCountry.
var fallbackRates = countryRates{
	CorporateTaxRate: decimal.NewFromFloat(0.25),
	VATRate:          decimal.NewFromFloat(0.20),
	EmployerTaxRate:  decimal.NewFromFloat(0.25),
	MLTInterestRate:  decimal.NewFromFloat(0.04),
	Language:         "en",
	CurrencySymbol:   "€",
}

// ratesFor returns the statutory rates for the given ISO 3166-1 alpha-2 country
// code, falling back to generic European defaults for unknown codes.
func ratesFor(country string) countryRates {
	if r, ok := ratesByCountry[country]; ok {
		return r
	}
	return fallbackRates
}

// defaultPlanConfigFromRates constructs a *model.PlanConfig from a resolved
// countryRates struct. Shared by PlanService and SeedService so both use the
// same logic regardless of whether rates came from the DB or the hard-coded map.
func defaultPlanConfigFromRates(tenantID, scenarioID uuid.UUID, country string, rates countryRates) *model.PlanConfig {
	if country == "" {
		country = "BE"
	}
	nextJan1 := time.Date(time.Now().Year()+1, time.January, 1, 0, 0, 0, 0, time.UTC)
	cfg := &model.PlanConfig{
		TenantScoped:          model.TenantScoped{ID: uuid.New()},
		ScenarioID:            scenarioID,
		Language:              rates.Language,
		Country:               country,
		CurrencySymbol:        rates.CurrencySymbol,
		ForecastStart:         nextJan1,
		FirstFiscalYearMonths: 12,
		SalaryMonthsPerYear:   12,
		MLTLoanTermYears:      5,
		AvgBillTermMonths:     3,
		CorporateTaxRate:      rates.CorporateTaxRate,
		VATRate:               rates.VATRate,
		EmployerTaxRate:       rates.EmployerTaxRate,
		MLTInterestRate:       rates.MLTInterestRate,
		DiscountRate:          decimal.NewFromFloat(0.10), // 10 % — analyst-set, not statutory
	}
	cfg.TenantID = tenantID
	return cfg
}
