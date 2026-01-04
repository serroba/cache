package lru_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/serroba/cache/lru"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLRUCache_Get(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](5)

	got, ok := c.Get("some")
	assert.False(t, ok)
	assert.Equal(t, 0, got)
}

func TestLRUCache_Set(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](5)
	c.Set("foo", 42)

	got, ok := c.Get("foo")
	require.True(t, ok)
	assert.Equal(t, 42, got)
}

func TestLRUCache_UpdateExistingKey(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](5)
	c.Set("key", 100)
	c.Set("key", 200)

	got, ok := c.Get("key")
	require.True(t, ok)
	assert.Equal(t, 200, got)
}

func TestLRUCache_Eviction(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	c.Set("d", 4) // should evict "a"

	_, ok := c.Get("a")
	assert.False(t, ok, "expected 'a' to be evicted")

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

func TestLRUCache_EvictionOrder(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// Access "a" to make it recently used
	c.Set("a", 1)

	// Add new item, should evict "b" (least recently used)
	c.Set("d", 4)

	_, ok := c.Get("b")
	assert.False(t, ok, "expected 'b' to be evicted")

	_, ok = c.Get("a")
	assert.True(t, ok, "expected 'a' to still exist after being accessed")
}

func TestLRUCache_GetUpdatesRecency(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// Access "a" via Get - should make it most recently used
	c.Get("a")

	// Add new item - should evict "b" (now the least recently used)
	c.Set("d", 4)

	_, ok := c.Get("a")
	assert.True(t, ok, "expected 'a' to still exist after being accessed via Get")

	_, ok = c.Get("b")
	assert.False(t, ok, "expected 'b' to be evicted (was least recently used)")
}

func TestLRUCache_CapacityOne(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](1)
	c.Set("a", 1)
	c.Set("b", 2)

	_, ok := c.Get("a")
	assert.False(t, ok, "expected 'a' to be evicted")

	v, ok := c.Get("b")
	require.True(t, ok)
	assert.Equal(t, 2, v)
}

func TestLRUCache_MultipleTypes(t *testing.T) {
	t.Parallel()

	c := lru.New[int, string](3)
	c.Set(1, "one")
	c.Set(2, "two")
	c.Set(3, "three")

	v, ok := c.Get(1)
	require.True(t, ok)
	assert.Equal(t, "one", v)

	v, ok = c.Get(2)
	require.True(t, ok)
	assert.Equal(t, "two", v)

	v, ok = c.Get(3)
	require.True(t, ok)
	assert.Equal(t, "three", v)
}

func TestLRUCache_ConcurrentWrites(t *testing.T) {
	t.Parallel()

	c := lru.New[int, int](100)

	var wg sync.WaitGroup

	numGoroutines := 100
	numOps := 100

	for i := range numGoroutines {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := range numOps {
				c.Set(id*numOps+j, j)
			}
		}(i)
	}

	wg.Wait()
}

func TestLRUCache_ConcurrentReadsAndWrites(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](100)

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
	for i := range 10 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := range 100 {
				c.Get(fmt.Sprintf("writer%d-key%d", id, j))
			}
		}(i)
	}

	wg.Wait()
}

func TestLRUCache_ConcurrentEviction(t *testing.T) {
	t.Parallel()

	c := lru.New[int, int](10) // Small capacity to force evictions

	var wg sync.WaitGroup

	numGoroutines := 50
	numOps := 100

	for i := range numGoroutines {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := range numOps {
				key := id*numOps + j
				c.Set(key, key)
				c.Get(key)
			}
		}(i)
	}

	wg.Wait()
}

func TestLRUCache_ConcurrentSameKey(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](10)

	var wg sync.WaitGroup

	numGoroutines := 100

	for i := range numGoroutines {
		wg.Add(1)

		go func(val int) {
			defer wg.Done()

			c.Set("shared", val)
			c.Get("shared")
		}(i)
	}

	wg.Wait()

	_, ok := c.Get("shared")
	assert.True(t, ok, "expected 'shared' key to exist")
}

func TestLRUCache_Peek(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// Peek "a" - should NOT update recency
	v, ok := c.Peek("a")
	require.True(t, ok)
	assert.Equal(t, 1, v)

	// Add new item - should evict "a" since Peek didn't update recency
	c.Set("d", 4)

	_, ok = c.Peek("a")
	assert.False(t, ok, "expected 'a' to be evicted (Peek should not update recency)")
}

func TestLRUCache_PeekNonExistent(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](3)

	v, ok := c.Peek("missing")
	assert.False(t, ok)
	assert.Equal(t, 0, v)
}

