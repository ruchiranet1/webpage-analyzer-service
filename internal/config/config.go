package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
type Config struct {
	HTTPServer     ServerConfig      `mapstructure:"http"`
	JWT            JWTConfig         `mapstructure:"jwt"`
	RateLimiter    RateLimiterConfig `mapstructure:"rateLimiter"`
	IdempotencyTTL time.Duration     `mapstructure:"idempotencyTTL"`
	Fetcher        FetcherConfig     `mapstructure:"fetcher"`
	LinkChecker    LinkCheckerConfig `mapstructure:"linkChecker"`
	LoggerLevel    string            `mapstructure:"loggerLevel"`
}

type ServerConfig struct {
	Port            string        `mapstructure:"port"`
	ShutdownTimeout time.Duration `mapstructure:"shutdownTimeout"`
}

type JWTConfig struct {
	Secret   string        `mapstructure:"secret"`
	TokenTTL time.Duration `mapstructure:"tokenTTL"`
}

type RateLimiterConfig struct {
	RPS   float64 `mapstructure:"rps"`
	Burst int     `mapstructure:"burst"`
}

type FetcherConfig struct {
	Timeout time.Duration `mapstructure:"timeout"`
}

type LinkCheckerConfig struct {
	Timeout time.Duration `mapstructure:"timeout"`
}

// Load loads configuration from file (config.properties) and environment variables.
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("http.port", "8080")
	v.SetDefault("http.shutdownTimeout", "30s")
	v.SetDefault("jwt.secret", "demo-web-user1")
	v.SetDefault("jwt.tokenTTL", "1h")
	v.SetDefault("rateLimiter.rps", 10.0)
	v.SetDefault("rateLimiter.burst", 20)
	v.SetDefault("idempotencyTTL", "15m")
	v.SetDefault("fetcher.timeout", "10s")
	v.SetDefault("linkChecker.timeout", "5s")
	v.SetDefault("loggerLevel", "debug")

	// 2. Set config file properties
	v.SetConfigName("config")
	v.SetConfigType("properties")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	// 3. Set environment variable properties
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 4. Read the config file
	if err := v.ReadInConfig(); err != nil {
		// Handle errors reading the config file
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error occurred
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found; right now set defaults and env vars
	}

	// 5. Unmarshal the config into the struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
