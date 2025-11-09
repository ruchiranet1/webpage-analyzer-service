package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// responseLogger wraps http.ResponseWriter to capture the status code.
// This is a simplified recorder compared to the idempotency one,
// as we only need the status code for logging.
type responseLogger struct {
	http.ResponseWriter
	statusCode int
}

func newResponseLogger(w http.ResponseWriter) *responseLogger {
	return &responseLogger{w, http.StatusOK} // Default to 200
}

func (rl *responseLogger) WriteHeader(statusCode int) {
	rl.statusCode = statusCode
	rl.ResponseWriter.WriteHeader(statusCode)
}

// Logger is a middleware that logs details about each incoming request
// and its response using structured logging (slog).
func Logger(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := r.Context()

			// Log the incoming request
			logger.InfoContext(ctx, "Request received",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("user_agent", r.UserAgent()),
			)

			// Wrap the response writer to capture the status code
			rl := newResponseLogger(w)

			// Call the next handler in the chain
			next.ServeHTTP(rl, r)

			// Log the response
			logger.InfoContext(ctx, "Response sent",
				slog.Int("status_code", rl.statusCode),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Duration("latency", time.Since(start)),
			)
		})
	}
}
