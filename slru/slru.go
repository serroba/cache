// Package slru provides a thread-safe Segmented LRU (SLRU) cache implementation.
//
// # When to Use SLRU
//
// Use SLRU when you need better scan resistance than standard LRU. SLRU protects
// frequently accessed items from being evicted by a burst of new entries. This is
// ideal for:
//   - Workloads mixing frequent "hot" items with occasional full scans
//   - Database caches where table scans shouldn't evict popular rows
//   - CDN/proxy caches where crawlers shouldn't evict popular content
//
// # How SLRU Works
//
// The cache is divided into two segments:
//   - Probation: New items start here (default 20% of capacity)
//   - Protected: Items promoted here after being accessed again (default 80%)
//
// When a probation item is accessed via Get, it's promoted to protected.
// When protected is full, its least recently used item is demoted back to probation.
// Eviction always happens from probation first, protecting frequently used items.
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
//	cache := slru.New[string, int](1000)  // 80% protected, 20% probation
//	cache.Set("key", 42)                   // Enters probation
//	cache.Get("key")                       // Promoted to protected
package slru

import "sync"

type segment uint8

const (
	probation segment = iota
	protected
)

type node[K comparable, V any] struct {
	key        K
	value      V
	segment    segment
	prev, next *node[K, V]
}

// Cache implements a Segmented LRU (SLRU) cache with probation and protected segments.
//
// New items enter the probation segment. When accessed again via [Cache.Get], they are
// promoted to the protected segment. This two-tier structure provides scan resistance:
// a burst of new items will only evict other new items in probation, not the frequently
// accessed items in protected.
//
// The zero value is not usable; create instances with [New] or [NewWithRatio].
type Cache[K comparable, V any] struct {
	mu sync.Mutex

	items map[K]*node[K, V]

	probationHead, probationTail *node[K, V]
	protectedHead, protectedTail *node[K, V]

	probationCap, protectedCap uint64
	probationLen, protectedLen uint64
}

// New creates a new SLRU cache with the given capacity using the default 80/20 split.
//
// The capacity is divided as:
//   - Protected segment: 80% (frequently accessed items)
//   - Probation segment: 20% (new items awaiting promotion)
//
// Use [NewWithRatio] if you need a different split.
//
// Example:
//
//	cache := slru.New[string, *Page](10000)  // 8000 protected, 2000 probation
func New[K comparable, V any](capacity uint64) *Cache[K, V] {
	return NewWithRatio[K, V](capacity, 80)
}

// NewWithRatio creates a new SLRU cache with a custom protected/probation ratio.
//
// Parameters:
//   - capacity: total number of items the cache can hold
//   - protectedPercent: percentage of capacity for the protected segment (0-100)
//
// The probation segment gets the remaining capacity. Both segments are guaranteed
// at least 1 slot.
//
// Example:
//
//	// 50/50 split for workloads with many unique accesses
//	cache := slru.NewWithRatio[string, int](1000, 50)
//
//	// 90/10 split for highly skewed access patterns
//	cache := slru.NewWithRatio[string, int](1000, 90)
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

	probationHead := &node[K, V]{segment: probation}
	probationTail := &node[K, V]{segment: probation}
	probationHead.next = probationTail
	probationTail.prev = probationHead

	protectedHead := &node[K, V]{segment: protected}
	protectedTail := &node[K, V]{segment: protected}
	protectedHead.next = protectedTail
	protectedTail.prev = protectedHead

	return &Cache[K, V]{
		items:         make(map[K]*node[K, V]),
		probationHead: probationHead,
		probationTail: probationTail,
		protectedHead: protectedHead,
		protectedTail: protectedTail,
		probationCap:  probationCap,
		protectedCap:  protectedCap,
	}
}

// Set adds or updates a key-value pair in the cache.
//
// Behavior:
//   - New keys: added to the probation segment
//   - Existing keys: value updated in place, item stays in its current segment
//
// New items must "earn" their place in the protected segment by being accessed
// again via [Cache.Get]. This is what gives SLRU its scan resistance.
//
// Example:
//
//	cache.Set("page:1", pageData)   // Enters probation
//	cache.Set("page:1", newData)    // Updates value, stays in probation
//	cache.Get("page:1")             // NOW promoted to protected
func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if n, ok := c.items[key]; ok {
		n.value = value
		c.moveToHead(n)

		return
	}

	n := &node[K, V]{key: key, value: value, segment: probation}
	c.items[key] = n
	c.addToHead(n, probation)
	c.probationLen++

	if c.probationLen > c.probationCap {
		c.evictFromProbation()
	}
}

