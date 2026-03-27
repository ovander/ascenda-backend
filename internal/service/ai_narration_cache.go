// Package service — AI narration result cache.
//
// # Design rationale
//
// AI API calls are expensive (latency + cost). Two consecutive narration
// requests for the *same financial data* should return the same result without
// hitting the AI provider again.
//
// The cache is content-addressed: the key is derived from a SHA-256 hash of
// the full NarrationContext. If any input field changes (a single assumption,
// a product KPI, the narration type) the hash changes and the old entry is
// never served. No explicit invalidation events are needed.
//
// The key also includes the narration feature type and the user role because
// both influence the prompt and therefore the narration text.
//
// Entries expire after a configurable TTL (default 2 h). This prevents
// unbounded growth and ensures that even stale-but-hash-identical contexts
// eventually rotate out.
//
// The cache is purely in-process (no Redis dependency). For multi-instance
// deployments a shared cache layer can be added later without changing the
// service interface.
package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// Cache configuration
// ============================================================================

const (
	// DefaultNarrationCacheSize is the maximum number of cached narration results.
	// Each entry holds a NarrationOutput (~1–4 KB of JSON), so 200 entries ≈ < 1 MB.
	DefaultNarrationCacheSize = 200

	// DefaultNarrationCacheTTL is the time-to-live for a cached narration result.
	// After this duration the entry is treated as a miss and the AI is called again.
	DefaultNarrationCacheTTL = 2 * time.Hour
)

// NarrationCacheConfig holds tunable parameters for the narration cache.
type NarrationCacheConfig struct {
	MaxSize int
	TTL     time.Duration
}

// DefaultNarrationCacheConfig returns the recommended production defaults.
func DefaultNarrationCacheConfig() NarrationCacheConfig {
	return NarrationCacheConfig{
		MaxSize: DefaultNarrationCacheSize,
		TTL:     DefaultNarrationCacheTTL,
	}
}

// ============================================================================
// NarrationCache — LRU + TTL in-memory cache
// ============================================================================

// NarrationCache stores AI narration results keyed by a content hash.
// It is safe for concurrent use.
type NarrationCache struct {
	mu      sync.RWMutex
	entries map[string]*narrationCacheEntry // key → entry
	order   []string                        // LRU order (oldest first)
	cfg     NarrationCacheConfig
}

type narrationCacheEntry struct {
	output    *NarrationOutput
	createdAt time.Time
}

// NewNarrationCache creates a NarrationCache with the given configuration.
func NewNarrationCache(cfg NarrationCacheConfig) *NarrationCache {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = DefaultNarrationCacheSize
	}
	if cfg.TTL <= 0 {
		cfg.TTL = DefaultNarrationCacheTTL
	}
	return &NarrationCache{
		entries: make(map[string]*narrationCacheEntry, cfg.MaxSize),
		order:   make([]string, 0, cfg.MaxSize),
		cfg:     cfg,
	}
}

// Get retrieves a cached narration result for the given context.
// Returns (nil, false) on a miss or if the entry has expired.
func (c *NarrationCache) Get(key string) (*NarrationOutput, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	// Expired?
	if time.Since(entry.createdAt) > c.cfg.TTL {
		c.mu.Lock()
		delete(c.entries, key)
		c.removeFromOrder(key)
		c.mu.Unlock()
		return nil, false
	}

	// Promote to most-recently-used position.
	c.mu.Lock()
	c.moveToEnd(key)
	c.mu.Unlock()

	return entry.output, true
}

// Put stores a narration result under the given key.
func (c *NarrationCache) Put(key string, output *NarrationOutput) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update in place if the key is already present.
	if _, ok := c.entries[key]; ok {
		c.entries[key] = &narrationCacheEntry{output: output, createdAt: time.Now()}
		c.moveToEnd(key)
		return
	}

	// Evict the oldest entry when at capacity.
	for len(c.entries) >= c.cfg.MaxSize && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldest)
	}

	c.entries[key] = &narrationCacheEntry{output: output, createdAt: time.Now()}
	c.order = append(c.order, key)
}

// Size returns the current number of cached entries (useful for tests / metrics).
func (c *NarrationCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Flush removes all entries from the cache. Useful for testing.
func (c *NarrationCache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*narrationCacheEntry, c.cfg.MaxSize)
	c.order = c.order[:0]
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// moveToEnd promotes key to the most-recently-used position.
// Must be called with c.mu held (write lock).
func (c *NarrationCache) moveToEnd(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, key)
			return
		}
	}
}

// removeFromOrder removes key from the LRU order slice.
// Must be called with c.mu held (write lock).
func (c *NarrationCache) removeFromOrder(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}

// ============================================================================
// Cache key computation
// ============================================================================

// NarrationCacheKey computes a deterministic cache key from a NarrationContext.
//
// Key structure: "<featureType>:<role>:<sha256prefix>"
//
// The SHA-256 is computed over the canonical JSON serialisation of the context.
// Because Go's encoding/json marshals struct fields in declaration order, the
// output is deterministic for the same input values. The first 16 hex chars
// (64 bits of entropy) are used; the probability of a collision within a 200-
// entry cache is negligible.
//
// The narration type is included in the hash (it is a field of NarrationContext),
// but also surfaced in the prefix so cache keys are human-readable in logs.
func NarrationCacheKey(nCtx *NarrationContext) string {
	b, err := json.Marshal(nCtx)
	if err != nil {
		// Fallback: non-cacheable key that will never match a stored entry.
		// This path is unreachable in practice (NarrationContext is always serialisable).
		return fmt.Sprintf("err:uncacheable:%d", time.Now().UnixNano())
	}

	sum := sha256.Sum256(b)
	hashPrefix := hex.EncodeToString(sum[:])[:16]

	role := string(nCtx.UserRole)
	if role == "" {
		role = "unknown"
	}
	featureType := string(nCtx.NarrationType)
	if featureType == "" {
		featureType = "inferred"
	}

	return fmt.Sprintf("%s:%s:%s", featureType, role, hashPrefix)
}
