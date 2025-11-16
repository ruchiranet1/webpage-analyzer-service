package queue

import (
	"context"
	"log/slog"

	"webpage-analyzer-service/internal/domain"
)

// stubQueue used for local development and testing.
type stubQueue struct {
	logger *slog.Logger
	// This channel acts as the "message bus"
	jobQueue chan *domain.AnalysisRequest
}

// NewStubQueue creates a new in-memory stub queue.
func NewStubQueue(logger *slog.Logger) (Publisher, Subscriber) {
	// A buffered channel for 100 pending jobs
	q := &stubQueue{
		logger:   logger,
		jobQueue: make(chan *domain.AnalysisRequest, 100),
	}
	return q, q
}

// Publish implements the Publisher interface.
func (s *stubQueue) Publish(ctx context.Context, job *domain.AnalysisRequest) error {
	s.logger.DebugContext(ctx, "Publishing job to stub queue", "requestId", job.RequestID)
	select {
	case s.jobQueue <- job:
		// Job successfully queued
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// This should only happen if the buffered channel is full
		s.logger.ErrorContext(ctx, "Stub queue is full, dropping job", "requestId", job.RequestID)
		return nil // We'll just drop it
	}
}

// Subscribe implements the Subscriber interface.
func (s *stubQueue) Subscribe(ctx context.Context, jobs chan<- *domain.AnalysisRequest) error {
	s.logger.Info("Stub queue subscriber started")

	for {
		select {
		case <-ctx.Done():
			// Context was canceled (e.g., server shutdown)
			s.logger.Info("Stub queue subscriber shutting down")
			return ctx.Err()
		case job := <-s.jobQueue:
			// We received a job, send it to the worker
			s.logger.DebugContext(ctx, "Sending job to worker", "requestId", job.RequestID)
			jobs <- job
		}
	}
}
