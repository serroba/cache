package clock_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/serroba/cache/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClockCache_GetEmpty(t *testing.T) {
	t.Parallel()

	c := clock.New[string, int](10)

	v, ok := c.Get("missing")
	assert.False(t, ok)
	assert.Equal(t, 0, v)
}

func TestClockCache_SetAndGet(t *testing.T) {
	t.Parallel()

	c := clock.New[string, int](10)
	c.Set("foo", 42)

	v, ok := c.Get("foo")
	require.True(t, ok)
	assert.Equal(t, 42, v)
}

func TestClockCache_UpdateExistingKey(t *testing.T) {
	t.Parallel()

	c := clock.New[string, int](10)
	c.Set("key", 100)
	c.Set("key", 200)

	v, ok := c.Get("key")
	require.True(t, ok)
	assert.Equal(t, 200, v)
}

func TestClockCache_Eviction(t *testing.T) {
	t.Parallel()

	c := clock.New[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	c.Set("d", 4) // should evict one item

	assert.Equal(t, uint64(3), c.Len())

	// At least d should exist
	v, ok := c.Get("d")
	require.True(t, ok)
	assert.Equal(t, 4, v)
}

func TestClockCache_SecondChance(t *testing.T) {
	t.Parallel()

	c := clock.New[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// Access "a" to set its reference bit
	c.Get("a")

	// Add new item - "a" should get second chance
	c.Set("d", 4)

	// "a" should still exist (got second chance)
	_, ok := c.Get("a")
	assert.True(t, ok, "expected 'a' to survive due to second chance")
}

func TestClockCache_Peek(t *testing.T) {
	t.Parallel()

	c := clock.New[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// Peek should not set reference bit
	v, ok := c.Peek("a")
	require.True(t, ok)
	assert.Equal(t, 1, v)

	// Add new item - "a" should be evicted (Peek didn't set reference bit)
	c.Set("d", 4)

	_, ok = c.Peek("a")
	assert.False(t, ok, "expected 'a' to be evicted (Peek should not set reference bit)")
}

func TestClockCache_PeekNonExistent(t *testing.T) {
	t.Parallel()

	c := clock.New[string, int](10)

	v, ok := c.Peek("missing")
	assert.False(t, ok)
	assert.Equal(t, 0, v)
}

func TestClockCache_Delete(t *testing.T) {
	t.Parallel()

	c := clock.New[string, int](10)
	c.Set("a", 1)
	c.Set("b", 2)

	ok := c.Delete("a")
	assert.True(t, ok)

	_, exists := c.Get("a")
	assert.False(t, exists)

	// "b" should still exist
	v, exists := c.Get("b")
	require.True(t, exists)
	assert.Equal(t, 2, v)
}

func TestClockCache_DeleteNonExistent(t *testing.T) {
	t.Parallel()

	c := clock.New[string, int](10)

	ok := c.Delete("missing")
	assert.False(t, ok)
}

func TestClockCache_Len(t *testing.T) {
	t.Parallel()

	c := clock.New[string, int](10)

	assert.Equal(t, uint64(0), c.Len())

	c.Set("a", 1)
	assert.Equal(t, uint64(1), c.Len())

	c.Set("b", 2)
	c.Set("c", 3)
	assert.Equal(t, uint64(3), c.Len())

	c.Delete("b")
	assert.Equal(t, uint64(2), c.Len())
}

func TestClockCache_LenAtCapacity(t *testing.T) {
	t.Parallel()

	c := clock.New[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	assert.Equal(t, uint64(3), c.Len())

	c.Set("d", 4)
	assert.Equal(t, uint64(3), c.Len())
}

func TestClockCache_CapacityOne(t *testing.T) {
	t.Parallel()

	c := clock.New[string, int](1)
	c.Set("a", 1)

	v, ok := c.Get("a")
	require.True(t, ok)
	assert.Equal(t, 1, v)

	c.Set("b", 2)
	assert.Equal(t, uint64(1), c.Len())

	_, ok = c.Get("a")
	assert.False(t, ok)

	v, ok = c.Get("b")
	require.True(t, ok)
	assert.Equal(t, 2, v)
}

func TestClockCache_MultipleTypes(t *testing.T) {
	t.Parallel()

	c := clock.New[int, string](10)
	c.Set(1, "one")
	c.Set(2, "two")

	v, ok := c.Get(1)
	require.True(t, ok)
	assert.Equal(t, "one", v)

	v, ok = c.Get(2)
	require.True(t, ok)
	assert.Equal(t, "two", v)
}

// Concurrency tests

func TestClockCache_ConcurrentWrites(t *testing.T) {
	t.Parallel()

	c := clock.New[int, int](100)

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

func TestClockCache_ConcurrentReadsAndWrites(t *testing.T) {
	t.Parallel()

	c := clock.New[string, int](100)

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

func TestClockCache_ConcurrentPeek(t *testing.T) {
	t.Parallel()

	c := clock.New[int, int](100)

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

func TestClockCache_ConcurrentDelete(t *testing.T) {
	t.Parallel()

	c := clock.New[int, int](100)

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

func TestClockCache_ConcurrentLen(t *testing.T) {
	t.Parallel()

	c := clock.New[int, int](100)

	var wg sync.WaitGroup

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

func TestClockCache_DeleteAndReuseSlot(t *testing.T) {
	t.Parallel()

	c := clock.New[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// Delete an item to create an empty slot
	c.Delete("b")

	// Add a new item - should use the empty slot
	c.Set("d", 4)

	assert.Equal(t, uint64(3), c.Len())

	v, ok := c.Get("d")
	require.True(t, ok)
	assert.Equal(t, 4, v)
}

func TestClockCache_EvictAfterDelete(t *testing.T) {
	t.Parallel()

	c := clock.New[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// Delete one item
	c.Delete("a")
	assert.Equal(t, uint64(2), c.Len())

	// Add two more items - second one should trigger eviction
	c.Set("d", 4)
	assert.Equal(t, uint64(3), c.Len())

	c.Set("e", 5)
	assert.Equal(t, uint64(3), c.Len())
}
