package linkchecker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// errRoundTripper simulates network-level errors by always returning an error.
type errRoundTripper struct {
	err error
}

func (e errRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, e.err
}

func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCheck_NoLinks(t *testing.T) {
	lc := &linkChecker{
		client: http.DefaultClient,
		logger: newDiscardLogger(),
	}

	if got := lc.Check(context.Background(), []string{}); got != 0 {
		t.Fatalf("expected 0 for no links, got %d", got)
	}
}

func TestIsLinkAccessible_OKAndNotFound(t *testing.T) {
	// Server returns 200 for /ok, 404 for anything else.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	lc := &linkChecker{
		client: server.Client(),
		logger: newDiscardLogger(),
	}

	ctx := context.Background()
	if !lc.isLinkAccessible(ctx, server.URL+"/ok") {
		t.Fatalf("expected /ok to be accessible")
	}
	if lc.isLinkAccessible(ctx, server.URL+"/nope") {
		t.Fatalf("expected /nope to be inaccessible")
	}
}

func TestCheck_CountsInaccessible(t *testing.T) {
	// Server returns 200 for /a, 500 for /b, 204 for /c
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			w.WriteHeader(http.StatusOK)
		case "/b":
			w.WriteHeader(http.StatusInternalServerError)
		case "/c":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	lc := &linkChecker{
		client: server.Client(),
		logger: newDiscardLogger(),
	}

	links := []string{
		server.URL + "/a", // accessible (200)
		server.URL + "/b", // inaccessible (500)
		server.URL + "/c", // accessible (204)
		server.URL + "/d", // inaccessible (404)
	}

	got := lc.Check(context.Background(), links)
	want := 2 // /b and /d are inaccessible

	if got != want {
		t.Fatalf("expected %d inaccessible links, got %d", want, got)
	}
}

func TestCheck_NetworkErrorCountsAllAsInaccessible(t *testing.T) {
	lc := &linkChecker{
		client: &http.Client{
			Transport: errRoundTripper{err: errors.New("network failure")},
			Timeout:   200 * time.Millisecond,
		},
		logger: newDiscardLogger(),
	}

	links := []string{
		"http://example.local/1",
		"http://example.local/2",
		"http://example.local/3",
	}

	got := lc.Check(context.Background(), links)
	if got != len(links) {
		t.Fatalf("expected all links (%d) to be counted inaccessible on network error, got %d", len(links), got)
	}
}

func TestCheck_CanceledContextCountsAllAsInaccessible(t *testing.T) {
	// Use a server that would otherwise be OK to ensure that the context cancellation
	// is what causes the inaccessible counts.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// simulate some work if it were to run
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	lc := &linkChecker{
		client: server.Client(),
		logger: newDiscardLogger(),
	}

	// Cancel the context before calling Check. Workers should detect ctx.Done()
	// and increment inaccessible counter without attempting requests.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	links := []string{
		server.URL + "/a",
		server.URL + "/b",
		server.URL + "/c",
	}

	got := lc.Check(ctx, links)
	if got != len(links) {
		t.Fatalf("expected all links (%d) to be counted inaccessible for canceled context, got %d", len(links), got)
	}
}

func TestIsLinkAccessible_MalformedURL(t *testing.T) {
	lc := &linkChecker{
		client: http.DefaultClient,
		logger: newDiscardLogger(),
	}

	// Malformed URL should cause NewRequestWithContext to fail and return false.
	malformed := "http://%"
	if lc.isLinkAccessible(context.Background(), malformed) {
		t.Fatalf("expected malformed URL to be considered inaccessible")
	}
}
