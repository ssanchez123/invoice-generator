package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port        string
	DBHost      string
	DBPort      int
	DBName      string
	DBUser      string
	DBPassword  string
	JWTSecret   string
	PDFStorage  string
	Environment string
}

func Load() (*Config, error) {
	port := getEnv("PORT", "8080")
	dbPort, err := strconv.Atoi(getEnv("DB_PORT", "5432"))
	if err != nil {
		return nil, fmt.Errorf("invalid DB_PORT: %w", err)
	}

	cfg := &Config{
		Port:        port,
		DBHost:      getEnv("DB_HOST", "localhost"),
		DBPort:      dbPort,
		DBName:      getEnv("DB_NAME", "invoicegen"),
		DBUser:      getEnv("DB_USER", "invoicegen"),
		DBPassword:  getEnv("DB_PASSWORD", "invoicegen_dev"),
		JWTSecret:   getEnv("JWT_SECRET", "dev-secret-change-in-production"),
		PDFStorage:  getEnv("PDF_STORAGE_PATH", "/tmp/pdfs"),
		Environment: getEnv("ENVIRONMENT", "development"),
	}

	if cfg.JWTSecret == "dev-secret-change-in-production" && cfg.Environment == "production" {
		return nil, fmt.Errorf("JWT_SECRET must be set in production")
	}

	return cfg, nil
}

func (c *Config) DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
