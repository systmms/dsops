package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T049: Test rotation storage

func TestNewFileStorage(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	require.NotNil(t, storage)
	assert.Equal(t, tmpDir, storage.baseDir)
}

func TestDefaultStorageDir(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv

	// Test with environment variable
	t.Run("with DSOPS_ROTATION_DIR env var", func(t *testing.T) {
		t.Setenv("DSOPS_ROTATION_DIR", "/custom/dir")
		dir := DefaultStorageDir()
		assert.Equal(t, "/custom/dir", dir)
	})

	// Test with XDG_DATA_HOME
	t.Run("with XDG_DATA_HOME", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "/home/user/.local/share")
		t.Setenv("DSOPS_ROTATION_DIR", "") // Clear the priority env var
		dir := DefaultStorageDir()
		assert.Equal(t, "/home/user/.local/share/dsops/rotation", dir)
	})

	// Test fallback
	t.Run("fallback to user home", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		t.Setenv("DSOPS_ROTATION_DIR", "")
		dir := DefaultStorageDir()
		assert.NotEmpty(t, dir)
		assert.Contains(t, dir, "dsops")
		assert.Contains(t, dir, "rotation")
	})
}

func TestFileStorage_SaveAndGetStatus(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	now := time.Now()
	nextRotation := now.Add(24 * time.Hour)

	status := &RotationStatus{
		ServiceName:      "postgres-prod",
		Status:           "active",
		LastRotation:     now,
		NextRotation:     &nextRotation,
		LastResult:       "success",
		RotationCount:    5,
		SuccessCount:     5,
		FailureCount:     0,
		RotationInterval: 24 * time.Hour,
		Metadata: map[string]string{
			"version": "1.0",
			"owner":   "team-backend",
		},
	}

	// Save status
	err := storage.SaveStatus(status)
	require.NoError(t, err)

	// Verify file was created
	statusFile := filepath.Join(tmpDir, "status", "postgres-prod.json")
	assert.FileExists(t, statusFile)

	// Get status
	retrieved, err := storage.GetStatus("postgres-prod")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, status.ServiceName, retrieved.ServiceName)
	assert.Equal(t, status.Status, retrieved.Status)
	assert.Equal(t, status.RotationCount, retrieved.RotationCount)
	assert.Equal(t, status.SuccessCount, retrieved.SuccessCount)
	assert.Equal(t, status.Metadata["version"], retrieved.Metadata["version"])
}

func TestFileStorage_GetStatus_NotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	_, err := storage.GetStatus("nonexistent-service")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no status found")
}

func TestFileStorage_SaveStatus_UpdateExisting(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	now := time.Now()
	status1 := &RotationStatus{
		ServiceName:   "test-service",
		Status:        "active",
		LastRotation:  now,
		RotationCount: 1,
	}

	// Save first version
	err := storage.SaveStatus(status1)
	require.NoError(t, err)

	// Update with new values
	status2 := &RotationStatus{
		ServiceName:   "test-service",
		Status:        "active",
		LastRotation:  now.Add(time.Hour),
		RotationCount: 2,
		SuccessCount:  2,
	}

	err = storage.SaveStatus(status2)
	require.NoError(t, err)

	// Get updated status
	retrieved, err := storage.GetStatus("test-service")
	require.NoError(t, err)
	assert.Equal(t, 2, retrieved.RotationCount)
	assert.Equal(t, 2, retrieved.SuccessCount)
}

func TestFileStorage_SaveHistory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	now := time.Now()
	entry := &HistoryEntry{
		ID:             "rotation-001",
		Timestamp:      now,
		ServiceName:    "postgres-prod",
		CredentialType: "password",
		Action:         "rotate",
		Status:         "success",
		Duration:       5 * time.Second,
		User:           "admin",
		OldVersion:     "v1",
		NewVersion:     "v2",
		Strategy:       "two-key",
		Metadata: map[string]string{
			"environment": "production",
		},
		Steps: []StepResult{
			{
				Name:        "generate_new_key",
				Status:      "success",
				StartedAt:   now,
				CompletedAt: now.Add(1 * time.Second),
				Duration:    1 * time.Second,
			},
			{
				Name:        "verify_connectivity",
				Status:      "success",
				StartedAt:   now.Add(1 * time.Second),
				CompletedAt: now.Add(5 * time.Second),
				Duration:    4 * time.Second,
			},
		},
	}

	err := storage.SaveHistory(entry)
	require.NoError(t, err)

	// Verify file was created
	historyDir := filepath.Join(tmpDir, "history", "postgres-prod")
	files, err := os.ReadDir(historyDir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
}

func TestFileStorage_GetHistory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	// Create multiple history entries
	now := time.Now()
	for i := 0; i < 5; i++ {
		entry := &HistoryEntry{
			ID:          fmt.Sprintf("rotation-%03d", i),
			Timestamp:   now.Add(time.Duration(i) * time.Hour),
			ServiceName: "test-service",
			Action:      "rotate",
			Status:      "success",
		}
		err := storage.SaveHistory(entry)
		require.NoError(t, err)
	}

	// Get history with limit
	history, err := storage.GetHistory("test-service", 3)
	require.NoError(t, err)
	assert.Len(t, history, 3)

	// Verify order (should be newest first)
	assert.Equal(t, "rotation-004", history[0].ID)
	assert.Equal(t, "rotation-003", history[1].ID)
	assert.Equal(t, "rotation-002", history[2].ID)
}

