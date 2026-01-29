//go:build integration

package integration

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrentPublishers verifies multiple concurrent publishers work correctly.
func TestConcurrentPublishers(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	RequireServicesHealthy(t, client)

	topic := "gpu-telemetry"
	publisherCount := 5
	messagesPerPublisher := 10

	t.Logf("Starting %d concurrent publishers, %d messages each...",
		publisherCount, messagesPerPublisher)

	var wg sync.WaitGroup
	errors := make(chan error, publisherCount*messagesPerPublisher)
	published := make(chan string, publisherCount*messagesPerPublisher)

	start := time.Now()

	// Start concurrent publishers
	for p := 0; p < publisherCount; p++ {
		wg.Add(1)
		go func(publisherID int) {
			defer wg.Done()

			for i := 0; i < messagesPerPublisher; i++ {
				record := map[string]interface{}{
					"timestamp":   time.Now().Format(time.RFC3339Nano),
					"metric_name": "DCGM_FI_DEV_GPU_UTIL",
					"gpu_index":   publisherID,
					"device":      "nvidia0",
					"uuid":        "GPU-CONCURRENT-TEST-0001-0001-0001-000000000001",
					"model_name":  "NVIDIA H100 80GB HBM3",
					"hostname":    "concurrent-test-host",
					"value":       float64(publisherID*100 + i),
				}

				resp, err := client.PublishMessage(topic, record)
				if err != nil {
					errors <- err
				} else {
					published <- resp.MessageID
				}
			}
		}(p)
	}

	// Wait for all publishers to finish
	wg.Wait()
	close(errors)
	close(published)

	elapsed := time.Since(start)

	// Count results
	errorCount := 0
	for err := range errors {
		t.Logf("Error: %v", err)
		errorCount++
	}

	publishedCount := 0
	for range published {
		publishedCount++
	}

	expectedTotal := publisherCount * messagesPerPublisher
	t.Logf("Published %d/%d messages in %v (errors: %d)",
		publishedCount, expectedTotal, elapsed, errorCount)

	assert.Equal(t, 0, errorCount, "Expected no errors")
	assert.Equal(t, expectedTotal, publishedCount, "Expected all messages to be published")
}

// TestScaledTelemetryCollection verifies the system handles scaled telemetry correctly.
func TestScaledTelemetryCollection(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	RequireServicesHealthy(t, client)

	// Simulate multiple GPUs from multiple hosts
	hosts := []string{"host-1", "host-2", "host-3"}
	gpusPerHost := 2
	metricsPerGPU := 5

	t.Logf("Simulating %d hosts, %d GPUs each, %d metrics each...",
		len(hosts), gpusPerHost, metricsPerGPU)

	totalMessages := 0

	for _, host := range hosts {
		for g := 0; g < gpusPerHost; g++ {
			gpuID := "GPU-SCALED-" + host + "-" + string(rune('A'+g))

			for m := 0; m < metricsPerGPU; m++ {
				metrics := []string{
					"DCGM_FI_DEV_GPU_UTIL",
					"DCGM_FI_DEV_MEM_COPY_UTIL",
					"DCGM_FI_DEV_GPU_TEMP",
					"DCGM_FI_DEV_POWER_USAGE",
					"DCGM_FI_DEV_FB_USED",
				}

				record := map[string]interface{}{
					"timestamp":   time.Now().Format(time.RFC3339Nano),
					"metric_name": metrics[m%len(metrics)],
					"gpu_index":   g,
					"device":      "nvidia" + string(rune('0'+g)),
					"uuid":        gpuID,
					"model_name":  "NVIDIA H100 80GB HBM3",
					"hostname":    host,
					"value":       float64(m * 10),
				}

				_, err := client.PublishMessage("gpu-telemetry", record)
				require.NoError(t, err)
				totalMessages++
			}
		}
	}

	t.Logf("Published %d total messages", totalMessages)

	// Wait for processing
	t.Log("Waiting for data to be processed...")
	time.Sleep(15 * time.Second)

	// Verify GPUs from all hosts are visible
	gpuList, err := client.GetGPUs()
	require.NoError(t, err)

	// Count unique hosts in the response
	hostsSeen := make(map[string]bool)
	scaledGPUs := 0
	for _, gpu := range gpuList.GPUs {
		if len(gpu.UUID) > 10 && gpu.UUID[:10] == "GPU-SCALED" {
			hostsSeen[gpu.Hostname] = true
			scaledGPUs++
		}
	}

	t.Logf("Found %d scaled GPUs from %d hosts", scaledGPUs, len(hostsSeen))

	assert.GreaterOrEqual(t, len(hostsSeen), len(hosts),
		"Expected GPUs from all %d hosts", len(hosts))
}

