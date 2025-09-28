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
	BaseUrlAPI     string
	S3Endpoint     string
	S3AccessKey    string
	S3SecretKey    string
	S3UseSSL       bool
	S3PublicDomain string
	S3Bucket       string
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
		BaseUrlAPI:     mustGetEnv("BASE_URL_API"),
		S3Endpoint:     mustGetEnv("S3_ENDPOINT"),
		S3AccessKey:    mustGetEnv("S3_ACCESS_KEY"),
		S3SecretKey:    mustGetEnv("S3_SECRET_KEY"),
		S3UseSSL:       getEnvBool("S3_USE_SSL", true),
		S3PublicDomain: mustGetEnv("S3_PUBLIC_DOMAIN"),
		S3Bucket:       mustGetEnv("S3_BUCKET"),
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

func getEnvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
