package idempotency

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInMemoryStore(t *testing.T) {
	s := NewInMemoryStore()

	// Set
	s.Set("req1", "result")

	// Get
	result, ok := s.Get("req1")
	assert.True(t, ok)
	assert.Equal(t, "result", result)

	// Miss
	_, ok = s.Get("req2")
	assert.False(t, ok)
}
