package fetcher // This was 'linkchecker'

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"webpage-analyzer-service/internal/analysis"
	"webpage-analyzer-service/internal/domain"

	"github.com/sony/gobreaker"
)

// httpFetcher is the concrete implementation of the analysis.PageFetcher interface.
type httpFetcher struct {
	client *http.Client
	cb     *gobreaker.CircuitBreaker
}

// NewHTTPFetcher creates a new instance of httpFetcher.
func NewHTTPFetcher(timeout time.Duration) analysis.PageFetcher {
	client := &http.Client{
		Timeout: timeout,
	}

	// Configure the Circuit Breaker
	var st gobreaker.Settings
	st.Name = "http_fetcher"
	st.MaxRequests = 3            // Max half-open requests
	st.Interval = 1 * time.Minute // Clears counts every minute
	st.Timeout = 5 * time.Second  // When "open", moves to "half-open" after 5s
	// When to trip: 5 consecutive failures
	st.ReadyToTrip = func(counts gobreaker.Counts) bool {
		return counts.ConsecutiveFailures > 5
	}

	return &httpFetcher{
		client: client,
		cb:     gobreaker.NewCircuitBreaker(st),
	}
}

// Fetch implements the analysis.PageFetcher interface.
// It wraps the HTTP GET call within the Circuit Breaker.
func (f *httpFetcher) Fetch(ctx context.Context, url string) (io.ReadCloser, *domain.AppError) {
	// Execute the request via the Circuit Breaker
	body, err := f.cb.Execute(func() (interface{}, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			// This is an internal error, not a remote server error
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set a user-agent to mimic a real browser
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")

		resp, err := f.client.Do(req)
		if err != nil {
			// This could be a timeout, DNS error, or connection refused
			return nil, &fetchError{
				statusCode: http.StatusServiceUnavailable, // Treat network errors as 503
				msg:        fmt.Sprintf("Failed to fetch URL: %s", err.Error()),
				internal:   err,
			}
		}

		// This is critical for the "inaccessible links" requirement.
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// We successfully got a response, but it's an error status.
			resp.Body.Close() // Must close the body to prevent leaks
			return nil, &fetchError{
				statusCode: resp.StatusCode,
				msg:        fmt.Sprintf("Received non-2xx status code: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode)),
				internal:   fmt.Errorf("status code %d", resp.StatusCode),
			}
		}
		return resp.Body, nil
	})

	// Handle errors from the Circuit Breaker
	if err != nil {
		// Check if it's our custom fetchError
		if fe, ok := err.(*fetchError); ok {
			return nil, domain.NewAppError(fe.statusCode, fe.msg, fe.internal)
		}

		// Check if the circuit breaker is open
		if err == gobreaker.ErrOpenState {
			return nil, domain.NewAppError(http.StatusServiceUnavailable, "Service is temporarily unavailable (Circuit Breaker open)", err)
		}

		// Other circuit breaker or internal errors
		return nil, domain.NewAppError(http.StatusInternalServerError, "Internal server error during fetch", err)
	}

	// Type-assert the successful result
	return body.(io.ReadCloser), nil
}

// fetchError is a custom error type to shuttle HTTP status codes
type fetchError struct {
	statusCode int
	msg        string
	internal   error
}

func (fe *fetchError) Error() string {
	return fe.internal.Error()
}
