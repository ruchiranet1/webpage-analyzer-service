package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"log/slog"
	"webpage-analyzer-service/internal/analyzer"
	"webpage-analyzer-service/internal/cache"
	"webpage-analyzer-service/internal/idempotency"
	"webpage-analyzer-service/internal/queue"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// setupTestHandler returns the handler and the **concrete** mock types.
func setupTestHandler() (*Handler, *cache.MockCache, *idempotency.MockStore, *queue.MockQueue) {
	mockCache := cache.NewMockCache()
	mockIdempotency := idempotency.NewMockStore()
	mockQueue := queue.NewMockQueue()
	concreteQueue := mockQueue.(*queue.MockQueue)

	mockAnalyzer := analyzer.New(mockCache, mockQueue)
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	return NewHandler(mockAnalyzer, mockCache, mockIdempotency, mockQueue, logger),
		mockCache, mockIdempotency, concreteQueue
}

func TestAnalyze_CacheHit(t *testing.T) {
	h, mockCache, _, _ := setupTestHandler()

	// Pre-populate cache
	result := &analyzer.AnalysisResult{HTMLVersion: "HTML5"}
	mockCache.Set("https://example.com", result, time.Minute)

	// Full valid request with ALL required fields
	req := AnalyzeRequest{
		RequestID: "req-123",
		UserID:    "user-1",
		Channel:   "web",
		Email:     "test@example.com",
		URL:       "https://example.com",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/analyzes", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Analyze(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp analyzer.AnalysisResult
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "HTML5", resp.HTMLVersion)
}

func TestAnalyze_Idempotency_Duplicate(t *testing.T) {
	h, _, mockIdempotency, _ := setupTestHandler()

	// Pre-store result
	result := &analyzer.AnalysisResult{HTMLVersion: "HTML5"}
	mockIdempotency.Set("req-123", result)

	req := AnalyzeRequest{
		RequestID: "req-123",
		UserID:    "user-1",
		Channel:   "web",
		Email:     "test@example.com",
		URL:       "https://example.com",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Analyze(c)

	assert.Equal(t, http.StatusConflict, w.Code)
	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.Equal(t, "wpa1100", errResp.ErrorCode)
}

func TestAnalyze_InvalidURL(t *testing.T) {
	h, _, _, _ := setupTestHandler()

	req := AnalyzeRequest{
		RequestID: "req1",
		UserID:    "user1",
		Channel:   "web",
		Email:     "test@example.com",
		URL:       "not-a-url",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Analyze(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.Equal(t, "wpa1201", errResp.ErrorCode)
}

func TestHealth(t *testing.T) {
	h, _, _, _ := setupTestHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/health", nil)

	h.Health(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status":"ok"}`, w.Body.String())
}
