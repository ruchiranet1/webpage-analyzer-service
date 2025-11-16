package http

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"webpage-analyzer-service/internal/constants"
	"webpage-analyzer-service/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockAnalysisService implements the analysis.Service interface
type MockAnalysisService struct {
	mock.Mock
}

func (m *MockAnalysisService) AnalyzePage(ctx context.Context, rawURL string) (*domain.AnalysisResult, *domain.AppError) {
	args := m.Called(ctx, rawURL)
	var res *domain.AnalysisResult
	if args.Get(0) != nil {
		res = args.Get(0).(*domain.AnalysisResult)
	}
	var appErr *domain.AppError
	if args.Get(1) != nil {
		appErr = args.Get(1).(*domain.AppError)
	}
	return res, appErr
}

// MockQueuePublisher implements the queue.Publisher interface
type MockQueuePublisher struct {
	mock.Mock
}

func (m *MockQueuePublisher) Publish(ctx context.Context, job *domain.AnalysisRequest) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

// testSetup creates a Handler instance with mocked dependencies.
func testSetup() (*Handler, *MockAnalysisService, *MockQueuePublisher, *bytes.Buffer) {
	mockAnalysis := new(MockAnalysisService)
	mockQueue := new(MockQueuePublisher)

	// Buffer to capture logs for assertion (structured logs are complex strings)
	logBuffer := new(bytes.Buffer)
	testLogger := slog.New(slog.NewJSONHandler(logBuffer, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler := NewHandler(mockAnalysis, testLogger, mockQueue)
	return handler, mockAnalysis, mockQueue, logBuffer
}

func TestHandler_HandleHealth(t *testing.T) {
	handler, _, _, _ := testSetup()
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	handler.HandleHealth(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rr.Body.String())
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
}

func TestHandler_readJSON_Success(t *testing.T) {
	handler, _, _, _ := testSetup()
	body := strings.NewReader(`{"requestId": "1", "url": "https://test.com"}`)
	req := httptest.NewRequest("POST", "/", body)
	rr := httptest.NewRecorder()
	var reqStruct domain.AnalysisRequest

	appErr := handler.readJSON(rr, req, &reqStruct)

	assert.Nil(t, appErr)
	assert.Equal(t, "1", reqStruct.RequestID)
}

func TestHandler_readJSON_InvalidJSON(t *testing.T) {
	handler, _, _, _ := testSetup()
	body := strings.NewReader(`{"requestId": "1"`) // Missing closing brace
	req := httptest.NewRequest("POST", "/", body)
	rr := httptest.NewRecorder()
	var reqStruct domain.AnalysisRequest

	appErr := handler.readJSON(rr, req, &reqStruct)

	require.NotNil(t, appErr)
	assert.Equal(t, http.StatusBadRequest, appErr.StatusCode)
	assert.Equal(t, constants.ErrInvalidRequest.Code, appErr.ErrorCode)
}

func TestHandler_readJSON_ExtraFields(t *testing.T) {
	handler, _, _, _ := testSetup()
	// Invalid due to DisallowUnknownFields()
	body := strings.NewReader(`{"requestId": "1", "extra": "field"}`)
	req := httptest.NewRequest("POST", "/", body)
	rr := httptest.NewRecorder()
	var reqStruct domain.AnalysisRequest

	appErr := handler.readJSON(rr, req, &reqStruct)

	require.NotNil(t, appErr)
	assert.Equal(t, http.StatusBadRequest, appErr.StatusCode)
	assert.Equal(t, constants.ErrInvalidRequest.Code, appErr.ErrorCode)
}

func TestHandler_readJSON_MultipleObjects(t *testing.T) {
	handler, _, _, _ := testSetup()
	body := strings.NewReader(`{"r": 1}{"r": 2}`) // Two objects
	req := httptest.NewRequest("POST", "/", body)
	rr := httptest.NewRecorder()
	var reqStruct domain.AnalysisRequest

	appErr := handler.readJSON(rr, req, &reqStruct)

	require.NotNil(t, appErr)
	assert.Equal(t, http.StatusBadRequest, appErr.StatusCode)
	assert.Equal(t, constants.ErrInvalidRequest.Code, appErr.ErrorCode)
	assert.Contains(t, appErr.Message, constants.ErrInvalidRequest.Message)
}

// --- Tests for Sync Handler ---
func TestHandler_AnalyzeSync_Success(t *testing.T) {
	handler, mockAnalysis, _, _ := testSetup()
	testURL := "https://example.com"
	testResult := &domain.AnalysisResult{Title: "Test"}

	// 1. Mock the request body (Decode must pass)
	body := strings.NewReader(`{"requestId": "sync-1", "url": "` + testURL + `"}`)
	req := httptest.NewRequest("POST", "/analyzes", body)
	rr := httptest.NewRecorder()

	// 2. Mock the analysis service to return success
	mockAnalysis.On("AnalyzePage", mock.Anything, testURL).Return(testResult, nil).Once()

	// 3. Execute
	handler.HandleAnalyzeSync(rr, req)

	// 4. Assert
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"title":"Test"`)
	mockAnalysis.AssertExpectations(t)
}

func TestHandler_AnalyzeSync_ServiceFailure(t *testing.T) {
	handler, mockAnalysis, _, logBuffer := testSetup()
	testURL := "https://example.com"
	internalErr := errors.New("network timeout")

	// 1. Mock the request body
	body := strings.NewReader(`{"requestId": "sync-fail", "url": "` + testURL + `"}`)
	req := httptest.NewRequest("POST", "/analyzes", body)
	rr := httptest.NewRecorder()

	// 2. Mock the analysis service to return a structured error
	appErr := domain.NewAppError(http.StatusServiceUnavailable, constants.ErrURLFetch, internalErr)
	mockAnalysis.On("AnalyzePage", mock.Anything, testURL).Return(nil, appErr).Once()

	// 3. Execute
	handler.HandleAnalyzeSync(rr, req)

	// 4. Assert
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Contains(t, rr.Body.String(), `"error_code":"WPA30001"`)

	// Assert that the internal error was logged
	assert.Contains(t, logBuffer.String(), `"error":"network timeout"`)
	mockAnalysis.AssertExpectations(t)
}

// --- Tests for Async Handler ---
func TestHandler_AnalyzeAsync_Success(t *testing.T) {
	handler, _, mockQueue, logBuffer := testSetup()
	testURL := "https://example.com/async"
	testID := "async-2"

	// 1. Mock the request body
	body := strings.NewReader(`{"requestId": "` + testID + `", "url": "` + testURL + `"}`)
	req := httptest.NewRequest("POST", "/analyzes/async", body)
	rr := httptest.NewRecorder()

	// 2. Mock the queue publisher to accept the job
	mockQueue.On("Publish", mock.Anything, mock.AnythingOfType("*domain.AnalysisRequest")).Return(nil).Once()

	// 3. Execute
	handler.HandleAnalyzeAsync(rr, req)

	// 4. Assert
	assert.Equal(t, http.StatusAccepted, rr.Code)
	assert.JSONEq(t, `{"message":"Analysis request accepted and is being processed.","requestId":"async-2"}`, rr.Body.String())
	assert.Contains(t, logBuffer.String(), "Async job accepted")
	mockQueue.AssertExpectations(t)
}

func TestHandler_AnalyzeAsync_QueueFailure(t *testing.T) {
	handler, _, mockQueue, logBuffer := testSetup()
	testURL := "https://example.com/async"
	internalErr := errors.New("queue full")

	// 1. Mock the request body
	body := strings.NewReader(`{"requestId": "async-fail", "url": "` + testURL + `"}`)
	req := httptest.NewRequest("POST", "/analyzes/async", body)
	rr := httptest.NewRecorder()

	// 2. Mock the queue publisher to return an error
	mockQueue.On("Publish", mock.Anything, mock.AnythingOfType("*domain.AnalysisRequest")).Return(internalErr).Once()

	// 3. Execute
	handler.HandleAnalyzeAsync(rr, req)

	// 4. Assert
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), `"error_code":"WPA99999"`)

	// Assert that the internal error was logged
	assert.Contains(t, logBuffer.String(), "Failed to publish async job")
	assert.Contains(t, logBuffer.String(), `"error":"queue full"`)
	mockQueue.AssertExpectations(t)
}

func TestHandler_writeError_UnknownInternalError(t *testing.T) {
	handler, _, _, _ := testSetup() // No longer need rrLogs
	rr := httptest.NewRecorder()

	// A standard, unwrapped internal error
	unknownErr := errors.New("database connection failed")

	handler.writeError(rr, unknownErr)

	// Assert HTTP response
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), `"error_code":"WPA99999"`)
	assert.Contains(t, rr.Body.String(), `"message":"An internal server error occurred"`)

}
