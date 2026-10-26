package config_test

import (
	"net/url"
	"testing"

	"github.com/santiagossaa/invoice-generator/internal/config"
)

func setEmptyConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"PORT", "DB_HOST", "DB_PORT", "DB_NAME", "DB_USER", "DB_PASSWORD",
		"DB_SSLMODE", "DB_SSLROOTCERT", "JWT_SECRET", "JWT_EXPIRY",
		"CORS_ALLOWED_ORIGINS", "LOGIN_RATE_LIMIT", "LOGIN_RATE_WINDOW", "ENVIRONMENT",
	} {
		t.Setenv(key, "")
	}
}

func setProductionConfigEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_NAME", "invoicegen")
	t.Setenv("DB_USER", "invoicegen")
	t.Setenv("DB_PASSWORD", "a-production-database-password")
	t.Setenv("JWT_SECRET", "a-production-jwt-secret-with-more-than-32-bytes")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://invoices.example.com")
}

func TestLoadDefaults(t *testing.T) {
	setEmptyConfigEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Port != "8080" || cfg.DBHost != "localhost" || cfg.DBPort != 5432 ||
		cfg.DBName != "invoicegen" || cfg.Environment != "development" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.DBSSLMode != "disable" || cfg.JWTExpiry.String() != "24h0m0s" {
		t.Fatalf("unexpected development security defaults: %+v", cfg)
	}
	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "http://localhost:5173" {
		t.Fatalf("AllowedOrigins = %v", cfg.AllowedOrigins)
	}
}

func TestLoadProductionConfig(t *testing.T) {
	setEmptyConfigEnv(t)
	setProductionConfigEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.DBSSLMode != "verify-full" || cfg.JWTExpiry.String() != "1h0m0s" {
		t.Fatalf("unexpected production defaults: %+v", cfg)
	}
}

func TestProductionRejectsUnsafeConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T)
	}{
		{
			name: "missing jwt secret",
			setup: func(t *testing.T) {
				t.Setenv("JWT_SECRET", "")
			},
		},
		{
			name: "short jwt secret",
			setup: func(t *testing.T) {
				t.Setenv("JWT_SECRET", "too-short")
			},
		},
		{
			name: "development database password",
			setup: func(t *testing.T) {
				t.Setenv("DB_PASSWORD", "invoicegen_dev")
			},
		},
		{
			name: "database tls disabled",
			setup: func(t *testing.T) {
				t.Setenv("DB_SSLMODE", "disable")
			},
		},
		{
			name: "missing cors allowlist",
			setup: func(t *testing.T) {
				t.Setenv("CORS_ALLOWED_ORIGINS", "")
			},
		},
		{
			name: "insecure cors origin",
			setup: func(t *testing.T) {
				t.Setenv("CORS_ALLOWED_ORIGINS", "http://invoices.example.com")
			},
		},
		{
			name: "loopback cors origin",
			setup: func(t *testing.T) {
				t.Setenv("CORS_ALLOWED_ORIGINS", "https://localhost")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setEmptyConfigEnv(t)
			setProductionConfigEnv(t)
			test.setup(t)
			if _, err := config.Load(); err == nil {
				t.Fatal("Load() accepted unsafe production configuration")
			}
		})
	}
}

func TestInvalidConfigValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "invalid database port", key: "DB_PORT", value: "not-a-number"},
		{name: "invalid jwt duration", key: "JWT_EXPIRY", value: "never"},
		{name: "invalid rate limit", key: "LOGIN_RATE_LIMIT", value: "0"},
		{name: "invalid rate window", key: "LOGIN_RATE_WINDOW", value: "10ms"},
		{name: "invalid database tls mode", key: "DB_SSLMODE", value: "plaintext"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setEmptyConfigEnv(t)
			t.Setenv(test.key, test.value)
			if _, err := config.Load(); err == nil {
				t.Fatal("Load() accepted invalid config")
			}
		})
	}
}

func TestDatabaseURLEscapesCredentialsAndUsesTLSMode(t *testing.T) {
	setEmptyConfigEnv(t)
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_USER", "invoice user")
	t.Setenv("DB_PASSWORD", "pass:word@secret")
	t.Setenv("DB_SSLMODE", "verify-ca")
	t.Setenv("DB_SSLROOTCERT", "/etc/ssl/certs/db-ca.pem")

	cfg, err := config.LoadDatabaseConfig()
	if err != nil {
		t.Fatalf("LoadDatabaseConfig() error: %v", err)
	}
	parsed, err := url.Parse(cfg.DatabaseURL())
	if err != nil {
		t.Fatalf("parse DatabaseURL(): %v", err)
	}
	password, _ := parsed.User.Password()
	if parsed.Scheme != "postgres" || parsed.User.Username() != "invoice user" ||
		password != "pass:word@secret" || parsed.Query().Get("sslmode") != "verify-ca" ||
		parsed.Query().Get("sslrootcert") != "/etc/ssl/certs/db-ca.pem" {
		t.Fatalf("unexpected database URL: scheme=%q user=%q sslmode=%q sslrootcert=%q",
			parsed.Scheme, parsed.User.Username(), parsed.Query().Get("sslmode"), parsed.Query().Get("sslrootcert"))
	}
}
