//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGatewayHealth verifies the gateway health endpoint.
func TestGatewayHealth(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	health, err := client.GetGatewayHealth()
	if err != nil {
		t.Skipf("Gateway not available: %v", err)
	}

	assert.Equal(t, "healthy", health.Status)
	assert.Equal(t, "healthy", health.Database)
	assert.NotEmpty(t, health.Version)
}

// TestGetGPUsEndpoint verifies the GET /api/v1/gpus endpoint.
func TestGetGPUsEndpoint(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	RequireServicesHealthy(t, client)

	// First, publish some test data so we have GPUs to query
	gpuID := "GPU-GETLIST-TEST-0001-0001-0001-000000000001"
	record := map[string]interface{}{
		"timestamp":   time.Now().Format(time.RFC3339),
		"metric_name": "DCGM_FI_DEV_GPU_UTIL",
		"gpu_index":   0,
		"device":      "nvidia0",
		"uuid":        gpuID,
		"model_name":  "NVIDIA H100 80GB HBM3",
		"hostname":    "getlist-test-host",
		"value":       50.0,
	}

	_, err := client.PublishMessage("gpu-telemetry", record)
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(10 * time.Second)

	gpuList, err := client.GetGPUs()
	require.NoError(t, err)

	// Response structure should be correct (GPUs may be empty slice, not nil)
	require.NotNil(t, gpuList)
	assert.Equal(t, len(gpuList.GPUs), gpuList.Count)
	assert.GreaterOrEqual(t, gpuList.Count, 1, "Expected at least 1 GPU after publishing")

	// Log for debugging
	t.Logf("Found %d GPUs", gpuList.Count)
	for _, gpu := range gpuList.GPUs {
		t.Logf("  GPU: %s (%s) on %s", gpu.UUID, gpu.ModelName, gpu.Hostname)
	}
}

// TestGetTelemetryEndpoint verifies the GET /api/v1/gpus/{id}/telemetry endpoint.
func TestGetTelemetryEndpoint(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	RequireServicesHealthy(t, client)

	// First, ensure we have at least one GPU
	gpuID := "GPU-API-TEST-0001-0001-0001-000000000001"

	// Publish some test data
	record := map[string]interface{}{
		"timestamp":   time.Now().Format(time.RFC3339),
		"metric_name": "DCGM_FI_DEV_GPU_UTIL",
		"gpu_index":   0,
		"device":      "nvidia0",
		"uuid":        gpuID,
		"model_name":  "NVIDIA H100 80GB HBM3",
		"hostname":    "api-test-host",
		"value":       42.0,
	}

	_, err := client.PublishMessage("gpu-telemetry", record)
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(10 * time.Second)

	// Get telemetry
	telemetry, err := client.GetTelemetry(gpuID, "", "")
	require.NoError(t, err)

	assert.NotNil(t, telemetry.Telemetry)
	assert.GreaterOrEqual(t, telemetry.Count, 1)
	assert.Equal(t, gpuID, telemetry.GPUUUID)

	t.Logf("Retrieved %d telemetry records (total: %d)", telemetry.Count, telemetry.TotalCount)
}

// TestTelemetryTimeFiltering verifies time window filtering works correctly.
func TestTelemetryTimeFiltering(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	RequireServicesHealthy(t, client)

	gpuID := "GPU-TIMEFILTER-TEST-0001-0001-0001-000000000001"

	// Record timestamps for verification
	beforeTime := time.Now()

	// Publish data
	for i := 0; i < 5; i++ {
		record := map[string]interface{}{
			"timestamp":   time.Now().Format(time.RFC3339),
			"metric_name": "DCGM_FI_DEV_GPU_UTIL",
			"gpu_index":   0,
			"device":      "nvidia0",
			"uuid":        gpuID,
			"model_name":  "NVIDIA H100 80GB HBM3",
			"hostname":    "timefilter-test-host",
			"value":       float64(i * 10),
		}

		_, err := client.PublishMessage("gpu-telemetry", record)
		require.NoError(t, err)
		time.Sleep(200 * time.Millisecond)
	}

	afterTime := time.Now()

	// Wait for processing
	time.Sleep(10 * time.Second)

	// Test: Query with time window that includes all records
	// Use RFC3339 format for consistency with the API
	startTime := beforeTime.Add(-1 * time.Minute).Format(time.RFC3339)
	endTime := afterTime.Add(1 * time.Minute).Format(time.RFC3339)

	telemetry, err := client.GetTelemetry(gpuID, startTime, endTime)
	require.NoError(t, err)

	t.Logf("Time filter test: found %d records between %s and %s",
		telemetry.Count, startTime, endTime)

	assert.GreaterOrEqual(t, telemetry.Count, 1, "Expected at least 1 record in time window")

	// Verify all returned records are within the time window
	startParsed, _ := time.Parse(time.RFC3339, startTime)
	endParsed, _ := time.Parse(time.RFC3339, endTime)

	for _, record := range telemetry.Telemetry {
		recordTime, err := time.Parse(time.RFC3339, record.Timestamp)
		if err != nil {
			continue // Skip if we can't parse
		}

		assert.True(t, recordTime.After(startParsed) || recordTime.Equal(startParsed),
			"Record timestamp %s should be >= start_time %s", record.Timestamp, startTime)
		assert.True(t, recordTime.Before(endParsed) || recordTime.Equal(endParsed),
			"Record timestamp %s should be <= end_time %s", record.Timestamp, endTime)
	}

	// Test: Query with time window in the past (should return no records for our test GPU)
	pastStart := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	pastEnd := time.Now().Add(-23 * time.Hour).Format(time.RFC3339)

	pastTelemetry, err := client.GetTelemetry(gpuID, pastStart, pastEnd)
	require.NoError(t, err)

	t.Logf("Past time filter: found %d records", pastTelemetry.Count)
	// This may or may not be 0 depending on existing data
}

