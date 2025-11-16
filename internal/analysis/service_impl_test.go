package analysis

import (
	"context"
	"errors"
	"io"
	"net/url"
	"strings"
	"testing"

	"log/slog"

	"webpage-analyzer-service/internal/constants"
	"webpage-analyzer-service/internal/domain"
)

// --- Mocks ---

type mockFetcher struct {
	body   io.ReadCloser
	appErr *domain.AppError
	called bool
	last   string
}

func (m *mockFetcher) Fetch(ctx context.Context, u string) (io.ReadCloser, *domain.AppError) {
	m.called = true
	m.last = u
	return m.body, m.appErr
}

type mockParser struct {
	result *domain.AnalysisResult
	links  []string
	err    error

	called   bool
	lastBase *url.URL
}

func (m *mockParser) Parse(ctx context.Context, r io.Reader, baseURL *url.URL) (*domain.AnalysisResult, []string, error) {
	m.called = true
	m.lastBase = baseURL
	if r != nil {
		// consume reader to mimic real parser behavior
		io.ReadAll(r)
	}
	return m.result, m.links, m.err
}

type mockChecker struct {
	count  int
	called bool
	last   []string
}

func (m *mockChecker) Check(ctx context.Context, links []string) int {
	m.called = true
	m.last = links
	return m.count
}

// compile-time assertions that mocks implement interfaces used by service_impl.go
var (
	_ PageFetcher = (*mockFetcher)(nil)
	_ PageParser  = (*mockParser)(nil)
	_ LinkChecker = (*mockChecker)(nil)
)

// --- Tests ---

func Test_normalizeURL_variants(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := &serviceImpl{
		fetcher: nil,
		parser:  nil,
		checker: nil,
		logger:  logger,
	}

	tests := []struct {
		name    string
		raw     string
		wantNil bool // whether returned URL should be nil and error non-nil
	}{
		{"empty", "", true},
		{"missing scheme", "example.com", true},
		{"invalid format", "http://%zz", true},
		{"host missing", "http://", true},
		{"valid https", "https://example.com/path", false},
		{"valid http", "http://example.com", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			u, appErr := svc.normalizeURL(tt.raw)
			if tt.wantNil {
				if u != nil {
					t.Fatalf("expected nil URL for input %q, got %v", tt.raw, u)
				}
				if appErr == nil {
					t.Fatalf("expected an error for input %q, got nil", tt.raw)
				}
			} else {
				if appErr != nil {
					t.Fatalf("did not expect an error for input %q, got %v", tt.raw, appErr)
				}
				if u == nil {
					t.Fatalf("expected non-nil URL for input %q, got nil", tt.raw)
				}
				// basic sanity check: scheme preserved
				if tt.raw != "" && !strings.HasPrefix(tt.raw, u.Scheme+":") {
					t.Fatalf("scheme mismatch: %q vs %q", tt.raw, u.Scheme)
				}
			}
		})
	}
}

func Test_AnalyzePage_FetchError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// fetcher returns an AppError to simulate failure
	fetcher := &mockFetcher{
		body:   nil,
		appErr: domain.NewAppErrorUser(500, constants.ErrURLInvalidFormat),
	}
	parser := &mockParser{}
	checker := &mockChecker{}

	svc := NewService(fetcher, parser, checker, logger)

	ctx := context.Background()
	result, appErr := svc.AnalyzePage(ctx, "https://example.com")
	if result != nil {
		t.Fatalf("expected nil result when fetch fails, got %#v", result)
	}
	if appErr == nil {
		t.Fatal("expected an app error when fetch fails, got nil")
	}
	// ensure fetcher was called with normalized URL
	if !fetcher.called {
		t.Fatal("expected fetcher to be called")
	}
}

func Test_AnalyzePage_ParseError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	body := io.NopCloser(strings.NewReader("<html></html>"))
	fetcher := &mockFetcher{
		body:   body,
		appErr: nil,
	}
	parser := &mockParser{
		result: nil,
		links:  nil,
		err:    errors.New("parse failed"),
	}
	checker := &mockChecker{}

	svc := NewService(fetcher, parser, checker, logger)

	ctx := context.Background()
	result, appErr := svc.AnalyzePage(ctx, "https://example.com")
	if result != nil {
		t.Fatalf("expected nil result when parse fails, got %#v", result)
	}
	if appErr == nil {
		t.Fatal("expected an app error when parse fails, got nil")
	}
	// ensure parser was called
	if !parser.called {
		t.Fatal("expected parser to be called")
	}
}

func Test_AnalyzePage_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Prepare a simple AnalysisResult.
	initialResult := &domain.AnalysisResult{
		Title: "Test Title",
		Links: domain.LinkCounts{Inaccessible: 0},
	}

	body := io.NopCloser(strings.NewReader("<html><a href=\"/foo\">link</a></html>"))

	fetcher := &mockFetcher{
		body:   body,
		appErr: nil,
	}

	parser := &mockParser{
		result: initialResult,
		links:  []string{"https://example.com/foo"},
		err:    nil,
	}

	checker := &mockChecker{
		count: 5,
	}

	svc := NewService(fetcher, parser, checker, logger)

	ctx := context.Background()
	result, appErr := svc.AnalyzePage(ctx, "https://example.com")
	if appErr != nil {
		t.Fatalf("did not expect error for successful run, got %v", appErr)
	}
	if result == nil {
		t.Fatal("expected non-nil result for successful run")
	}

	// title should be preserved
	if result.Title != "Test Title" {
		t.Fatalf("expected title %q, got %q", "Test Title", result.Title)
	}

	// Links.Inaccessible should be set by the service from the checker result
	if result.Links.Inaccessible != 5 {
		t.Fatalf("expected Links.Inaccessible to be 5, got %d", result.Links.Inaccessible)
	}

	// Ensure checker saw the links emitted by parser
	if !checker.called {
		t.Fatal("expected checker to be called")
	}
	if len(checker.last) != 1 || checker.last[0] != "https://example.com/foo" {
		t.Fatalf("checker saw unexpected links: %#v", checker.last)
	}
}
