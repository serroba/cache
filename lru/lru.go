package lru

type node[K comparable, V any] struct {
	key        K
	value      V
	prev, next *node[K, V]
}

type Cache[K comparable, V any] struct {
	capacity   uint64
	items      map[K]*node[K, V]
	head, tail *node[K, V]
}

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

func (c *Cache[K, V]) Set(key K, value V) {
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

func (c *Cache[K, V]) Get(key K) (V, bool) {
	if v, ok := c.items[key]; ok {
		return v.value, ok
	}

	var v V

	return v, false
}
