package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"log/slog"

	"golang.org/x/time/rate"
)

// Test that when the limiter allows the request, the next handler is executed.
func TestRateLimit_AllowsNextHandler(t *testing.T) {
	limiter := rate.NewLimiter(rate.Inf, 10) // generous limiter so Allow() returns true
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))

	mw := RateLimit(limiter, logger)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Called", "true")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d; got %d", http.StatusOK, rr.Code)
	}
	if got := rr.Header().Get("X-Called"); got != "true" {
		t.Fatalf("expected next handler to be called and set header X-Called=true; got %q", got)
	}
	if body := rr.Body.String(); body != "ok" {
		t.Fatalf("unexpected body: %q", body)
	}
}

// Test that when the limiter is exhausted the middleware returns 429 and does not call next.
func TestRateLimit_DeniesWhenExceeded(t *testing.T) {
	// limiter that has an initial single token; second immediate request should be denied
	limiter := rate.NewLimiter(rate.Limit(1), 1)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))

	mw := RateLimit(limiter, logger)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If this is called on the second request, the test should fail.
		t.Fatalf("next handler should not be called when rate limit is exceeded")
	})

	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.RemoteAddr = "5.6.7.8:4321"

	// First request consumes the single token and should be allowed.
	rr1 := httptest.NewRecorder()
	handlerAllower := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("first"))
	}))
	handlerAllower.ServeHTTP(rr1, req)
	if rr1.Code != http.StatusOK {
		t.Fatalf("expected first request to be allowed with status %d; got %d", http.StatusOK, rr1.Code)
	}
	if rr1.Body.String() != "first" {
		t.Fatalf("unexpected first body: %q", rr1.Body.String())
	}

	// Second request should be denied.
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req)
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d; got %d", http.StatusTooManyRequests, rr2.Code)
	}
	if ct := rr2.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json; got %q", ct)
	}
	expectedBody := `{"code": 429, "message": "Too Many Requests"}`
	if body := rr2.Body.String(); body != expectedBody {
		t.Fatalf("unexpected body for denied request: %q (want %q)", body, expectedBody)
	}
}
