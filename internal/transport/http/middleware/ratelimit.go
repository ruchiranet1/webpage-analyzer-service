package middleware

import (
	"log/slog"
	"net/http"

	"golang.org/x/time/rate"
)

// RateLimit is the middleware that enforces a global rate limit on incoming requests.
// It uses the "golang.org/x/time/rate" package which implements the
// "Token Bucket" algorithm.
func RateLimit(limiter *rate.Limiter, logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the request is allowed.
			// Allow() is a non-blocking check.
			if !limiter.Allow() {
				// Request is not allowed (bucket is empty)
				logger.WarnContext(r.Context(), "Rate limit exceeded", "remote_addr", r.RemoteAddr)

				// We can't use our standard writeErrorResponse here easily
				// because it's in the 'auth.go' file.
				// This indicates we should move helpers like
				// writeErrorResponse to a shared 'transport/http/response' package.
				// For now, we will write the error directly.
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
