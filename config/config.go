package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL    string
	CacheURL       string
	LoggerLevel    string
	ContextTimeout int
	JWTSecretKey   string
}

func LoadConfig() *Config {
	// Jadikan optional: kalau .env nggak ada (di k8s), lanjut pakai ENV
	_ = godotenv.Load()

	cfg := &Config{
		DatabaseURL:    mustGetEnv("DATABASE_URL"),
		CacheURL:       getEnv("CACHE_URL", ""),
		LoggerLevel:    getEnv("LOGGER_LEVEL", "info"),
		ContextTimeout: getEnvInt("CONTEXT_TIMEOUT", 10),
		JWTSecretKey:   mustGetEnv("JWT_SECRET_KEY"),
	}
	return cfg
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func mustGetEnv(key string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	log.Fatalf("%s is required", key)
	return ""
}
