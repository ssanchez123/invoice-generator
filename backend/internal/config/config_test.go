package config_test

import (
	"os"
	"testing"

	"github.com/santiagossaa/invoice-generator/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	// Clear env vars to test defaults
	os.Unsetenv("PORT")
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_PORT")
	os.Unsetenv("DB_NAME")
	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("ENVIRONMENT")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Port)
	}
	if cfg.DBHost != "localhost" {
		t.Errorf("DBHost = %q, want localhost", cfg.DBHost)
	}
	if cfg.DBPort != 5432 {
		t.Errorf("DBPort = %d, want 5432", cfg.DBPort)
	}
	if cfg.DBName != "invoicegen" {
		t.Errorf("DBName = %q, want invoicegen", cfg.DBName)
	}
	if cfg.Environment != "development" {
		t.Errorf("Environment = %q, want development", cfg.Environment)
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("PORT", "3000")
	os.Setenv("DB_HOST", "db.example.com")
	os.Setenv("DB_PORT", "5433")
	os.Setenv("DB_NAME", "mydb")
	os.Setenv("DB_USER", "myuser")
	os.Setenv("DB_PASSWORD", "mypass")
	os.Setenv("JWT_SECRET", "super-secret")
	os.Setenv("ENVIRONMENT", "production")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("DB_NAME")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("ENVIRONMENT")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Port != "3000" {
		t.Errorf("Port = %q, want 3000", cfg.Port)
	}
	if cfg.DBHost != "db.example.com" {
		t.Errorf("DBHost = %q, want db.example.com", cfg.DBHost)
	}
	if cfg.DBPort != 5433 {
		t.Errorf("DBPort = %d, want 5433", cfg.DBPort)
	}
	if cfg.JWTSecret != "super-secret" {
		t.Errorf("JWTSecret = %q, want super-secret", cfg.JWTSecret)
	}
	if cfg.Environment != "production" {
		t.Errorf("Environment = %q, want production", cfg.Environment)
	}
}

func TestProductionRequiresJWTSecret(t *testing.T) {
	os.Setenv("ENVIRONMENT", "production")
	os.Unsetenv("JWT_SECRET")
	defer os.Unsetenv("ENVIRONMENT")

	_, err := config.Load()
	if err == nil {
		t.Error("Load() in production without JWT_SECRET should error")
	}
}

func TestInvalidDBPort(t *testing.T) {
	os.Setenv("DB_PORT", "not-a-number")
	defer os.Unsetenv("DB_PORT")

	_, err := config.Load()
	if err == nil {
		t.Error("Load() with invalid DB_PORT should error")
	}
}