// Get retrieves a value and promotes probation items to protected.
//
// Returns:
//   - (value, true) if the key exists
//   - (zero value, false) if the key does not exist
//
// Promotion behavior:
//   - Items in probation are promoted to the protected segment
//   - Items already in protected are moved to the front (most recently used)
//   - If protected is full, its LRU item is demoted back to probation
//
// This promotion mechanism is what provides SLRU's scan resistance. Use
// [Cache.Peek] if you need to read without promoting.
//
// Example:
//
//	cache.Set("item", data)           // In probation
//	cache.Get("item")                 // Promoted to protected
//	cache.Get("item")                 // Stays in protected, moved to front
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	n, ok := c.items[key]
	if !ok {
		var zero V

		return zero, false
	}

	if n.segment == probation {
		c.promote(n)
	} else {
		c.moveToHead(n)
	}

	return n.value, true
}

// Peek retrieves a value without promoting it.
//
// Returns:
//   - (value, true) if the key exists
//   - (zero value, false) if the key does not exist
//
// Unlike [Cache.Get], this does not promote probation items to protected.
// Use Peek when you need to check a value without affecting the cache's
// eviction behavior.
//
// Example:
//
//	// Check item without promoting it
//	if data, ok := cache.Peek("temp-item"); ok {
//	    // Item found but stays in its current segment
//	}
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if n, ok := c.items[key]; ok {
		return n.value, true
	}

	var zero V

	return zero, false
}

// Delete removes a key from the cache, regardless of which segment it's in.
//
// Returns true if the key existed and was removed, false if the key was not found.
//
// Example:
//
//	cache.Delete("expired-session")
func (c *Cache[K, V]) Delete(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	n, ok := c.items[key]
	if !ok {
		return false
	}

	c.removeNode(n)

	if n.segment == probation {
		c.probationLen--
	} else {
		c.protectedLen--
	}

	delete(c.items, key)

	return true
}

// Len returns the total number of items across both segments.
//
// This is the combined count of items in probation and protected segments.
//
// Example:
//
//	fmt.Printf("Cache has %d items\n", cache.Len())
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.items)
}

// promote moves a node from probation to protected segment.
func (c *Cache[K, V]) promote(n *node[K, V]) {
	c.removeNode(n)
	c.probationLen--

	n.segment = protected
	c.addToHead(n, protected)
	c.protectedLen++

	if c.protectedLen > c.protectedCap {
		c.demoteLRU()
	}
}

// demoteLRU moves the LRU item from protected back to probation.
// This is only called when protectedLen > protectedCap, so protected is never empty.
// Note: This cannot cause probation overflow because:
// - promote() removes 1 from probation and demoteLRU adds 1 back (net zero change)
// - probationLen never exceeds probationCap after Set() completes.
func (c *Cache[K, V]) demoteLRU() {
	lru := c.protectedTail.prev

	c.removeNode(lru)
	c.protectedLen--

	lru.segment = probation
	c.addToHead(lru, probation)
	c.probationLen++
}

// evictFromProbation removes the LRU item from the probation segment.
// This is only called when probationLen > probationCap, so probation is never empty.
func (c *Cache[K, V]) evictFromProbation() {
	lru := c.probationTail.prev

	c.removeNode(lru)
	c.probationLen--

	delete(c.items, lru.key)
}

// removeNode removes a node from its current linked list.
func (c *Cache[K, V]) removeNode(n *node[K, V]) {
	n.prev.next = n.next
	n.next.prev = n.prev
}

// addToHead adds a node to the head of the specified segment's list.
func (c *Cache[K, V]) addToHead(n *node[K, V], seg segment) {
	var head *node[K, V]

	if seg == probation {
		head = c.probationHead
	} else {
		head = c.protectedHead
	}

	n.next = head.next
	n.prev = head
	head.next.prev = n
	head.next = n
}

// moveToHead moves an existing node to the head of its segment's list.
func (c *Cache[K, V]) moveToHead(n *node[K, V]) {
	c.removeNode(n)
	c.addToHead(n, n.segment)
}
