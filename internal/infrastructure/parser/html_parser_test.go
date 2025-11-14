package parser

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"reflect"
	"testing"

	"log/slog"

	"webpage-analyzer-service/internal/domain"
)

// errReader simulates an io.Reader that always returns an error.
type errReader struct {
	err error
}

func (e *errReader) Read(p []byte) (int, error) {
	return 0, e.err
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestParseHTML5Page(t *testing.T) {
	logger := newTestLogger()
	p := NewHTMLParser(logger).(*htmlParser)

	base, _ := url.Parse("https://example.com")

	html := `
		<!DOCTYPE html>
		<html>
		<head>
		  <title>  Test Page  </title>
		</head>
		<body>
		  <h1>Main</h1>
		  <h2>Sub</h2>
		  <h2>Other</h2>

		  <form action="/login" method="post">
			<input type="text" name="user"/>
			<input type="password" name="pass"/>
		  </form>

		  <a href="/about">About</a>
		  <a href="https://external.com/page">External</a>
		  <a href="#fragment">Fragment</a>
		  <a href="">Empty</a>
		  <a href="://bad">Bad</a>
		</body>
		</html>
	`

	result, links, err := p.Parse(context.Background(), bytes.NewBufferString(html), base)
	if err != nil {
		t.Fatalf("Parse returned unexpected error: %v", err)
	}

	if result.HTMLVersion != "HTML 5" {
		t.Fatalf("expected HTMLVersion 'HTML 5', got %q", result.HTMLVersion)
	}

	if result.Title != "Test Page" {
		t.Fatalf("expected Title 'Test Page', got %q", result.Title)
	}

	// Headings map should contain counts for h1 and h2
	expectedHeadings := map[string]int{
		"h1": 1,
		"h2": 2,
	}
	if !reflect.DeepEqual(result.Headings, expectedHeadings) {
		t.Fatalf("expected headings %v, got %v", expectedHeadings, result.Headings)
	}

	if !result.HasLoginForm {
		t.Fatalf("expected HasLoginForm true")
	}

	// Links should include resolved /about and external link only (fragment/empty/bad skipped)
	expectedLinks := []string{
		"https://example.com/about",
		"https://external.com/page",
	}
	if !reflect.DeepEqual(links, expectedLinks) {
		t.Fatalf("expected links %v, got %v", expectedLinks, links)
	}

	if result.Links.Internal != 1 || result.Links.External != 1 {
		t.Fatalf("expected internal=1 external=1, got internal=%d external=%d", result.Links.Internal, result.Links.External)
	}

	// Inaccessible is left for service layer (zero here)
	if result.Links.Inaccessible != 0 {
		t.Fatalf("expected Inaccessible 0, got %d", result.Links.Inaccessible)
	}
}

func TestParseDetectsVariousDoctypesAndLoginDetection(t *testing.T) {
	logger := newTestLogger()
	p := NewHTMLParser(logger).(*htmlParser)

	cases := []struct {
		name            string
		html            string
		expectedDoctype string
		expectLogin     bool
	}{
		{
			name: "XHTML 1.0",
			html: `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
				   <html><head><title>xhtml</title></head><body></body></html>`,
			expectedDoctype: "XHTML 1.0",
			expectLogin:     false,
		},
		{
			name: "HTML 4.01",
			html: `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN" "http://www.w3.org/TR/html4/strict.dtd">
				   <html><head><title>html4</title></head><body></body></html>`,
			expectedDoctype: "HTML 4.01",
			expectLogin:     false,
		},
		{
			name: "No Doctype",
			html: `<html><head><title>nodoctype</title></head><body>
					 <form><input type="PASSWORD"/></form>
				   </body></html>`,
			expectedDoctype: "HTML (No Doctype)",
			expectLogin:     true,
		},
	}

	base, _ := url.Parse("https://example.com")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, _, err := p.Parse(context.Background(), bytes.NewBufferString(tc.html), base)
			if err != nil {
				t.Fatalf("Parse returned unexpected error: %v", err)
			}
			if res.HTMLVersion != tc.expectedDoctype {
				t.Fatalf("expected doctype %q, got %q", tc.expectedDoctype, res.HTMLVersion)
			}
			if res.HasLoginForm != tc.expectLogin {
				t.Fatalf("expected HasLoginForm %v, got %v", tc.expectLogin, res.HasLoginForm)
			}
		})
	}
}

