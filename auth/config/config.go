package config

import (
	"fmt"
	"time"

	"github.com/joho/godotenv"
	"github.com/sakashimaa/billing-microservice/pkg/utils/env"
)

type Config struct {
	Auth AuthConfig
}

type AuthConfig struct {
	AccessSecret  []byte
	RefreshSecret []byte
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	Issuer        string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	accessTTL, err := time.ParseDuration(env.ParseEnvWithFallback("ACCESS_TTL", "15m"))
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse ACCESS_TTL: %w", err)
	}

	refreshTTL, err := time.ParseDuration(env.ParseEnvWithFallback("REFRESH_TTL", "720h"))
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse REFRESH_TTL: %w", err)
	}

	accessSecret := []byte(env.ParseEnvWithFallback("AUTH_ACCESS_SECRET", ""))
	refreshSecret := []byte(env.ParseEnvWithFallback("AUTH_REFRESH_SECRET", ""))

	if len(accessSecret) < 32 {
		return Config{}, fmt.Errorf("access secret must be at least 16 bytes")
	}

	if len(refreshSecret) < 32 {
		return Config{}, fmt.Errorf("refresh secret must be at least 16 bytes")
	}

	issuer := env.ParseEnvWithFallback("AUTH_ISSUER", "billing-auth-microservice")
	return Config{
		Auth: AuthConfig{
			AccessSecret:  accessSecret,
			RefreshSecret: refreshSecret,
			AccessTTL:     accessTTL,
			RefreshTTL:    refreshTTL,
			Issuer:        issuer,
		},
	}, nil
}
