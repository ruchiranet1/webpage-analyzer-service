package analyzer

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"webpage-analyzer-service/internal/cache"
	"webpage-analyzer-service/internal/queue"

	"golang.org/x/net/html"
)

type Job struct {
	RequestID string
	UserID    string
	Channel   string
	Email     string
	URL       string
}

// orchestrates webpage analysis with caching and async support.
// next stage : Add metrics, circuit breaker, rate limiting
type Analyzer struct {
	cache cache.Cache
	queue queue.Queue
}

func New(c cache.Cache, q queue.Queue) *Analyzer {
	return &Analyzer{cache: c, queue: q}
}

func (a *Analyzer) Analyze(ctx context.Context, job Job) (*AnalysisResult, error) {
	// 1. Cache lookup
	if cached, ok := a.cache.Get(job.URL); ok {
		return cached.(*AnalysisResult), nil
	}

	// 2. HTTP fetch with timeout
	// TODO read the values from config.
	body, _, err := fetchWithTimeout(ctx, job.URL, 30*time.Second)
	if err != nil {
		return nil, err
	}

	// 3. HTML parse
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	// 4. Metadata extraction
	result := &AnalysisResult{
		HTMLVersion:  detectHTMLVersion(doc),
		PageTitle:    getTitle(doc),
		Headings:     countHeadings(doc),
		HasLoginForm: hasLoginForm(doc),
	}

	// 5. Concurrent link checking
	links := extractLinks(doc, job.URL)
	result.Links.Internal, result.Links.External, result.Links.Inaccessible = checkLinksConcurrently(ctx, links)

	// 6. Cache store
	a.cache.Set(job.URL, result, 10*time.Minute)
	return result, nil
}

// TODO: Add retry, exponential backoff
func fetchWithTimeout(ctx context.Context, urlStr string, timeout time.Duration) (string, int, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return string(body), resp.StatusCode, nil
}
