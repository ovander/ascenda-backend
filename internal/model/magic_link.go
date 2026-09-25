package model

import (
	"time"

	"github.com/google/uuid"
)

// MagicLinkToken stores a single-use, short-lived sign-in token.
// The raw token is never persisted; only its SHA-256 hash is stored.
type MagicLinkToken struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"-"`
	Email       string     `gorm:"type:varchar(255);not null;index"               json:"-"`
	TokenHash   string     `gorm:"type:varchar(64);not null;uniqueIndex"           json:"-"` // hex SHA-256
	RedirectURL string     `gorm:"type:text"                                      json:"-"`  // post-auth destination
	ExpiresAt   time.Time  `gorm:"not null"                                       json:"-"`
	UsedAt      *time.Time `gorm:"index"                                          json:"-"`
	CreatedAt   time.Time  `gorm:"autoCreateTime"                                 json:"-"`
}

// TableName returns the GORM table name.
func (MagicLinkToken) TableName() string { return "magic_link_tokens" }
