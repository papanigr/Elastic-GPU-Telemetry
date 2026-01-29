//go:build integration

package integration

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEndToEndTelemetryFlow verifies the complete data flow:
// Publish to MQ -> Collector consumes -> Persists to DB -> Gateway serves via API
func TestEndToEndTelemetryFlow(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	// Skip if services aren't running
	RequireServicesHealthy(t, client)

	// Create a unique test telemetry record
	testTimestamp := time.Now().Format(time.RFC3339)
	testRecord := map[string]interface{}{
		"timestamp":   testTimestamp,
		"metric_name": "DCGM_FI_DEV_GPU_UTIL",
		"gpu_index":   0,
		"device":      "nvidia0",
		"uuid":        "GPU-E2E-TEST-0001-0001-0001-000000000001",
		"model_name":  "NVIDIA H100 80GB HBM3",
		"hostname":    "e2e-test-host",
		"value":       99.9,
	}

	// Step 1: Publish message to MQ
	t.Log("Step 1: Publishing telemetry to MQ...")
	pubResp, err := client.PublishMessage("gpu-telemetry", testRecord)
	require.NoError(t, err)
	assert.Equal(t, "ok", pubResp.Status)
	assert.NotEmpty(t, pubResp.MessageID)
	t.Logf("Published message ID: %s", pubResp.MessageID)

	// Step 2: Wait for collector to process (poll for data in Gateway)
	t.Log("Step 2: Waiting for data to appear in Gateway API...")

	var foundGPU bool
	var gpuUUID string

	WaitForCondition(t, func() bool {
		gpuList, err := client.GetGPUs()
		if err != nil {
			t.Logf("Error getting GPUs: %v", err)
			return false
		}

		for _, gpu := range gpuList.GPUs {
			if gpu.UUID == "GPU-E2E-TEST-0001-0001-0001-000000000001" {
				foundGPU = true
				gpuUUID = gpu.UUID
				return true
			}
		}
		return false
	}, 30*time.Second, "GPU to appear in API")

	assert.True(t, foundGPU, "Expected GPU to be found in API response")

	// Step 3: Query telemetry for the GPU
	t.Log("Step 3: Querying telemetry for the GPU...")
	telemetry, err := client.GetTelemetry(gpuUUID, "", "")
	require.NoError(t, err)

	assert.GreaterOrEqual(t, telemetry.Count, 1)
	t.Logf("Found %d telemetry records", telemetry.Count)

	// Verify the record content
	found := false
	for _, record := range telemetry.Telemetry {
		if record.UUID == gpuUUID && record.MetricName == "DCGM_FI_DEV_GPU_UTIL" {
			assert.Equal(t, 99.9, record.Value)
			assert.Equal(t, "e2e-test-host", record.Hostname)
			found = true
			break
		}
	}
	assert.True(t, found, "Expected telemetry record not found")

	t.Log("End-to-end test passed!")
}

// TestMultipleGPUTelemetry verifies telemetry from multiple GPUs is handled correctly.
func TestMultipleGPUTelemetry(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	RequireServicesHealthy(t, client)

	// Publish telemetry for 3 different GPUs
	gpuIDs := []string{
		"GPU-MULTI-TEST-0001-0001-0001-000000000001",
		"GPU-MULTI-TEST-0002-0002-0002-000000000002",
		"GPU-MULTI-TEST-0003-0003-0003-000000000003",
	}

	t.Log("Publishing telemetry for multiple GPUs...")
	for i, gpuID := range gpuIDs {
		record := map[string]interface{}{
			"timestamp":   time.Now().Format(time.RFC3339),
			"metric_name": "DCGM_FI_DEV_GPU_UTIL",
			"gpu_index":   i,
			"device":      "nvidia" + string(rune('0'+i)),
			"uuid":        gpuID,
			"model_name":  "NVIDIA H100 80GB HBM3",
			"hostname":    "multi-test-host",
			"value":       float64(50 + i*10),
		}

		_, err := client.PublishMessage("gpu-telemetry", record)
		require.NoError(t, err)
	}

	// Wait for processing
	t.Log("Waiting for data to be processed...")
	time.Sleep(10 * time.Second)

	// Verify all GPUs appear
	gpuList, err := client.GetGPUs()
	require.NoError(t, err)

	foundCount := 0
	for _, gpu := range gpuList.GPUs {
		for _, expectedID := range gpuIDs {
			if gpu.UUID == expectedID {
				foundCount++
				break
			}
		}
	}

	assert.GreaterOrEqual(t, foundCount, len(gpuIDs), "Expected all test GPUs to be found")
	t.Logf("Found %d/%d test GPUs", foundCount, len(gpuIDs))
}

