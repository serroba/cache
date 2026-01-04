// Package fifo provides a thread-safe FIFO (First In, First Out) cache implementation.
//
// # When to Use FIFO
//
// Use FIFO when you want the simplest possible eviction strategy. Items are evicted
// strictly in insertion order, regardless of how often they're accessed. This is ideal for:
//   - Time-based data where older entries naturally become less relevant
//   - Message queues or event buffers with size limits
//   - Simple caching where recency doesn't predict future access
//   - Scenarios where predictable eviction order is more important than hit rate
//
// # FIFO vs LRU
//
// FIFO is simpler but less adaptive than LRU:
//   - FIFO: oldest item evicted, even if frequently accessed
//   - LRU: least recently accessed item evicted
//
// Choose FIFO when simplicity matters more than optimal hit rate.
//
// # Thread Safety
//
// All methods are safe for concurrent use. The cache uses a mutex internally.
//
// # Performance
//
// All operations (Get, Set, Delete, Peek, Len) are O(1).
//
// # Example Usage
//
//	cache := fifo.New[string, int](100)
//	cache.Set("first", 1)
//	cache.Set("second", 2)
//	// When full, "first" will be evicted before "second"
package fifo

import "sync"

type node[K comparable, V any] struct {
	key        K
	value      V
	prev, next *node[K, V]
}

// Cache implements a FIFO (First In, First Out) cache.
//
// Items are evicted in the order they were added, regardless of access patterns.
// This is the simplest eviction strategy with O(1) operations and predictable
// behavior.
//
// The zero value is not usable; create instances with [New].
type Cache[K comparable, V any] struct {
	mu         sync.Mutex
	items      map[K]*node[K, V]
	head, tail *node[K, V] // head = newest, tail = oldest
	capacity   uint64
}

// New creates a new FIFO cache with the specified maximum capacity.
//
// The capacity determines how many key-value pairs the cache can hold.
// When this limit is exceeded, the oldest item is automatically evicted.
//
// Example:
//
//	cache := fifo.New[string, *Event](1000)
func New[K comparable, V any](capacity uint64) *Cache[K, V] {
	head := &node[K, V]{}
	tail := &node[K, V]{}
	head.next = tail
	tail.prev = head

	return &Cache[K, V]{
		items:    make(map[K]*node[K, V]),
		head:     head,
		tail:     tail,
		capacity: capacity,
	}
}

// Set adds or updates a key-value pair in the cache.
//
// Behavior:
//   - If the key exists: updates the value but keeps original insertion order
//   - If the key is new and cache is full: evicts the oldest item first
//   - If the key is new and cache has space: adds item as newest
//
// Unlike LRU, updating an existing key does NOT move it to the front.
// The item retains its original position in the eviction queue.
//
// Example:
//
//	cache.Set("event:1", event1)  // Oldest
//	cache.Set("event:2", event2)
//	cache.Set("event:1", updated) // Still oldest, just updated value
func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing - don't change position (FIFO keeps insertion order)
	if n, ok := c.items[key]; ok {
		n.value = value

		return
	}

	// Evict if at capacity
	if uint64(len(c.items)) >= c.capacity {
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
//
// Returns:
//   - (value, true) if the key exists
//   - (zero value, false) if the key does not exist
//
// Unlike LRU, accessing a key does NOT affect eviction order. The oldest
// item will still be evicted first, regardless of how often it's accessed.
//
// Example:
//
//	if event, ok := cache.Get("event:123"); ok {
//	    process(event)
//	}
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

// Peek retrieves a value from the cache.
//
// Returns:
//   - (value, true) if the key exists
//   - (zero value, false) if the key does not exist
//
// In FIFO, Peek behaves identically to [Cache.Get] since neither affects
// eviction order. This method exists for API compatibility with other cache
// implementations.
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	return c.Get(key)
}

// Delete removes a key from the cache.
//
// Returns true if the key existed and was removed, false if the key was not found.
//
// Example:
//
//	cache.Delete("processed-event")
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

// Len returns the current number of items in the cache.
//
// This value is always <= the capacity specified in [New].
//
// Example:
//
//	fmt.Printf("Buffer has %d events\n", cache.Len())
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
