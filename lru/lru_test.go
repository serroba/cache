package lru_test

import (
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/serroba/cache/lru"
)

func TestLRUCache_Get(t *testing.T) {
	type args[K comparable] struct {
		key K
	}

	type want[V any] struct {
		value V
		ok    bool
	}

	type testCase[K comparable, V any] struct {
		name string
		c    *lru.Cache[K, V]
		args args[K]
		want want[V]
	}

	tests := []testCase[string, int]{
		{
			name: "Test on empty cache",
			c:    lru.New[string, int](5),
			args: args[string]{key: "some"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := tt.c.Get(tt.args.key)
			if !reflect.DeepEqual(got, tt.want.value) {
				t.Errorf("Get() got = %v, want %v", got, tt.want)
			}

			if got1 != tt.want.ok {
				t.Errorf("Get() got1 = %v, want %v", got1, tt.want.value)
			}
		})
	}
}

func TestLRUCache_Set(t *testing.T) {
	type args[K comparable, V any] struct {
		key   K
		value V
	}

	type want[V any] struct {
		value V
		ok    bool
	}

	type testCase[K comparable, V any] struct {
		name string
		c    *lru.Cache[K, V]
		args args[K, V]
		want want[V]
	}

	tests := []testCase[string, int]{
		{
			name: "Set and retrieve value",
			c:    lru.New[string, int](5),
			args: args[string, int]{key: "foo", value: 42},
			want: want[int]{value: 42, ok: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.c.Set(tt.args.key, tt.args.value)

			got, got1 := tt.c.Get(tt.args.key)
			if !reflect.DeepEqual(got, tt.want.value) {
				t.Errorf("Set() got = %v, want %v", got, tt.want.value)
			}

			if got1 != tt.want.ok {
				t.Errorf("Set() got1 = %v, want %v", got1, tt.want.ok)
			}
		})
	}
}

func TestLRUCache_UpdateExistingKey(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](5)
	c.Set("key", 100)
	c.Set("key", 200)

	got, ok := c.Get("key")
	if !ok {
		t.Error("expected key to exist")
	}

	if got != 200 {
		t.Errorf("expected updated value 200, got %v", got)
	}
}

func TestLRUCache_Eviction(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	c.Set("d", 4) // should evict "a"

	if _, ok := c.Get("a"); ok {
		t.Error("expected 'a' to be evicted")
	}

	if v, ok := c.Get("b"); !ok || v != 2 {
		t.Errorf("expected 'b' to exist with value 2, got %v", v)
	}

	if v, ok := c.Get("c"); !ok || v != 3 {
		t.Errorf("expected 'c' to exist with value 3, got %v", v)
	}

	if v, ok := c.Get("d"); !ok || v != 4 {
		t.Errorf("expected 'd' to exist with value 4, got %v", v)
	}
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

	if _, ok := c.Get("b"); ok {
		t.Error("expected 'b' to be evicted")
	}

	if _, ok := c.Get("a"); !ok {
		t.Error("expected 'a' to still exist after being accessed")
	}
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

	if _, ok := c.Get("a"); !ok {
		t.Error("expected 'a' to still exist after being accessed via Get")
	}

	if _, ok := c.Get("b"); ok {
		t.Error("expected 'b' to be evicted (was least recently used)")
	}
}

func TestLRUCache_CapacityOne(t *testing.T) {
	t.Parallel()

	c := lru.New[string, int](1)
	c.Set("a", 1)
	c.Set("b", 2)

	if _, ok := c.Get("a"); ok {
		t.Error("expected 'a' to be evicted")
	}

	if v, ok := c.Get("b"); !ok || v != 2 {
		t.Errorf("expected 'b' to exist with value 2, got %v", v)
	}
}

func TestLRUCache_MultipleTypes(t *testing.T) {
	t.Parallel()

	c := lru.New[int, string](3)
	c.Set(1, "one")
	c.Set(2, "two")
	c.Set(3, "three")

	if v, ok := c.Get(1); !ok || v != "one" {
		t.Errorf("expected 'one', got %v", v)
	}

	if v, ok := c.Get(2); !ok || v != "two" {
		t.Errorf("expected 'two', got %v", v)
	}

	if v, ok := c.Get(3); !ok || v != "three" {
		t.Errorf("expected 'three', got %v", v)
	}
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

	// Verify the key exists with some value
	if _, ok := c.Get("shared"); !ok {
		t.Error("expected 'shared' key to exist")
	}
}
