package fetcher // This was 'linkchecker'

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"webpage-analyzer-service/internal/analysis"
	"webpage-analyzer-service/internal/constants"
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
			errDef := constants.ErrURLFetch.Errorf(err.Error())
			return nil, domain.NewAppError(http.StatusServiceUnavailable, errDef, err)
		}

		// Got the success response, but link is not accessible.
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			errMsg := fmt.Sprintf("Received non-2xx status code: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
			errDef := constants.ErrURLFetch.Errorf(errMsg)
			return nil, domain.NewAppError(resp.StatusCode, errDef, fmt.Errorf("status code %d", resp.StatusCode))
		}
		return resp.Body, nil
	})

	// Handle errors from the Circuit Breaker
	if err != nil {
		// Check if the circuit breaker is open
		if err == gobreaker.ErrOpenState {
			return nil, domain.NewAppError(http.StatusServiceUnavailable, constants.ErrCircuitBreakerOpen, err)
		}

		// Check if the circuit breaker is open
		var appErr *domain.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}

		// wrap it as a generic internal error
		return nil, domain.NewInternalError(constants.ErrInternalServer, err)
	}
	return body.(io.ReadCloser), nil
}
