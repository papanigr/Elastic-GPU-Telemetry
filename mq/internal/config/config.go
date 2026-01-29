// Package config provides configuration for the MQ broker.
package config

import (
	"time"

	"github.com/spf13/viper"
)

// Config holds the configuration for the MQ Broker.
type Config struct {
	// GRPCPort is the gRPC server port (for service communication).
	GRPCPort int `mapstructure:"grpc_port"`

	// HTTPPort is the HTTP server port (for admin APIs).
	HTTPPort int `mapstructure:"http_port"`

	// MaxQueueSize is the maximum number of messages per topic.
	MaxQueueSize int `mapstructure:"max_queue_size"`

	// MaxMessageAge is the maximum age of a message before it's dropped.
	MaxMessageAge time.Duration `mapstructure:"max_message_age"`

	// AckTimeout is the time to wait for an ACK before redelivering.
	AckTimeout time.Duration `mapstructure:"ack_timeout"`

	// CleanupInterval is how often to run the cleanup routine.
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`

	// LogLevel is the logging level.
	LogLevel string `mapstructure:"log_level"`

	// DLQ Configuration

	// MaxRetries is the maximum number of delivery attempts before moving to DLQ.
	MaxRetries int `mapstructure:"max_retries"`

	// DLQMaxRetries is the maximum number of DLQ retry attempts before marking as Dead.
	DLQMaxRetries int `mapstructure:"dlq_max_retries"`

	// DLQRetryDelay is the delay between DLQ auto-retry attempts.
	DLQRetryDelay time.Duration `mapstructure:"dlq_retry_delay"`

	// DLQEnabled enables the Dead Letter Queue functionality.
	DLQEnabled bool `mapstructure:"dlq_enabled"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		GRPCPort:        8081,
		HTTPPort:        8082,
		MaxQueueSize:    10000,
		MaxMessageAge:   5 * time.Minute,
		AckTimeout:      30 * time.Second,
		CleanupInterval: 10 * time.Second,
		LogLevel:        "info",
		// DLQ defaults
		MaxRetries:    3,
		DLQMaxRetries: 3,
		DLQRetryDelay: 5 * time.Minute,
		DLQEnabled:    true,
	}
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() (Config, error) {
	config := DefaultConfig()

	v := viper.New()

	// Set defaults
	v.SetDefault("grpc_port", config.GRPCPort)
	v.SetDefault("http_port", config.HTTPPort)
	v.SetDefault("max_queue_size", config.MaxQueueSize)
	v.SetDefault("max_message_age", config.MaxMessageAge)
	v.SetDefault("ack_timeout", config.AckTimeout)
	v.SetDefault("cleanup_interval", config.CleanupInterval)
	v.SetDefault("log_level", config.LogLevel)
	// DLQ defaults
	v.SetDefault("max_retries", config.MaxRetries)
	v.SetDefault("dlq_max_retries", config.DLQMaxRetries)
	v.SetDefault("dlq_retry_delay", config.DLQRetryDelay)
	v.SetDefault("dlq_enabled", config.DLQEnabled)

	// Environment variable bindings
	v.SetEnvPrefix("MQ")
	v.AutomaticEnv()

	v.BindEnv("grpc_port", "MQ_GRPC_PORT")
	v.BindEnv("http_port", "MQ_HTTP_PORT")
	v.BindEnv("max_queue_size", "MQ_MAX_QUEUE_SIZE")
	v.BindEnv("max_message_age", "MQ_MAX_MESSAGE_AGE")
	v.BindEnv("ack_timeout", "MQ_ACK_TIMEOUT")
	v.BindEnv("cleanup_interval", "MQ_CLEANUP_INTERVAL")
	v.BindEnv("log_level", "MQ_LOG_LEVEL")
	// DLQ env bindings
	v.BindEnv("max_retries", "MQ_MAX_RETRIES")
	v.BindEnv("dlq_max_retries", "MQ_DLQ_MAX_RETRIES")
	v.BindEnv("dlq_retry_delay", "MQ_DLQ_RETRY_DELAY")
	v.BindEnv("dlq_enabled", "MQ_DLQ_ENABLED")

	// Try to read config file
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/mq/")
	v.AddConfigPath(".")
	_ = v.ReadInConfig()

	if err := v.Unmarshal(&config); err != nil {
		return config, err
	}

	return config, nil
}
