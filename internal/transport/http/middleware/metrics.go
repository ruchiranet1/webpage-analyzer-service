package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// metricsResponseWriter wraps http.ResponseWriter to capture the status code
// for our metrics.
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newMetricsResponseWriter(w http.ResponseWriter) *metricsResponseWriter {
	return &metricsResponseWriter{w, http.StatusOK}
}

func (mrw *metricsResponseWriter) WriteHeader(statusCode int) {
	mrw.statusCode = statusCode
	mrw.ResponseWriter.WriteHeader(statusCode)
}

// --- Prometheus Metrics Definitions ---
// We use promauto to automatically register the metrics with the default registry.
// This is an application of the Observer pattern.

var (
	// httpRequestsTotal counts the total number of HTTP requests.
	// It has labels for status code, method, and path.
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "web_analyzer_http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"code", "method", "path"},
	)

	// httpRequestDuration observes the duration of HTTP requests.
	// It uses a histogram to track request latencies.
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "web_analyzer_http_request_duration_seconds",
			Help:    "Histogram of HTTP request latencies in seconds.",
			Buckets: prometheus.DefBuckets, // Default buckets: .005s to 10s
		},
		[]string{"path"},
	)
)

// Metrics is the middleware handler that records Prometheus metrics.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the response writer to capture status code
		mrw := newMetricsResponseWriter(w)

		// Call the next handler in the chain
		next.ServeHTTP(mrw, r)

		// Record the metrics after the handler has finished
		duration := time.Since(start)
		path := r.URL.Path
		statusCode := strconv.Itoa(mrw.statusCode)
		method := r.Method

		// Observe the request duration
		httpRequestDuration.WithLabelValues(path).Observe(duration.Seconds())

		// Increment the request counter
		httpRequestsTotal.WithLabelValues(statusCode, method, path).Inc()
	})
}
