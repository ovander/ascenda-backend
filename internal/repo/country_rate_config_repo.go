package repo

import (
	"ascenda/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CountryRateConfigRepository defines the data-access interface.
type CountryRateConfigRepository interface {
	List() ([]*model.CountryRateConfig, error)
	GetByCode(code string) (*model.CountryRateConfig, error)
	Upsert(cfg *model.CountryRateConfig) error
}

// CountryRateConfigRepo is the GORM implementation.
type CountryRateConfigRepo struct {
	db *gorm.DB
}

func NewCountryRateConfigRepo(db *gorm.DB) *CountryRateConfigRepo {
	return &CountryRateConfigRepo{db: db}
}

func (r *CountryRateConfigRepo) List() ([]*model.CountryRateConfig, error) {
	var out []*model.CountryRateConfig
	if err := r.db.Order("country_code ASC").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (r *CountryRateConfigRepo) GetByCode(code string) (*model.CountryRateConfig, error) {
	var out model.CountryRateConfig
	if err := r.db.Where("country_code = ?", code).First(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

// Upsert inserts or updates a country rate config using ON CONFLICT DO UPDATE.
func (r *CountryRateConfigRepo) Upsert(cfg *model.CountryRateConfig) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "country_code"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"country_name",
			"corporate_tax_rate",
			"vat_rate",
			"employer_tax_rate",
			"mlt_interest_rate",
			"language",
			"currency_symbol",
			"updated_at",
		}),
	}).Create(cfg).Error
}

// compile-time check
var _ CountryRateConfigRepository = (*CountryRateConfigRepo)(nil)
