package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
type Config struct {
	HTTP struct {
		Port            int           `mapstructure:"port"`
		ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	} `mapstructure:"http"`

	Logger struct {
		Level string `mapstructure:"level"`
	} `mapstructure:"logger"`

	JWT struct {
		Secret   string        `mapstructure:"secret"`
		TokenTTL time.Duration `mapstructure:"token_ttl"`
	} `mapstructure:"jwt"`

	RateLimiter struct {
		RPS   float64 `mapstructure:"rps"`
		Burst int     `mapstructure:"burst"`
	} `mapstructure:"rate_limiter"`

	Services struct {
		FetcherTimeout     time.Duration `mapstructure:"fetcher_timeout"`
		LinkCheckerTimeout time.Duration `mapstructure:"link_checker_timeout"`
	} `mapstructure:"services"`

	Middleware struct {
		IdempotencyTTL time.Duration `mapstructure:"idempotency_ttl"`
	} `mapstructure:"middleware"`
}

// Load reads configuration from config.properties and environment variables.
func Load() (*Config, error) {
	v := viper.New()

	// 1. Set config file details
	v.SetConfigName("config")
	v.SetConfigType("properties")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.AddConfigPath("/etc/app")
	v.AddConfigPath("$HOME/.config")

	// 2. Set defaults
	setDefaults(v)

	// 3. Enable environment variable overriding
	v.AutomaticEnv()
	// This allows HTTP_PORT to override http.port
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 4. Read the config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; warn but continue
		} else {
			// Config file was found but another error occurred
			return nil, err
		}
	}

	// 5. Unmarshal the config into our struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// setDefaults defines all the default values for the configuration.
func setDefaults(v *viper.Viper) {
	v.SetDefault("http.port", 8080)
	v.SetDefault("http.shutdown_timeout", 10*time.Second)

	v.SetDefault("logger.level", "info")

	v.SetDefault("jwt.secret", "my-user1")
	v.SetDefault("jwt.token_ttl", 1*time.Hour)

	v.SetDefault("rate_limiter.rps", 10.0) // 10 requests per second
	v.SetDefault("rate_limiter.burst", 20) // Allow bursts of 20 requests

	v.SetDefault("services.fetcher_timeout", 10*time.Second)
	v.SetDefault("services.link_checker_timeout", 5*time.Second)

	v.SetDefault("middleware.idempotency_ttl", 24*time.Hour)
}