func TestLRUCache_Delete(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)

	// Delete existing key
	ok := c.Delete("a")
	assert.True(t, ok, "Delete() should return true for existing key")

	_, exists := c.Get("a")
	assert.False(t, exists, "expected 'a' to be deleted")

	// Verify "b" still exists
	v, exists := c.Get("b")
	require.True(t, exists)
	assert.Equal(t, 2, v)
}

func TestLRUCache_DeleteNonExistent(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](3)
	c.Set("a", 1)

	ok := c.Delete("missing")
	assert.False(t, ok, "Delete() should return false for non-existent key")

	// Verify cache is unchanged
	v, exists := c.Get("a")
	require.True(t, exists)
	assert.Equal(t, 1, v)
}

func TestLRUCache_DeleteUpdatesLen(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](5)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	assert.Equal(t, 3, c.Len())

	c.Delete("b")

	assert.Equal(t, 2, c.Len())
}

func TestLRUCache_Len(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](5)

	assert.Equal(t, 0, c.Len())

	c.Set("a", 1)
	assert.Equal(t, 1, c.Len())

	c.Set("b", 2)
	c.Set("c", 3)
	assert.Equal(t, 3, c.Len())

	// Update existing key - length should stay same
	c.Set("a", 100)
	assert.Equal(t, 3, c.Len())
}

func TestLRUCache_LenWithEviction(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](2)
	c.Set("a", 1)
	c.Set("b", 2)

	assert.Equal(t, 2, c.Len())

	c.Set("c", 3) // should evict "a"

	assert.Equal(t, 2, c.Len())
}

func TestLRUCache_ConcurrentDeletes(t *testing.T) {
	t.Parallel()

	c := lru.New[int, int](1000)

	// Pre-populate
	for i := range 1000 {
		c.Set(i, i)
	}

	var wg sync.WaitGroup

	// Concurrent deletions
	for i := range 100 {
		wg.Add(1)

		go func(start int) {
			defer wg.Done()

			for j := range 10 {
				c.Delete(start*10 + j)
			}
		}(i)
	}

	wg.Wait()
}

func TestLRUCache_ConcurrentDeletesAndReads(t *testing.T) {
	t.Parallel()

	c := lru.New[int, int](100)

	// Pre-populate
	for i := range 100 {
		c.Set(i, i)
	}

	var wg sync.WaitGroup

	// Deleters
	for i := range 10 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := range 10 {
				c.Delete(id*10 + j)
			}
		}(i)
	}

	// Readers
	for range 10 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for j := range 100 {
				c.Get(j)
			}
		}()
	}

	wg.Wait()
}

func TestLRUCache_ConcurrentDeletesAndWrites(t *testing.T) {
	t.Parallel()

	c := lru.New[int, int](100)

	var wg sync.WaitGroup

	// Writers
	for i := range 10 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := range 100 {
				c.Set(id*100+j, j)
			}
		}(i)
	}

	// Deleters
	for i := range 10 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := range 100 {
				c.Delete(id*100 + j)
			}
		}(i)
	}

	wg.Wait()
}

func TestLRUCache_ConcurrentPeek(t *testing.T) {
	t.Parallel()

	c := lru.New[int, int](100)

	// Pre-populate
	for i := range 100 {
		c.Set(i, i)
	}

	var wg sync.WaitGroup

	// Concurrent peeks
	for range 100 {
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

func TestLRUCache_ConcurrentPeekAndWrites(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](100)

	var wg sync.WaitGroup

	// Writers
	for i := range 10 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := range 100 {
				c.Set(fmt.Sprintf("key%d-%d", id, j), j)
			}
		}(i)
	}

	// Peekers
	for i := range 10 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := range 100 {
				c.Peek(fmt.Sprintf("key%d-%d", id, j))
			}
		}(i)
	}

	wg.Wait()
}

func TestLRUCache_ConcurrentAllOperations(t *testing.T) {
	t.Parallel()

	c := lru.New[int, int](50)

	var wg sync.WaitGroup

	// Writers
	for i := range 10 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := range 50 {
				c.Set(id*50+j, j)
			}
		}(i)
	}

	// Readers (Get)
	for range 10 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for j := range 100 {
				c.Get(j)
			}
		}()
	}

	// Peekers
	for range 10 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for j := range 100 {
				c.Peek(j)
			}
		}()
	}

	// Deleters
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

func TestLRUCache_ConcurrentLen(t *testing.T) {
	t.Parallel()

	c := lru.New[int, int](100)

	var wg sync.WaitGroup

	// Writers
	for i := range 10 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := range 50 {
				c.Set(id*50+j, j)
				c.Len() // concurrent Len calls while writing
			}
		}(i)
	}

	// Deleters
	for range 10 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for j := range 50 {
				c.Delete(j)
				c.Len() // concurrent Len calls while deleting
			}
		}()
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
