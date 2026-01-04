package slru

import (
	"sync"

	"github.com/serroba/cache/lru"
)

// Cache implements a Segmented LRU cache.
// Items enter through the probation segment and are promoted to the protected
// segment on subsequent access. This helps protect frequently accessed items
// from being evicted by a burst of new entries.
type Cache[K comparable, V any] struct {
	mu               sync.Mutex
	probationSegment *lru.Cache[K, V]
	protectedSegment *lru.Cache[K, V]
}

// New creates a new SLRU cache with the given capacity.
// The capacity is split 80/20 between protected and probation segments.
func New[K comparable, V any](capacity uint64) *Cache[K, V] {
	return NewWithRatio[K, V](capacity, 80)
}

// NewWithRatio creates a new SLRU cache with the given capacity and protected ratio.
// The protectedPercent parameter specifies what percentage of capacity goes to the
// protected segment (0-100). The remainder goes to probation.
// For example, protectedPercent=80 means 80% protected, 20% probation.
func NewWithRatio[K comparable, V any](capacity uint64, protectedPercent uint8) *Cache[K, V] {
	if protectedPercent > 100 {
		protectedPercent = 100
	}

	protectedCap := capacity * uint64(protectedPercent) / 100
	probationCap := capacity - protectedCap

	if protectedCap == 0 {
		protectedCap = 1
	}

	if probationCap == 0 {
		probationCap = 1
	}

	return &Cache[K, V]{
		probationSegment: lru.New[K, V](probationCap),
		protectedSegment: lru.New[K, V](protectedCap),
	}
}

// Set adds or updates a value in the cache.
// New items are added to the probation segment.
// Existing items are updated in their current segment.
func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key exists in protected segment
	if _, ok := c.protectedSegment.Peek(key); ok {
		c.protectedSegment.Set(key, value)

		return
	}

	// Check if key exists in probation segment
	if _, ok := c.probationSegment.Peek(key); ok {
		c.probationSegment.Set(key, value)

		return
	}

	// New key goes to probation
	c.probationSegment.Set(key, value)
}

// Get retrieves a value from the cache.
// If the item is in probation, it gets promoted to protected.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check protected segment first (hot items)
	if v, ok := c.protectedSegment.Get(key); ok {
		return v, true
	}

	// Check probation segment - if found, promote to protected
	if v, ok := c.probationSegment.Peek(key); ok {
		c.probationSegment.Delete(key)
		c.protectedSegment.Set(key, v)

		return v, true
	}

	var zero V

	return zero, false
}

// Peek returns the value for a key without promoting it.
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if v, ok := c.protectedSegment.Peek(key); ok {
		return v, true
	}

	if v, ok := c.probationSegment.Peek(key); ok {
		return v, true
	}

	var zero V

	return zero, false
}

// Delete removes a key from the cache.
func (c *Cache[K, V]) Delete(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.protectedSegment.Delete(key) {
		return true
	}

	return c.probationSegment.Delete(key)
}

// Len returns the total number of items in both segments.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.protectedSegment.Len() + c.probationSegment.Len()
}