// TestTelemetryPagination verifies pagination works correctly.
func TestTelemetryPagination(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	RequireServicesHealthy(t, client)

	gpuID := "GPU-PAGINATION-TEST-0001-0001-0001-000000000001"

	// Publish enough data for pagination
	t.Log("Publishing data for pagination test...")
	for i := 0; i < 25; i++ {
		record := map[string]interface{}{
			"timestamp":   time.Now().Format(time.RFC3339Nano),
			"metric_name": "DCGM_FI_DEV_GPU_UTIL",
			"gpu_index":   0,
			"device":      "nvidia0",
			"uuid":        gpuID,
			"model_name":  "NVIDIA H100 80GB HBM3",
			"hostname":    "pagination-test-host",
			"value":       float64(i),
		}

		_, err := client.PublishMessage("gpu-telemetry", record)
		require.NoError(t, err)
	}

	// Wait for processing
	time.Sleep(15 * time.Second)

	// Get all telemetry to check total count
	allTelemetry, err := client.GetTelemetry(gpuID, "", "")
	require.NoError(t, err)

	t.Logf("Total records: %d (returned: %d)", allTelemetry.TotalCount, allTelemetry.Count)

	// The default page size should limit the returned count
	// but TotalCount should reflect the actual total
	assert.LessOrEqual(t, allTelemetry.Count, 100, "Default page size should limit results")
}

// TestNonExistentGPU verifies correct error handling for non-existent GPUs.
func TestNonExistentGPU(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	RequireServicesHealthy(t, client)

	// Try to get telemetry for a GPU that doesn't exist
	_, err := client.GetTelemetry("GPU-DOES-NOT-EXIST-0000-0000-0000", "", "")

	// Should return an error (404)
	assert.Error(t, err, "Expected error for non-existent GPU")
	if err != nil {
		t.Logf("Got expected error: %v", err)
	}
}

// TestGPUListUniqueness verifies GPUs are listed only once.
func TestGPUListUniqueness(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	RequireServicesHealthy(t, client)

	gpuID := "GPU-UNIQUE-TEST-0001-0001-0001-000000000001"

	// Publish multiple records for the same GPU
	t.Log("Publishing multiple records for same GPU...")
	for i := 0; i < 10; i++ {
		record := map[string]interface{}{
			"timestamp":   time.Now().Format(time.RFC3339Nano),
			"metric_name": "DCGM_FI_DEV_GPU_UTIL",
			"gpu_index":   0,
			"device":      "nvidia0",
			"uuid":        gpuID,
			"model_name":  "NVIDIA H100 80GB HBM3",
			"hostname":    "unique-test-host",
			"value":       float64(i * 10),
		}

		_, err := client.PublishMessage("gpu-telemetry", record)
		require.NoError(t, err)
	}

	// Wait for processing
	time.Sleep(10 * time.Second)

	// Get GPU list
	gpuList, err := client.GetGPUs()
	require.NoError(t, err)

	// Count occurrences of our test GPU
	count := 0
	for _, gpu := range gpuList.GPUs {
		if gpu.UUID == gpuID {
			count++
		}
	}

	assert.Equal(t, 1, count, "GPU should appear exactly once in the list")
	t.Logf("GPU %s appears %d time(s) in the list", gpuID, count)
}
