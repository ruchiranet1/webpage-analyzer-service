package middleware

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"webpage-analyzer-service/internal/constants"
	"webpage-analyzer-service/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAuthService implements the auth.Service interface
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) GenerateToken(ctx context.Context, userID, email string) (string, *domain.AppError) {
	args := m.Called(ctx, userID, email)
	// Note: This method is not used in the middleware but must be defined for the interface
	var appErr *domain.AppError
	if args.Get(1) != nil {
		appErr = args.Get(1).(*domain.AppError)
	}
	return args.String(0), appErr
}

func (m *MockAuthService) ValidateRequest(ctx context.Context, r *http.Request) (string, *domain.AppError) {
	args := m.Called(ctx, r)
	var appErr *domain.AppError
	if args.Get(1) != nil {
		appErr = args.Get(1).(*domain.AppError)
	}
	return args.String(0), appErr
}

// MockNextHandler ensures the middleware calls the next handler in the chain.
type MockNextHandler struct {
	mock.Mock
}

func (m *MockNextHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.Called(w, r)
}

// testSetup creates mocks and the final middleware chain for testing.
func testSetup() (*MockAuthService, *MockNextHandler, http.Handler) {
	mockAuth := new(MockAuthService)
	mockNext := new(MockNextHandler)

	// Create a silent logger for tests (writes to io.Discard)
	testLogger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	mw := Authenticate(mockAuth, testLogger)

	chain := mw(mockNext)

	return mockAuth, mockNext, chain
}

// --- Tests ---

func TestAuthMiddleware_Success(t *testing.T) {
	mockAuth, mockNext, chain := testSetup()

	// 1. Setup Auth Mock: Success
	expectedUserID := "user-123"
	mockAuth.On("ValidateRequest", mock.Anything, mock.Anything).Return(expectedUserID, nil).Once()

	// 2. Setup Next Mock: Ensure it's called
	mockNext.On("ServeHTTP", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		// 3. Assertion: Check if the UserID was injected into the context
		r := args.Get(1).(*http.Request)
		userID := r.Context().Value(UserIDContextKey)
		assert.Equal(t, expectedUserID, userID, "UserID must be injected into the context on success")
	}).Once()

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	// 4. Execute
	chain.ServeHTTP(rr, req)

	// 5. Verify
	mockAuth.AssertExpectations(t)
	mockNext.AssertExpectations(t)
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status OK since the handler was called")
}

func TestAuthMiddleware_Failure_TokenExpired(t *testing.T) {
	mockAuth, mockNext, chain := testSetup()

	// 1. Setup Auth Mock: Failure (Expired Token)
	// Use the correct NewAppError constructor with the ErrorDefinition
	authErr := domain.NewAppError(http.StatusUnauthorized, constants.ErrTokenExpired, errors.New("token exp"))
	mockAuth.On("ValidateRequest", mock.Anything, mock.Anything).Return("", authErr).Once()

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	// 2. Execute
	chain.ServeHTTP(rr, req)

	// 3. Verify
	mockAuth.AssertExpectations(t)
	mockNext.AssertNotCalled(t, "ServeHTTP", mock.Anything, mock.Anything) // Next handler MUST NOT be called

	// 4. Assert Response
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	assert.Contains(t, rr.Body.String(), constants.ErrTokenExpired.Code, "Response must contain the correct error code")
	assert.Contains(t, rr.Body.String(), constants.ErrTokenExpired.Message, "Response must contain the correct message")
}

func TestAuthMiddleware_Failure_InvalidFormat(t *testing.T) {
	mockAuth, mockNext, chain := testSetup()

	// 1. Setup Auth Mock: Failure (Header Format Error)
	authErr := domain.NewAppErrorUser(http.StatusUnauthorized, constants.ErrAuthHeaderFormat)
	mockAuth.On("ValidateRequest", mock.Anything, mock.Anything).Return("", authErr).Once()

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	// 2. Execute
	chain.ServeHTTP(rr, req)

	// 3. Verify
	mockAuth.AssertExpectations(t)
	mockNext.AssertNotCalled(t, "ServeHTTP", mock.Anything, mock.Anything) // Next handler MUST NOT be called

	// 4. Assert Response
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), constants.ErrAuthHeaderFormat.Code, "Response must contain the correct error code")
}
