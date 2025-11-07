package queue

import "context"

type MockQueue struct{}

func NewMockQueue() Queue { return &MockQueue{} }

func (m *MockQueue) Push(_ context.Context, _ Job) error { return nil }
func (m *MockQueue) Pop(_ context.Context) (Job, error) {
	select {
	case <-context.Background().Done():
		return Job{}, context.Background().Err()
	}
}
