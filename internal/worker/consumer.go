package worker

import (
	"context"
	"log/slog"
	"time"

	"webpage-analyzer-service/internal/analysis"
	"webpage-analyzer-service/internal/domain"
	"webpage-analyzer-service/internal/infrastructure/queue"
)

// Consumer subscribes to the queue and processes jobs using the analysis.Service.
type Consumer struct {
	subscriber  queue.Subscriber
	analysisSvc analysis.Service
	logger      *slog.Logger
}

// NewConsumer creates a new worker consumer.
func NewConsumer(
	subscriber queue.Subscriber,
	analysisSvc analysis.Service,
	logger *slog.Logger,
) *Consumer {
	return &Consumer{
		subscriber:  subscriber,
		analysisSvc: analysisSvc,
		logger:      logger,
	}
}

// Start begins the worker's job processing loop.
func (c *Consumer) Start(ctx context.Context) {
	// Create a channel for the subscriber to send jobs to
	jobs := make(chan *domain.AnalysisRequest)

	// Start the subscriber in its own goroutine
	// It will block and listen for jobs, sending them to our `jobs` channel
	go func() {
		if err := c.subscriber.Subscribe(ctx, jobs); err != nil {
			c.logger.ErrorContext(ctx, "Queue subscriber failed", "error", err)
		}
	}()

	c.logger.Info("Worker consumer started, waiting for jobs...")

	// Start the processing loop
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Worker consumer shutting down")
			return
		case job := <-jobs:
			// We received a job, process it
			c.processJob(ctx, job)
		}
	}
}

// processJob handles a single analysis request.
func (c *Consumer) processJob(ctx context.Context, job *domain.AnalysisRequest) {
	// Create a new context for this specific job, with a timeout
	// This prevents one long-running job from holding up the worker.
	jobCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Add job details to the logger for this context
	jobLogger := c.logger.With(
		slog.String("requestId", job.RequestID),
		slog.String("url", job.URL),
		slog.String("email", job.Email),
	)
	jobLogger.InfoContext(jobCtx, "Processing async job")

	// Call the *exact same* analysis service as the HTTP handler
	result, appErr := c.analysisSvc.AnalyzePage(jobCtx, job.URL)
	if appErr != nil {
		// Job failed
		jobLogger.ErrorContext(jobCtx, "Async job failed", "error", appErr.InternalError())
		return
	}

	// Job succeeded
	jobLogger.InfoContext(jobCtx, "Async job complete", "title", result.Title)
	// 1. Save the `result` to a database.
	// 2. Send a notification email to `job.Email`.
}
