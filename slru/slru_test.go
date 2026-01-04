package slru_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/serroba/cache/slru"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSLRUCache_GetEmpty(t *testing.T) {
	t.Parallel()

	c := slru.New[string, int](10)

	v, ok := c.Get("missing")
	assert.False(t, ok)
	assert.Equal(t, 0, v)
}

func TestSLRUCache_NewWithRatio(t *testing.T) {
	t.Parallel()

	// 50/50 split
	c := slru.NewWithRatio[string, int](10, 50)
	c.Set("a", 1)

	v, ok := c.Get("a")
	require.True(t, ok)
	assert.Equal(t, 1, v)
}

func TestSLRUCache_NewWithRatioEdgeCases(t *testing.T) {
	t.Parallel()

	// 100% protected (probation gets minimum of 1)
	c1 := slru.NewWithRatio[string, int](10, 100)
	c1.Set("a", 1)
	v, ok := c1.Get("a")
	require.True(t, ok)
	assert.Equal(t, 1, v)

	// 0% protected (protected gets minimum of 1)
	c2 := slru.NewWithRatio[string, int](10, 0)
	c2.Set("b", 2)
	v, ok = c2.Get("b")
	require.True(t, ok)
	assert.Equal(t, 2, v)

	// Over 100% should be clamped
	c3 := slru.NewWithRatio[string, int](10, 150)
	c3.Set("c", 3)
	v, ok = c3.Get("c")
	require.True(t, ok)
	assert.Equal(t, 3, v)
}

func TestSLRUCache_SetAndGet(t *testing.T) {
	t.Parallel()

	c := slru.New[string, int](10)
	c.Set("foo", 42)

	v, ok := c.Get("foo")
	require.True(t, ok)
	assert.Equal(t, 42, v)
}

func TestSLRUCache_Promotion(t *testing.T) {
	t.Parallel()

	c := slru.New[string, int](10)

	// First Set goes to probation
	c.Set("a", 1)

	// First Get promotes to protected
	v, ok := c.Get("a")
	require.True(t, ok)
	assert.Equal(t, 1, v)

	// Second Get should still work (now in protected)
	v, ok = c.Get("a")
	require.True(t, ok)
	assert.Equal(t, 1, v)
}

func TestSLRUCache_ProtectedSurvivesEviction(t *testing.T) {
	t.Parallel()

	// Capacity 10: protected=8, probation=2
	c := slru.New[string, int](10)

	// Add items to probation
	c.Set("a", 1)
	c.Set("b", 2)

	// Promote "a" to protected
	c.Get("a")

	// Fill probation with new items (should evict "b" but not "a")
	c.Set("c", 3)
	c.Set("d", 4)
	c.Set("e", 5)

	// "a" should still exist (in protected)
	v, ok := c.Get("a")
	require.True(t, ok)
	assert.Equal(t, 1, v)

	// "b" may be evicted from probation
	// (depends on probation capacity, which is 2 for capacity 10)
}

func TestSLRUCache_UpdateExistingKey(t *testing.T) {
	t.Parallel()

	c := slru.New[string, int](10)
	c.Set("key", 100)
	c.Set("key", 200)

	v, ok := c.Get("key")
	require.True(t, ok)
	assert.Equal(t, 200, v)
}

func TestSLRUCache_UpdatePromotedKey(t *testing.T) {
	t.Parallel()

	c := slru.New[string, int](10)
	c.Set("key", 100)
	c.Get("key") // promote to protected
	c.Set("key", 200)

	v, ok := c.Get("key")
	require.True(t, ok)
	assert.Equal(t, 200, v)
}

func TestSLRUCache_Peek(t *testing.T) {
	t.Parallel()

	c := slru.New[string, int](10)
	c.Set("a", 1)

	// Peek should not promote
	v, ok := c.Peek("a")
	require.True(t, ok)
	assert.Equal(t, 1, v)

	// Item should still be in probation (Peek again to verify it exists)
	v, ok = c.Peek("a")
	require.True(t, ok)
	assert.Equal(t, 1, v)
}

func TestSLRUCache_PeekNonExistent(t *testing.T) {
	t.Parallel()

	c := slru.New[string, int](10)

	v, ok := c.Peek("missing")
	assert.False(t, ok)
	assert.Equal(t, 0, v)
}

func TestSLRUCache_PeekProtected(t *testing.T) {
	t.Parallel()

	c := slru.New[string, int](10)
	c.Set("a", 1)
	c.Get("a") // promote to protected

	v, ok := c.Peek("a")
	require.True(t, ok)
	assert.Equal(t, 1, v)
}

func TestSLRUCache_Delete(t *testing.T) {
	t.Parallel()

	c := slru.New[string, int](10)
	c.Set("a", 1)

	ok := c.Delete("a")
	assert.True(t, ok)

	_, exists := c.Get("a")
	assert.False(t, exists)
}

func TestSLRUCache_DeletePromoted(t *testing.T) {
	t.Parallel()

	c := slru.New[string, int](10)
	c.Set("a", 1)
	c.Get("a") // promote to protected

	ok := c.Delete("a")
	assert.True(t, ok)

	_, exists := c.Get("a")
	assert.False(t, exists)
}

func TestSLRUCache_DeleteNonExistent(t *testing.T) {
	t.Parallel()

	c := slru.New[string, int](10)

	ok := c.Delete("missing")
	assert.False(t, ok)
}

