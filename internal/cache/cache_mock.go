package cache

import "time"

type MockCache struct {
	items map[string]cacheItem // reuse cacheItem from cache.go
}

func NewMockCache() *MockCache {
	return &MockCache{items: make(map[string]cacheItem)}
}

func (m *MockCache) Get(url string) (any, bool) {
	item, ok := m.items[url]
	if !ok || time.Now().After(item.expiry) {
		return nil, false
	}
	return item.result, true
}

func (m *MockCache) Set(url string, result any, ttl time.Duration) {
	m.items[url] = cacheItem{result: result, expiry: time.Now().Add(ttl)}
}
