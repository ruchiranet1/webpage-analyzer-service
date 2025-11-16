package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Full(t *testing.T) {
	// --- 1. Save original working directory ---
	originalWD, err := os.Getwd()
	require.NoError(t, err, "Failed to get current working directory")

	// --- 2. Create a temporary directory for the test ---
	tempDir := t.TempDir()

	// --- 3. Create a dummy config.properties file in the temp dir ---
	configContent := `
http.port=9090
logger.level=debug
jwt.token_ttl=30m
rate_limiter.rps=50.5
`
	configPath := filepath.Join(tempDir, "config.properties")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err, "Failed to write temporary config file")

	// --- 4. Change working directory to the temp dir ---
	err = os.Chdir(tempDir)
	require.NoError(t, err, "Failed to change working directory to temp dir")

	// --- 5. Defer cleanup: restore original working directory ---
	defer func() {
		err := os.Chdir(originalWD)
		if err != nil {
			t.Logf("Failed to change directory back to %s: %v", originalWD, err)
		}
	}()

	// --- 6. Set environment variables to test overrides ---
	// This will override the file value (9090)
	t.Setenv("HTTP_PORT", "1234")

	// This will override the default value (10s)
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "15s")

	// This will override the default ("my-user1")
	t.Setenv("JWT_SECRET", "env-secret-key")

	// This will override the default value (20)
	t.Setenv("RATE_LIMITER_BURST", "100")

	// --- 7. Run the Load function ---
	cfg, err := Load()

	// --- 8. Assertions ---
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Check Env > File > Default (HTTP_PORT=1234 > http.port=9090 > 8080)
	assert.Equal(t, 1234, cfg.HTTP.Port)

	// Check Env > Default (HTTP_SHUTDOWN_TIMEOUT=15s > 10s)
	assert.Equal(t, 15*time.Second, cfg.HTTP.ShutdownTimeout)

	// Check File > Default (logger.level=debug > "info")
	assert.Equal(t, "debug", cfg.Logger.Level)

	// Check Env > Default (JWT_SECRET="env-secret-key" > "my-user1")
	assert.Equal(t, "env-secret-key", cfg.JWT.Secret)

	// Check File > Default (jwt.token_ttl=30m > 1h)
	assert.Equal(t, 30*time.Minute, cfg.JWT.TokenTTL)

	// Check File > Default (rate_limiter.rps=50.5 > 10.0)
	assert.Equal(t, 50.5, cfg.RateLimiter.RPS)

	// Check Env > Default (RATE_LIMITER_BURST=100 > 20)
	assert.Equal(t, 100, cfg.RateLimiter.Burst)

	// Check Default (services.fetcher_timeout=10s)
	assert.Equal(t, 10*time.Second, cfg.Services.FetcherTimeout)

	// Check Default (services.link_checker_timeout=5s)
	assert.Equal(t, 5*time.Second, cfg.Services.LinkCheckerTimeout)

	// Check Default (middleware.idempotency_ttl=24h)
	assert.Equal(t, 24*time.Hour, cfg.Middleware.IdempotencyTTL)
}

// TestLoad_NoFile tests that loading works fine even if the
func TestLoad_NoFile(t *testing.T) {
	// --- 1. Save original working directory ---
	originalWD, err := os.Getwd()
	require.NoError(t, err, "Failed to get current working directory")

	// --- 2. Create a temporary directory for the test ---
	tempDir := t.TempDir()

	// --- 3. Change working directory to the temp dir ---
	err = os.Chdir(tempDir)
	require.NoError(t, err, "Failed to change working directory to temp dir")

	// --- 4. Defer cleanup: restore original working directory ---
	defer func() {
		err := os.Chdir(originalWD)
		if err != nil {
			t.Logf("Failed to change directory back to %s: %v", originalWD, err)
		}
	}()

	// --- 5. Set *one* env var to prove it works ---
	t.Setenv("LOGGER_LEVEL", "error")
	t.Setenv("HTTP_PORT", "5555")

	// --- 6. Run the Load function ---
	cfg, err := Load()

	// --- 7. Assertions ---
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Check Env > Default
	assert.Equal(t, "error", cfg.Logger.Level)
	assert.Equal(t, 5555, cfg.HTTP.Port)

	// Check Default (since no file was loaded)
	assert.Equal(t, 10*time.Second, cfg.HTTP.ShutdownTimeout)
	assert.Equal(t, 1*time.Hour, cfg.JWT.TokenTTL)
	assert.Equal(t, 10.0, cfg.RateLimiter.RPS)
}
