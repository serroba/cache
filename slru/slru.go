// Package slru provides a thread-safe Segmented LRU cache implementation.
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

// Cache implements a Segmented LRU cache.
// Items enter through the probation segment and are promoted to the protected
// segment on subsequent access. This helps protect frequently accessed items
// from being evicted by a burst of new entries.
type Cache[K comparable, V any] struct {
	mu sync.Mutex

	items map[K]*node[K, V]

	probationHead, probationTail *node[K, V]
	protectedHead, protectedTail *node[K, V]

	probationCap, protectedCap uint64
	probationLen, protectedLen uint64
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

// Set adds or updates a value in the cache.
// New items are added to the probation segment.
// Existing items are updated in their current segment.
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
		c.evictFrom(probation)
	}
}

// Get retrieves a value from the cache.
// If the item is in probation, it gets promoted to protected.
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

// Peek returns the value for a key without promoting it.
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if n, ok := c.items[key]; ok {
		return n.value, true
	}

	var zero V

	return zero, false
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

	if n.segment == probation {
		c.probationLen--
	} else {
		c.protectedLen--
	}

	delete(c.items, key)

	return true
}

// Len returns the total number of items in both segments.
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
func (c *Cache[K, V]) demoteLRU() {
	lru := c.protectedTail.prev
	if lru == c.protectedHead {
		return
	}

	c.removeNode(lru)
	c.protectedLen--

	lru.segment = probation
	c.addToHead(lru, probation)
	c.probationLen++

	if c.probationLen > c.probationCap {
		c.evictFrom(probation)
	}
}

// evictFrom removes the LRU item from the specified segment.
func (c *Cache[K, V]) evictFrom(seg segment) {
	var tail *node[K, V]

	if seg == probation {
		tail = c.probationTail
	} else {
		tail = c.protectedTail
	}

	lru := tail.prev

	if seg == probation && lru == c.probationHead {
		return
	}

	if seg == protected && lru == c.protectedHead {
		return
	}

	c.removeNode(lru)

	if seg == probation {
		c.probationLen--
	} else {
		c.protectedLen--
	}

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
