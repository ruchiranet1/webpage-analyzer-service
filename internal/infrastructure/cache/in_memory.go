package cache

import (
	"context"
	"sync"
	"time"
)

// inMemoryCache uses a standard Go map and a RWMutex for concurrent access.
type inMemoryCache struct {
	// Pattern: Monitor (using a mutex to guard shared state)
	mu   sync.RWMutex
	data map[string]cacheItem
}

// cacheItem holds the data and its expiration time.
type cacheItem struct {
	value     []byte
	expiresAt time.Time
}

// NewInMemoryCache is the Factory Function for in-memory cache.
func NewInMemoryCache() Service {
	c := &inMemoryCache{
		data: make(map[string]cacheItem),
	}
	return c
}

// Set implements the cache.Service interface.
func (c *inMemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	// Lock the map for writing
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check for context cancellation before performing the write
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	c.data[key] = cacheItem{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

// Get implements the cache.Service interface.
func (c *inMemoryCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	// Lock the map for reading
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	default:
	}

	item, found := c.data[key]
	if !found {
		return nil, false, nil // Not found, no error
	}

	// Check for expiration
	if time.Now().After(item.expiresAt) {
		// Key is expired.
		// A background cleaner goroutine would handle the deletion.
		return nil, false, nil
	}

	return item.value, true, nil
}
