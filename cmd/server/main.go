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

	"webpage-analyzer-service/internal/analysis"
	"webpage-analyzer-service/internal/auth"
	"webpage-analyzer-service/internal/config"
	"webpage-analyzer-service/internal/infrastructure/cache"
	"webpage-analyzer-service/internal/infrastructure/fetcher"
	"webpage-analyzer-service/internal/infrastructure/linkchecker"
	"webpage-analyzer-service/internal/infrastructure/logging"
	"webpage-analyzer-service/internal/infrastructure/parser"
	"webpage-analyzer-service/internal/infrastructure/queue"
	"webpage-analyzer-service/internal/infrastructure/validation"
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
	// 1. --- Configuration ---
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// 2.Logging
	logger := logging.NewLogger(cfg.LoggerLevel)
	slog.SetDefault(logger)
	logger.Info("Configuration loaded. Starting application...")

	// 3. Rate Limiter
	limiter := rate.NewLimiter(rate.Limit(cfg.RateLimiter.RPS), cfg.RateLimiter.Burst)

	// Validation
	validator, err := validation.NewURLValidator()
	if err != nil {
		return fmt.Errorf("failed to create url validator: %w", err)
	}

	// Infrastructure: Fetcher, Parser, LinkChecker
	pageFetcher := fetcher.NewHTTPFetcher(cfg.Fetcher.Timeout)
	pageParser := parser.NewHTMLParser(logger.With("component", "parser"))
	linkChecker := linkchecker.NewLinkChecker(cfg.LinkChecker.Timeout, logger.With("component", "linkchecker"))

	// Infrastructure: Caching (for Idempotency)
	cacheSvc := cache.NewInMemoryCache()

	// Infrastructure: Auth Service
	authSvc, err := auth.NewJWTService(cfg.JWT.Secret, cfg.JWT.TokenTTL)

	if err != nil {
		return fmt.Errorf("failed to create auth service: %w", err)
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

	//  Transport Layer (HTTP)
	logger.Debug("Initializing HTTP transport...")
	//  Create the Handler (injects services)
	httpHandler := httptransport.NewHandler(
		analysisSvc,
		validator,
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
		IdempotencyTTL: cfg.IdempotencyTTL,
	}
	router := httptransport.NewRouter(routerConfig)

	// Create the HTTP Server
	srv := &http.Server{
		Addr:    ":" + cfg.HTTPServer.Port,
		Handler: router,
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
		consumer.Start(ctx) // This will block until context is canceled
		logger.Info("Worker has shut down.")
	}()

	// Start the HTTP server
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info(fmt.Sprintf("HTTP server listening on :%s", cfg.HTTPServer.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server error", "error", err)
			stop()
		}
		logger.Info("HTTP server has shut down.")
	}()

	//  --- Graceful Shutdown ---
	// Block here until context is canceled (e.g., SIGINT)
	<-ctx.Done()
	logger.Info("Shutdown signal received. Shutting down services...")

	// Create a new context for shutdown with a timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.HTTPServer.ShutdownTimeout)
	defer shutdownCancel()

	// Shutdown the HTTP server
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server graceful shutdown failed", "error", err)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	logger.Info("All services shut down gracefully.")
	return nil
}
