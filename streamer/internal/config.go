// Package internal provides the internal implementation of the telemetry streamer.
package internal

import (
	"time"

	"github.com/spf13/viper"
)

// Config holds the configuration for the Telemetry Streamer.
type Config struct {
	// CSVFilePath is the path to the CSV file containing telemetry data.
	CSVFilePath string `mapstructure:"csv_file_path"`

	// MQBrokerURL is the URL of the message queue broker.
	MQBrokerURL string `mapstructure:"mq_broker_url"`

	// Topic is the message queue topic to publish telemetry to.
	Topic string `mapstructure:"topic"`

	// StreamInterval is the interval between streaming each telemetry record.
	StreamInterval time.Duration `mapstructure:"stream_interval"`

	// BatchSize is the number of records to read and publish in each batch.
	BatchSize int `mapstructure:"batch_size"`

	// MQEnabled indicates whether to publish to the message queue.
	// If false, records are logged but not published.
	MQEnabled bool `mapstructure:"mq_enabled"`

	// LogLevel is the logging level (debug, info, warn, error).
	LogLevel string `mapstructure:"log_level"`
}

// DefaultConfig returns the default configuration values.
func DefaultConfig() Config {
	return Config{
		CSVFilePath:    "/data/dcgm_metrics.csv",
		MQBrokerURL:    "http://mq-broker:8081",
		Topic:          "gpu-telemetry",
		StreamInterval: 100 * time.Millisecond,
		BatchSize:      1,
		MQEnabled:      true,
		LogLevel:       "info",
	}
}

// LoadConfig loads configuration from environment variables and config files.
func LoadConfig() (Config, error) {
	config := DefaultConfig()

	// Set up Viper
	v := viper.New()

	// Set default values
	v.SetDefault("csv_file_path", config.CSVFilePath)
	v.SetDefault("mq_broker_url", config.MQBrokerURL)
	v.SetDefault("topic", config.Topic)
	v.SetDefault("stream_interval", config.StreamInterval)
	v.SetDefault("batch_size", config.BatchSize)
	v.SetDefault("mq_enabled", config.MQEnabled)
	v.SetDefault("log_level", config.LogLevel)

	// Environment variable bindings
	v.SetEnvPrefix("STREAMER")
	v.AutomaticEnv()

	// Bind specific environment variables
	v.BindEnv("csv_file_path", "STREAMER_CSV_FILE_PATH")
	v.BindEnv("mq_broker_url", "STREAMER_MQ_BROKER_URL")
	v.BindEnv("topic", "STREAMER_TOPIC")
	v.BindEnv("stream_interval", "STREAMER_STREAM_INTERVAL")
	v.BindEnv("batch_size", "STREAMER_BATCH_SIZE")
	v.BindEnv("mq_enabled", "STREAMER_MQ_ENABLED")
	v.BindEnv("log_level", "STREAMER_LOG_LEVEL")

	// Try to read config file (optional)
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/streamer/")
	v.AddConfigPath(".")

	// Read config file if it exists (ignore error if not found)
	_ = v.ReadInConfig()

	// Unmarshal into config struct
	if err := v.Unmarshal(&config); err != nil {
		return config, err
	}

	return config, nil
}
