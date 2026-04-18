package config

import (
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost           string
	DBPort           string
	DBUser           string
	DBPassword       string
	DBName           string
	AppPort          string
	JWTAccessSecret  string
	JWTRefreshSecret string
	JWTAccessTTL     time.Duration
	JWTRefreshTTL    time.Duration
}

func Load() *Config {
	_ = godotenv.Load(".env")

	return &Config{
		DBHost:           get("DB_HOST", "localhost"),
		DBPort:           get("DB_PORT", "5432"),
		DBUser:           get("DB_USER", "postgres"),
		DBPassword:       get("DB_PASSWORD", "postgres"),
		DBName:           get("DB_NAME", "haulagex"),
		AppPort:          get("APP_PORT", "8080"),
		JWTAccessSecret:  get("JWT_ACCESS_SECRET", "dev_access_secret"),
		JWTRefreshSecret: get("JWT_REFRESH_SECRET", "dev_refresh_secret"),
		JWTAccessTTL:     mustDuration(get("JWT_ACCESS_TTL", "15m")),
		JWTRefreshTTL:    mustDuration(get("JWT_REFRESH_TTL", "168h")),
	}
}

func get(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func mustDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 15 * time.Minute
	}
	return d
}
