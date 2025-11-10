package linkchecker

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"webpage-analyzer-service/internal/analysis"
)

// maxWorkers defines the number of concurrent goroutines in the worker pool.
const maxWorkers = 20

// linkChecker is the concrete implementation of analysis.LinkChecker.
type linkChecker struct {
	client *http.Client
	logger *slog.Logger
}

// NewLinkChecker creates a new concurrent link checker.
func NewLinkChecker(timeout time.Duration, logger *slog.Logger) analysis.LinkChecker {
	return &linkChecker{
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		logger: logger,
	}
}

// Check implements the analysis.LinkChecker interface.
func (lc *linkChecker) Check(ctx context.Context, links []string) int {
	if len(links) == 0 {
		return 0
	}

	var inaccessibleCount atomic.Int64

	// A WaitGroup to wait for all jobs to finish.
	var wg sync.WaitGroup

	// The `jobs` channel is buffered. This isn't strictly necessary
	jobs := make(chan string, len(links))

	// 1. Start the worker pool (CSP Pattern)
	numWorkers := min(maxWorkers, len(links))
	for w := 0; w < numWorkers; w++ {
		go lc.worker(ctx, &wg, jobs, &inaccessibleCount)
	}

	// 2. Add all links to the jobs channel
	wg.Add(len(links))
	for _, link := range links {
		jobs <- link
	}

	// 3. Close the jobs channel to signal workers there's no more work
	close(jobs)

	// 4. Wait for all goroutines to finish
	wg.Wait()

	return int(inaccessibleCount.Load())
}

// worker is a single goroutine that processes links from the jobs channel.
func (lc *linkChecker) worker(ctx context.Context, wg *sync.WaitGroup, jobs <-chan string, inaccessibleCount *atomic.Int64) {
	// Process jobs until the channel is closed and empty
	for link := range jobs {
		// Ensure WaitGroup is decremented even if the worker panics
		func() {
			defer wg.Done()

			// Check if the parent context (from the HTTP request) has been canceled
			select {
			case <-ctx.Done():
				// Don't even start the request if the context is canceled
				inaccessibleCount.Add(1)
				return
			default:
				// Continue
			}

			if !lc.isLinkAccessible(ctx, link) {
				inaccessibleCount.Add(1)
			}
		}()
	}
}

// isLinkAccessible returns `true` if the status is 2xx, `false` otherwise.
func (lc *linkChecker) isLinkAccessible(ctx context.Context, link string) bool {
	// Create request with context for cancellation
	req, err := http.NewRequestWithContext(ctx, "HEAD", link, nil)
	if err != nil {
		lc.logger.DebugContext(ctx, "Link check failed (request creation)", "link", link, "error", err)
		return false // Malformed URL
	}

	// Set a user-agent
	req.Header.Set("User-Agent", "web-page-analyzer-bot/1.0")

	resp, err := lc.client.Do(req)
	if err != nil {
		// This includes timeouts, connection refused, DNS errors, etc.
		lc.logger.DebugContext(ctx, "Link check failed (network error)", "link", link, "error", err)
		return false
	}
	defer resp.Body.Close()

	// consider any 2xx status code as "accessible".
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		lc.logger.DebugContext(ctx, "Link check failed (non-2xx status)", "link", link, "status", resp.StatusCode)
		return false
	}

	// This link is accessible
	return true
}
