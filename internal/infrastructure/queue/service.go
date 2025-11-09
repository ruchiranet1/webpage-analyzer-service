package queue

import (
	"context"

	"webpage-analyzer-service/internal/domain"
)

// Publisher defines the interface for publishing jobs to the queue.
type Publisher interface {
	Publish(ctx context.Context, job *domain.AnalysisRequest) error
}

// Subscriber defines the interface for subscribing to jobs from the queue.
type Subscriber interface {
	Subscribe(ctx context.Context, jobs chan<- *domain.AnalysisRequest) error
}
