package http

import (
	"log/slog"
	"net/http"
	"net/http/pprof"
	"time"

	"webpage-analyzer-service/internal/infrastructure/cache"
	mw "webpage-analyzer-service/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

// RouterConfig holds all the dependencies needed to build the router.
type RouterConfig struct {
	Handler *Handler
	//AuthSvc        auth.Service
	CacheSvc       cache.Service
	Logger         *slog.Logger
	RateLimiter    *rate.Limiter
	IdempotencyTTL time.Duration
}

// NewRouter creates and configures the main HTTP router for the application.
func NewRouter(cfg *RouterConfig) *chi.Mux {
	r := chi.NewRouter()

	// --- Create Middleware Instances ---
	//authMiddleware := mw.Authenticate(cfg.AuthSvc, cfg.Logger)
	logMiddleware := mw.Logger(cfg.Logger)
	rateLimitMiddleware := mw.RateLimit(cfg.RateLimiter, cfg.Logger)

	// The idempotency middleware is instantiated here
	idempotencyMiddleware := mw.NewIdempotencyMiddleware(
		cfg.CacheSvc,
		cfg.Logger,
		cfg.IdempotencyTTL,
	)

	// These apply to *every* request, including health checks and metrics.
	r.Use(logMiddleware)
	r.Use(mw.Metrics) // Apply metrics globally

	// --- Observability & Health Routes ---
	r.Get("/health", cfg.Handler.HandleHealth)

	// Mount the Prometheus metrics handler to expose /metrics
	r.Handle("/metrics", promhttp.Handler())

	// Mount the pprof profiling handlers
	r.Mount("/debug/pprof", pprofHandlers())

	// --- API v1 Routes ---
	r.Route("/api/v1", func(r chi.Router) {
		// Apply middleware for the v1 API group:
		r.Use(rateLimitMiddleware)
		//r.Use(authMiddleware)

		// Synchronous endpoint:
		r.With(idempotencyMiddleware.Middleware).Post("/analyzes", cfg.Handler.HandleAnalyzeSync)

		// Asynchronous endpoint:
		r.Post("/analyzes/async", cfg.Handler.HandleAnalyzeAsync)
	})

	return r
}

// pprofHandlers returns a sub-router for pprof endpoints.
func pprofHandlers() http.Handler {
	r := chi.NewRouter()
	r.HandleFunc("/", pprof.Index)
	r.HandleFunc("/cmdline", pprof.Cmdline)
	r.HandleFunc("/profile", pprof.Profile)
	r.HandleFunc("/symbol", pprof.Symbol)
	r.HandleFunc("/trace", pprof.Trace)
	r.Handle("/goroutine", pprof.Handler("goroutine"))
	r.Handle("/heap", pprof.Handler("heap"))
	r.Handle("/threadcreate", pprof.Handler("threadcreate"))
	r.Handle("/block", pprof.Handler("block"))
	r.Handle("/allocs", pprof.Handler("allocs"))
	return r
}
