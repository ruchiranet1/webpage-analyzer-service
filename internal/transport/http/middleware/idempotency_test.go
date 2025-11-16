package middleware

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockCache satisfies the cache.Service interface for testing.
type mockCache struct {
	GetFunc func(ctx context.Context, key string) (val []byte, found bool, err error)
	SetFunc func(ctx context.Context, key string, val []byte, ttl time.Duration) error
}

func (m *mockCache) Get(ctx context.Context, key string) (val []byte, found bool, err error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, key)
	}
	return nil, false, nil // Default: cache miss
}

func (m *mockCache) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	if m.SetFunc != nil {
		return m.SetFunc(ctx, key, val, ttl)
	}
	return nil // Default: success
}

// noopLogger returns a logger that discards all output.
var noopLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// testHandler is a simple http.Handler that marks itself as called
func testHandler(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.Header().Set("X-From-Handler", "true")
		w.WriteHeader(http.StatusCreated) // Use a non-200 success code for clarity
		w.Write([]byte("response from handler"))
	})
}

// TestResponseRecorder verifies our internal recorder wrapper works as expected.
func TestResponseRecorder(t *testing.T) {
	rr := httptest.NewRecorder()
	recorder := newResponseRecorder(rr)

	// Test default status
	if recorder.statusCode != http.StatusOK {
		t.Errorf("default statuscode wrong: got %d, want %d", recorder.statusCode, http.StatusOK)
	}

	// Write headers, status, and body
	recorder.Header().Set("X-Test", "true")
	recorder.WriteHeader(http.StatusAccepted)
	recorder.Write([]byte("hello"))
	recorder.Write([]byte(" world"))

	// Check our recorder's captured values
	if recorder.statusCode != http.StatusAccepted {
		t.Errorf("captured statuscode wrong: got %d, want %d", recorder.statusCode, http.StatusAccepted)
	}
	if recorder.body.String() != "hello world" {
		t.Errorf("captured body wrong: got %q, want %q", recorder.body.String(), "hello world")
	}

	// Check the underlying ResponseWriter
	if rr.Code != http.StatusAccepted {
		t.Errorf("underlying statuscode wrong: got %d, want %d", rr.Code, http.StatusAccepted)
	}
	if rr.Body.String() != "hello world" {
		t.Errorf("underlying body wrong: got %q, want %q", rr.Body.String(), "hello world")
	}
	if rr.Header().Get("X-Test") != "true" {
		t.Error("underlying header not set")
	}
}

// TestIdempotencyMiddleware_NoKey tests the case where no Idempotency-Key is provided.
func TestIdempotencyMiddleware_NoKey(t *testing.T) {
	// Cache should not be touched at all
	mockCache := &mockCache{
		GetFunc: func(ctx context.Context, key string) ([]byte, bool, error) {
			t.Fatal("Cache.Get should not be called")
			return nil, false, nil
		},
		SetFunc: func(ctx context.Context, key string, val []byte, ttl time.Duration) error {
			t.Fatal("Cache.Set should not be called")
			return nil
		},
	}
	handlerCalled := false

	mw := NewIdempotencyMiddleware(mockCache, noopLogger, time.Minute)
	handler := mw.Middleware(testHandler(&handlerCalled))

	req := httptest.NewRequest("POST", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("handler was not called, but should have been")
	}
	if rr.Code != http.StatusCreated {
		t.Errorf("wrong status code: got %d, want %d", rr.Code, http.StatusCreated)
	}
	if rr.Body.String() != "response from handler" {
		t.Errorf("wrong body: got %q", rr.Body.String())
	}
}

// TestIdempotencyMiddleware_CacheMiss tests the case where the key is not in the cache.
func TestIdempotencyMiddleware_CacheMiss(t *testing.T) {
	getCalled := false
	setCalled := false
	handlerCalled := false
	key := "test-key-miss"

	mockCache := &mockCache{
		GetFunc: func(ctx context.Context, k string) ([]byte, bool, error) {
			getCalled = true
			if k != key {
				t.Errorf("wrong cache key for Get: got %q, want %q", k, key)
			}
			return nil, false, nil // Cache miss
		},
		SetFunc: func(ctx context.Context, k string, val []byte, ttl time.Duration) error {
			setCalled = true
			if k != key {
				t.Errorf("wrong cache key for Set: got %q, want %q", k, key)
			}

			// Verify the data being cached is correct
			var resp cachedResponse
			if err := json.Unmarshal(val, &resp); err != nil {
				t.Fatalf("failed to unmarshal cached value: %v", err)
			}

			if resp.StatusCode != http.StatusCreated {
				t.Errorf("cached status wrong: got %d, want %d", resp.StatusCode, http.StatusCreated)
			}
			if string(resp.Body) != "response from handler" {
				t.Errorf("cached body wrong: got %q", string(resp.Body))
			}
			if resp.Headers.Get("X-From-Handler") != "true" {
				t.Error("cached header missing")
			}
			return nil
		},
	}

	mw := NewIdempotencyMiddleware(mockCache, noopLogger, time.Minute)
	handler := mw.Middleware(testHandler(&handlerCalled))

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set(IdempotencyKeyHeader, key)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !getCalled {
		t.Error("Cache.Get was not called")
	}
	if !handlerCalled {
		t.Error("handler was not called")
	}
	if !setCalled {
		t.Error("Cache.Set was not called")
	}

	if rr.Code != http.StatusCreated {
		t.Errorf("wrong status code: got %d, want %d", rr.Code, http.StatusCreated)
	}
	if rr.Body.String() != "response from handler" {
		t.Errorf("wrong body: got %q", rr.Body.String())
	}
}

