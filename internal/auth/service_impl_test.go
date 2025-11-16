package auth

import (
	"context"
	"net/http"
	"testing"
	"time"

	"webpage-analyzer-service/internal/constants"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/suite"
)

// AuthServiceSuite defines the test suite for the auth service
type AuthServiceSuite struct {
	suite.Suite
	service  Service
	secret   string
	tokenTTL time.Duration
}

// SetupTest initializes the service for each test
func (s *AuthServiceSuite) SetupTest() {
	s.secret = "a-very-strong-and-secret-key-for-testing"
	s.tokenTTL = time.Hour * 1

	var err error
	s.service, err = NewJWTService(s.secret, s.tokenTTL)

	s.Require().NoError(err, "Failed to create auth service for test")
	s.Require().NotNil(s.service, "Auth service is nil after creation")
}

// TestAuthService runs the test suite
func TestAuthService(t *testing.T) {
	suite.Run(t, new(AuthServiceSuite))
}

// TestNewJWTService tests the factory function
func (s *AuthServiceSuite) TestNewJWTService() {
	s.Run("Success", func() {
		svc, err := NewJWTService("a-valid-secret", time.Minute)
		s.NoError(err)
		s.NotNil(svc)
	})

	s.Run("Fail - Empty Secret", func() {
		svc, err := NewJWTService("", time.Minute)
		s.Error(err)
		s.Nil(svc)
		s.Contains(err.Error(), constants.MsgFailedToCreateAuthSvc)
		s.Contains(err.Error(), "secret cannot be empty")
	})
}

// TestGenerateToken tests the token generation logic
func (s *AuthServiceSuite) TestGenerateToken() {
	userID := "user-id-123"
	email := "test@example.com"
	ctx := context.Background()

	tokenString, appErr := s.service.GenerateToken(ctx, userID, email)

	s.Nil(appErr)
	s.NotEmpty(tokenString, "Generated token string should not be empty")

	// As a sanity check, let's parse the token (without validating signature)
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &CustomClaims{})
	s.NoError(err, "Failed to parse the generated token")

	claims, ok := token.Claims.(*CustomClaims)
	s.True(ok, "Token claims are not of type CustomClaims")
	s.Equal(userID, claims.UserID)
	s.Equal(email, claims.Email)
	s.Equal("web-page-analyzer", claims.Issuer)
}

// TestValidateRequest tests all paths for request validation
func (s *AuthServiceSuite) TestValidateRequest() {
	ctx := context.Background()
	testUserID := "user-valid-456"
	testEmail := "valid@user.com"

	// --- Helper: Generate a valid token for success tests ---
	validToken, appErr := s.service.GenerateToken(ctx, testUserID, testEmail)
	s.Require().Nil(appErr)

	s.Run("Success - Valid Token", func() {
		req, _ := http.NewRequest("GET", "/token", nil)
		req.Header.Set("Authorization", "Bearer "+validToken)

		userID, err := s.service.ValidateRequest(ctx, req)
		s.Nil(err)
		s.Equal(testUserID, userID)
	})

	s.Run("Fail - No Authorization Header", func() {
		req, _ := http.NewRequest("GET", "/token", nil)
		// No header set

		_, err := s.service.ValidateRequest(ctx, req)
		s.NotNil(err)
		s.Equal(http.StatusUnauthorized, err.StatusCode)
		s.Equal(constants.ErrAuthHeaderMissing.Message, err.Message)
	})

	s.Run("Fail - Malformed Header (Missing 'Bearer' prefix)", func() {
		req, _ := http.NewRequest("GET", "/token", nil)
		req.Header.Set("Authorization", validToken) // Just the token, no "Bearer "

		_, err := s.service.ValidateRequest(ctx, req)
		s.NotNil(err)
		s.Equal(http.StatusUnauthorized, err.StatusCode)
		s.Equal(constants.ErrAuthHeaderFormat.Message, err.Message)
	})

	s.Run("Fail - Malformed Header (Wrong Scheme)", func() {
		req, _ := http.NewRequest("GET", "/token", nil)
		req.Header.Set("Authorization", "Token "+validToken) // "Token" instead of "Bearer"

		_, err := s.service.ValidateRequest(ctx, req)
		s.NotNil(err)
		s.Equal(http.StatusUnauthorized, err.StatusCode)
		s.Equal(constants.ErrAuthHeaderFormat.Message, err.Message)
	})

	s.Run("Fail - Expired Token", func() {
		// Create a service with a negative TTL to generate an expired token
		expiredSvc, _ := NewJWTService(s.secret, -time.Minute*5)
		expiredToken, _ := expiredSvc.GenerateToken(ctx, testUserID, testEmail)

		req, _ := http.NewRequest("GET", "/token", nil)
		req.Header.Set("Authorization", "Bearer "+expiredToken)

		// Validate with the *main* service
		_, err := s.service.ValidateRequest(ctx, req)
		s.NotNil(err)
		s.Equal(http.StatusUnauthorized, err.StatusCode)
		s.Equal(constants.ErrTokenExpired.Message, err.Message)
	})

	s.Run("Fail - Invalid Signature (Wrong Secret)", func() {
		// Create a token using a different secret
		rogueSvc, _ := NewJWTService("this-is-the-wrong-secret", s.tokenTTL)
		rogueToken, _ := rogueSvc.GenerateToken(ctx, testUserID, testEmail)

		req, _ := http.NewRequest("GET", "/token", nil)
		req.Header.Set("Authorization", "Bearer "+rogueToken)

		// Validate with the *main* service (which has the correct secret)
		_, err := s.service.ValidateRequest(ctx, req)
		s.NotNil(err)
		s.Equal(http.StatusUnauthorized, err.StatusCode)
		s.Equal(constants.ErrTokenInvalid.Message, err.Message)
	})

	s.Run("Fail - Invalid Token (Garbage String)", func() {
		req, _ := http.NewRequest("GET", "/token", nil)
		req.Header.Set("Authorization", "Bearer not.a.real.token")

		_, err := s.service.ValidateRequest(ctx, req)
		s.NotNil(err)
		s.Equal(http.StatusUnauthorized, err.StatusCode)
		s.Equal(constants.ErrTokenInvalid.Message, err.Message)
	})

	s.Run("Fail - Missing UserID Claim", func() {
		// Manually craft a token that is valid but missing the UserID
		claims := &CustomClaims{
			Email: testEmail, // UserID is missing
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.tokenTTL)),
				Issuer:    "web-page-analyzer",
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		// Sign it with the correct secret
		signedToken, _ := token.SignedString([]byte(s.secret))

		req, _ := http.NewRequest("GET", "/token", nil)
		req.Header.Set("Authorization", "Bearer "+signedToken)

		_, err := s.service.ValidateRequest(ctx, req)
		s.NotNil(err)
		s.Equal(http.StatusUnauthorized, err.StatusCode)
		s.Equal(constants.ErrTokenClaims.Message, err.Message)
	})
}
