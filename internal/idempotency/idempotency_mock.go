package idempotency

type MockStore struct {
	items map[string]any
}

// right now, in memory cache only, later need to get from distributed cache.
func NewMockStore() *MockStore {
	return &MockStore{items: make(map[string]any)}
}

func (m *MockStore) Get(id string) (any, bool) {
	r, ok := m.items[id]
	return r, ok
}

func (m *MockStore) Set(id string, r any) {
	m.items[id] = r
}
