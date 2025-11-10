package http

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"webpage-analyzer-service/internal/analysis"
	"webpage-analyzer-service/internal/constants"
	"webpage-analyzer-service/internal/domain"
	"webpage-analyzer-service/internal/infrastructure/queue"
)

// Handler holds all the dependencies required for our HTTP handlers.
type Handler struct {
	analysisSvc analysis.Service
	logger      *slog.Logger
	asyncPub    queue.Publisher // Now enabled
}

// NewHandler is the factory function for creating a new Handler.
func NewHandler(
	analysisSvc analysis.Service,
	logger *slog.Logger,
	asyncPub queue.Publisher,
) *Handler {
	return &Handler{
		analysisSvc: analysisSvc,
		logger:      logger,
		asyncPub:    asyncPub,
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
	if appErr := h.readJSON(w, r, &req); appErr != nil {
		h.writeError(w, appErr)
		return
	}

	// 2. Call the analysis service
	result, appErr := h.analysisSvc.AnalyzePage(ctx, req.URL)
	if appErr != nil {
		h.logger.ErrorContext(ctx, "Analysis service failed", "url", req.URL, "requestId", req.RequestID, "error", appErr.InternalError())
		h.writeError(w, appErr)
		return
	}

	// 3. Write the successful response
	h.writeJSON(w, http.StatusOK, result)
}

// HandleAnalyzeAsync handles the asynchronous page analysis request.
// POST /api/v1/analyzes/async
func (h *Handler) HandleAnalyzeAsync(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Decode the request payload
	var req domain.AnalysisRequest
	if appErr := h.readJSON(w, r, &req); appErr != nil {
		h.writeError(w, appErr)
		return
	}

	// 2. Publish the job to the message queue
	if err := h.asyncPub.Publish(ctx, &req); err != nil {
		h.logger.ErrorContext(ctx, "Failed to publish async job", "requestId", req.RequestID, "error", err)
		h.writeError(w, domain.NewInternalError(constants.ErrInternalServer, err))
		return
	}

	// 3. Write the "Accepted" response
	h.logger.InfoContext(ctx, "Async job accepted", "requestId", req.RequestID, "url", req.URL)
	h.writeJSON(w, http.StatusAccepted, map[string]string{
		"message":   "Analysis request accepted and is being processed.",
		"requestId": req.RequestID,
	})
}

// writeJSON is a helper for sending standardized JSON responses.
func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to write JSON response", "error", err)
	}
}

// writeError is a helper for sending standardized JSON error responses.
func (h *Handler) writeError(w http.ResponseWriter, err error) {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		h.writeJSON(w, appErr.StatusCode, appErr)
	} else {
		h.writeJSON(w, http.StatusInternalServerError, domain.NewInternalError(
			constants.ErrInternalServer,
			err,
		))
	}
}

// readJSON is a helper for decoding JSON request bodies safely.
func (h *Handler) readJSON(w http.ResponseWriter, r *http.Request, dst interface{}) *domain.AppError {
	// Use http.MaxBytesReader to prevent giant request bodies (DoS attack)
	maxBytes := 1_048_576 // 1MB
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return domain.NewAppError(http.StatusBadRequest, constants.ErrInvalidRequest, err)
	}
	// Check that the body wasn't empty
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return domain.NewAppErrorUser(http.StatusBadRequest,
			constants.ErrInvalidRequest.Errorf("Request body must only contain a single JSON object"))
	}

	return nil
}
