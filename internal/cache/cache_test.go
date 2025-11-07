package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInMemoryCache(t *testing.T) {
	c := NewInMemory()

	// Set
	c.Set("https://example.com", "result", time.Minute)

	// Get
	result, ok := c.Get("https://example.com")
	assert.True(t, ok)
	assert.Equal(t, "result", result)

	// Miss
	_, ok = c.Get("https://missing.com")
	assert.False(t, ok)
}

func TestInMemoryCache_Expiry(t *testing.T) {
	c := NewInMemory()
	c.Set("url", "data", 10*time.Millisecond)

	time.Sleep(20 * time.Millisecond)
	_, ok := c.Get("url")
	assert.False(t, ok)
}
