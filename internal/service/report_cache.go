package service

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"kerplan/internal/event"
	"kerplan/internal/model"
)

// ReportCache is a simple in-memory LRU cache for computed full plan reports.
// It uses the ReportInvalidator to know when a cached entry is stale.
type ReportCache struct {
	mu          sync.RWMutex
	entries     map[uuid.UUID]*cacheEntry // keyed by scenarioID
	order       []uuid.UUID              // LRU order (oldest first)
	maxSize     int
	invalidator *event.ReportInvalidator
}

type cacheEntry struct {
	report    *model.FullPlanOutput
	createdAt time.Time
}

// NewReportCache creates a ReportCache with the given max size and invalidator.
func NewReportCache(maxSize int, invalidator *event.ReportInvalidator) *ReportCache {
	return &ReportCache{
		entries:     make(map[uuid.UUID]*cacheEntry),
		order:       make([]uuid.UUID, 0, maxSize),
		maxSize:     maxSize,
		invalidator: invalidator,
	}
}

// Get returns a cached report if it exists and is not stale.
func (c *ReportCache) Get(scenarioID uuid.UUID) (*model.FullPlanOutput, bool) {
	if c.invalidator.IsStale(scenarioID) {
		return nil, false
	}

	c.mu.RLock()
	entry, ok := c.entries[scenarioID]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	// Move to end of LRU order (most recently used)
	c.mu.Lock()
	c.moveToEnd(scenarioID)
	c.mu.Unlock()

	return entry.report, true
}

// Put stores a report in the cache and marks it fresh.
func (c *ReportCache) Put(scenarioID uuid.UUID, report *model.FullPlanOutput) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If already present, update in place
	if _, ok := c.entries[scenarioID]; ok {
		c.entries[scenarioID] = &cacheEntry{report: report, createdAt: time.Now()}
		c.moveToEnd(scenarioID)
		c.invalidator.MarkFresh(scenarioID)
		return
	}

	// Evict oldest if at capacity
	if len(c.entries) >= c.maxSize && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldest)
	}

	c.entries[scenarioID] = &cacheEntry{report: report, createdAt: time.Now()}
	c.order = append(c.order, scenarioID)
	c.invalidator.MarkFresh(scenarioID)
}

// moveToEnd moves the given scenarioID to the end of the LRU order.
// Must be called with c.mu held.
func (c *ReportCache) moveToEnd(scenarioID uuid.UUID) {
	for i, id := range c.order {
		if id == scenarioID {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, scenarioID)
			return
		}
	}
}

// Size returns the current number of entries in the cache. Useful for testing.
func (c *ReportCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
