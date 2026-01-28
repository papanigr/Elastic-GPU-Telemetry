package internal

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds the collector configuration.
type Config struct {
	// MQBrokerAddr is the gRPC address of the message queue broker (host:port).
	MQBrokerAddr string `mapstructure:"mq_broker_addr"`

	// Topic is the message queue topic to consume from.
	Topic string `mapstructure:"topic"`

	// ConsumerGroup is the consumer group name for load balancing.
	ConsumerGroup string `mapstructure:"consumer_group"`

	// ConsumerID is the unique identifier for this consumer instance.
	// If empty, a UUID will be generated.
	ConsumerID string `mapstructure:"consumer_id"`

	// BatchSize is the number of messages to consume in each batch.
	BatchSize int `mapstructure:"batch_size"`

	// PollInterval is the interval between consume requests.
	PollInterval time.Duration `mapstructure:"poll_interval"`

	// NumWorkers is the number of worker goroutines for concurrent processing.
	NumWorkers int `mapstructure:"num_workers"`

	// DBEnabled indicates whether to persist to database.
	// If false, records are logged but not persisted.
	DBEnabled bool `mapstructure:"db_enabled"`

	// DatabaseURL is the PostgreSQL connection string.
	DatabaseURL string `mapstructure:"database_url"`

	// LogLevel is the logging level (debug, info, warn, error).
	LogLevel string `mapstructure:"log_level"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		MQBrokerAddr:  "mq-broker:8081",
		Topic:         "gpu-telemetry",
		ConsumerGroup: "telemetry-collectors",
		ConsumerID:    "", // Will be generated
		BatchSize:     10,
		PollInterval:  1 * time.Second,
		NumWorkers:    3,
		DBEnabled:     false,
		DatabaseURL:   "postgres://postgres:postgres@localhost:5432/telemetry?sslmode=disable",
		LogLevel:      "info",
	}
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() (Config, error) {
	config := DefaultConfig()

	v := viper.New()
	v.SetEnvPrefix("COLLECTOR")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Set default values
	v.SetDefault("mq_broker_addr", config.MQBrokerAddr)
	v.SetDefault("topic", config.Topic)
	v.SetDefault("consumer_group", config.ConsumerGroup)
	v.SetDefault("consumer_id", config.ConsumerID)
	v.SetDefault("batch_size", config.BatchSize)
	v.SetDefault("poll_interval", config.PollInterval)
	v.SetDefault("num_workers", config.NumWorkers)
	v.SetDefault("db_enabled", config.DBEnabled)
	v.SetDefault("database_url", config.DatabaseURL)
	v.SetDefault("log_level", config.LogLevel)

	if err := v.Unmarshal(&config); err != nil {
		return config, err
	}

	return config, nil
}
