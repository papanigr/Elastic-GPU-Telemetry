package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear env vars
	os.Unsetenv("GATEWAY_HTTP_PORT")
	os.Unsetenv("GATEWAY_DATABASE_URL")
	os.Unsetenv("GATEWAY_LOG_LEVEL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.HTTPPort != 8080 {
		t.Errorf("Expected HTTPPort 8080, got %d", cfg.HTTPPort)
	}

	if cfg.DefaultPageSize != 100 {
		t.Errorf("Expected DefaultPageSize 100, got %d", cfg.DefaultPageSize)
	}

	if cfg.MaxPageSize != 1000 {
		t.Errorf("Expected MaxPageSize 1000, got %d", cfg.MaxPageSize)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected LogLevel 'info', got %s", cfg.LogLevel)
	}

	if cfg.ReadTimeout != 30*time.Second {
		t.Errorf("Expected ReadTimeout 30s, got %v", cfg.ReadTimeout)
	}

	if cfg.WriteTimeout != 30*time.Second {
		t.Errorf("Expected WriteTimeout 30s, got %v", cfg.WriteTimeout)
	}

	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("Expected ShutdownTimeout 10s, got %v", cfg.ShutdownTimeout)
	}

	if cfg.DBMaxOpenConns != 25 {
		t.Errorf("Expected DBMaxOpenConns 25, got %d", cfg.DBMaxOpenConns)
	}

	if cfg.DBMaxIdleConns != 5 {
		t.Errorf("Expected DBMaxIdleConns 5, got %d", cfg.DBMaxIdleConns)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	os.Setenv("GATEWAY_HTTP_PORT", "9090")
	os.Setenv("GATEWAY_DATABASE_URL", "postgres://custom:custom@localhost/db")
	os.Setenv("GATEWAY_LOG_LEVEL", "debug")
	os.Setenv("GATEWAY_DEFAULT_PAGE_SIZE", "50")
	os.Setenv("GATEWAY_MAX_PAGE_SIZE", "500")

	defer func() {
		os.Unsetenv("GATEWAY_HTTP_PORT")
		os.Unsetenv("GATEWAY_DATABASE_URL")
		os.Unsetenv("GATEWAY_LOG_LEVEL")
		os.Unsetenv("GATEWAY_DEFAULT_PAGE_SIZE")
		os.Unsetenv("GATEWAY_MAX_PAGE_SIZE")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.HTTPPort != 9090 {
		t.Errorf("Expected HTTPPort 9090, got %d", cfg.HTTPPort)
	}

	if cfg.DatabaseURL != "postgres://custom:custom@localhost/db" {
		t.Errorf("Expected custom DatabaseURL, got %s", cfg.DatabaseURL)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("Expected LogLevel 'debug', got %s", cfg.LogLevel)
	}

	if cfg.DefaultPageSize != 50 {
		t.Errorf("Expected DefaultPageSize 50, got %d", cfg.DefaultPageSize)
	}

	if cfg.MaxPageSize != 500 {
		t.Errorf("Expected MaxPageSize 500, got %d", cfg.MaxPageSize)
	}
}

func TestConfig_Validate(t *testing.T) {
	cfg := &Config{
		HTTPPort:        8080,
		DefaultPageSize: 100,
		MaxPageSize:     1000,
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Unexpected validation error: %v", err)
	}
}
