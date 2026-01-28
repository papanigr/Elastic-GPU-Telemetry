package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds the gateway configuration.
type Config struct {
	// Server settings
	HTTPPort         int           `mapstructure:"http_port"`
	ReadTimeout      time.Duration `mapstructure:"read_timeout"`
	WriteTimeout     time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout  time.Duration `mapstructure:"shutdown_timeout"`

	// Database settings
	DatabaseURL     string        `mapstructure:"database_url"`
	DBMaxOpenConns  int           `mapstructure:"db_max_open_conns"`
	DBMaxIdleConns  int           `mapstructure:"db_max_idle_conns"`
	DBConnMaxLife   time.Duration `mapstructure:"db_conn_max_life"`

	// Pagination defaults
	DefaultPageSize int `mapstructure:"default_page_size"`
	MaxPageSize     int `mapstructure:"max_page_size"`

	// Logging
	LogLevel string `mapstructure:"log_level"`
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("http_port", 8080)
	v.SetDefault("read_timeout", 30*time.Second)
	v.SetDefault("write_timeout", 30*time.Second)
	v.SetDefault("shutdown_timeout", 10*time.Second)

	v.SetDefault("database_url", "postgres://postgres:postgres@localhost:5432/telemetry?sslmode=disable")
	v.SetDefault("db_max_open_conns", 25)
	v.SetDefault("db_max_idle_conns", 5)
	v.SetDefault("db_conn_max_life", 5*time.Minute)

	v.SetDefault("default_page_size", 100)
	v.SetDefault("max_page_size", 1000)

	v.SetDefault("log_level", "info")

	// Environment variable prefix
	v.SetEnvPrefix("GATEWAY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// Add validation logic if needed
	return nil
}
