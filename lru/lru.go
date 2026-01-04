// Package lru provides a thread-safe LRU (Least Recently Used) cache implementation.
//
// # When to Use LRU
//
// Use LRU when you want to keep frequently accessed items in cache. Items that
// haven't been accessed recently are evicted first. This is ideal for:
//   - Database query caching where recent queries are likely to repeat
//   - Session storage where active sessions should stay cached
//   - Any workload with temporal locality (recent items accessed again soon)
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
//	cache := lru.New[string, int](100)  // Cache up to 100 items
//	cache.Set("user:123", 42)
//	if val, ok := cache.Get("user:123"); ok {
//	    fmt.Println(val) // 42
//	}
package lru

import "sync"

type node[K comparable, V any] struct {
	key        K
	value      V
	prev, next *node[K, V]
}

// Cache is a thread-safe LRU (Least Recently Used) cache.
//
// Items are evicted based on access recency: the least recently accessed item
// is removed when the cache reaches capacity. Both Get and Set operations
// mark an item as "recently used", moving it to the front of the eviction queue.
//
// The zero value is not usable; create instances with [New].
type Cache[K comparable, V any] struct {
	mu sync.Mutex

	capacity   uint64
	items      map[K]*node[K, V]
	head, tail *node[K, V]
}

// New creates a new LRU cache with the specified maximum capacity.
//
// The capacity determines how many key-value pairs the cache can hold.
// When this limit is exceeded, the least recently used item is automatically evicted.
//
// Example:
//
//	// Create a cache that holds up to 1000 items
//	cache := lru.New[string, *User](1000)
//
//	// Keys must be comparable (string, int, etc.)
//	// Values can be any type
//	cache := lru.New[int, []byte](500)
func New[K comparable, V any](capacity uint64) *Cache[K, V] {
	head := &node[K, V]{}
	tail := &node[K, V]{}
	head.next = tail
	tail.prev = head

	return &Cache[K, V]{
		capacity: capacity,
		items:    make(map[K]*node[K, V]),
		head:     head,
		tail:     tail,
	}
}

// Set adds or updates a key-value pair in the cache.
//
// Behavior:
//   - If the key exists: updates the value and marks it as most recently used
//   - If the key is new and cache is full: evicts the least recently used item first
//   - If the key is new and cache has space: simply adds the item
//
// The operation is atomic and thread-safe.
//
// Example:
//
//	cache.Set("session:abc", sessionData)  // Add new item
//	cache.Set("session:abc", updatedData)  // Update existing, moves to front
func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if n, ok := c.items[key]; ok {
		n.value = value
		c.items[key] = n
		c.moveToHead(n)
	} else {
		n := &node[K, V]{key: key, value: value}
		c.items[key] = n
		c.addNodeToHead(n)

		if uint64(len(c.items)) > c.capacity {
			lru := c.tail.prev
			c.removeNode(lru)
			delete(c.items, lru.key)
		}
	}
}

func (c *Cache[K, V]) moveToHead(node *node[K, V]) {
	c.removeNode(node)
	c.addNodeToHead(node)
}

func (c *Cache[K, V]) removeNode(node *node[K, V]) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

func (c *Cache[K, V]) addNodeToHead(node *node[K, V]) {
	node.next = c.head.next
	node.prev = c.head
	c.head.next.prev = node
	c.head.next = node
}

// Get retrieves a value from the cache and marks it as recently used.
//
// Returns:
//   - (value, true) if the key exists
//   - (zero value, false) if the key does not exist
//
// Important: This method updates the item's recency, preventing it from being
// evicted. Use [Cache.Peek] if you need to check a value without affecting
// eviction order.
//
// Example:
//
//	if user, ok := cache.Get("user:123"); ok {
//	    // user found and is now "recently used"
//	    fmt.Println(user.Name)
//	} else {
//	    // user not in cache, fetch from database
//	}
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if v, ok := c.items[key]; ok {
		c.moveToHead(v)

		return v.value, ok
	}

	var v V

	return v, false
}

// Peek retrieves a value without marking it as recently used.
//
// Returns:
//   - (value, true) if the key exists
//   - (zero value, false) if the key does not exist
//
// Unlike [Cache.Get], this does not affect the eviction order. Use Peek when you
// need to check if a value exists or read it without preventing its eviction.
//
// Example:
//
//	// Check if item exists without affecting LRU order
//	if _, ok := cache.Peek("temp-key"); ok {
//	    fmt.Println("key exists but won't be protected from eviction")
//	}
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if v, ok := c.items[key]; ok {
		return v.value, ok
	}

	var v V

	return v, false
}

// Delete removes a key from the cache.
//
// Returns true if the key existed and was removed, false if the key was not found.
//
// Example:
//
//	if cache.Delete("session:expired") {
//	    fmt.Println("session removed")
//	}
func (c *Cache[K, V]) Delete(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if n, ok := c.items[key]; ok {
		c.removeNode(n)
		delete(c.items, key)

		return true
	}

	return false
}

// Len returns the current number of items in the cache.
//
// This value is always <= the capacity specified in [New].
//
// Example:
//
//	fmt.Printf("Cache contains %d items\n", cache.Len())
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.items)
}
