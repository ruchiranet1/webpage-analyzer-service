package auth

import (
	"context"
	"net/http"

	"webpage-analyzer-service/internal/domain"
)

// Service defines the interface for all authentication and authorization logic.
type Service interface {
	// GenerateToken creates a new authentication token (e.g., JWT) for a user.
	GenerateToken(ctx context.Context, userID string, email string) (string, *domain.AppError)

	// parses the token ("Bearer token" from Authorization header),
	ValidateRequest(ctx context.Context, r *http.Request) (userID string, err *domain.AppError)
}
