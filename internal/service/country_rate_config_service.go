package service

import (
	"errors"

	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"ascenda/internal/model"
	"ascenda/internal/pkg/apierror"
	"ascenda/internal/repo"
)

// CountryRateConfigService manages the DB-backed country rate configs and
// provides a ratesFor-compatible lookup used by PlanService / SeedService.
type CountryRateConfigService struct {
	repo   repo.CountryRateConfigRepository
	logger *logrus.Entry
}

func NewCountryRateConfigService(r repo.CountryRateConfigRepository, logger *logrus.Entry) *CountryRateConfigService {
	return &CountryRateConfigService{repo: r, logger: logger}
}

// SeedDefaults inserts the hard-coded defaults for any country code that is not
// yet present in the database. Existing rows are left untouched so admin edits
// are preserved across restarts.
func (s *CountryRateConfigService) SeedDefaults() error {
	for code, cr := range ratesByCountry {
		name := countryNames[code]
		cfg := &model.CountryRateConfig{
			CountryCode:      code,
			CountryName:      name,
			CorporateTaxRate: cr.CorporateTaxRate,
			VATRate:          cr.VATRate,
			EmployerTaxRate:  cr.EmployerTaxRate,
			MLTInterestRate:  cr.MLTInterestRate,
			Language:         cr.Language,
			CurrencySymbol:   cr.CurrencySymbol,
		}
		existing, err := s.repo.GetByCode(code)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if existing != nil {
			continue // already seeded — preserve any admin edits
		}
		if err := s.repo.Upsert(cfg); err != nil {
			s.logger.WithError(err).WithField("country_code", code).Warn("failed to seed country rate config")
		}
	}
	s.logger.Info("country rate configs seeded")
	return nil
}

// RatesFor returns the DB-backed rates for a country, falling back to the
// hard-coded fallbackRates if the code is not found.
func (s *CountryRateConfigService) RatesFor(country string) countryRates {
	cfg, err := s.repo.GetByCode(country)
	if err != nil || cfg == nil {
		// graceful fallback — DB may not yet be seeded in test/local setups
		return ratesFor(country)
	}
	return countryRates{
		CorporateTaxRate: cfg.CorporateTaxRate,
		VATRate:          cfg.VATRate,
		EmployerTaxRate:  cfg.EmployerTaxRate,
		MLTInterestRate:  cfg.MLTInterestRate,
		Language:         cfg.Language,
		CurrencySymbol:   cfg.CurrencySymbol,
	}
}

// List returns all country rate configs ordered by country code.
func (s *CountryRateConfigService) List() ([]*model.CountryRateConfig, error) {
	return s.repo.List()
}

// GetByCode returns a single country rate config.
func (s *CountryRateConfigService) GetByCode(code string) (*model.CountryRateConfig, error) {
	cfg, err := s.repo.GetByCode(code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierror.NotFound("country config", code)
		}
		return nil, apierror.Internal("failed to fetch country config")
	}
	return cfg, nil
}

// UpdateRequest is the admin update payload.
type UpdateCountryRateConfigRequest struct {
	CountryName      *string          `json:"countryName,omitempty"`
	CorporateTaxRate *decimal.Decimal `json:"corporateTaxRate,omitempty"`
	VATRate          *decimal.Decimal `json:"vatRate,omitempty"`
	EmployerTaxRate  *decimal.Decimal `json:"employerTaxRate,omitempty"`
	MLTInterestRate  *decimal.Decimal `json:"mltInterestRate,omitempty"`
}

// Update applies a partial update to an existing country rate config.
func (s *CountryRateConfigService) Update(code string, req UpdateCountryRateConfigRequest) (*model.CountryRateConfig, error) {
	cfg, err := s.repo.GetByCode(code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierror.NotFound("country config", code)
		}
		return nil, apierror.Internal("failed to fetch country config")
	}
	if req.CountryName != nil {
		cfg.CountryName = *req.CountryName
	}
	if req.CorporateTaxRate != nil {
		cfg.CorporateTaxRate = *req.CorporateTaxRate
	}
	if req.VATRate != nil {
		cfg.VATRate = *req.VATRate
	}
	if req.EmployerTaxRate != nil {
		cfg.EmployerTaxRate = *req.EmployerTaxRate
	}
	if req.MLTInterestRate != nil {
		cfg.MLTInterestRate = *req.MLTInterestRate
	}
	if err := s.repo.Upsert(cfg); err != nil {
		return nil, apierror.Internal("failed to update country config")
	}
	return cfg, nil
}

// ResetToDefault restores a country's rates to the hard-coded defaults.
func (s *CountryRateConfigService) ResetToDefault(code string) (*model.CountryRateConfig, error) {
	cr, ok := ratesByCountry[code]
	if !ok {
		cr = fallbackRates
	}
	cfg := &model.CountryRateConfig{
		CountryCode:      code,
		CountryName:      countryNames[code],
		CorporateTaxRate: cr.CorporateTaxRate,
		VATRate:          cr.VATRate,
		EmployerTaxRate:  cr.EmployerTaxRate,
		MLTInterestRate:  cr.MLTInterestRate,
		Language:         cr.Language,
		CurrencySymbol:   cr.CurrencySymbol,
	}
	if err := s.repo.Upsert(cfg); err != nil {
		return nil, apierror.Internal("failed to reset country config")
	}
	return cfg, nil
}

// countryNames maps ISO codes to display names for the seeder.
var countryNames = map[string]string{
	"BE": "Belgium",
	"FR": "France",
	"LU": "Luxembourg",
	"NL": "Netherlands",
	"DE": "Germany",
	"ES": "Spain",
	"IT": "Italy",
	"PT": "Portugal",
	"IE": "Ireland",
	"CH": "Switzerland",
	"GB": "United Kingdom",
	"US": "United States",
	"CA": "Canada",
}
