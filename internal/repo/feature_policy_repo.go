package repo

import (
	"fmt"
	"time"

	"ascenda/internal/model"
	"gorm.io/gorm"
)

// FeaturePolicyRepository is the persistence interface for feature policies.
type FeaturePolicyRepository interface {
	List() ([]*model.FeaturePolicy, error)
	GetByFeature(feature string) (*model.FeaturePolicy, error)
	Upsert(p *model.FeaturePolicy) error
	SeedDefaults(defaults []model.FeaturePolicy) error
}

// FeaturePolicyRepo is the GORM implementation.
type FeaturePolicyRepo struct {
	db *gorm.DB
}

func NewFeaturePolicyRepo(db *gorm.DB) *FeaturePolicyRepo {
	return &FeaturePolicyRepo{db: db}
}

func (r *FeaturePolicyRepo) List() ([]*model.FeaturePolicy, error) {
	var policies []*model.FeaturePolicy
	if err := r.db.Order("category, feature").Find(&policies).Error; err != nil {
		return nil, err
	}
	return policies, nil
}

func (r *FeaturePolicyRepo) GetByFeature(feature string) (*model.FeaturePolicy, error) {
	var p model.FeaturePolicy
	if err := r.db.First(&p, "feature = ?", feature).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// Upsert inserts or fully replaces a policy row, updating the timestamp.
func (r *FeaturePolicyRepo) Upsert(p *model.FeaturePolicy) error {
	p.UpdatedAt = time.Now().UTC()
	return r.db.Save(p).Error
}

// SeedDefaults inserts the default rows using ON CONFLICT DO NOTHING so that
// existing admin overrides made at runtime are never overwritten on restart.
// json.RawMessage is already valid JSON — passed directly as a string literal.
func (r *FeaturePolicyRepo) SeedDefaults(defaults []model.FeaturePolicy) error {
	now := time.Now().UTC()
	const q = `INSERT INTO feature_policies
		(feature, category, label, feature_type, freemium, pro, enterprise, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (feature) DO NOTHING`

	for _, p := range defaults {
		if err := r.db.Exec(q,
			p.Feature, p.Category, p.Label, string(p.FeatureType),
			string(p.Freemium), string(p.Pro), string(p.Enterprise),
			now,
		).Error; err != nil {
			return fmt.Errorf("seed feature_policies %q: %w", p.Feature, err)
		}
	}
	return nil
}
