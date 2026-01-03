package lru

type node[K comparable, V any] struct {
	key   K
	value V
	// prev, next *node[K, V]
}

type Cache[K comparable, V any] struct {
	capacity   uint64
	items      map[K]*node[K, V]
	head, tail *node[K, V]
}

func New[K comparable, V any](capacity uint64) *Cache[K, V] {
	return &Cache[K, V]{
		capacity: capacity,
		items:    make(map[K]*node[K, V]),
		head:     &node[K, V]{},
		tail:     &node[K, V]{},
	}
}

func (c *Cache[K, V]) Set(key K, value V) {
	c.items[key] = &node[K, V]{key: key, value: value}
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	if v, ok := c.items[key]; ok {
		return v.value, ok
	}

	var v V

	return v, false
}
