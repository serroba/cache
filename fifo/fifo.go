// Package fifo provides a thread-safe FIFO (First In, First Out) cache implementation.
package fifo

import "sync"

type node[K comparable, V any] struct {
	key        K
	value      V
	prev, next *node[K, V]
}

// Cache implements a FIFO cache.
// Items are evicted in the order they were added, regardless of access patterns.
// This is the simplest eviction strategy with O(1) operations.
type Cache[K comparable, V any] struct {
	mu         sync.Mutex
	items      map[K]*node[K, V]
	head, tail *node[K, V] // head = newest, tail = oldest
	capacity   int
}

// New creates a new FIFO cache with the given capacity.
func New[K comparable, V any](capacity uint64) *Cache[K, V] {
	c := capacity
	if c > uint64(maxInt) {
		c = uint64(maxInt)
	}

	size := int(c) //nolint:gosec // bounds checked above

	head := &node[K, V]{}
	tail := &node[K, V]{}
	head.next = tail
	tail.prev = head

	return &Cache[K, V]{
		items:    make(map[K]*node[K, V]),
		head:     head,
		tail:     tail,
		capacity: size,
	}
}

const maxInt = int(^uint(0) >> 1)

// Set adds or updates a value in the cache.
// If the cache is full, it evicts the oldest item.
func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing - don't change position (FIFO keeps insertion order)
	if n, ok := c.items[key]; ok {
		n.value = value

		return
	}

	// Evict if at capacity
	if len(c.items) >= c.capacity {
		c.evict()
	}

	// Insert at head (newest)
	n := &node[K, V]{key: key, value: value}
	n.next = c.head.next
	n.prev = c.head
	c.head.next.prev = n
	c.head.next = n

	c.items[key] = n
}

// Get retrieves a value from the cache.
// Unlike LRU, accessing a key does NOT affect eviction order.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	n, ok := c.items[key]
	if !ok {
		var zero V

		return zero, false
	}

	return n.value, true
}

// Peek retrieves a value from the cache (same as Get for FIFO).
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	return c.Get(key)
}

// Delete removes a key from the cache.
func (c *Cache[K, V]) Delete(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	n, ok := c.items[key]
	if !ok {
		return false
	}

	c.removeNode(n)
	delete(c.items, key)

	return true
}

// Len returns the number of items in the cache.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.items)
}

// evict removes the oldest item (at tail) from the cache.
// Must be called with lock held.
func (c *Cache[K, V]) evict() {
	oldest := c.tail.prev
	if oldest == c.head {
		return
	}

	c.removeNode(oldest)
	delete(c.items, oldest.key)
}

// removeNode removes a node from the linked list.
func (c *Cache[K, V]) removeNode(n *node[K, V]) {
	n.prev.next = n.next
	n.next.prev = n.prev
}