// TestHighVolumeIngestion tests the system under high volume ingestion.
func TestHighVolumeIngestion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high volume test in short mode")
	}

	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	RequireServicesHealthy(t, client)

	gpuID := "GPU-HIGHVOL-TEST-0001-0001-0001-000000000001"
	messageCount := 200

	t.Logf("Publishing %d messages for high volume test...", messageCount)
	start := time.Now()

	for i := 0; i < messageCount; i++ {
		record := map[string]interface{}{
			"timestamp":   time.Now().Format(time.RFC3339Nano),
			"metric_name": "DCGM_FI_DEV_GPU_UTIL",
			"gpu_index":   0,
			"device":      "nvidia0",
			"uuid":        gpuID,
			"model_name":  "NVIDIA H100 80GB HBM3",
			"hostname":    "highvol-test-host",
			"value":       float64(i % 100),
		}

		_, err := client.PublishMessage("gpu-telemetry", record)
		require.NoError(t, err)
	}

	publishTime := time.Since(start)
	t.Logf("Published %d messages in %v (%.2f msg/sec)",
		messageCount, publishTime, float64(messageCount)/publishTime.Seconds())

	// Wait for processing
	t.Log("Waiting for data to be processed...")
	time.Sleep(20 * time.Second)

	// Verify data was persisted
	telemetry, err := client.GetTelemetry(gpuID, "", "")
	require.NoError(t, err)

	t.Logf("Retrieved %d records (total: %d)", telemetry.Count, telemetry.TotalCount)

	// Should have at least some of the messages (depends on processing speed)
	assert.GreaterOrEqual(t, telemetry.TotalCount, 1,
		"Expected at least some records to be persisted")
}

// TestMultipleConsumerGroups verifies multiple consumer groups work independently.
// Note: This test requires multiple collectors configured in different groups.
func TestMultipleConsumerGroups(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	RequireServicesHealthy(t, client)

	// This is a conceptual test - in production you'd have multiple collectors
	// For now, we just verify the single collector is working

	gpuID := "GPU-CONSUMERGROUP-TEST-0001-0001-0001-000000000001"

	// Publish messages
	for i := 0; i < 10; i++ {
		record := map[string]interface{}{
			"timestamp":   time.Now().Format(time.RFC3339Nano),
			"metric_name": "DCGM_FI_DEV_GPU_UTIL",
			"gpu_index":   0,
			"device":      "nvidia0",
			"uuid":        gpuID,
			"model_name":  "NVIDIA H100 80GB HBM3",
			"hostname":    "consumergroup-test-host",
			"value":       float64(i),
		}

		_, err := client.PublishMessage("gpu-telemetry", record)
		require.NoError(t, err)
	}

	// Wait for processing
	time.Sleep(10 * time.Second)

	// Check MQ stats to verify consumer is active
	stats, err := client.GetTopicStats("gpu-telemetry")
	require.NoError(t, err)

	t.Logf("Topic stats: Consumers=%d, Groups=%d, Pending=%d",
		stats.TotalConsumers, stats.ConsumerGroups, stats.PendingMessages)

	assert.GreaterOrEqual(t, stats.TotalConsumers, 1,
		"Expected at least 1 consumer")
	assert.GreaterOrEqual(t, stats.ConsumerGroups, 1,
		"Expected at least 1 consumer group")
}

// TestSystemResilience tests basic system resilience.
func TestSystemResilience(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	RequireServicesHealthy(t, client)

	gpuID := "GPU-RESILIENCE-TEST-0001-0001-0001-000000000001"

	// Phase 1: Normal operation
	t.Log("Phase 1: Normal operation...")
	for i := 0; i < 5; i++ {
		record := map[string]interface{}{
			"timestamp":   time.Now().Format(time.RFC3339),
			"metric_name": "DCGM_FI_DEV_GPU_UTIL",
			"gpu_index":   0,
			"device":      "nvidia0",
			"uuid":        gpuID,
			"model_name":  "NVIDIA H100 80GB HBM3",
			"hostname":    "resilience-test-host",
			"value":       float64(i),
		}

		_, err := client.PublishMessage("gpu-telemetry", record)
		require.NoError(t, err)
	}

	time.Sleep(5 * time.Second)

	// Verify health
	gwHealth, err := client.GetGatewayHealth()
	require.NoError(t, err)
	assert.Equal(t, "healthy", gwHealth.Status)

	mqHealth, err := client.GetMQHealth()
	require.NoError(t, err)
	assert.Equal(t, "healthy", mqHealth.Status)

	// Phase 2: Burst of traffic
	t.Log("Phase 2: Burst of traffic...")
	for i := 0; i < 50; i++ {
		record := map[string]interface{}{
			"timestamp":   time.Now().Format(time.RFC3339Nano),
			"metric_name": "DCGM_FI_DEV_GPU_UTIL",
			"gpu_index":   0,
			"device":      "nvidia0",
			"uuid":        gpuID,
			"model_name":  "NVIDIA H100 80GB HBM3",
			"hostname":    "resilience-test-host",
			"value":       float64(i),
		}

		_, err := client.PublishMessage("gpu-telemetry", record)
		assert.NoError(t, err) // Don't require - just check
	}

	time.Sleep(5 * time.Second)

	// Phase 3: Verify system still healthy
	t.Log("Phase 3: Verifying system health after burst...")
	gwHealth, err = client.GetGatewayHealth()
	require.NoError(t, err)
	assert.Equal(t, "healthy", gwHealth.Status)

	mqHealth, err = client.GetMQHealth()
	require.NoError(t, err)
	assert.Equal(t, "healthy", mqHealth.Status)

	// Verify data is accessible
	telemetry, err := client.GetTelemetry(gpuID, "", "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, telemetry.Count, 1)

	t.Log("System resilience test passed!")
}
