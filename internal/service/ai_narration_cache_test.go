package service

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Helpers
// ============================================================================

// stubOutput returns a minimal non-nil NarrationOutput for cache tests.
func stubOutput(title string) *NarrationOutput {
	return &NarrationOutput{
		Title:         title,
		Summary:       "stub summary",
		Paragraphs:    []NarrationParagraph{{Content: "stub paragraph", Type: "info"}},
		KeyTakeaways:  []string{"takeaway"},
		IsAIGenerated: true,
	}
}

// tinyCache creates a NarrationCache with a very small size for eviction tests.
func tinyCache(maxSize int) *NarrationCache {
	return NewNarrationCache(NarrationCacheConfig{
		MaxSize: maxSize,
		TTL:     10 * time.Minute,
	})
}

// ============================================================================
// NarrationCache — basic operations
// ============================================================================

func TestNarrationCache_GetMiss_EmptyCache(t *testing.T) {
	c := tinyCache(10)
	out, ok := c.Get("missing-key")
	assert.False(t, ok)
	assert.Nil(t, out)
}

func TestNarrationCache_PutGet_Hit(t *testing.T) {
	c := tinyCache(10)
	want := stubOutput("Revenue Summary")

	c.Put("k1", want)

	got, ok := c.Get("k1")
	require.True(t, ok)
	assert.Equal(t, want.Title, got.Title)
}

func TestNarrationCache_Get_ExpiredEntry_ReturnsMiss(t *testing.T) {
	c := NewNarrationCache(NarrationCacheConfig{
		MaxSize: 10,
		TTL:     1 * time.Millisecond, // expires almost immediately
	})
	c.Put("k1", stubOutput("will expire"))
	time.Sleep(5 * time.Millisecond)

	_, ok := c.Get("k1")
	assert.False(t, ok, "expired entry should return false")
	assert.Equal(t, 0, c.Size(), "expired entry should be evicted on Get")
}

func TestNarrationCache_UpdateInPlace(t *testing.T) {
	c := tinyCache(10)
	first := stubOutput("first")
	second := stubOutput("second")

	c.Put("k1", first)
	c.Put("k1", second) // update

	got, ok := c.Get("k1")
	require.True(t, ok)
	assert.Equal(t, "second", got.Title)
	assert.Equal(t, 1, c.Size(), "update must not grow size")
}

func TestNarrationCache_LRUEviction_RemovesOldest(t *testing.T) {
	c := tinyCache(3)

	c.Put("k1", stubOutput("one"))
	c.Put("k2", stubOutput("two"))
	c.Put("k3", stubOutput("three"))

	// Access k1 to move it to MRU; k2 becomes oldest.
	c.Get("k1")

	// Adding k4 should evict k2 (oldest unused).
	c.Put("k4", stubOutput("four"))
	assert.Equal(t, 3, c.Size())

	_, k1ok := c.Get("k1")
	_, k2ok := c.Get("k2")
	_, k4ok := c.Get("k4")
	assert.True(t, k1ok, "k1 (recently accessed) should survive eviction")
	assert.False(t, k2ok, "k2 (oldest unused) should be evicted")
	assert.True(t, k4ok, "k4 (just added) should survive eviction")
}

func TestNarrationCache_LRUEviction_OldestByInsertOrder(t *testing.T) {
	c := tinyCache(2)
	c.Put("first", stubOutput("first"))
	c.Put("second", stubOutput("second"))
	c.Put("third", stubOutput("third")) // should evict "first"

	_, firstOk := c.Get("first")
	_, secondOk := c.Get("second")
	assert.False(t, firstOk, "first entry should be evicted")
	assert.True(t, secondOk)
}

func TestNarrationCache_Flush_EmptiesAll(t *testing.T) {
	c := tinyCache(10)
	c.Put("k1", stubOutput("a"))
	c.Put("k2", stubOutput("b"))
	c.Put("k3", stubOutput("c"))
	require.Equal(t, 3, c.Size())

	c.Flush()

	assert.Equal(t, 0, c.Size())
	_, ok := c.Get("k1")
	assert.False(t, ok)
}

func TestNarrationCache_Size_TracksCorrectly(t *testing.T) {
	c := tinyCache(10)
	assert.Equal(t, 0, c.Size())

	c.Put("k1", stubOutput("a"))
	assert.Equal(t, 1, c.Size())

	c.Put("k2", stubOutput("b"))
	assert.Equal(t, 2, c.Size())

	c.Put("k1", stubOutput("a-updated")) // update, not grow
	assert.Equal(t, 2, c.Size())
}

func TestNarrationCache_ConcurrentAccess_NoDataRace(t *testing.T) {
	c := tinyCache(50)
	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("k%d", id)
			c.Put(key, stubOutput(key))
			c.Get(key)
			c.Get(fmt.Sprintf("k%d", (id+1)%goroutines))
		}(i)
	}
	wg.Wait()
	assert.GreaterOrEqual(t, c.Size(), 0) // no panic = success
}

