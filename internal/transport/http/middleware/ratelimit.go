package middleware

import (
	"log/slog"
	"net/http"

	"golang.org/x/time/rate"
)

// RateLimit is the middleware that enforces a global rate limit on incoming requests.
func RateLimit(limiter *rate.Limiter, logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the request is allowed.
			// Allow() is a non-blocking check.
			if !limiter.Allow() {
				// Request is not allowed (bucket is empty)
				logger.WarnContext(r.Context(), "Rate limit exceeded", "remote_addr", r.RemoteAddr)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"code": 429, "message": "Too Many Requests"}`))
				return
			}

			// Request is allowed, proceed to the next handler
			next.ServeHTTP(w, r)
		})
	}
}
