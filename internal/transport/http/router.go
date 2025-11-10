package http

import (
	"log/slog"
	"net/http"
	"net/http/pprof"
	"time"

	"webpage-analyzer-service/internal/auth"
	"webpage-analyzer-service/internal/infrastructure/cache"
	mw "webpage-analyzer-service/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

// RouterConfig holds all the dependencies needed to build the router.
type RouterConfig struct {
	Handler        *Handler
	AuthSvc        auth.Service
	CacheSvc       cache.Service
	Logger         *slog.Logger
	RateLimiter    *rate.Limiter
	IdempotencyTTL time.Duration
}

// NewRouter creates and configures the main HTTP router for the application.
func NewRouter(cfg *RouterConfig) *chi.Mux {
	r := chi.NewRouter()

	// --- Create Middleware Instances ---
	authMiddleware := mw.Authenticate(cfg.AuthSvc, cfg.Logger)
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

	// --- Observability & Health Routes ---
	// These routes are not part of our API but are crucial for operations.
	r.Get("/health", cfg.Handler.HandleHealth)

	// Mount the Prometheus metrics handler to expose /metrics
	r.Handle("/metrics", promhttp.Handler())

	// Mount the pprof profiling handlers with /debug/pprof/*
	r.Mount("/debug", pprofHandlers())

	// --- API v1 Routes ---
	r.Route("/api/v1", func(r chi.Router) {
		// Apply middleware for the v1 API group:
		r.Use(mw.Metrics)
		r.Use(rateLimitMiddleware)
		r.Use(authMiddleware)

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
	r.HandleFunc("/pprof", pprof.Index)
	r.HandleFunc("/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/pprof/profile", pprof.Profile)
	r.HandleFunc("/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/pprof/trace", pprof.Trace)
	r.Handle("/pprof/goroutine", pprof.Handler("goroutine"))
	r.Handle("/pprof/heap", pprof.Handler("heap"))
	r.Handle("/pprof/threadcreate", pprof.Handler("threadcreate"))
	r.Handle("/pprof/block", pprof.Handler("block"))
	return r
}