// ============================================================================
// NarrationCacheKey — determinism and sensitivity
// ============================================================================

func TestNarrationCacheKey_Deterministic_SameInput(t *testing.T) {
	ctx := minimalCtx()
	ctx.NarrationType = NarrationTypePlanSummary

	key1 := NarrationCacheKey(ctx)
	key2 := NarrationCacheKey(ctx)
	assert.Equal(t, key1, key2, "same context must always produce the same key")
}

func TestNarrationCacheKey_Format_ThreeSegments(t *testing.T) {
	ctx := minimalCtx()
	ctx.NarrationType = NarrationTypePlanSummary
	key := NarrationCacheKey(ctx)

	parts := splitKey(key)
	require.Len(t, parts, 3, "key must be featureType:role:hash")
	assert.Equal(t, "plan_summary", parts[0])
	assert.Equal(t, "owner", parts[1])
	assert.Len(t, parts[2], 16, "hash prefix must be 16 hex chars")
}

func TestNarrationCacheKey_DifferentNarrationType_DifferentKey(t *testing.T) {
	ctx1 := minimalCtx()
	ctx1.NarrationType = NarrationTypePlanSummary
	ctx2 := minimalCtx()
	ctx2.NarrationType = NarrationTypeCashRunway

	assert.NotEqual(t, NarrationCacheKey(ctx1), NarrationCacheKey(ctx2))
}

func TestNarrationCacheKey_DifferentRole_DifferentKey(t *testing.T) {
	ctx1 := minimalCtx()
	ctx1.UserRole = NarrationRoleOwner
	ctx1.NarrationType = NarrationTypePlanSummary
	ctx2 := minimalCtx()
	ctx2.UserRole = NarrationRoleUser
	ctx2.NarrationType = NarrationTypePlanSummary

	assert.NotEqual(t, NarrationCacheKey(ctx1), NarrationCacheKey(ctx2))
}

func TestNarrationCacheKey_RevenueChange_DifferentKey(t *testing.T) {
	v1 := 1_000_000.0
	v2 := 1_000_001.0
	m1 := &FinancialMetric{Label: "Revenue", Value: v1, Currency: "EUR"}
	m2 := &FinancialMetric{Label: "Revenue", Value: v2, Currency: "EUR"}

	ctx1 := minimalCtx()
	ctx1.NarrationType = NarrationTypePlanSummary
	ctx1.Revenue = m1

	ctx2 := minimalCtx()
	ctx2.NarrationType = NarrationTypePlanSummary
	ctx2.Revenue = m2

	assert.NotEqual(t, NarrationCacheKey(ctx1), NarrationCacheKey(ctx2),
		"one-unit revenue change must produce a different cache key")
}

func TestNarrationCacheKey_ProductEconomicsChange_DifferentKey(t *testing.T) {
	ctx1 := minimalCtx()
	ctx1.NarrationType = NarrationTypeUnitEconomics
	ctx1.ProductEconomics = []ProductEconomics{
		{ProductName: "Starter", DriverType: "saas",
			Metrics: []FinancialMetric{{Label: "ARPU", Value: 49.0}}},
	}

	ctx2 := minimalCtx()
	ctx2.NarrationType = NarrationTypeUnitEconomics
	ctx2.ProductEconomics = []ProductEconomics{
		{ProductName: "Starter", DriverType: "saas",
			Metrics: []FinancialMetric{{Label: "ARPU", Value: 55.0}}}, // ARPU changed
	}

	assert.NotEqual(t, NarrationCacheKey(ctx1), NarrationCacheKey(ctx2),
		"changing a product ARPU must produce a different cache key")
}

func TestNarrationCacheKey_AssumptionFlagChange_DifferentKey(t *testing.T) {
	ctx1 := minimalCtx()
	ctx1.NarrationType = NarrationTypeAssumptionReview
	ctx1.AssumptionFlags = []AssumptionFlag{
		{ProductName: "P1", FieldName: "churnRate", Value: 0.05, Reason: "above benchmark"},
	}

	ctx2 := minimalCtx()
	ctx2.NarrationType = NarrationTypeAssumptionReview
	ctx2.AssumptionFlags = []AssumptionFlag{
		{ProductName: "P1", FieldName: "churnRate", Value: 0.12, Reason: "above benchmark"},
	}

	assert.NotEqual(t, NarrationCacheKey(ctx1), NarrationCacheKey(ctx2))
}

// ============================================================================
// Helpers
// ============================================================================

func splitKey(key string) []string {
	var parts []string
	var cur []byte
	for i := 0; i < len(key); i++ {
		if key[i] == ':' {
			parts = append(parts, string(cur))
			cur = cur[:0]
		} else {
			cur = append(cur, key[i])
		}
	}
	parts = append(parts, string(cur))
	return parts
}
