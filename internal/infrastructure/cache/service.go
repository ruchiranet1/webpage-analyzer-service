package cache

import (
	"context"
	"time"
)

// Service defines the interface for a generic key-value cache.
type Service interface {
	// Set stores a value in the cache with a given key and TTL.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Get retrieves a value from the cache by its key.
	Get(ctx context.Context, key string) (value []byte, found bool, err error)
}
