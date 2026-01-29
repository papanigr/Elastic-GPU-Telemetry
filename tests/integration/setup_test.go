//go:build integration

// Package integration contains system/integration tests for the GPU Telemetry Pipeline.
// These tests verify the end-to-end behavior of all components working together.
//
// Run with: go test -tags=integration -v ./integration/...
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"testing"
	"time"
)

// TestConfig holds the configuration for integration tests.
type TestConfig struct {
	GatewayURL  string
	MQURL       string
	MQGRPCAddr  string
	PostgresURL string
	Timeout     time.Duration
}

// DefaultTestConfig returns the default test configuration.
// These match the ports in Makefile and docker-compose.test.yml
// External ports: Gateway=8085, MQ HTTP=8083, MQ gRPC=8084, Postgres=5433
// Using 127.0.0.1 instead of localhost to ensure IPv4 (containers bind to 0.0.0.0)
func DefaultTestConfig() TestConfig {
	return TestConfig{
		GatewayURL:  getEnv("TEST_GATEWAY_URL", "http://127.0.0.1:8085"),
		MQURL:       getEnv("TEST_MQ_URL", "http://127.0.0.1:8083"),
		MQGRPCAddr:  getEnv("TEST_MQ_GRPC_ADDR", "127.0.0.1:8084"),
		PostgresURL: getEnv("TEST_POSTGRES_URL", "postgres://postgres:postgres@127.0.0.1:5433/telemetry_test?sslmode=disable"),
		Timeout:     30 * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// TestClient wraps HTTP client with helper methods for testing.
type TestClient struct {
	config     TestConfig
	httpClient *http.Client
}

// NewTestClient creates a new test client.
func NewTestClient(cfg TestConfig) *TestClient {
	return &TestClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// --- Gateway API Methods ---

// GPUListResponse represents the response from GET /api/v1/gpus
type GPUListResponse struct {
	GPUs  []GPU `json:"gpus"`
	Count int   `json:"count"`
}

// GPU represents a GPU in the response.
type GPU struct {
	UUID      string `json:"uuid"`
	GPUIndex  int    `json:"gpu_index"`
	Device    string `json:"device"`
	ModelName string `json:"model_name"`
	Hostname  string `json:"hostname"`
	LastSeen  string `json:"last_seen"`
}

// TelemetryListResponse represents the response from GET /api/v1/gpus/{id}/telemetry
type TelemetryListResponse struct {
	Telemetry  []TelemetryRecord `json:"telemetry"`
	Count      int               `json:"count"`
	TotalCount int               `json:"total_count"`
	GPUUUID    string            `json:"gpu_uuid"`
	StartTime  *string           `json:"start_time,omitempty"`
	EndTime    *string           `json:"end_time,omitempty"`
}

// TelemetryRecord represents a telemetry record.
type TelemetryRecord struct {
	ID         string  `json:"id"`
	Timestamp  string  `json:"timestamp"`
	MetricName string  `json:"metric_name"`
	GPUIndex   int     `json:"gpu_index"`
	Device     string  `json:"device"`
	UUID       string  `json:"uuid"`
	ModelName  string  `json:"model_name"`
	Hostname   string  `json:"hostname"`
	Value      float64 `json:"value"`
	ReceivedAt string  `json:"received_at"`
}

// HealthResponse represents health check response.
type HealthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database,omitempty"`
	Version  string `json:"version,omitempty"`
	Time     string `json:"time,omitempty"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Details string `json:"details,omitempty"`
}

// GetGatewayHealth checks gateway health.
func (c *TestClient) GetGatewayHealth() (*HealthResponse, error) {
	resp, err := c.httpClient.Get(c.config.GatewayURL + "/health")
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway health: %w", err)
	}
	defer resp.Body.Close()

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, fmt.Errorf("failed to decode health response: %w", err)
	}

	return &health, nil
}

// GetGPUs fetches the list of GPUs.
func (c *TestClient) GetGPUs() (*GPUListResponse, error) {
	resp, err := c.httpClient.Get(c.config.GatewayURL + "/api/v1/gpus")
	if err != nil {
		return nil, fmt.Errorf("failed to get GPUs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var gpuList GPUListResponse
	if err := json.NewDecoder(resp.Body).Decode(&gpuList); err != nil {
		return nil, fmt.Errorf("failed to decode GPU list: %w", err)
	}

	return &gpuList, nil
}

// GetTelemetry fetches telemetry for a specific GPU.
func (c *TestClient) GetTelemetry(gpuUUID string, startTime, endTime string) (*TelemetryListResponse, error) {
	baseURL := fmt.Sprintf("%s/api/v1/gpus/%s/telemetry", c.config.GatewayURL, gpuUUID)

	// Add query parameters if provided (properly URL-encoded)
	params := neturl.Values{}
	if startTime != "" {
		params.Set("start_time", startTime)
	}
	if endTime != "" {
		params.Set("end_time", endTime)
	}
	if len(params) > 0 {
		baseURL += "?" + params.Encode()
	}

	resp, err := c.httpClient.Get(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get telemetry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var telemetry TelemetryListResponse
	if err := json.NewDecoder(resp.Body).Decode(&telemetry); err != nil {
		return nil, fmt.Errorf("failed to decode telemetry: %w", err)
	}

	return &telemetry, nil
}

// --- MQ API Methods ---

// TopicStats represents MQ topic statistics.
type TopicStats struct {
	Topic            string `json:"topic"`
	QueueSize        int    `json:"queue_size"`
	MaxSize          int    `json:"max_size"`
	PendingMessages  int    `json:"pending_messages"`
	ConsumerGroups   int    `json:"consumer_groups"`
	TotalConsumers   int    `json:"total_consumers"`
	DroppedMessages  int64  `json:"dropped_messages"`
	QueueUtilization string `json:"queue_utilization"`
}

// PublishRequest represents a message to publish.
type PublishRequest struct {
	ID        string          `json:"id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp string          `json:"timestamp,omitempty"`
}

// PublishResponse represents the response from publishing.
// Matches mq/pkg/models/message.go PublishResponse
type PublishResponse struct {
	Status    string `json:"status"` // "ok" on success
	MessageID string `json:"message_id"`
}

// GetMQHealth checks MQ health.
func (c *TestClient) GetMQHealth() (*HealthResponse, error) {
	resp, err := c.httpClient.Get(c.config.MQURL + "/health")
	if err != nil {
		return nil, fmt.Errorf("failed to get MQ health: %w", err)
	}
	defer resp.Body.Close()

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, fmt.Errorf("failed to decode health response: %w", err)
	}

	return &health, nil
}

// GetTopicStats fetches topic statistics.
// Note: Stats are under /admin not /api/v1 per mq/internal/api/router.go
func (c *TestClient) GetTopicStats(topic string) (*TopicStats, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/admin/topics/%s/stats", c.config.MQURL, topic))
	if err != nil {
		return nil, fmt.Errorf("failed to get topic stats: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var stats TopicStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to decode topic stats: %w", err)
	}

	return &stats, nil
}

// PublishMessage publishes a message to a topic.
func (c *TestClient) PublishMessage(topic string, payload interface{}) (*PublishResponse, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req := PublishRequest{
		Payload:   payloadBytes,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(
		fmt.Sprintf("%s/api/v1/topics/%s/messages", c.config.MQURL, topic),
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to publish message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var pubResp PublishResponse
	if err := json.NewDecoder(resp.Body).Decode(&pubResp); err != nil {
		return nil, fmt.Errorf("failed to decode publish response: %w", err)
	}

	return &pubResp, nil
}

// --- Test Helpers ---

// WaitForServices waits for all services to be healthy.
func WaitForServices(t *testing.T, client *TestClient, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)

	// Wait for Gateway
	for time.Now().Before(deadline) {
		health, err := client.GetGatewayHealth()
		if err == nil && health.Status == "healthy" {
			t.Log("Gateway is healthy")
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Wait for MQ
	for time.Now().Before(deadline) {
		health, err := client.GetMQHealth()
		if err == nil && health.Status == "healthy" {
			t.Log("MQ is healthy")
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Final check
	if time.Now().After(deadline) {
		t.Fatal("Timeout waiting for services to be healthy")
	}
}

// WaitForCondition waits for a condition to be true.
func WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration, msg string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("Timeout waiting for condition: %s", msg)
}

// RequireServicesHealthy checks that all services are healthy before running a test.
func RequireServicesHealthy(t *testing.T, client *TestClient) {
	t.Helper()

	// Check Gateway
	gwHealth, err := client.GetGatewayHealth()
	if err != nil {
		t.Skipf("Gateway not available: %v (run docker-compose first)", err)
	}
	if gwHealth.Status != "healthy" {
		t.Skipf("Gateway not healthy: %s", gwHealth.Status)
	}

	// Check MQ
	mqHealth, err := client.GetMQHealth()
	if err != nil {
		t.Skipf("MQ not available: %v (run docker-compose first)", err)
	}
	if mqHealth.Status != "healthy" {
		t.Skipf("MQ not healthy: %s", mqHealth.Status)
	}
}
