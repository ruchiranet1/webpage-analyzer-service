package cache

import (
	"sync"
	"time"
)

type Cache interface {
	Get(url string) (any, bool)
	Set(url string, result any, ttl time.Duration)
}

type inMemoryCache struct {
	mu    sync.RWMutex
	items map[string]cacheItem
}

type cacheItem struct {
	result any
	expiry time.Time
}

func NewInMemory() Cache {
	return &inMemoryCache{items: make(map[string]cacheItem)}
}

func (c *inMemoryCache) Get(url string) (any, bool) {
	c.mu.RLock()
	item, ok := c.items[url]
	c.mu.RUnlock()
	if !ok || time.Now().After(item.expiry) {
		return nil, false
	}
	return item.result, true
}

func (c *inMemoryCache) Set(url string, result any, ttl time.Duration) {
	c.mu.Lock()
	c.items[url] = cacheItem{result: result, expiry: time.Now().Add(ttl)}
	c.mu.Unlock()
}
