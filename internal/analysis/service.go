package analysis

import (
	"context"
	"io"
	"net/url"

	"webpage-analyzer-service/internal/domain"
)

// The `infrastructure` layer will provide the "adapters" (implementations).
type PageFetcher interface {
	Fetch(ctx context.Context, url string) (io.ReadCloser, *domain.AppError)
}

// PageParser defines the contract for parsing the HTML content of a page.
type PageParser interface {
	// Parse takes a reader and the page's base URL (for resolving relative links).
	Parse(ctx context.Context, r io.Reader, baseURL *url.URL) (*domain.AnalysisResult, []string, error)
}

// LinkChecker defines the contract for checking the accessibility of links.
type LinkChecker interface {
	// Check takes a slice of link URLs and returns the count of *inaccessible* links.
	Check(ctx context.Context, links []string) (inaccessibleCount int)
}

// Service defines the central application use case: analyzing a web page.
type Service interface {
	// AnalyzePage is the primary business logic method.
	AnalyzePage(ctx context.Context, rawURL string) (*domain.AnalysisResult, *domain.AppError)
}
