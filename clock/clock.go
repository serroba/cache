// Package clock provides a thread-safe Clock (Second Chance) cache implementation.
package clock

import "sync"

type entry[K comparable, V any] struct {
	key        K
	value      V
	referenced bool
}

// Cache implements a Clock cache (also known as Second Chance).
// It approximates LRU with O(1) access time by using a circular buffer
// and a reference bit instead of reordering on every access.
type Cache[K comparable, V any] struct {
	mu       sync.Mutex
	items    map[K]int // key -> index in ring
	ring     []*entry[K, V]
	hand     int // clock hand position
	capacity int
	size     int
}

// New creates a new Clock cache with the given capacity.
func New[K comparable, V any](capacity uint64) *Cache[K, V] {
	c := capacity
	if c > uint64(maxInt) {
		c = uint64(maxInt)
	}

	size := int(c) //nolint:gosec // bounds checked above

	return &Cache[K, V]{
		items:    make(map[K]int),
		ring:     make([]*entry[K, V], size),
		capacity: size,
	}
}

const maxInt = int(^uint(0) >> 1)

// Set adds or updates a value in the cache.
// If the cache is full, it evicts an item using the clock algorithm.
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

// Get retrieves a value from the cache.
// Accessing a key sets its reference bit.
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

// Len returns the number of items in the cache.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.size
}

// evict removes an item using the clock algorithm.
// Must be called with lock held.
func (c *Cache[K, V]) evict() {
	for {
		e := c.ring[c.hand]

		if e == nil {
			c.advanceHand()

			continue
		}

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
func (c *Cache[K, V]) findEmptySlot() int {
	start := c.hand

	for {
		if c.ring[c.hand] == nil {
			idx := c.hand
			c.advanceHand()

			return idx
		}

		c.advanceHand()

		if c.hand == start {
			// Should never happen if called correctly
			return 0
		}
	}
}

// advanceHand moves the clock hand forward.
func (c *Cache[K, V]) advanceHand() {
	c.hand = (c.hand + 1) % c.capacity
}
