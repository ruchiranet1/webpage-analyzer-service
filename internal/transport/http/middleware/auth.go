package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"webpage-analyzer-service/internal/auth"
	"webpage-analyzer-service/internal/domain"
)

// to prevent key collisions in the context map.
type contextKey string

// to store the authenticated userID  in the request context.
const UserIDContextKey contextKey = "userID"

func Authenticate(authSvc auth.Service, logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Use the auth service to validate the request
			userID, appErr := authSvc.ValidateRequest(r.Context(), r)
			if appErr != nil {
				// If validation fails, write the error and stop the request
				logger.WarnContext(r.Context(), "Authentication failed", "error", appErr.InternalError(), "client_ip", r.RemoteAddr)
				writeErrorResponse(w, appErr)
				return
			}

			// 2. Authentication successful.
			// Inject the userID into the request context so downstream
			ctx := context.WithValue(r.Context(), UserIDContextKey, userID)

			// 3. Call the next handler in the chain with the modified context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeErrorResponse(w http.ResponseWriter, err *domain.AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.StatusCode)
	json.NewEncoder(w).Encode(err)
}
