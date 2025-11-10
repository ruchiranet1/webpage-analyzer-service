package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"webpage-analyzer-service/internal/constants"
	"webpage-analyzer-service/internal/domain"

	"github.com/golang-jwt/jwt/v5"
)

// jwtService is the concrete implementation of the auth.Service interface.
type jwtService struct {
	// jwtSecret is the secret key used to validate tokens.
	jwtSecret []byte
	tokenTTL  time.Duration
}

// NewJWTService is the Factory Function for our auth service.
func NewJWTService(secret string, tokenTTL time.Duration) (Service, error) {
	if secret == "" {
		return nil, fmt.Errorf("%s: secret cannot be empty", constants.MsgFailedToCreateAuthSvc)
	}
	return &jwtService{
		jwtSecret: []byte(secret),
		tokenTTL:  tokenTTL,
	}, nil
}

// CustomClaims defines the custom data we'll store in our JWT.
type CustomClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// GenerateToken creates and signs a new JWT for a given user.
func (s *jwtService) GenerateToken(ctx context.Context, userID string, email string) (string, *domain.AppError) {
	// Set custom claims
	claims := &CustomClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			// Set token expiration
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "web-page-analyzer",
		},
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with our secret
	signedToken, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", domain.NewInternalError(constants.ErrTokenSign, err)
	}

	return signedToken, nil
}

// ValidateRequest parses and validates a JWT from an HTTP request's
func (s *jwtService) ValidateRequest(ctx context.Context, r *http.Request) (string, *domain.AppError) {

	// TODO need to resolve this later
	return "temp-bypassed", nil

	tokenString, appErr := s.extractTokenFromHeader(r)
	if appErr != nil {
		return "", appErr
	}

	// Parse the token
	claims := &CustomClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Ensure the signing method is what we expect
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	// Handle parsing errors
	if err != nil {
		if err == jwt.ErrTokenExpired {
			return "", domain.NewAppError(http.StatusUnauthorized, constants.ErrTokenExpired, err)
		}
		return "", domain.NewAppError(http.StatusUnauthorized, constants.ErrTokenInvalid, err)
	}

	// Check if the token is valid and we have our claims
	if !token.Valid || claims.UserID == "" {
		return "", domain.NewAppErrorUser(http.StatusUnauthorized, constants.ErrTokenClaims)
	}

	// Token is valid, return the UserID
	return claims.UserID, nil
}

// extractTokenFromHeader helper function to get the "Bearer <token>" string.
func (s *jwtService) extractTokenFromHeader(r *http.Request) (string, *domain.AppError) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", domain.NewAppErrorUser(http.StatusUnauthorized, constants.ErrAuthHeaderMissing)
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", domain.NewAppErrorUser(http.StatusUnauthorized, constants.ErrAuthHeaderFormat)
	}

	return parts[1], nil
}
