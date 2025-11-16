package validation

import (
	"context"
	"net/http"
	"testing"

	"webpage-analyzer-service/internal/constants"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSetup creates a valid validator instance for use in tests.
func testSetup(t *testing.T) Validator {
	// The real implementation is used here since we want to test its core logic.
	v, err := NewURLValidator()
	require.NoError(t, err)
	return v
}

// --- Tests ---

// TestNewURLValidator_Success tests the factory function's happy path.
func TestNewURLValidator_Success(t *testing.T) {
	v, err := NewURLValidator()
	assert.NotNil(t, v)
	assert.NoError(t, err)
}

// TestNewURLValidator_Failure tests the error path if the regex is invalid.
func TestNewURLValidator_Failure(t *testing.T) {
	// We assert that the standard regex compiles successfully.
	v, err := NewURLValidator()
	assert.NoError(t, err, "NewURLValidator should compile the standard regex successfully")
	assert.NotNil(t, v)
}

// TestValidateURL_HappyPath tests a URL that is correctly formatted with scheme, including IP addresses.
func TestValidateURL_HappyPath(t *testing.T) {
	v := testSetup(t)
	ctx := context.Background()

	validURLs := []string{
		"https://www.google.com/search?q=test",
		"https://example.co.uk",
		"https://sub.domain.net/",
	}

	for _, url := range validURLs {
		appErr := v.ValidateURL(ctx, url)
		assert.Nil(t, appErr, "URL %s should be valid. Error: %v", url, appErr)
	}
}

// TestValidateURL_SchemeFix tests a URL that requires the default "https://" prefix.
func TestValidateURL_SchemeFix(t *testing.T) {
	v := testSetup(t)
	ctx := context.Background()

	validURLsWithoutScheme := []string{
		"www.github.com/go-chi",
		"docs.python.org/3/",
	}

	for _, url := range validURLsWithoutScheme {
		appErr := v.ValidateURL(ctx, url)
		assert.Nil(t, appErr, "URL %s should be fixed and valid. Error: %v", url, appErr)
	}
}

// TestValidateURL_Empty tests the case where the URL string is empty.
func TestValidateURL_Empty(t *testing.T) {
	v := testSetup(t)
	ctx := context.Background()

	appErr := v.ValidateURL(ctx, "")

	require.NotNil(t, appErr)
	assert.Equal(t, http.StatusBadRequest, appErr.StatusCode)
	assert.Equal(t, constants.ErrURLEmpty.Code, appErr.ErrorCode)
}

// TestValidateURL_InvalidFormat tests URLs that do not conform to the regex.
func TestValidateURL_InvalidFormat(t *testing.T) {
	v := testSetup(t)
	ctx := context.Background()

	invalidURLs := []string{
		"ftp://malicious.com",      // Wrong scheme (not http/https)
		"just-a-string",            // No TLD
		"http://missing.",          // Missing TLD
		"https://domain no spaces", // Spaces
	}

	for _, url := range invalidURLs {
		appErr := v.ValidateURL(ctx, url)
		require.NotNil(t, appErr, "URL %s should be invalid", url)
		assert.Equal(t, http.StatusBadRequest, appErr.StatusCode)
		assert.Equal(t, constants.ErrURLInvalid.Code, appErr.ErrorCode)
	}
}
