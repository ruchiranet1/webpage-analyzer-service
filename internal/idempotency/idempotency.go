package idempotency

import "sync"

type Store interface {
	Get(requestID string) (any, bool)
	Set(requestID string, result any)
}

type inMemoryStore struct {
	mu    sync.RWMutex
	items map[string]any
}

func NewInMemoryStore() Store {
	return &inMemoryStore{items: make(map[string]any)}
}

// right now, in memory cache only, later need to get from distributed cache.
func (s *inMemoryStore) Get(id string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.items[id]
	return r, ok
}

func (s *inMemoryStore) Set(id string, r any) {
	s.mu.Lock()
	s.items[id] = r
	s.mu.Unlock()
}
