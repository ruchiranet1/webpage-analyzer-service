package http

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"webpage-analyzer-service/internal/analysis"
	"webpage-analyzer-service/internal/domain"
	"webpage-analyzer-service/internal/infrastructure/queue" // Now imported
	"webpage-analyzer-service/internal/infrastructure/validation"
	// We will need a queue publisher interface for the async handler
	// "webpage-analyzer-service/internal/infrastructure/queue"
)

// Handler holds all the dependencies required for our HTTP handlers.
// This is an application of the Dependency Injection (DI) pattern.
type Handler struct {
	analysisSvc analysis.Service
	validator   validation.Validator
	logger      *slog.Logger
	asyncPub    queue.Publisher // Now enabled
}

// NewHandler is the factory function for creating a new Handler.
func NewHandler(
	analysisSvc analysis.Service,
	validator validation.Validator,
	logger *slog.Logger,
	asyncPub queue.Publisher, // Now enabled
) *Handler {
	return &Handler{
		analysisSvc: analysisSvc,
		validator:   validator,
		logger:      logger,
		asyncPub:    asyncPub, // Now enabled
	}
}

// HandleHealth is a simple health check endpoint.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	// A real health check might also ping its database or other dependencies.
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleAnalyzeSync handles the synchronous page analysis request.
// POST /api/v1/analyzes
func (h *Handler) HandleAnalyzeSync(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Decode the request payload
	var req domain.AnalysisRequest
	if err := h.readJSON(w, r, &req); err != nil {
		h.writeError(w, err)
		return
	}

	// 2. Validate the URL from the payload
	if appErr := h.validator.ValidateURL(ctx, req.URL); appErr != nil {
		h.logger.WarnContext(ctx, "Invalid URL in request", "url", req.URL, "requestId", req.RequestID)
		h.writeError(w, appErr)
		return
	}

	// 3. Call the analysis service
	// This is the core "use case" call.
	result, appErr := h.analysisSvc.AnalyzePage(ctx, req.URL)
	if appErr != nil {
		h.logger.ErrorContext(ctx, "Analysis service failed", "url", req.URL, "requestId", req.RequestID, "error", appErr.InternalError())
		h.writeError(w, appErr)
		return
	}

	// 4. Write the successful response
	h.writeJSON(w, http.StatusOK, result)
}

// HandleAnalyzeAsync handles the asynchronous page analysis request.
// POST /api/v1/analyzes/async
func (h *Handler) HandleAnalyzeAsync(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Decode the request payload
	var req domain.AnalysisRequest
	if err := h.readJSON(w, r, &req); err != nil {
		h.writeError(w, err)
		return
	}

	// 2. Validate the URL
	if appErr := h.validator.ValidateURL(ctx, req.URL); appErr != nil {
		h.logger.WarnContext(ctx, "Invalid URL in async request", "url", req.URL, "requestId", req.RequestID)
		h.writeError(w, appErr)
		return
	}

	// 3. Publish the job to the message queue
	// (This part is commented out as we haven't built the queue yet)
	if err := h.asyncPub.Publish(ctx, &req); err != nil {
		h.logger.ErrorContext(ctx, "Failed to publish async job", "requestId", req.RequestID, "error", err)
		h.writeError(w, domain.NewAppError(500, "Failed to schedule analysis", err))
		return
	}

	// 4. Write the "Accepted" response
	h.logger.InfoContext(ctx, "Async job accepted", "requestId", req.RequestID, "url", req.URL)
	h.writeJSON(w, http.StatusAccepted, map[string]string{
		"message":   "Analysis request accepted and is being processed.",
		"requestId": req.RequestID,
	})
}

// --- HTTP Helper Functions ---

// writeJSON is a helper for sending standardized JSON responses.
func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// This is tricky: we can't write an error *after* the header is sent.
		// We just log it.
		h.logger.Error("Failed to write JSON response", "error", err)
	}
}

// writeError is a helper for sending standardized JSON error responses.
func (h *Handler) writeError(w http.ResponseWriter, err error) {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		// This is a known, application-level error
		h.writeJSON(w, appErr.Code, appErr)
	} else {
		// This is an unknown, internal error (like a JSON parsing error)
		h.writeJSON(w, http.StatusInternalServerError, domain.NewAppError(
			http.StatusInternalServerError,
			"An internal server error occurred",
			err,
		))
	}
}

// readJSON is a helper for decoding JSON request bodies safely.
func (h *Handler) readJSON(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	// Use http.MaxBytesReader to prevent giant request bodies (DoS attack)
	maxBytes := 1_048_576 // 1MB
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // Strict parsing

	if err := dec.Decode(dst); err != nil {
		// ... (error handling for various JSON errors) ...
		return domain.NewAppError(http.StatusBadRequest, "Invalid JSON request body", err)
	}

	// Check that the body wasn't empty
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return domain.NewAppError(http.StatusBadRequest, "Request body must only contain a single JSON object", nil)
	}

	return nil
}
