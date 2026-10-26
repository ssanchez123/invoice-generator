package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	developmentJWTSecret = "dev-secret-change-in-production"
	developmentDBPass    = "invoicegen_dev"
)

type Config struct {
	Port            string
	DBHost          string
	DBPort          int
	DBName          string
	DBUser          string
	DBPassword      string
	DBSSLMode       string
	DBSSLRootCert   string
	JWTSecret       string
	JWTExpiry       time.Duration
	AllowedOrigins  []string
	LoginRateLimit  int
	LoginRateWindow time.Duration
	Environment     string
}

func Load() (*Config, error) {
	cfg, err := LoadDatabaseConfig()
	if err != nil {
		return nil, err
	}

	cfg.JWTSecret = getEnvRaw("JWT_SECRET", developmentJWTSecret)
	if cfg.Environment == "production" {
		if cfg.JWTSecret == developmentJWTSecret || len([]byte(strings.TrimSpace(cfg.JWTSecret))) < 32 || strings.TrimSpace(cfg.JWTSecret) != cfg.JWTSecret {
			return nil, fmt.Errorf("JWT_SECRET must contain at least 32 bytes in production")
		}
	}

	expiryDefault := "24h"
	if cfg.Environment == "production" {
		expiryDefault = "1h"
	}
	cfg.JWTExpiry, err = time.ParseDuration(getEnv("JWT_EXPIRY", expiryDefault))
	if err != nil || cfg.JWTExpiry <= 0 || cfg.JWTExpiry > 7*24*time.Hour {
		return nil, fmt.Errorf("JWT_EXPIRY must be a positive duration no greater than 168h")
	}

	cfg.AllowedOrigins, err = allowedOrigins(cfg.Environment)
	if err != nil {
		return nil, err
	}

	cfg.LoginRateLimit, err = positiveIntEnv("LOGIN_RATE_LIMIT", 10)
	if err != nil || cfg.LoginRateLimit > 1000 {
		return nil, fmt.Errorf("LOGIN_RATE_LIMIT must be between 1 and 1000")
	}
	cfg.LoginRateWindow, err = time.ParseDuration(getEnv("LOGIN_RATE_WINDOW", "1m"))
	if err != nil || cfg.LoginRateWindow < time.Second || cfg.LoginRateWindow > time.Hour {
		return nil, fmt.Errorf("LOGIN_RATE_WINDOW must be between 1s and 1h")
	}

	return cfg, nil
}

// LoadDatabaseConfig loads database settings for API and worker processes.
func LoadDatabaseConfig() (*Config, error) {
	environment := strings.ToLower(strings.TrimSpace(getEnv("ENVIRONMENT", "development")))
	if environment != "development" && environment != "test" && environment != "production" {
		return nil, fmt.Errorf("ENVIRONMENT must be development, test, or production")
	}

	port, err := positiveIntEnv("DB_PORT", 5432)
	if err != nil || port > 65535 {
		return nil, fmt.Errorf("DB_PORT must be between 1 and 65535")
	}
	apiPort, err := positiveIntEnv("PORT", 8080)
	if err != nil || apiPort > 65535 {
		return nil, fmt.Errorf("PORT must be between 1 and 65535")
	}

	sslDefault := "disable"
	if environment == "production" {
		sslDefault = "verify-full"
	}
	sslMode := strings.ToLower(strings.TrimSpace(getEnv("DB_SSLMODE", sslDefault)))
	switch sslMode {
	case "disable", "require", "verify-ca", "verify-full":
	default:
		return nil, fmt.Errorf("DB_SSLMODE must be disable, require, verify-ca, or verify-full")
	}
	if environment == "production" && sslMode == "disable" {
		return nil, fmt.Errorf("DB_SSLMODE must enable TLS in production")
	}

	cfg := &Config{
		Port:          strconv.Itoa(apiPort),
		DBHost:        getEnv("DB_HOST", "localhost"),
		DBPort:        port,
		DBName:        getEnv("DB_NAME", "invoicegen"),
		DBUser:        getEnv("DB_USER", "invoicegen"),
		DBPassword:    getEnvRaw("DB_PASSWORD", developmentDBPass),
		DBSSLMode:     sslMode,
		DBSSLRootCert: strings.TrimSpace(os.Getenv("DB_SSLROOTCERT")),
		Environment:   environment,
	}

	if environment == "production" {
		if cfg.DBHost == "localhost" || cfg.DBHost == "127.0.0.1" || cfg.DBHost == "" {
			return nil, fmt.Errorf("DB_HOST must identify the production database")
		}
		if strings.TrimSpace(os.Getenv("DB_HOST")) == "" ||
			strings.TrimSpace(os.Getenv("DB_NAME")) == "" ||
			strings.TrimSpace(os.Getenv("DB_USER")) == "" ||
			os.Getenv("DB_PASSWORD") == "" {
			return nil, fmt.Errorf("DB_HOST, DB_NAME, DB_USER, and DB_PASSWORD must be explicitly set in production")
		}
		if cfg.DBName == "" || cfg.DBUser == "" || cfg.DBPassword == "" || cfg.DBPassword == developmentDBPass {
			return nil, fmt.Errorf("DB_NAME, DB_USER, and a non-development DB_PASSWORD are required in production")
		}
	}

	return cfg, nil
}

func (c *Config) DatabaseURL() string {
	query := url.Values{"sslmode": []string{c.DBSSLMode}}
	if c.DBSSLRootCert != "" {
		query.Set("sslrootcert", c.DBSSLRootCert)
	}
	return (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.DBUser, c.DBPassword),
		Host:     net.JoinHostPort(c.DBHost, strconv.Itoa(c.DBPort)),
		Path:     "/" + c.DBName,
		RawQuery: query.Encode(),
	}).String()
}

func allowedOrigins(environment string) ([]string, error) {
	raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if raw == "" {
		if environment == "production" {
			return nil, fmt.Errorf("CORS_ALLOWED_ORIGINS is required in production")
		}
		return []string{"http://localhost:5173"}, nil
	}

	origins := strings.Split(raw, ",")
	for i, origin := range origins {
		origin = strings.TrimSpace(origin)
		parsed, err := url.Parse(origin)
		if err != nil || origin == "*" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
			return nil, fmt.Errorf("CORS_ALLOWED_ORIGINS contains an invalid origin")
		}
		if environment == "production" && (parsed.Scheme != "https" || strings.EqualFold(parsed.Hostname(), "localhost") || net.ParseIP(parsed.Hostname()) != nil && net.ParseIP(parsed.Hostname()).IsLoopback()) {
			return nil, fmt.Errorf("production CORS origins must use HTTPS and must not be loopback hosts")
		}
		origins[i] = strings.TrimSuffix(origin, "/")
	}
	return origins, nil
}

func positiveIntEnv(key string, fallback int) (int, error) {
	raw := getEnv(key, strconv.Itoa(fallback))
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return value, nil
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getEnvRaw(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
