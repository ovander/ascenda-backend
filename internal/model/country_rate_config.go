package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// CountryRateConfig stores statutory financial rates for one country in the DB.
// It mirrors the hard-coded countryRates struct in service/country_defaults.go
// but is persisted so platform admins can update rates without a code deploy.
//
// Primary key is the ISO 3166-1 alpha-2 country code (e.g. "FR", "BE").
// There is no tenant scope — these are platform-wide defaults.
type CountryRateConfig struct {
	CountryCode      string          `gorm:"primaryKey;type:varchar(2)"         json:"countryCode"`
	CountryName      string          `gorm:"type:varchar(100);not null;default:''"  json:"countryName"`
	CorporateTaxRate decimal.Decimal `gorm:"type:numeric(6,4);not null;default:0"  json:"corporateTaxRate"`
	VATRate          decimal.Decimal `gorm:"type:numeric(6,4);not null;default:0"  json:"vatRate"`
	EmployerTaxRate  decimal.Decimal `gorm:"type:numeric(6,4);not null;default:0"  json:"employerTaxRate"`
	MLTInterestRate  decimal.Decimal `gorm:"type:numeric(6,4);not null;default:0"  json:"mltInterestRate"`
	Language         string          `gorm:"type:varchar(10);not null;default:'en'" json:"language"`
	CurrencySymbol   string          `gorm:"type:varchar(10);not null;default:'€'"  json:"currencySymbol"`
	UpdatedAt        time.Time       `gorm:"autoUpdateTime;not null;default:now()" json:"updatedAt"`
}

func (CountryRateConfig) TableName() string { return "country_rate_configs" }
