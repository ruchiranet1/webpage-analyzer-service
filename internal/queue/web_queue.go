package queue

import "context"

type Queue interface {
	Push(ctx context.Context, job Job) error
	Pop(ctx context.Context) (Job, error)
}

type Job struct {
	RequestID string
	URL       string
	Email     string
}