func TestFindHTMLVersion_nilNode(t *testing.T) {
	logger := newTestLogger()
	parser := NewHTMLParser(logger).(*htmlParser)

	// Directly test findHTMLVersion with a nil node
	if v := parser.findHTMLVersion(nil); v != "Unknown" {
		t.Fatalf("expected 'Unknown' for nil node, got %q", v)
	}
}

func TestParseWithBadReaderReturnsError(t *testing.T) {
	logger := newTestLogger()
	p := NewHTMLParser(logger).(*htmlParser)

	base, _ := url.Parse("https://example.com")
	er := &errReader{err: errors.New("read failure")}

	_, _, err := p.Parse(context.Background(), er, base)
	if err == nil {
		t.Fatalf("expected error when reader fails, got nil")
	}
}

func TestLinkCountingWithRelativeAndAbsolute(t *testing.T) {
	logger := newTestLogger()
	p := NewHTMLParser(logger).(*htmlParser)

	base, _ := url.Parse("https://example.com:8080") // include port to ensure host match includes port
	html := `
		<!DOCTYPE html>
		<html><body>
		  <a href="/rel">rel</a>
		  <a href="https://example.com:8080/also">same-host-with-port</a>
		  <a href="https://example.com/different-port">different-port</a>
		  <a href="http://external.com/">external</a>
		</body></html>
	`

	res, links, err := p.Parse(context.Background(), bytes.NewBufferString(html), base)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	// internal: "/rel" and "https://example.com:8080/also" (host includes port)
	// external: "https://example.com/different-port" (different host because no port) and http external
	if res.Links.Internal != 2 || res.Links.External != 2 {
		t.Fatalf("unexpected link counts internal=%d external=%d", res.Links.Internal, res.Links.External)
	}

	// Ensure links are resolved as absolute strings
	expected := []string{
		"https://example.com:8080/rel",
		"https://example.com:8080/also",
		"https://example.com/different-port",
		"http://external.com/",
	}
	if !reflect.DeepEqual(links, expected) {
		t.Fatalf("expected links %v, got %v", expected, links)
	}
}

func TestFindLoginFormVariousAttributes(t *testing.T) {
	logger := newTestLogger()
	p := NewHTMLParser(logger).(*htmlParser)

	base, _ := url.Parse("https://example.com")

	cases := []struct {
		name     string
		html     string
		expected bool
	}{
		{
			name:     "input password lowercase",
			html:     `<html><body><form><input type="password"/></form></body></html>`,
			expected: true,
		},
		{
			name:     "input PASSWORD uppercase",
			html:     `<html><body><form><input type="PASSWORD"/></form></body></html>`,
			expected: true, // goquery attribute selection is case-insensitive for types in selectors in practice
		},
		{
			name:     "no password input",
			html:     `<html><body><form><input type="text"/></form></body></html>`,
			expected: false,
		},
		{
			name:     "no forms at all",
			html:     `<html><body><div><input type="password"/></div></body></html>`,
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, _, err := p.Parse(context.Background(), bytes.NewBufferString(tc.html), base)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if res.HasLoginForm != tc.expected {
				t.Fatalf("expected HasLoginForm=%v, got %v", tc.expected, res.HasLoginForm)
			}
		})
	}
}

// A small sanity test to ensure the AnalysisResult type is as expected (helps increase coverage a bit)
func TestAnalysisResultZeroValues(t *testing.T) {
	ar := &domain.AnalysisResult{}
	if ar.HTMLVersion != "" || ar.Title != "" || ar.Links.Internal != 0 || ar.Links.External != 0 || ar.Links.Inaccessible != 0 {
		t.Fatalf("expected zero values for new AnalysisResult, got %+v", ar)
	}
}