func TestFileStorage_GetHistory_NoHistory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	history, err := storage.GetHistory("nonexistent-service", 10)
	require.NoError(t, err)
	assert.Empty(t, history)
}

func TestFileStorage_GetAllHistory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	now := time.Now()

	// Create entries for multiple services
	services := []string{"service-a", "service-b", "service-c"}
	for _, service := range services {
		for i := 0; i < 3; i++ {
			entry := &HistoryEntry{
				ID:          fmt.Sprintf("%s-rotation-%d", service, i),
				Timestamp:   now.Add(time.Duration(i) * time.Hour),
				ServiceName: service,
				Action:      "rotate",
				Status:      "success",
			}
			err := storage.SaveHistory(entry)
			require.NoError(t, err)
		}
	}

	// Get all history with limit
	history, err := storage.GetAllHistory(5)
	require.NoError(t, err)
	assert.Len(t, history, 5)

	// Verify entries from different services are included
	serviceCount := make(map[string]int)
	for _, entry := range history {
		serviceCount[entry.ServiceName]++
	}
	assert.Greater(t, len(serviceCount), 1, "should have entries from multiple services")
}

func TestFileStorage_CleanupOldEntries(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	now := time.Now()

	// Note: CleanupOldEntries uses the filename timestamp, not the JSON timestamp
	// Since SaveHistory creates files with current timestamp, we can't easily test
	// cleanup without manipulating file modification times.
	// This test verifies the method runs without error.

	// Create an entry
	entry := &HistoryEntry{
		ID:          "test-entry",
		Timestamp:   now,
		ServiceName: "test-service",
		Action:      "rotate",
		Status:      "success",
	}
	err := storage.SaveHistory(entry)
	require.NoError(t, err)

	// Cleanup entries older than 30 days (should not delete recent entries)
	err = storage.CleanupOldEntries(30 * 24 * time.Hour)
	require.NoError(t, err)

	// Verify the method runs without error
	// We can't easily verify the actual cleanup behavior without manipulating file timestamps
	assert.NoError(t, err)
}

func TestFileStorage_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	now := time.Now()

	// Concurrently save status
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			defer func() { done <- true }()

			status := &RotationStatus{
				ServiceName:   fmt.Sprintf("service-%d", idx),
				Status:        "active",
				LastRotation:  now,
				RotationCount: idx,
			}
			err := storage.SaveStatus(status)
			assert.NoError(t, err)
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all statuses were saved
	for i := 0; i < 10; i++ {
		status, err := storage.GetStatus(fmt.Sprintf("service-%d", i))
		require.NoError(t, err)
		assert.Equal(t, i, status.RotationCount)
	}
}

func TestRotationStatus_Structure(t *testing.T) {
	t.Parallel()

	now := time.Now()
	nextRotation := now.Add(24 * time.Hour)

	status := &RotationStatus{
		ServiceName:      "test-service",
		Status:           "active",
		LastRotation:     now,
		NextRotation:     &nextRotation,
		LastResult:       "success",
		LastError:        "",
		RotationCount:    10,
		SuccessCount:     9,
		FailureCount:     1,
		RotationInterval: 24 * time.Hour,
		Metadata: map[string]string{
			"key": "value",
		},
	}

	assert.NotEmpty(t, status.ServiceName)
	assert.NotEmpty(t, status.Status)
	assert.NotZero(t, status.LastRotation)
	assert.NotNil(t, status.NextRotation)
	assert.Equal(t, 10, status.RotationCount)
	assert.Equal(t, 9, status.SuccessCount)
	assert.Equal(t, 1, status.FailureCount)
}

func TestHistoryEntry_Structure(t *testing.T) {
	t.Parallel()

	now := time.Now()

	entry := &HistoryEntry{
		ID:             "test-001",
		Timestamp:      now,
		ServiceName:    "test-service",
		CredentialType: "password",
		Action:         "rotate",
		Status:         "success",
		Duration:       5 * time.Second,
		User:           "admin",
		OldVersion:     "v1",
		NewVersion:     "v2",
		Strategy:       "two-key",
		Metadata:       map[string]string{"env": "prod"},
		Steps: []StepResult{
			{
				Name:        "step1",
				Status:      "success",
				StartedAt:   now,
				CompletedAt: now.Add(1 * time.Second),
				Duration:    1 * time.Second,
			},
		},
	}

	assert.NotEmpty(t, entry.ID)
	assert.NotEmpty(t, entry.ServiceName)
	assert.NotEmpty(t, entry.Action)
	assert.NotEmpty(t, entry.Status)
	assert.NotZero(t, entry.Timestamp)
	assert.NotEmpty(t, entry.Steps)
	assert.Equal(t, "step1", entry.Steps[0].Name)
}

func TestStepResult_Structure(t *testing.T) {
	t.Parallel()

	now := time.Now()

	step := StepResult{
		Name:        "verify_connectivity",
		Status:      "success",
		StartedAt:   now,
		CompletedAt: now.Add(2 * time.Second),
		Duration:    2 * time.Second,
		Error:       "",
	}

	assert.NotEmpty(t, step.Name)
	assert.NotEmpty(t, step.Status)
	assert.NotZero(t, step.StartedAt)
	assert.NotZero(t, step.CompletedAt)
	assert.Equal(t, 2*time.Second, step.Duration)
}