// TestIdempotencyMiddleware_CacheHit tests the case where the key is found in the cache.
func TestIdempotencyMiddleware_CacheHit(t *testing.T) {
	getCalled := false
	handlerCalled := false
	key := "test-key-hit"

	// Pre-caching a response
	cachedBody := []byte("cached response")
	cachedResp := cachedResponse{
		StatusCode: http.StatusOK,
		Headers: http.Header{
			"Content-Type": {"application/json"},
			"X-Cached":     {"true"},
		},
		Body: cachedBody,
	}
	cachedData, err := json.Marshal(cachedResp)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	mockCache := &mockCache{
		GetFunc: func(ctx context.Context, k string) ([]byte, bool, error) {
			getCalled = true
			if k != key {
				t.Errorf("wrong cache key for Get: got %q, want %q", k, key)
			}
			return cachedData, true, nil // Cache hit
		},
		SetFunc: func(ctx context.Context, k string, val []byte, ttl time.Duration) error {
			t.Fatal("Cache.Set should not be called on cache hit")
			return nil
		},
	}

	mw := NewIdempotencyMiddleware(mockCache, noopLogger, time.Minute)
	handler := mw.Middleware(testHandler(&handlerCalled))

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set(IdempotencyKeyHeader, key)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !getCalled {
		t.Error("Cache.Get was not called")
	}
	if handlerCalled {
		t.Error("handler was called, but should not have been")
	}

	// Verify the *cached* response was written
	if rr.Code != http.StatusOK {
		t.Errorf("wrong status code: got %d, want %d", rr.Code, http.StatusOK)
	}
	// json.Marshal uses base64 for []byte, so we must decode it from the raw JSON
	if rr.Body.String() != string(cachedBody) {
		t.Errorf("wrong body: got %q, want %q", rr.Body.String(), string(cachedBody))
	}
	if rr.Header().Get("X-Cached") != "true" {
		t.Error("cached header 'X-Cached' missing")
	}
	if rr.Header().Get("Content-Type") != "application/json" {
		t.Error("cached header 'Content-Type' missing")
	}
}

// TestIdempotencyMiddleware_CacheGetError tests failure during cache.Get.
func TestIdempotencyMiddleware_CacheGetError(t *testing.T) {
	getCalled := false
	handlerCalled := false
	key := "test-key-get-err"

	mockCache := &mockCache{
		GetFunc: func(ctx context.Context, k string) ([]byte, bool, error) {
			getCalled = true
			return nil, false, errors.New("cache exploded") // Cache Get Error
		},
		SetFunc: func(ctx context.Context, k string, val []byte, ttl time.Duration) error {
			t.Fatal("Cache.Set should not be called")
			return nil
		},
	}

	mw := NewIdempotencyMiddleware(mockCache, noopLogger, time.Minute)
	handler := mw.Middleware(testHandler(&handlerCalled))

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set(IdempotencyKeyHeader, key)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !getCalled {
		t.Error("Cache.Get was not called")
	}
	if !handlerCalled {
		t.Error("handler was not called, but should have been (fallback)")
	}

	// Verify the *handler* response was written
	if rr.Code != http.StatusCreated {
		t.Errorf("wrong status code: got %d, want %d", rr.Code, http.StatusCreated)
	}
	if rr.Body.String() != "response from handler" {
		t.Errorf("wrong body: got %q", rr.Body.String())
	}
}

