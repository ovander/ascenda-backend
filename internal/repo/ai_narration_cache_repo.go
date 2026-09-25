package repo

import (
	"errors"
	"time"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AINarrationCacheRepository is the interface for persistent narration cache storage.
type AINarrationCacheRepository interface {
	// Get retrieves a cached output for (tenantID, cacheKey).
	// Returns (nil, nil) on a miss.
	Get(tenantID uuid.UUID, cacheKey string) ([]byte, error)

	// Put upserts a cached output under (tenantID, cacheKey).
	Put(tenantID uuid.UUID, cacheKey string, output []byte) error

	// DeleteByTenant removes all cached entries for a tenant.
	// Useful when a tenant's data is reset or when the tenant is deleted.
	DeleteByTenant(tenantID uuid.UUID) error
}

// AINarrationCacheRepo is the GORM implementation of AINarrationCacheRepository.
type AINarrationCacheRepo struct {
	db *gorm.DB
}

// NewAINarrationCacheRepo creates a new AINarrationCacheRepo.
func NewAINarrationCacheRepo(db *gorm.DB) *AINarrationCacheRepo {
	return &AINarrationCacheRepo{db: db}
}

// Get retrieves a cached narration output and increments the hit counter.
func (r *AINarrationCacheRepo) Get(tenantID uuid.UUID, cacheKey string) ([]byte, error) {
	var row model.AINarrationCache
	err := r.db.
		Where("tenant_id = ? AND cache_key = ?", tenantID, cacheKey).
		First(&row).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Update hit stats in the background — a failure here is non-critical.
	now := time.Now()
	r.db.Model(&row).Updates(map[string]interface{}{
		"hit_count":   gorm.Expr("hit_count + 1"),
		"last_hit_at": now,
	})

	return row.Output, nil
}

// Put upserts a narration result, resetting the hit counter on conflict.
func (r *AINarrationCacheRepo) Put(tenantID uuid.UUID, cacheKey string, output []byte) error {
	row := model.AINarrationCache{
		CacheKey:  cacheKey,
		TenantID:  tenantID,
		Output:    output,
		HitCount:  0,
		CreatedAt: time.Now(),
	}
	return r.db.
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "cache_key"}},
			DoUpdates: clause.AssignmentColumns([]string{"output", "hit_count", "created_at"}),
		}).
		Create(&row).Error
}

// DeleteByTenant removes all cached entries for a tenant.
func (r *AINarrationCacheRepo) DeleteByTenant(tenantID uuid.UUID) error {
	return r.db.
		Where("tenant_id = ?", tenantID).
		Delete(&model.AINarrationCache{}).Error
}
