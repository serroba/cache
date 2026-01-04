// Package clock provides a thread-safe Clock (Second Chance) cache implementation.
//
// # When to Use Clock
//
// Use Clock when you want LRU-like behavior with simpler implementation and
// potentially better performance characteristics. Clock is ideal for:
//   - Memory-constrained environments where simpler data structures help
//   - Workloads where approximate LRU is sufficient
//   - Systems where you want second-chance behavior for recently accessed items
//
// # How Clock Works
//
// Clock uses a circular buffer with a "hand" pointer and reference bits:
//  1. Each item has a reference bit, set to true when accessed
//  2. On eviction, the hand sweeps the buffer looking for items to evict
//  3. If an item's reference bit is true, it gets a "second chance": bit cleared, hand moves on
//  4. If an item's reference bit is false, it's evicted
//
// This approximates LRU: frequently accessed items keep getting their bit set,
// surviving eviction sweeps.
//
// # Thread Safety
//
// All methods are safe for concurrent use. The cache uses a mutex internally.
//
// # Performance
//
// All operations (Get, Set, Delete, Peek, Len) are O(1) amortized.
//
// # Example Usage
//
//	cache := clock.New[string, int](100)
//	cache.Set("key", 42)
//	cache.Get("key")        // Sets reference bit
//	// On eviction, "key" gets a second chance
package clock

import "sync"

type entry[K comparable, V any] struct {
	key        K
	value      V
	referenced bool
}

// Cache implements a Clock cache (also known as Second Chance).
//
// It approximates LRU with O(1) access time by using a circular buffer
// and a reference bit instead of reordering on every access. When an item
// is accessed, its reference bit is set. During eviction, items with set
// bits get a "second chance" (bit cleared), while items with cleared bits
// are evicted.
//
// The zero value is not usable; create instances with [New].
type Cache[K comparable, V any] struct {
	mu       sync.Mutex
	items    map[K]uint64
	ring     []*entry[K, V]
	hand     uint64
	capacity uint64
	size     uint64
}

// New creates a new Clock cache with the specified maximum capacity.
//
// The capacity determines how many key-value pairs the cache can hold.
// When this limit is exceeded, items are evicted using the clock algorithm.
//
// Example:
//
//	cache := clock.New[string, *Session](1000)
func New[K comparable, V any](capacity uint64) *Cache[K, V] {
	return &Cache[K, V]{
		items:    make(map[K]uint64),
		ring:     make([]*entry[K, V], capacity),
		capacity: capacity,
	}
}

// Set adds or updates a key-value pair in the cache.
//
// Behavior:
//   - If the key exists: updates the value and sets the reference bit (second chance)
//   - If the key is new and cache is full: evicts an item using clock algorithm first
//   - If the key is new and cache has space: simply adds the item
//
// New items start with their reference bit cleared, making them eligible for
// eviction until they are accessed via [Cache.Get].
//
// Example:
//
//	cache.Set("config", configData)
//	cache.Set("config", newConfig)  // Updates and sets reference bit
func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing
	if idx, ok := c.items[key]; ok {
		c.ring[idx].value = value
		c.ring[idx].referenced = true

		return
	}

	// Need to evict if at capacity
	if c.size >= c.capacity {
		c.evict()
	}

	// Find empty slot (after eviction or if not full)
	idx := c.findEmptySlot()
	c.ring[idx] = &entry[K, V]{
		key:        key,
		value:      value,
		referenced: false,
	}
	c.items[key] = idx
	c.size++
}

// Get retrieves a value from the cache and sets its reference bit.
//
// Returns:
//   - (value, true) if the key exists
//   - (zero value, false) if the key does not exist
//
// Setting the reference bit gives the item a "second chance" during eviction.
// Use [Cache.Peek] if you need to check a value without affecting eviction.
//
// Example:
//
//	if session, ok := cache.Get("session:abc"); ok {
//	    // session found, now protected from immediate eviction
//	}
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	idx, ok := c.items[key]
	if !ok {
		var zero V

		return zero, false
	}

	c.ring[idx].referenced = true

	return c.ring[idx].value, true
}

// Peek retrieves a value without setting the reference bit.
//
// Returns:
//   - (value, true) if the key exists
//   - (zero value, false) if the key does not exist
//
// Unlike [Cache.Get], this does not give the item a "second chance" during
// eviction. Use Peek when you need to check a value without affecting the
// cache's eviction behavior.
//
// Example:
//
//	// Check without protecting from eviction
//	if _, ok := cache.Peek("maybe-expired"); ok {
//	    // Item exists but won't get second chance
//	}
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	idx, ok := c.items[key]
	if !ok {
		var zero V

		return zero, false
	}

	return c.ring[idx].value, true
}

// Delete removes a key from the cache.
//
// Returns true if the key existed and was removed, false if the key was not found.
// The slot in the ring buffer is marked as empty and can be reused.
//
// Example:
//
//	cache.Delete("invalidated-token")
func (c *Cache[K, V]) Delete(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	idx, ok := c.items[key]
	if !ok {
		return false
	}

	c.ring[idx] = nil
	delete(c.items, key)
	c.size--

	return true
}

// Len returns the current number of items in the cache.
//
// This value is always <= the capacity specified in [New].
//
// Example:
//
//	fmt.Printf("Cache contains %d items\n", cache.Len())
func (c *Cache[K, V]) Len() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.size
}

// evict removes an item using the clock algorithm.
// Must be called with lock held and when size >= capacity (cache is full).
// Since the cache is full, all slots are occupied; no nil checks needed.
func (c *Cache[K, V]) evict() {
	for {
		e := c.ring[c.hand]

		if e.referenced {
			// Give second chance
			e.referenced = false

			c.advanceHand()

			continue
		}

		// Evict this entry
		delete(c.items, e.key)
		c.ring[c.hand] = nil
		c.size--

		return
	}
}

// findEmptySlot finds an empty slot in the ring.
// Must be called with lock held and when there's guaranteed to be an empty slot.
// This is always called after evict() has freed a slot, so an empty slot exists.
func (c *Cache[K, V]) findEmptySlot() uint64 {
	for {
		if c.ring[c.hand] == nil {
			idx := c.hand
			c.advanceHand()

			return idx
		}

		c.advanceHand()
	}
}

// advanceHand moves the clock hand forward.
func (c *Cache[K, V]) advanceHand() {
	c.hand = (c.hand + 1) % c.capacity
}
