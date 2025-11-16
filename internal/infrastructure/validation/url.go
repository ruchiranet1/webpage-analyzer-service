package validation

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"webpage-analyzer-service/internal/constants"
	"webpage-analyzer-service/internal/domain"
)

// Validator interface defines the contract for URL validation.
type Validator interface {
	ValidateURL(ctx context.Context, rawURL string) *domain.AppError
}

// urlValidator is the concrete implementation.
type urlValidator struct {
	// use a compiled regex for URL validation
	urlRegex *regexp.Regexp
}

// NewURLValidator creates a new validator.
func NewURLValidator() (Validator, error) {
	// This regex is a common, reasonably strict pattern.
	// It checks for http/https schemes, a domain, and optional path/query.
	r, err := regexp.Compile(`^https(s)?:\/\/[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}(\/[a-zA-Z0-9.-]*)*(\?[a-zA-Z0-9-_=&]*)?$`)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", constants.MsgFailedToCreateValidator, err)
	}

	return &urlValidator{
		urlRegex: r,
	}, nil
}

// ValidateURL implements the Validator interface.
func (v *urlValidator) ValidateURL(ctx context.Context, rawURL string) *domain.AppError {
	if rawURL == "" {
		return domain.NewAppErrorUser(http.StatusBadRequest, constants.ErrURLEmpty)
	}

	if v.urlRegex.MatchString(rawURL) {
		return nil
	}

	// Try adding a default scheme
	prefixedURL := "https://" + rawURL
	if v.urlRegex.MatchString(prefixedURL) {
		return nil
	}

	// If it's still not valid, return an error
	return domain.NewAppErrorUser(http.StatusBadRequest, constants.ErrURLInvalid)
}