// TestTelemetryOrdering verifies telemetry is ordered by timestamp descending.
func TestTelemetryOrdering(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	RequireServicesHealthy(t, client)

	gpuID := "GPU-ORDER-TEST-0001-0001-0001-000000000001"

	// Publish multiple records with different timestamps
	t.Log("Publishing ordered telemetry records...")
	for i := 0; i < 5; i++ {
		record := map[string]interface{}{
			"timestamp":   time.Now().Add(time.Duration(i) * time.Second).Format(time.RFC3339),
			"metric_name": "DCGM_FI_DEV_GPU_UTIL",
			"gpu_index":   0,
			"device":      "nvidia0",
			"uuid":        gpuID,
			"model_name":  "NVIDIA H100 80GB HBM3",
			"hostname":    "order-test-host",
			"value":       float64(i * 10),
		}

		_, err := client.PublishMessage("gpu-telemetry", record)
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for processing
	time.Sleep(10 * time.Second)

	// Get telemetry
	telemetry, err := client.GetTelemetry(gpuID, "", "")
	require.NoError(t, err)

	if len(telemetry.Telemetry) < 2 {
		t.Skip("Not enough telemetry records for ordering test")
	}

	// Verify descending order (newest first)
	for i := 0; i < len(telemetry.Telemetry)-1; i++ {
		t1, _ := time.Parse(time.RFC3339, telemetry.Telemetry[i].Timestamp)
		t2, _ := time.Parse(time.RFC3339, telemetry.Telemetry[i+1].Timestamp)

		assert.True(t, t1.After(t2) || t1.Equal(t2),
			"Telemetry should be ordered by timestamp descending: %s should be >= %s",
			telemetry.Telemetry[i].Timestamp, telemetry.Telemetry[i+1].Timestamp)
	}

	t.Log("Ordering test passed!")
}

// TestBatchTelemetryProcessing verifies batch processing of multiple telemetry records.
func TestBatchTelemetryProcessing(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	RequireServicesHealthy(t, client)

	gpuID := "GPU-BATCH-TEST-0001-0001-0001-000000000001"
	batchSize := 20

	t.Logf("Publishing batch of %d telemetry records...", batchSize)
	for i := 0; i < batchSize; i++ {
		record := map[string]interface{}{
			"timestamp":   time.Now().Format(time.RFC3339Nano),
			"metric_name": "DCGM_FI_DEV_GPU_UTIL",
			"gpu_index":   0,
			"device":      "nvidia0",
			"uuid":        gpuID,
			"model_name":  "NVIDIA H100 80GB HBM3",
			"hostname":    "batch-test-host",
			"value":       float64(i),
		}

		_, err := client.PublishMessage("gpu-telemetry", record)
		require.NoError(t, err)
	}

	// Wait for processing
	t.Log("Waiting for batch to be processed...")
	time.Sleep(15 * time.Second)

	// Verify all records were processed
	telemetry, err := client.GetTelemetry(gpuID, "", "")
	require.NoError(t, err)

	assert.GreaterOrEqual(t, telemetry.TotalCount, batchSize,
		"Expected at least %d records, got %d", batchSize, telemetry.TotalCount)

	t.Logf("Batch test passed! Found %d records", telemetry.TotalCount)
}

// TestTelemetryPayloadIntegrity verifies telemetry values are preserved through the pipeline.
func TestTelemetryPayloadIntegrity(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	RequireServicesHealthy(t, client)

	gpuID := "GPU-INTEGRITY-TEST-0001-0001-0001-000000000001"

	// Create record with specific values
	expectedValue := 87.654
	expectedMetric := "DCGM_FI_DEV_MEM_COPY_UTIL"
	expectedHostname := "integrity-test-host"
	expectedModel := "NVIDIA A100 80GB"

	record := map[string]interface{}{
		"timestamp":   time.Now().Format(time.RFC3339),
		"metric_name": expectedMetric,
		"gpu_index":   3,
		"device":      "nvidia3",
		"uuid":        gpuID,
		"model_name":  expectedModel,
		"hostname":    expectedHostname,
		"value":       expectedValue,
	}

	t.Log("Publishing telemetry with specific values...")
	_, err := client.PublishMessage("gpu-telemetry", record)
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(10 * time.Second)

	// Retrieve and verify
	telemetry, err := client.GetTelemetry(gpuID, "", "")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(telemetry.Telemetry), 1)

	// Find our record
	var found *TelemetryRecord
	for _, r := range telemetry.Telemetry {
		if r.MetricName == expectedMetric {
			found = &r
			break
		}
	}

	require.NotNil(t, found, "Expected telemetry record not found")

	// Verify all fields
	assert.Equal(t, expectedValue, found.Value, "Value mismatch")
	assert.Equal(t, expectedMetric, found.MetricName, "Metric name mismatch")
	assert.Equal(t, expectedHostname, found.Hostname, "Hostname mismatch")
	assert.Equal(t, expectedModel, found.ModelName, "Model name mismatch")
	assert.Equal(t, 3, found.GPUIndex, "GPU index mismatch")
	assert.Equal(t, gpuID, found.UUID, "UUID mismatch")

	t.Log("Payload integrity test passed!")
}

// Helper to pretty print JSON for debugging
func prettyJSON(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
