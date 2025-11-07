package queue

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMockQueue_PushPop(t *testing.T) {
	q := NewMockQueue()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Push should not panic
	err := q.Push(ctx, Job{URL: "https://example.com", Email: "a@b.c"})
	assert.NoError(t, err)

	// Pop should timeout (mock never returns)
	_, err = q.Pop(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
