package main

import (
	"os"

	"webpage-analyzer-service/internal/analyzer"
	v1 "webpage-analyzer-service/internal/api/v1"
	"webpage-analyzer-service/internal/cache"
	"webpage-analyzer-service/internal/config"
	"webpage-analyzer-service/internal/idempotency"
	"webpage-analyzer-service/internal/logger"
	"webpage-analyzer-service/internal/queue"

	"github.com/gin-gonic/gin"
)

// This is the main entry point of the webpage analyzer API service.

func main() {
	// 1. GIN MODE: Force release mode to suppress debug logs in production
	gin.SetMode(gin.ReleaseMode)

	// 2. CONFIGURATION: Load from environment or config file
	cfg := config.Load()

	// 3. LOGGER: Structured logging
	log := logger.New()

	// 4. CACHING LAYER: Currently in-memory
	// next stage: Replace with Redis (distributed, TTL, pub/sub)
	cache := cache.NewInMemory()

	// 5. IDEMPOTENCY STORE: Prevent duplicate processing
	// next stage: Replace with Redis (SETNX + expiry)
	idempotency := idempotency.NewInMemoryStore()

	// 6. JOB QUEUE: Decouple analysis from HTTP request

	// Current: Mock queue → synchronous processing in handler
	// next stage: Replace with AWS SQS (or Kafka/RabbitMQ)
	queue := queue.NewMockQueue()

	// 7. ANALYZER: Core business logic
	// Depends only on interfaces → easy to mock/test
	// next stage: Add timeout, circuit breaker, rate limiting
	analyzer := analyzer.New(cache, queue)

	// 8. HTTP ROUTER & HANDLERS
	// Gin router with default middleware (recovery, logger)
	// next stage: Add rate limiting, request ID middleware, CORS
	router := gin.Default()

	// v1 API group – versioned, clean, extensible
	// Handler holds all dependencies via constructor injection
	handler := v1.NewHandler(analyzer, cache, idempotency, queue, log)

	// POST /api/v1/analyzes → Submit webpage for analysis
	// next stage: Return 202 Accepted
	router.POST("/api/v1/analyzes", handler.Analyze)

	// GET /health → Health check
	router.GET("/health", handler.Health)

	// 9. SERVER STARTUP
	// TODO ,
	addr := ":" + cfg.Port
	log.Info("server starting", "address", "http://localhost"+addr)

	// Future: Use http.Server with graceful shutdown
	if err := router.Run(addr); err != nil {
		log.Error("server failed", "error", err)
		os.Exit(1)
	}
}
