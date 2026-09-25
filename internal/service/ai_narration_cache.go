// Package service — AI narration result cache.
//
// Infrastructure is delegated to github.com/ovander/backendkit/ainarration:
//   - CacheConfig / DefaultCacheConfig — tunable LRU+TTL parameters
//   - NarrationCache (in-memory) — LRU eviction + TTL via ainarration.NarrationCache;
//     Ascenda's domain-specific NarrationOutput is JSON-serialised into
//     ainarration.NarrationOutput.Narrative so the backendkit mechanics are
//     reused without coupling to backendkit's generic output type.
//   - CacheKey — SHA-256 content-addressed key computation
//
// DB-backed and layered cache implementations remain Ascenda-specific because
// they depend on Ascenda's repo layer and the richer NarrationOutput schema.
package service

import (
	"encoding/json"

	"ascenda/internal/repo"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/ainarration"
)

// ============================================================================
// NarrationCacher — interface
// ============================================================================

// NarrationCacher is the interface satisfied by all cache implementations.
// tenantID is required so DB-backed implementations can scope storage per
// tenant. In-memory implementations may ignore it (the content hash in the
// key already provides effective tenant isolation).
type NarrationCacher interface {
	Get(tenantID uuid.UUID, key string) (*NarrationOutput, bool)
	Put(tenantID uuid.UUID, key string, output *NarrationOutput)
	Flush()
	Size() int
}

// ============================================================================
// Cache configuration — delegated to backendkit/ainarration
// ============================================================================

// NarrationCacheConfig holds tunable parameters for the narration cache.
// It is a type alias for ainarration.CacheConfig so the two packages stay in sync.
type NarrationCacheConfig = ainarration.CacheConfig

// DefaultNarrationCacheConfig returns the recommended production defaults
// (200 entries, 2-hour TTL) from backendkit.
func DefaultNarrationCacheConfig() NarrationCacheConfig {
	return ainarration.DefaultCacheConfig()
}

// ============================================================================
// NarrationCache — in-memory LRU+TTL (backed by ainarration.NarrationCache)
// ============================================================================

// NarrationCache is an in-memory LRU+TTL cache of AI narration results.
// It wraps ainarration.NarrationCache, serialising Ascenda's NarrationOutput
// into the generic Narrative field so no LRU/TTL logic is duplicated here.
// It is safe for concurrent use.
type NarrationCache struct {
	inner *ainarration.NarrationCache
}

// NewNarrationCache creates a NarrationCache with the given configuration.
func NewNarrationCache(cfg NarrationCacheConfig) *NarrationCache {
	return &NarrationCache{inner: ainarration.NewNarrationCache(cfg)}
}

// Get retrieves a cached narration result. Returns (nil, false) on a miss or
// if the entry has expired.
func (c *NarrationCache) Get(tenantID uuid.UUID, key string) (*NarrationOutput, bool) {
	raw, ok := c.inner.Get(tenantID, key)
	if !ok || raw == nil {
		return nil, false
	}
	var out NarrationOutput
	if err := json.Unmarshal([]byte(raw.Narrative), &out); err != nil {
		return nil, false
	}
	return &out, true
}

// Put stores output under key. Evicts the LRU entry when at capacity.
func (c *NarrationCache) Put(tenantID uuid.UUID, key string, output *NarrationOutput) {
	b, err := json.Marshal(output)
	if err != nil {
		return
	}
	c.inner.Put(tenantID, key, &ainarration.NarrationOutput{Narrative: string(b)})
}

// Size returns the current number of cached entries.
func (c *NarrationCache) Size() int { return c.inner.Size() }

// Flush empties the cache. Useful in tests.
func (c *NarrationCache) Flush() { c.inner.Flush() }

// ============================================================================
// Cache key computation — delegated to backendkit/ainarration
// ============================================================================

// NarrationCacheKey computes a deterministic cache key from a NarrationContext.
//
// Key format: "<narratType>:<role>:<sha256-16-chars>"
//
// Delegates to ainarration.CacheKey for consistent key derivation across
// all Kerplan services that use backendkit.
func NarrationCacheKey(nCtx *NarrationContext) string {
	return ainarration.CacheKey(string(nCtx.NarrationType), string(nCtx.UserRole), nCtx)
}

// ============================================================================
// DBNarrationCache — DB-backed implementation of NarrationCacher
// ============================================================================

// DBNarrationCache stores narration results in PostgreSQL.
// It satisfies NarrationCacher and is safe for concurrent use.
type DBNarrationCache struct {
	repo repo.AINarrationCacheRepository
}

// NewDBNarrationCache creates a DB-backed cache.
func NewDBNarrationCache(r repo.AINarrationCacheRepository) *DBNarrationCache {
	return &DBNarrationCache{repo: r}
}

func (c *DBNarrationCache) Get(tenantID uuid.UUID, key string) (*NarrationOutput, bool) {
	raw, err := c.repo.Get(tenantID, key)
	if err != nil || raw == nil {
		return nil, false
	}
	var out NarrationOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, false
	}
	return &out, true
}

func (c *DBNarrationCache) Put(tenantID uuid.UUID, key string, output *NarrationOutput) {
	raw, err := json.Marshal(output)
	if err != nil {
		return
	}
	_ = c.repo.Put(tenantID, key, raw)
}

// Flush is a no-op for the DB cache — use DeleteByTenant on the repo directly
// for targeted eviction.
func (c *DBNarrationCache) Flush() {}

// Size is not tracked efficiently in the DB implementation; returns -1.
func (c *DBNarrationCache) Size() int { return -1 }

// ============================================================================
// LayeredNarrationCache — L1 (in-memory) + L2 (DB)
// ============================================================================

// LayeredNarrationCache combines a fast in-memory L1 cache with a persistent
// DB L2 cache.
//
// Read path:  L1 hit → return immediately.
//
//	L1 miss + L2 hit → promote to L1, return.
//	Both miss → return (nil, false) — caller will call AI.
//
// Write path: Always write to both L1 and L2.
type LayeredNarrationCache struct {
	l1 *NarrationCache
	l2 *DBNarrationCache
}

// NewLayeredNarrationCache creates a two-layer cache.
func NewLayeredNarrationCache(l1Cfg NarrationCacheConfig, dbRepo repo.AINarrationCacheRepository) *LayeredNarrationCache {
	return &LayeredNarrationCache{
		l1: NewNarrationCache(l1Cfg),
		l2: NewDBNarrationCache(dbRepo),
	}
}

func (c *LayeredNarrationCache) Get(tenantID uuid.UUID, key string) (*NarrationOutput, bool) {
	if out, ok := c.l1.Get(tenantID, key); ok {
		return out, true
	}
	if out, ok := c.l2.Get(tenantID, key); ok {
		c.l1.Put(tenantID, key, out)
		return out, true
	}
	return nil, false
}

func (c *LayeredNarrationCache) Put(tenantID uuid.UUID, key string, output *NarrationOutput) {
	c.l1.Put(tenantID, key, output)
	c.l2.Put(tenantID, key, output)
}

func (c *LayeredNarrationCache) Flush() {
	c.l1.Flush()
	c.l2.Flush()
}

func (c *LayeredNarrationCache) Size() int { return c.l1.Size() }
