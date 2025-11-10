package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"webpage-analyzer-service/internal/infrastructure/cache"
)

// IdempotencyKeyHeader is the standard HTTP header used for idempotency.
const IdempotencyKeyHeader = "Idempotency-Key"

// cachedResponse is the struct we will serialize and store in the cache.
type cachedResponse struct {
	StatusCode int         `json:"status_code"`
	Headers    http.Header `json:"headers"`
	Body       []byte      `json:"body"`
}

// responseRecorder is a wrapper for http.ResponseWriter to capture the response.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

// NewResponseRecorder creates a new recorder.
func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // Default
		body:           new(bytes.Buffer),
	}
}

// WriteHeader captures the status code.
func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response body.
func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// IdempotencyMiddleware provides a middleware for handling idempotent requests.
type IdempotencyMiddleware struct {
	cache  cache.Service
	logger *slog.Logger
	ttl    time.Duration
}

// NewIdempotencyMiddleware is the factory function.
func NewIdempotencyMiddleware(c cache.Service, l *slog.Logger, ttl time.Duration) *IdempotencyMiddleware {
	return &IdempotencyMiddleware{
		cache:  c,
		logger: l,
		ttl:    ttl,
	}
}

// Middleware returns the http.Handler func that implements the pattern.
func (m *IdempotencyMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// 1. Get the idempotency key from the header.
		key := r.Header.Get(IdempotencyKeyHeader)
		if key == "" {
			m.logger.WarnContext(ctx, "Idempotency-Key header missing", "path", r.URL.Path)
			next.ServeHTTP(w, r)
			return
		}

		// 2. Check the cache for this key
		val, found, err := m.cache.Get(ctx, key)
		if err != nil {
			// If cache fails, log and pass through. Don't fail the request.
			m.logger.ErrorContext(ctx, "Cache GET failed for idempotency", "key", key, "error", err)
			next.ServeHTTP(w, r)
			return
		}

		// 3. Cache HIT: We found a response.
		if found {
			m.logger.DebugContext(ctx, "Idempotency cache HIT", "key", key)
			var resp cachedResponse
			if err := json.Unmarshal(val, &resp); err != nil {
				// Cached data is corrupt. Log and treat as a cache miss.
				m.logger.ErrorContext(ctx, "Failed to unmarshal cached response", "key", key, "error", err)
				// Fallthrough to execute the request again
			} else {
				// Replay the cached response
				for k, v := range resp.Headers {
					w.Header()[k] = v
				}
				w.WriteHeader(resp.StatusCode)
				w.Write(resp.Body)
				return
			}
		}

		// 4. Cache MISS: We've never seen this key.
		m.logger.DebugContext(ctx, "Idempotency cache MISS", "key", key)

		// Create our response recorder to capture the *next* handler's response
		recorder := newResponseRecorder(w)

		// Call the next handler in the chain (e.g., our main API handler)
		next.ServeHTTP(recorder, r)

		// 5. After the handler, cache the response
		if recorder.statusCode < 500 {
			resp := cachedResponse{
				StatusCode: recorder.statusCode,
				Headers:    recorder.Header(),
				Body:       recorder.body.Bytes(),
			}

			// Serialize the response
			data, err := json.Marshal(resp)
			if err != nil {
				m.logger.ErrorContext(ctx, "Failed to marshal response for cache", "key", key, "error", err)
				// Don't fail, just couldn't cache
				return
			}

			// *** THIS IS THE KEY ***
			if err := m.cache.Set(ctx, key, data, m.ttl); err != nil {
				m.logger.ErrorContext(ctx, "Cache SET failed for idempotency", "key", key, "error", err)
			}
		}
	})
}
