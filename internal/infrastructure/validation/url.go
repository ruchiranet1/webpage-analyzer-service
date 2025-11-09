package validation

import (
	"context"
	"fmt"
	"regexp"

	"webpage-analyzer-service/internal/domain"
)

// Validator defines the interface for our validation logic.
// While we only have URL validation now, this interface allows
// us to easily add more validators (e.g., for emails)
// without changing the handler's dependency.
type Validator interface {
	ValidateURL(ctx context.Context, rawURL string) *domain.AppError
}

// urlValidator is the concrete implementation.
type urlValidator struct {
	// We compile the regex once when the validator is created.
	// This is the "Compile Once, Use Many" pattern, which is
	// much more efficient than compiling on every request.
	urlRegex *regexp.Regexp
}

// NewURLValidator creates a new validator.
func NewURLValidator() (Validator, error) {
	// This regex is a common, reasonably strict pattern.
	// It checks for http/https schemes, a domain, and optional path/query.
	// Per your requirement, we use the standard `regexp` package.
	// Note: Perfect URL regex is notoriously difficult. This is a good
	// balance of correctness and simplicity.
	// It requires a scheme (http or https).
	r, err := regexp.Compile(`^https(s)?:\/\/[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}(\/[a-zA-Z0-9.-]*)*(\?[a-zA-Z0-9-_=&]*)?$`)
	if err != nil {
		return nil, fmt.Errorf("failed to compile URL regex: %w", err)
	}

	return &urlValidator{
		urlRegex: r,
	}, nil
}

// ValidateURL implements the Validator interface.
func (v *urlValidator) ValidateURL(ctx context.Context, rawURL string) *domain.AppError {
	if rawURL == "" {
		return domain.NewAppError(400, "URL cannot be empty", nil)
	}

	// The analysis service already adds a default scheme.
	// Our regex *requires* a scheme, so we'll check without it first,
	// and then with a default 'https://' prefix.
	if v.urlRegex.MatchString(rawURL) {
		return nil // URL is valid as-is
	}

	// Try adding a default scheme
	prefixedURL := "https://" + rawURL
	if v.urlRegex.MatchString(prefixedURL) {
		return nil // URL is valid with the default prefix
	}

	// If it's still not valid, return an error
	return domain.NewAppError(400, "Invalid URL format. Must be a valid URL (e.g., https://example.com)", nil)
}
