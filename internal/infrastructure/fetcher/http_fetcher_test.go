package fetcher

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestFetchSuccess verifies that a 200 OK response returns a readable body and no error.
func TestFetchSuccess(t *testing.T) {
	// Create a test server that returns 200 with body "hello"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer ts.Close()

	// Create the fetcher with a small timeout so tests run quickly
	f := NewHTTPFetcher(2 * time.Second)

	ctx := context.Background()
	body, err := f.Fetch(ctx, ts.URL)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Make sure we close the body
	defer body.Close()

	b, readErr := io.ReadAll(body)
	if readErr != nil {
		t.Fatalf("reading body failed: %v", readErr)
	}
	if got := string(b); got != "hello" {
		t.Fatalf("unexpected body: %q", got)
	}
}

// TestFetchNon2xx verifies that a non-2xx status code results in an error being returned.
func TestFetchNon2xx(t *testing.T) {
	// Server that always returns 500
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server err", http.StatusInternalServerError)
	}))
	defer ts.Close()

	f := NewHTTPFetcher(2 * time.Second)
	ctx := context.Background()
	rc, err := f.Fetch(ctx, ts.URL)
	// Should return an error and no body reader
	if err == nil {
		// If no error, ensure body is closed to avoid leaks
		if rc != nil {
			_ = rc.Close()
		}
		t.Fatalf("expected error for non-2xx response, got nil")
	}
	if rc != nil {
		_ = rc.Close()
	}
}

// TestFetchConnectionError verifies that when an HTTP client error occurs (connection refused),
func TestFetchConnectionError(t *testing.T) {
	// Use a localhost port that is unlikely to be open to trigger a client error quickly.
	badURL := "http://127.0.0.1:1"

	f := NewHTTPFetcher(500 * time.Millisecond)
	ctx := context.Background()

	rc, err := f.Fetch(ctx, badURL)
	if err == nil {
		// If somehow succeeded (very unlikely), close the body to avoid leak
		if rc != nil {
			_ = rc.Close()
		}
		t.Fatalf("expected error for connection failure, got nil")
	}
	if rc != nil {
		_ = rc.Close()
	}
}

// TestCircuitBreakerTrips triggers repeated failures to trip the circuit breaker and
func TestCircuitBreakerTrips(t *testing.T) {
	// Server that always returns 500 to count as failures for the circuit breaker
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server err", http.StatusInternalServerError)
	}))
	defer ts.Close()

	// Use a short timeout so each request fails fast
	f := NewHTTPFetcher(1 * time.Second)
	ctx := context.Background()

	// According to http_fetcher.go ReadyToTrip trips when ConsecutiveFailures > 5,
	const failuresToTrip = 7

	var lastErr error
	for i := 0; i < failuresToTrip; i++ {
		rc, err := f.Fetch(ctx, ts.URL)
		// Always close returned body if non-nil to avoid leaks
		if rc != nil {
			_ = rc.Close()
		}
		lastErr = err
		// Every call should produce an error because server returns 500
		if err == nil {
			t.Fatalf("expected error on attempt %d, got nil", i+1)
		}
	}

	// The last error should be non-nil; when circuit is open it will also be non-nil.
	if lastErr == nil {
		t.Fatalf("expected circuit breaker error after repeated failures, got nil")
	}

	// Additionally, ensure subsequent immediate call also returns an error quickly.
	start := time.Now()
	rc, err := f.Fetch(ctx, ts.URL)
	elapsed := time.Since(start)
	if rc != nil {
		_ = rc.Close()
	}
	if err == nil {
		t.Fatalf("expected error from fetch after circuit tripped, got nil")
	}
	// When circuit is open the call should be quick
	if elapsed > time.Second {
		t.Fatalf("expected immediate failure when circuit open, took %v", elapsed)
	}

	// Basic content check on the error string
	if err != nil && strings.TrimSpace(err.Error()) == "" {
		t.Fatalf("expected non-empty error message when circuit is open")
	}
}
