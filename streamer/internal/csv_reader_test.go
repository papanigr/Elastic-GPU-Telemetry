package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCSVReader(t *testing.T) {
	// Create a temporary CSV file
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test_metrics.csv")

	csvContent := `timestamp,metric_name,gpu_id,device,uuid,modelName,Hostname,container,pod,namespace,value,labels_raw
"2025-07-18T20:42:34Z","DCGM_FI_DEV_GPU_UTIL","0","nvidia0","GPU-5fd4f087-86f3-7a43-b711-4771313afc50","NVIDIA H100 80GB HBM3","mtv5-dgx1-hgpu-031","","","","0","test_labels"
"2025-07-18T20:42:34Z","DCGM_FI_DEV_GPU_TEMP","1","nvidia1","GPU-bc7a12ab-4998-fdc5-0785-2678a929a142","NVIDIA H100 80GB HBM3","mtv5-dgx1-hgpu-031","","","","100","test_labels"
`
	err := os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(t, err)

	logger := zerolog.Nop()

	t.Run("creates reader successfully", func(t *testing.T) {
		reader, err := NewCSVReader(csvPath, logger)
		require.NoError(t, err)
		require.NotNil(t, reader)
		defer reader.Close()

		assert.Equal(t, 12, len(reader.GetHeaders()))
	})

	t.Run("fails on non-existent file", func(t *testing.T) {
		reader, err := NewCSVReader("/non/existent/file.csv", logger)
		assert.Error(t, err)
		assert.Nil(t, reader)
	})
}

func TestCSVReader_ReadNext(t *testing.T) {
	// Create a temporary CSV file
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test_metrics.csv")

	csvContent := `timestamp,metric_name,gpu_id,device,uuid,modelName,Hostname,container,pod,namespace,value,labels_raw
"2025-07-18T20:42:34Z","DCGM_FI_DEV_GPU_UTIL","0","nvidia0","GPU-5fd4f087-86f3-7a43-b711-4771313afc50","NVIDIA H100 80GB HBM3","mtv5-dgx1-hgpu-031","","","","42.5","test_labels"
"2025-07-18T20:42:34Z","DCGM_FI_DEV_GPU_TEMP","1","nvidia1","GPU-bc7a12ab-4998-fdc5-0785-2678a929a142","NVIDIA H100 80GB HBM3","mtv5-dgx1-hgpu-022","container1","pod1","default","75.3","test_labels"
`
	err := os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(t, err)

	logger := zerolog.Nop()
	reader, err := NewCSVReader(csvPath, logger)
	require.NoError(t, err)
	defer reader.Close()

	t.Run("reads first record correctly", func(t *testing.T) {
		record, err := reader.ReadNext()
		require.NoError(t, err)
		require.NotNil(t, record)

		assert.Equal(t, "DCGM_FI_DEV_GPU_UTIL", record.MetricName)
		assert.Equal(t, 0, record.GPUIndex)
		assert.Equal(t, "nvidia0", record.Device)
		assert.Equal(t, "GPU-5fd4f087-86f3-7a43-b711-4771313afc50", record.UUID)
		assert.Equal(t, "NVIDIA H100 80GB HBM3", record.ModelName)
		assert.Equal(t, "mtv5-dgx1-hgpu-031", record.Hostname)
		assert.Equal(t, 42.5, record.Value)
		// Timestamp should be current time, not from CSV
		assert.False(t, record.Timestamp.IsZero())
	})

	t.Run("reads second record correctly", func(t *testing.T) {
		record, err := reader.ReadNext()
		require.NoError(t, err)
		require.NotNil(t, record)

		assert.Equal(t, "DCGM_FI_DEV_GPU_TEMP", record.MetricName)
		assert.Equal(t, 1, record.GPUIndex)
		assert.Equal(t, "nvidia1", record.Device)
		assert.Equal(t, "mtv5-dgx1-hgpu-022", record.Hostname)
		assert.Equal(t, "container1", record.Container)
		assert.Equal(t, "pod1", record.Pod)
		assert.Equal(t, "default", record.Namespace)
		assert.Equal(t, 75.3, record.Value)
	})

	t.Run("loops back to beginning after EOF", func(t *testing.T) {
		// Read next should loop back to the first record
		record, err := reader.ReadNext()
		require.NoError(t, err)
		require.NotNil(t, record)

		// Should be back to the first record
		assert.Equal(t, "DCGM_FI_DEV_GPU_UTIL", record.MetricName)
		assert.Equal(t, 0, record.GPUIndex)
	})
}

func TestCSVReader_GetLineNumber(t *testing.T) {
	// Create a temporary CSV file
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test_metrics.csv")

	csvContent := `timestamp,metric_name,gpu_id,device,uuid,modelName,Hostname,container,pod,namespace,value,labels_raw
"2025-07-18T20:42:34Z","DCGM_FI_DEV_GPU_UTIL","0","nvidia0","GPU-5fd4f087","NVIDIA H100","host1","","","","0",""
"2025-07-18T20:42:34Z","DCGM_FI_DEV_GPU_TEMP","1","nvidia1","GPU-bc7a12ab","NVIDIA H100","host1","","","","100",""
`
	err := os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(t, err)

	logger := zerolog.Nop()
	reader, err := NewCSVReader(csvPath, logger)
	require.NoError(t, err)
	defer reader.Close()

	// After opening, line number should be 1 (header line)
	assert.Equal(t, 1, reader.GetLineNumber())

	// Read first record
	_, err = reader.ReadNext()
	require.NoError(t, err)
	assert.Equal(t, 2, reader.GetLineNumber())

	// Read second record
	_, err = reader.ReadNext()
	require.NoError(t, err)
	assert.Equal(t, 3, reader.GetLineNumber())
}

func TestCSVReader_ParsesInvalidValues(t *testing.T) {
	// Create a temporary CSV file with invalid values
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test_metrics.csv")

	csvContent := `timestamp,metric_name,gpu_id,device,uuid,modelName,Hostname,container,pod,namespace,value,labels_raw
"2025-07-18T20:42:34Z","DCGM_FI_DEV_GPU_UTIL","invalid_gpu_id","nvidia0","GPU-5fd4f087","NVIDIA H100","host1","","","","invalid_value",""
`
	err := os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(t, err)

	logger := zerolog.Nop()
	reader, err := NewCSVReader(csvPath, logger)
	require.NoError(t, err)
	defer reader.Close()

	// Should handle invalid values gracefully
	record, err := reader.ReadNext()
	require.NoError(t, err)
	require.NotNil(t, record)

	// Invalid gpu_id should default to 0
	assert.Equal(t, 0, record.GPUIndex)
	// Invalid value should default to 0
	assert.Equal(t, float64(0), record.Value)
}
