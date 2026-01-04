# cache

[![CI](https://github.com/serroba/cache/actions/workflows/ci.yml/badge.svg)](https://github.com/serroba/cache/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/serroba/cache/branch/main/graph/badge.svg)](https://codecov.io/gh/serroba/cache)
[![Go Report Card](https://goreportcard.com/badge/github.com/serroba/cache)](https://goreportcard.com/report/github.com/serroba/cache)
[![Go Reference](https://pkg.go.dev/badge/github.com/serroba/cache.svg)](https://pkg.go.dev/github.com/serroba/cache)

A Go library providing thread-safe, generic cache implementations with different eviction algorithms.

## Installation

```bash
go get github.com/serroba/cache
```

## Algorithms

| Algorithm | Best For                           | Eviction Strategy                   |
|-----------|------------------------------------|-------------------------------------|
| **LRU**   | General purpose caching            | Evicts least recently accessed item |
| **SLRU**  | Scan-resistant workloads           | Two-segment LRU with promotion      |
| **Clock** | Memory-efficient LRU approximation | Second-chance algorithm             |
| **FIFO**  | Simple, predictable eviction       | Evicts oldest inserted item         |

## Quick Start

All cache implementations share the same API:

```go
cache := lru.New[string, int](100)  // Create cache with capacity 100
cache.Set("key", 42)                 // Add or update
val, ok := cache.Get("key")          // Retrieve (affects eviction in LRU)
val, ok = cache.Peek("key")          // Retrieve without affecting eviction
cache.Delete("key")                  // Remove
length := cache.Len()                // Get current size
```

## Choosing an Algorithm

### LRU (Least Recently Used)

Best for workloads with temporal locality where recently accessed items are likely to be accessed again.

```go
import "github.com/serroba/cache/lru"

// Create a cache for user sessions
sessions := lru.New[string, *Session](10000)

// Store a session
sessions.Set("session:abc123", &Session{UserID: 42, ExpiresAt: time.Now().Add(24*time.Hour)})

// Retrieve - marks as recently used, won't be evicted soon
if session, ok := sessions.Get("session:abc123"); ok {
    fmt.Printf("User %d logged in\n", session.UserID)
}

// Check without affecting eviction order
if _, ok := sessions.Peek("session:abc123"); ok {
    fmt.Println("Session exists")
}
```

**When to use LRU:**
- Database query result caching
- Session storage
- API response caching
- Any cache where recent access predicts future access

### SLRU (Segmented LRU)

Best for workloads that mix frequently accessed "hot" items with occasional full scans. SLRU prevents scans from evicting popular items.

```go
import "github.com/serroba/cache/slru"

// Default: 80% protected, 20% probation
cache := slru.New[string, *Page](10000)

// Custom ratio: 50% protected, 50% probation
cache := slru.NewWithRatio[string, *Page](10000, 50)

// New items enter probation
cache.Set("page:home", homePage)

// First access promotes to protected segment
cache.Get("page:home")  // Now protected from scan eviction

// Subsequent accesses keep it in protected
cache.Get("page:home")  // Stays in protected, moved to front
```

**How SLRU works:**
1. New items enter the **probation** segment
2. When accessed again, items are **promoted** to the protected segment
3. When protected is full, its LRU item is **demoted** back to probation
4. Eviction always happens from probation first

**When to use SLRU:**
- CDN/proxy caches where crawlers shouldn't evict popular content
- Database buffer pools where table scans shouldn't evict hot pages
- Any cache mixing frequent hits with occasional full iterations

### Clock (Second Chance)

Best when you want LRU-like behavior with simpler implementation. Uses a circular buffer with reference bits.

```go
import "github.com/serroba/cache/clock"

cache := clock.New[string, []byte](5000)

cache.Set("image:1", imageData)
cache.Get("image:1")  // Sets reference bit - gets "second chance" on eviction

// When evicting:
// 1. If reference bit is set: clear it, move on (second chance)
// 2. If reference bit is clear: evict this item
```

**When to use Clock:**
- Memory-constrained environments
- When approximate LRU is sufficient
- Systems where simpler data structures help with debugging

### FIFO (First In, First Out)

Best when insertion order determines relevance, not access patterns.

```go
import "github.com/serroba/cache/fifo"

// Event buffer - oldest events evicted first
events := fifo.New[string, *Event](1000)

events.Set("event:1", event1)  // First in
events.Set("event:2", event2)
events.Set("event:3", event3)

// Accessing doesn't change eviction order
events.Get("event:1")  // Still first to be evicted

// When capacity is reached, event:1 is evicted first
events.Set("event:1001", newEvent)
```

**When to use FIFO:**
- Time-series data where older entries become less relevant
- Message queues with size limits
- Audit logs or event buffers
- When predictable eviction order matters more than hit rate

## Common Patterns

### Cache-Aside Pattern

```go
func GetUser(id string) (*User, error) {
    // Try cache first
    if user, ok := cache.Get(id); ok {
        return user, nil
    }

    // Cache miss - fetch from database
    user, err := db.GetUser(id)
    if err != nil {
        return nil, err
    }

    // Store in cache for next time
    cache.Set(id, user)
    return user, nil
}
```

### Write-Through Pattern

```go
func UpdateUser(user *User) error {
    // Update database first
    if err := db.UpdateUser(user); err != nil {
        return err
    }

    // Then update cache
    cache.Set(user.ID, user)
    return nil
}
```

### Invalidation

```go
func DeleteUser(id string) error {
    if err := db.DeleteUser(id); err != nil {
        return err
    }

    // Remove from cache
    cache.Delete(id)
    return nil
}
```

## Thread Safety

All cache implementations are safe for concurrent use. They use a mutex internally, so you don't need external synchronization:

```go
cache := lru.New[string, int](1000)

var wg sync.WaitGroup
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        cache.Set(fmt.Sprintf("key:%d", id), id)
        cache.Get(fmt.Sprintf("key:%d", id))
    }(i)
}
wg.Wait()
```

## Performance

All operations across all implementations are O(1):

| Operation | Time Complexity |
|-----------|-----------------|
| `Get`     | O(1)            |
| `Set`     | O(1)            |
| `Peek`    | O(1)            |
| `Delete`  | O(1)            |
| `Len`     | O(1)            |

## API Reference

All cache types implement the same interface:

```go
type Cache[K comparable, V any] interface {
    // Set adds or updates a key-value pair
    Set(key K, value V)

    // Get retrieves a value (may affect eviction order depending on algorithm)
    Get(key K) (V, bool)

    // Peek retrieves a value without affecting eviction order
    Peek(key K) (V, bool)

    // Delete removes a key
    Delete(key K) bool

    // Len returns the current number of items
    Len() int
}
```

### Behavior Differences

| Method           | LRU            | SLRU                         | Clock              | FIFO        |
|------------------|----------------|------------------------------|--------------------|-------------|
| `Get`            | Moves to front | Promotes probation→protected | Sets reference bit | No effect   |
| `Set` (existing) | Moves to front | Stays in segment             | Sets reference bit | No effect   |
| `Peek`           | No effect      | No effect                    | No effect          | Same as Get |

## License

MIT