// TestIdempotencyMiddleware_CacheHit_UnmarshalError tests corrupt data in the cache.
func TestIdempotencyMiddleware_CacheHit_UnmarshalError(t *testing.T) {
	getCalled := false
	setCalled := false
	handlerCalled := false
	key := "test-key-corrupt"

	// This is corrupt JSON (or just not a cachedResponse struct)
	corruptData := []byte("this is not json")

	mockCache := &mockCache{
		GetFunc: func(ctx context.Context, k string) ([]byte, bool, error) {
			getCalled = true
			return corruptData, true, nil // Cache hit, but corrupt
		},
		SetFunc: func(ctx context.Context, k string, val []byte, ttl time.Duration) error {
			setCalled = true
			return nil
		},
	}

	mw := NewIdempotencyMiddleware(mockCache, noopLogger, time.Minute)
	handler := mw.Middleware(testHandler(&handlerCalled))

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set(IdempotencyKeyHeader, key)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !getCalled {
		t.Error("Cache.Get was not called")
	}
	if !handlerCalled {
		t.Error("handler was not called, but should have been (fallback)")
	}
	if !setCalled {
		t.Error("Cache.Set was not called, but should have been (to fix corrupt cache)")
	}

	// Verify the *handler* response was written
	if rr.Code != http.StatusCreated {
		t.Errorf("wrong status code: got %d, want %d", rr.Code, http.StatusCreated)
	}
	if rr.Body.String() != "response from handler" {
		t.Errorf("wrong body: got %q", rr.Body.String())
	}
}

// TestIdempotencyMiddleware_CacheMiss_HandlerError500 tests that 5xx responses are not cached.
func TestIdempotencyMiddleware_CacheMiss_HandlerError500(t *testing.T) {
	getCalled := false
	setCalled := false
	handlerCalled := false
	key := "test-key-500"

	mockCache := &mockCache{
		GetFunc: func(ctx context.Context, k string) ([]byte, bool, error) {
			getCalled = true
			return nil, false, nil // Cache miss
		},
		SetFunc: func(ctx context.Context, k string, val []byte, ttl time.Duration) error {
			setCalled = true
			t.Fatal("Cache.Set should not be called on 5xx response")
			return nil
		},
	}

	// Custom handler for this test
	handler500 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("oh no"))
	})

	mw := NewIdempotencyMiddleware(mockCache, noopLogger, time.Minute)
	handler := mw.Middleware(handler500)

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set(IdempotencyKeyHeader, key)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !getCalled {
		t.Error("Cache.Get was not called")
	}
	if !handlerCalled {
		t.Error("handler was not called")
	}
	if setCalled {
		t.Error("Cache.Set was called, but should not have been")
	}

	// Verify the 500 response was passed through
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("wrong status code: got %d, want %d", rr.Code, http.StatusInternalServerError)
	}
	if rr.Body.String() != "oh no" {
		t.Errorf("wrong body: got %q", rr.Body.String())
	}
}

// TestIdempotencyMiddleware_CacheMiss_SetError tests that a cache.Set failure doesn't fail the request.
func TestIdempotencyMiddleware_CacheMiss_SetError(t *testing.T) {
	getCalled := false
	setCalled := false
	handlerCalled := false
	key := "test-key-set-err"

	mockCache := &mockCache{
		GetFunc: func(ctx context.Context, k string) ([]byte, bool, error) {
			getCalled = true
			return nil, false, nil // Cache miss
		},
		SetFunc: func(ctx context.Context, k string, val []byte, ttl time.Duration) error {
			setCalled = true
			return errors.New("cache set failed") // Cache Set Error
		},
	}

	mw := NewIdempotencyMiddleware(mockCache, noopLogger, time.Minute)
	handler := mw.Middleware(testHandler(&handlerCalled))

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set(IdempotencyKeyHeader, key)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !getCalled {
		t.Error("Cache.Get was not called")
	}
	if !handlerCalled {
		t.Error("handler was not called")
	}
	if !setCalled {
		t.Error("Cache.Set was not called")
	}

	if rr.Code != http.StatusCreated {
		t.Errorf("wrong status code: got %d, want %d", rr.Code, http.StatusCreated)
	}
	if rr.Body.String() != "response from handler" {
		t.Errorf("wrong body: got %q", rr.Body.String())
	}
}

// This test is for the `cachedResponse` struct itself, ensuring json.Marshal
func TestCachedResponse_JSON_Marshal(t *testing.T) {
	resp := cachedResponse{
		StatusCode: 200,
		Headers:    http.Header{"X-Test": {"true"}},
		Body:       []byte("hello"),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Check that the body was base64 encoded
	// "hello" -> "aGVsbG8="
	expectedBodyB64 := base64.StdEncoding.EncodeToString([]byte("hello"))

	if !bytes.Contains(data, []byte(`"body":"`+expectedBodyB64+`"`)) {
		t.Errorf("marshaled JSON does not contain correct base64 body: got %s", data)
	}

	// Now test the round-trip
	var unmarshaledResp cachedResponse
	if err := json.Unmarshal(data, &unmarshaledResp); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaledResp.StatusCode != resp.StatusCode {
		t.Error("StatusCode mismatch")
	}
	if string(unmarshaledResp.Body) != string(resp.Body) {
		t.Error("Body mismatch")
	}
	if unmarshaledResp.Headers.Get("X-Test") != "true" {
		t.Error("Headers mismatch")
	}
}
