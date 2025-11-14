package cache

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// Test basic Set and Get behavior.
func TestInMemoryCache_SetGet(t *testing.T) {
	c := NewInMemoryCache()

	ctx := context.Background()
	key := "key1"
	val := []byte("value1")

	if err := c.Set(ctx, key, val, time.Minute); err != nil {
		t.Fatalf("Set returned unexpected error: %v", err)
	}

	got, ok, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get returned unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected key to be present")
	}
	if !bytes.Equal(got, val) {
		t.Fatalf("value mismatch: got %q want %q", string(got), string(val))
	}
}

// Test that getting a non-existent key returns (nil, false, nil).
func TestInMemoryCache_GetNotFound(t *testing.T) {
	c := NewInMemoryCache()
	ctx := context.Background()

	got, ok, err := c.Get(ctx, "does-not-exist")
	if err != nil {
		t.Fatalf("unexpected error for missing key: %v", err)
	}
	if ok {
		t.Fatalf("expected ok==false for missing key")
	}
	if got != nil {
		t.Fatalf("expected nil value for missing key, got %v", got)
	}
}

// Test expiration behavior: after TTL passes, key should no longer be returned.
func TestInMemoryCache_Expiration(t *testing.T) {
	c := NewInMemoryCache()
	ctx := context.Background()

	key := "expiring"
	val := []byte("shortlived")

	// short TTL
	if err := c.Set(ctx, key, val, 50*time.Millisecond); err != nil {
		t.Fatalf("Set returned unexpected error: %v", err)
	}

	// Immediately available
	if got, ok, err := c.Get(ctx, key); err != nil || !ok || !bytes.Equal(got, val) {
		t.Fatalf("immediate Get failed: got=%v ok=%v err=%v", got, ok, err)
	}

	// Wait for expiration
	time.Sleep(75 * time.Millisecond)

	got, ok, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get after expiry returned error: %v", err)
	}
	if ok {
		t.Fatalf("expected key to be expired and not found")
	}
	if got != nil {
		t.Fatalf("expected nil value after expiry, got %v", got)
	}
}

// Test that Set returns context error when context is canceled before call.
func TestInMemoryCache_Set_ContextCanceled(t *testing.T) {
	c := NewInMemoryCache()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := c.Set(ctx, "k", []byte("v"), time.Minute)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled from Set, got: %v", err)
	}

	// Ensure that the key was not set
	got, ok, err := c.Get(context.Background(), "k")
	if err != nil {
		t.Fatalf("Get returned unexpected error: %v", err)
	}
	if ok || got != nil {
		t.Fatalf("expected key to not be set after canceled Set")
	}
}

// Test that Get returns context error if context is already canceled.
func TestInMemoryCache_Get_ContextCanceled(t *testing.T) {
	c := NewInMemoryCache()

	// prepare a key so that path would normally succeed
	if err := c.Set(context.Background(), "kc", []byte("v"), time.Minute); err != nil {
		t.Fatalf("Set returned unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := c.Get(ctx, "kc")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled from Get, got: %v", err)
	}
}

// Test that Set overwrites an existing value.
func TestInMemoryCache_Overwrite(t *testing.T) {
	c := NewInMemoryCache()
	ctx := context.Background()

	key := "dup"
	v1 := []byte("first")
	v2 := []byte("second")

	if err := c.Set(ctx, key, v1, time.Minute); err != nil {
		t.Fatalf("first Set error: %v", err)
	}
	if err := c.Set(ctx, key, v2, time.Minute); err != nil {
		t.Fatalf("second Set error: %v", err)
	}

	got, ok, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected key to exist after overwrite")
	}
	if !bytes.Equal(got, v2) {
		t.Fatalf("expected overwritten value %q, got %q", string(v2), string(got))
	}
}

// Concurrent access test: multiple goroutines calling Set and Get concurrently.
func TestInMemoryCache_ConcurrentAccess(t *testing.T) {
	c := NewInMemoryCache()
	ctx := context.Background()

	const (
		nGoroutines = 50
		nOps        = 200
	)

	var wg sync.WaitGroup
	wg.Add(nGoroutines * 2) // writers + readers

	// Writers
	for i := 0; i < nGoroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			for j := 0; j < nOps; j++ {
				k := "key-w-" + itoa(i) + "-" + itoa(j%10)
				v := []byte("v" + itoa(j))
				// Use short TTL to exercise expiry reads too
				_ = c.Set(ctx, k, v, 200*time.Millisecond)
				// small pause to increase interleaving
				time.Sleep(time.Microsecond)
			}
		}()
	}

	// Readers
	for i := 0; i < nGoroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			for j := 0; j < nOps; j++ {
				k := "key-w-" + itoa(i) + "-" + itoa(j%10)
				_, _, _ = c.Get(ctx, k)
				time.Sleep(time.Microsecond)
			}
		}()
	}

	wg.Wait()
}

// itoa is a tiny integer-to-string helper to avoid importing strconv in many small tests.
func itoa(i int) string {
	// simple, sufficient for test ids
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i = i / 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
