package repo

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"ascenda/internal/model"
)

// MagicLinkRepo handles persistence for single-use sign-in tokens.
type MagicLinkRepo struct {
	db *gorm.DB
}

// NewMagicLinkRepo creates a new MagicLinkRepo.
func NewMagicLinkRepo(db *gorm.DB) *MagicLinkRepo {
	return &MagicLinkRepo{db: db}
}

// Create inserts a new magic-link token.
func (r *MagicLinkRepo) Create(t *model.MagicLinkToken) error {
	return r.db.Create(t).Error
}

// FindByTokenHash retrieves an unused, unexpired token by its SHA-256 hex hash.
// Returns (nil, nil) if not found — callers should treat that as "invalid token".
func (r *MagicLinkRepo) FindByTokenHash(hash string) (*model.MagicLinkToken, error) {
	var t model.MagicLinkToken
	err := r.db.
		Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", hash, time.Now()).
		First(&t).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// MarkUsed stamps the token with the current time, making it single-use.
func (r *MagicLinkRepo) MarkUsed(id uuid.UUID) error {
	now := time.Now()
	return r.db.Model(&model.MagicLinkToken{}).
		Where("id = ?", id).
		Update("used_at", now).Error
}

// DeleteExpired removes tokens that have passed their expiry and are no longer needed.
// Safe to call from a periodic background job.
func (r *MagicLinkRepo) DeleteExpired() error {
	return r.db.
		Where("expires_at < ?", time.Now()).
		Delete(&model.MagicLinkToken{}).Error
}