func TestSLRUCache_Len(t *testing.T) {
	t.Parallel()

	c := slru.New[string, int](10)

	assert.Equal(t, 0, c.Len())

	c.Set("a", 1)
	assert.Equal(t, 1, c.Len())

	c.Set("b", 2)
	assert.Equal(t, 2, c.Len())

	c.Get("a") // promote doesn't change total len
	assert.Equal(t, 2, c.Len())

	c.Delete("a")
	assert.Equal(t, 1, c.Len())
}

func TestSLRUCache_SmallCapacity(t *testing.T) {
	t.Parallel()

	// Edge case: capacity 1 should still work
	c := slru.New[string, int](1)
	c.Set("a", 1)

	v, ok := c.Get("a")
	require.True(t, ok)
	assert.Equal(t, 1, v)
}

func TestSLRUCache_MultipleTypes(t *testing.T) {
	t.Parallel()

	c := slru.New[int, string](10)
	c.Set(1, "one")
	c.Set(2, "two")

	v, ok := c.Get(1)
	require.True(t, ok)
	assert.Equal(t, "one", v)
}

// Concurrency tests

func TestSLRUCache_ConcurrentWrites(t *testing.T) {
	t.Parallel()

	c := slru.New[int, int](100)

	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := range 100 {
				c.Set(id*100+j, j)
			}
		}(i)
	}

	wg.Wait()
}

func TestSLRUCache_ConcurrentReadsAndWrites(t *testing.T) {
	t.Parallel()

	c := slru.New[string, int](100)

	// Pre-populate
	for i := range 50 {
		c.Set(fmt.Sprintf("key%d", i), i)
	}

	var wg sync.WaitGroup

	// Writers
	for i := range 10 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := range 100 {
				c.Set(fmt.Sprintf("writer%d-key%d", id, j), j)
			}
		}(i)
	}

	// Readers
	for range 10 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for j := range 100 {
				c.Get(fmt.Sprintf("key%d", j%50))
			}
		}()
	}

	wg.Wait()
}

func TestSLRUCache_ConcurrentPeek(t *testing.T) {
	t.Parallel()

	c := slru.New[int, int](100)

	// Pre-populate
	for i := range 100 {
		c.Set(i, i)
	}

	var wg sync.WaitGroup

	for range 20 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for j := range 100 {
				c.Peek(j)
			}
		}()
	}

	wg.Wait()
}

func TestSLRUCache_ConcurrentDelete(t *testing.T) {
	t.Parallel()

	c := slru.New[int, int](100)

	// Pre-populate
	for i := range 100 {
		c.Set(i, i)
	}

	var wg sync.WaitGroup

	for range 10 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for j := range 100 {
				c.Delete(j)
			}
		}()
	}

	wg.Wait()
}

func TestSLRUCache_ConcurrentLen(t *testing.T) {
	t.Parallel()

	c := slru.New[int, int](100)

	var wg sync.WaitGroup

	// Writers
	for i := range 10 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := range 50 {
				c.Set(id*50+j, j)
				c.Len()
			}
		}(i)
	}

	// Len readers
	for range 10 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range 100 {
				c.Len()
			}
		}()
	}

	wg.Wait()
}

func TestSLRUCache_DemoteLRU(t *testing.T) {
	t.Parallel()

	// Create cache with small capacity to trigger demotion
	// Capacity 5 with default 80% protected = 4 protected, 1 probation
	c := slru.New[string, int](5)

	// Add and immediately promote items to protected
	// (to avoid probation eviction before promotion)
	c.Set("a", 1)
	c.Get("a") // promote: protected [a]

	c.Set("b", 2)
	c.Get("b") // promote: protected [b, a]

	c.Set("c", 3)
	c.Get("c") // promote: protected [c, b, a]

	c.Set("d", 4)
	c.Get("d") // promote: protected [d, c, b, a] - now protected is full (cap=4)

	// Add a new item to probation
	c.Set("e", 5)

	// Now promote "e" - this should trigger demoteLRU
	// "a" is LRU in protected and should be demoted to probation
	c.Get("e") // promotes e, demotes a to probation

	// Verify "e" is accessible (now in protected)
	v, ok := c.Get("e")
	require.True(t, ok)
	assert.Equal(t, 5, v)

	// "a" should still be in cache (demoted to probation)
	// Use Peek to avoid re-promoting
	v, ok = c.Peek("a")
	require.True(t, ok, "expected 'a' to be demoted to probation, not evicted")
	assert.Equal(t, 1, v)
}

func TestSLRUCache_DemoteTriggersProbationEviction(t *testing.T) {
	t.Parallel()

	// Create cache where demotion will cause probation overflow
	// Capacity 3 with 66% protected = 2 protected, 1 probation
	c := slru.NewWithRatio[string, int](3, 66)

	// Add and immediately promote items to protected
	c.Set("a", 1)
	c.Get("a") // protected: [a]

	c.Set("b", 2)
	c.Get("b") // protected: [b, a] (full, cap=2)

	// Add new item to probation
	c.Set("c", 3) // probation: [c] (cap=1)

	// Promote c - this should:
	// 1. Move c from probation to protected
	// 2. Demote a (LRU in protected) to probation
	// 3. probation now has 'a' (len=1, cap=1)
	c.Get("c")

	// Add another item to probation (this will evict 'a' from probation)
	c.Set("d", 4) // probation: [d], 'a' evicted

	// Promote d - demotes 'b' to probation
	c.Get("d")

	// d and c should exist (in protected)
	v, ok := c.Get("d")
	require.True(t, ok)
	assert.Equal(t, 4, v)

	v, ok = c.Get("c")
	require.True(t, ok)
	assert.Equal(t, 3, v)

	// Total items should not exceed capacity
	assert.LessOrEqual(t, c.Len(), 3)
}
