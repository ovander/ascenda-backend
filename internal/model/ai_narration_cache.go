package model

import (
	"time"

	"github.com/google/uuid"
)

// AINarrationCache persists a single AI narration result.
//
// The primary key is (TenantID, CacheKey) where CacheKey is a content-addressed
// SHA-256 hash of the full NarrationContext.  If any input changes the hash
// changes and the old row is never matched again — no explicit invalidation
// is needed.
//
// There is intentionally no TTL column.  The content hash provides all the
// invalidation semantics required.  Stale-but-never-matched rows are harmless
// and can be cleaned up with a periodic maintenance job if needed.
type AINarrationCache struct {
	CacheKey  string     `gorm:"type:varchar(120);primaryKey" json:"cache_key"`
	TenantID  uuid.UUID  `gorm:"type:uuid;primaryKey" json:"tenant_id"`
	Output    []byte     `gorm:"type:jsonb;not null" json:"-"` // JSON-encoded NarrationOutput
	HitCount  int        `gorm:"not null;default:0" json:"hit_count"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
	LastHitAt *time.Time `json:"last_hit_at,omitempty"`
}

// TableName returns the table name for GORM.
func (AINarrationCache) TableName() string {
	return "ai_narration_cache"
}
