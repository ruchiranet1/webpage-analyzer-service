package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/time/rate"

	"github.com/rs/cors"

	"webpage-analyzer-service/internal/analysis"
	"webpage-analyzer-service/internal/auth"
	"webpage-analyzer-service/internal/config"
	"webpage-analyzer-service/internal/constants"
	"webpage-analyzer-service/internal/infrastructure/cache"
	"webpage-analyzer-service/internal/infrastructure/fetcher"
	"webpage-analyzer-service/internal/infrastructure/linkchecker"
	"webpage-analyzer-service/internal/infrastructure/logging"
	"webpage-analyzer-service/internal/infrastructure/parser"
	"webpage-analyzer-service/internal/infrastructure/queue"
	httptransport "webpage-analyzer-service/internal/transport/http"
	"webpage-analyzer-service/internal/worker"
)

// This is the main entrypoint of the application.
func main() {
	// Use a background context for the application's lifecycle
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// run() contains the main application logic.
	if err := run(ctx, stop); err != nil {
		fmt.Fprintf(os.Stderr, "Application error: %v\n", err)
		os.Exit(1)
	}
}

// run is the main application function.
func run(ctx context.Context, stop context.CancelFunc) error {
	// 1. --- Bootstrap Logging ---
	bootstrapLogger := logging.NewLogger("info") // Default to "info"
	slog.SetDefault(bootstrapLogger)
	bootstrapLogger.Info("Bootstrapping application...")

	// 2. --- Configuration ---
	bootstrapLogger.Info("Loading configuration...")
	cfg, err := config.Load()
	if err != nil {
		// Use correct slog format
		bootstrapLogger.Error(constants.MsgFailedToLoadConfig, "error", err)
		return fmt.Errorf("%s: %w", constants.MsgFailedToLoadConfig, err)
	}
	logger := logging.NewLogger(cfg.Logger.Level)
	slog.SetDefault(logger)
	logger.Info("Configuration loaded. Starting application...", "log_level", cfg.Logger.Level)

	// 3. Rate Limiter
	limiter := rate.NewLimiter(rate.Limit(cfg.RateLimiter.RPS), cfg.RateLimiter.Burst)

	// Infrastructure: Fetcher, Parser, LinkChecker
	pageFetcher := fetcher.NewHTTPFetcher(cfg.Services.FetcherTimeout)
	pageParser := parser.NewHTMLParser(logger.With("component", "parser"))
	linkChecker := linkchecker.NewLinkChecker(cfg.Services.LinkCheckerTimeout, logger.With("component", "linkchecker"))

	// Infrastructure: Caching (for Idempotency)
	cacheSvc := cache.NewInMemoryCache()

	// Infrastructure: Auth Service
	authSvc, err := auth.NewJWTService(cfg.JWT.Secret, cfg.JWT.TokenTTL)

	if err != nil {
		logger.Error(constants.MsgFailedToCreateAuthSvc, constants.Error, err)
		return fmt.Errorf("%s: %w", constants.MsgFailedToCreateAuthSvc, err)
	}

	// Infrastructure: Queue (for Async API - next stage)
	queuePub, queueSub := queue.NewStubQueue(logger.With("component", "queue"))

	// Application Core (The "Use Case" Layer) ---
	logger.Debug("Initializing application services...")
	analysisSvc := analysis.NewService(
		pageFetcher,
		pageParser,
		linkChecker,
		logger.With("component", "analysis_service"),
	)

	// 	Transport Layer (HTTP)
	logger.Debug("Initializing HTTP transport...")
	// 	Create the Handler (injects services)
	httpHandler := httptransport.NewHandler(
		analysisSvc,
		logger.With("component", "http_handler"),
		queuePub,
	)

	// Create the Router
	routerConfig := &httptransport.RouterConfig{
		Handler:        httpHandler,
		AuthSvc:        authSvc,
		CacheSvc:       cacheSvc,
		Logger:         logger.With("component", "http_router"),
		RateLimiter:    limiter,
		IdempotencyTTL: cfg.Middleware.IdempotencyTTL,
	}
	router := httptransport.NewRouter(routerConfig)

	// --- CORS Handling ---
	c := cors.New(cors.Options{
		// TODO: Temporarily allowing all origins
		AllowedOrigins:     []string{"*"},
		AllowedMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:     []string{"Authorization", "Content-Type", "Idempotency-Key"},
		AllowCredentials:   true,
		OptionsPassthrough: false,
		Debug:              cfg.Logger.Level == "debug",
	})

	// Wrap the existing router with the CORS middleware
	corsRouter := c.Handler(router)

	// Create the HTTP Server
	srv := &http.Server{
		Addr:    ":" + fmt.Sprint(cfg.HTTP.Port),
		Handler: corsRouter, // Use the CORS-wrapped router
		// Add standard timeouts, TODO need to add to the config.
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// --- Worker Layer (Async) ---
	logger.Debug("Initializing async worker...")
	consumer := worker.NewConsumer(
		queueSub,
		analysisSvc,
		logger.With("component", "worker"),
	)

	// Use a WaitGroup to wait for all services to shut down
	var wg sync.WaitGroup

	// Start the worker consumer
	wg.Add(1)
	go func() {
		defer wg.Done()
		consumer.Start(ctx)
		logger.Info("Worker has shut down.")
	}()

	// Start the HTTP server
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info(fmt.Sprintf("HTTP server listening on :%d", cfg.HTTP.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(constants.MsgFailedToHttpServer, constants.Error, err)
			stop()
		}
		logger.Info("HTTP server has shut down.")
	}()

	// 	--- Graceful Shutdown ---
	// Block here until context is canceled (e.g., SIGINT)
	<-ctx.Done()
	logger.Info("Shutdown signal received. Shutting down services...")

	// Create a new context for shutdown with a timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer shutdownCancel()

	// Shutdown the HTTP server
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error(constants.MsgFailedToStopHttpServer, constants.Error, err)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	logger.Info("All services shut down gracefully.")
	return nil
}
