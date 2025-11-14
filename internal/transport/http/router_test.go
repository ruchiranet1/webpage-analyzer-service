package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"log/slog"

	"golang.org/x/time/rate"
)

// tests to ensure the router registers the expected endpoints.
func TestNewRouter_HealthMetricsPprof(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	cfg := &RouterConfig{
		Handler:        &Handler{},
		AuthSvc:        nil,
		CacheSvc:       nil,
		Logger:         logger,
		RateLimiter:    rate.NewLimiter(rate.Inf, 0),
		IdempotencyTTL: time.Minute,
	}

	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	// /health
	res, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /health, got %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(body), "ok") {
		t.Fatalf("/health body does not contain ok: %s", string(body))
	}

	// /metrics
	res, err = http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics failed: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /metrics, got %d", res.StatusCode)
	}

	// /debug/pprof/
	res, err = http.Get(ts.URL + "/debug/pprof/")
	if err != nil {
		t.Fatalf("GET /debug/pprof/ failed: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /debug/pprof/, got %d", res.StatusCode)
	}
}

func TestPprofHandlers_Subpaths(t *testing.T) {
	r := pprofHandlers()

	paths := []string{
		"/",
		"/cmdline",
		"/goroutine",
		"/heap",
		"/allocs",
	}

	for _, p := range paths {
		req := httptest.NewRequest("GET", p, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("pprof handler for %s returned %d, want %d", p, w.Code, http.StatusOK)
		}
	}
}
