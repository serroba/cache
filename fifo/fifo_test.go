package fifo_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/serroba/cache/fifo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFIFOCache_GetEmpty(t *testing.T) {
	t.Parallel()

	c := fifo.New[string, int](10)

	v, ok := c.Get("missing")
	assert.False(t, ok)
	assert.Equal(t, 0, v)
}

func TestFIFOCache_SetAndGet(t *testing.T) {
	t.Parallel()

	c := fifo.New[string, int](10)
	c.Set("foo", 42)

	v, ok := c.Get("foo")
	require.True(t, ok)
	assert.Equal(t, 42, v)
}

func TestFIFOCache_UpdateExistingKey(t *testing.T) {
	t.Parallel()

	c := fifo.New[string, int](10)
	c.Set("key", 100)
	c.Set("key", 200)

	v, ok := c.Get("key")
	require.True(t, ok)
	assert.Equal(t, 200, v)
}

func TestFIFOCache_EvictionOrder(t *testing.T) {
	t.Parallel()

	c := fifo.New[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// Access "a" - should NOT prevent eviction in FIFO
	c.Get("a")

	// Add new item - should evict "a" (oldest)
	c.Set("d", 4)

	_, ok := c.Get("a")
	assert.False(t, ok, "expected 'a' to be evicted (FIFO ignores access)")

	// b, c, d should exist
	v, ok := c.Get("b")
	require.True(t, ok)
	assert.Equal(t, 2, v)

	v, ok = c.Get("c")
	require.True(t, ok)
	assert.Equal(t, 3, v)

	v, ok = c.Get("d")
	require.True(t, ok)
	assert.Equal(t, 4, v)
}

func TestFIFOCache_Peek(t *testing.T) {
	t.Parallel()

	c := fifo.New[string, int](10)
	c.Set("a", 1)

	v, ok := c.Peek("a")
	require.True(t, ok)
	assert.Equal(t, 1, v)
}

func TestFIFOCache_PeekNonExistent(t *testing.T) {
	t.Parallel()

	c := fifo.New[string, int](10)

	v, ok := c.Peek("missing")
	assert.False(t, ok)
	assert.Equal(t, 0, v)
}

func TestFIFOCache_Delete(t *testing.T) {
	t.Parallel()

	c := fifo.New[string, int](10)
	c.Set("a", 1)
	c.Set("b", 2)

	ok := c.Delete("a")
	assert.True(t, ok)

	_, exists := c.Get("a")
	assert.False(t, exists)

	v, exists := c.Get("b")
	require.True(t, exists)
	assert.Equal(t, 2, v)
}

func TestFIFOCache_DeleteNonExistent(t *testing.T) {
	t.Parallel()

	c := fifo.New[string, int](10)

	ok := c.Delete("missing")
	assert.False(t, ok)
}

func TestFIFOCache_DeleteAndEvict(t *testing.T) {
	t.Parallel()

	c := fifo.New[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// Delete "a" (oldest)
	c.Delete("a")

	// Add two more items
	c.Set("d", 4)
	c.Set("e", 5)

	// "b" should now be evicted (it's the oldest remaining)
	_, ok := c.Get("b")
	assert.False(t, ok, "expected 'b' to be evicted")

	// c, d, e should exist
	_, ok = c.Get("c")
	assert.True(t, ok)

	_, ok = c.Get("d")
	assert.True(t, ok)

	_, ok = c.Get("e")
	assert.True(t, ok)
}

func TestFIFOCache_Len(t *testing.T) {
	t.Parallel()

	c := fifo.New[string, int](10)

	assert.Equal(t, 0, c.Len())

	c.Set("a", 1)
	assert.Equal(t, 1, c.Len())

	c.Set("b", 2)
	c.Set("c", 3)
	assert.Equal(t, 3, c.Len())

	c.Delete("b")
	assert.Equal(t, 2, c.Len())
}

func TestFIFOCache_LenAtCapacity(t *testing.T) {
	t.Parallel()

	c := fifo.New[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	assert.Equal(t, 3, c.Len())

	c.Set("d", 4)
	assert.Equal(t, 3, c.Len())
}

func TestFIFOCache_CapacityOne(t *testing.T) {
	t.Parallel()

	c := fifo.New[string, int](1)
	c.Set("a", 1)

	v, ok := c.Get("a")
	require.True(t, ok)
	assert.Equal(t, 1, v)

	c.Set("b", 2)
	assert.Equal(t, 1, c.Len())

	_, ok = c.Get("a")
	assert.False(t, ok)

	v, ok = c.Get("b")
	require.True(t, ok)
	assert.Equal(t, 2, v)
}

func TestFIFOCache_MultipleTypes(t *testing.T) {
	t.Parallel()

	c := fifo.New[int, string](10)
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

func TestFIFOCache_ConcurrentWrites(t *testing.T) {
	t.Parallel()

	c := fifo.New[int, int](100)

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

func TestFIFOCache_ConcurrentReadsAndWrites(t *testing.T) {
	t.Parallel()

	c := fifo.New[string, int](100)

	for i := range 50 {
		c.Set(fmt.Sprintf("key%d", i), i)
	}

	var wg sync.WaitGroup

	for i := range 10 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := range 100 {
				c.Set(fmt.Sprintf("writer%d-key%d", id, j), j)
			}
		}(i)
	}

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

func TestFIFOCache_ConcurrentDelete(t *testing.T) {
	t.Parallel()

	c := fifo.New[int, int](100)

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

func TestFIFOCache_ConcurrentLen(t *testing.T) {
	t.Parallel()

	c := fifo.New[int, int](100)

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

func TestFIFOCache_DeleteMiddleItem(t *testing.T) {
	t.Parallel()

	c := fifo.New[string, int](5)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	c.Set("d", 4)
	c.Set("e", 5)

	// Delete middle item
	ok := c.Delete("c")
	assert.True(t, ok)
	assert.Equal(t, 4, c.Len())

	// Add new item - should not evict since we have room
	c.Set("f", 6)
	assert.Equal(t, 5, c.Len())

	// "a" should still exist as oldest
	v, ok := c.Get("a")
	require.True(t, ok)
	assert.Equal(t, 1, v)
}

func TestFIFOCache_DeleteHeadAndTail(t *testing.T) {
	t.Parallel()

	c := fifo.New[string, int](3)
	c.Set("a", 1) // oldest (tail)
	c.Set("b", 2)
	c.Set("c", 3) // newest (head)

	// Delete oldest
	c.Delete("a")
	assert.Equal(t, 2, c.Len())

	// Delete newest
	c.Delete("c")
	assert.Equal(t, 1, c.Len())

	// Only "b" should remain
	_, ok := c.Get("a")
	assert.False(t, ok)

	_, ok = c.Get("c")
	assert.False(t, ok)

	v, ok := c.Get("b")
	require.True(t, ok)
	assert.Equal(t, 2, v)
}

func TestFIFOCache_ZeroCapacity(t *testing.T) {
	t.Parallel()

	// Edge case: capacity 0 means every Set triggers evict on empty list
	c := fifo.New[string, int](0)

	// First Set on zero capacity - evict finds empty list and returns early
	c.Set("a", 1)
	// Item is stored (evict can't evict from empty list)
	assert.Equal(t, 1, c.Len())

	// Second Set - now evict finds the item and evicts it
	c.Set("b", 2)
	// Still only 1 item (evicted a, then stored b)
	assert.Equal(t, 1, c.Len())

	// "b" should exist, "a" should be evicted
	_, ok := c.Get("a")
	assert.False(t, ok)

	v, ok := c.Get("b")
	require.True(t, ok)
	assert.Equal(t, 2, v)
}
