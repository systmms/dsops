package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/internal/rotation/storage"
	"gopkg.in/yaml.v3"
)

func TestRotationStatusCommand(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	// Create test config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")
	
	configData := &config.Definition{
		Version: 0,
		Services: map[string]config.ServiceConfig{
			"postgres-prod": {
				Type: "postgresql",
				Config: map[string]interface{}{
					"host": "db.example.com",
				},
			},
			"stripe-api": {
				Type: "stripe",
			},
		},
		Envs: map[string]config.Environment{
			"test": {},
		},
	}
	
	configBytes, _ := yaml.Marshal(configData)
	require.NoError(t, os.WriteFile(configPath, configBytes, 0644))
	
	cfg.Path = configPath
	require.NoError(t, cfg.Load())

	// Create test storage with some data
	storageDir := filepath.Join(tempDir, ".dsops", "rotation")
	_ = os.MkdirAll(storageDir, 0755)
	// Set environment variable to use test storage dir
	_ = os.Setenv("DSOPS_ROTATION_DIR", storageDir)
	defer func() { _ = os.Unsetenv("DSOPS_ROTATION_DIR") }()
	store := storage.NewFileStorage(storageDir)
	
	// Add test status for postgres-prod to show active state
	pgStatus := &storage.RotationStatus{
		ServiceName:   "postgres-prod",
		Status:        "active",
		LastRotation:  time.Now().Add(-24 * time.Hour),
		LastResult:    "success",
		RotationCount: 5,
		SuccessCount:  4,
		FailureCount:  1,
	}
	require.NoError(t, store.SaveStatus(pgStatus))
	
	// Don't add status for stripe-api to test "Never Rotated" state

	t.Run("list all services", func(t *testing.T) {
		cmd := NewRotationStatusCmd(cfg)
		output := captureOutput(t, cmd, nil)
		
		assert.Contains(t, output, "postgres-prod")
		assert.Contains(t, output, "stripe-api")
		assert.Contains(t, output, "Active")
		assert.Contains(t, output, "Never Rotated")
	})

	t.Run("show specific service", func(t *testing.T) {
		cmd := NewRotationStatusCmd(cfg)
		output := captureOutput(t, cmd, []string{"postgres-prod"})
		
		assert.Contains(t, output, "postgres-prod")
		assert.Contains(t, output, "Active")
		assert.NotContains(t, output, "stripe-api")
	})

	t.Run("json output", func(t *testing.T) {
		cmd := NewRotationStatusCmd(cfg)
		output := captureOutput(t, cmd, []string{"--format", "json"})
		
		var result map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(output), &result))
		
		assert.Contains(t, result, "postgres-prod")
		pgStatus := result["postgres-prod"].(map[string]interface{})
		assert.Equal(t, "active", pgStatus["status"])
		assert.Equal(t, float64(5), pgStatus["rotation_count"])
	})
}

func TestRotationHistoryCommand(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	// Create test config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")
	
	configData := &config.Definition{
		Version: 0,
		Services: map[string]config.ServiceConfig{
			"postgres-prod": {
				Type: "postgresql",
				Config: map[string]interface{}{
					"host": "db.example.com",
				},
			},
		},
		Envs: map[string]config.Environment{
			"test": {},
		},
	}
	
	configBytes, _ := yaml.Marshal(configData)
	require.NoError(t, os.WriteFile(configPath, configBytes, 0644))
	
	cfg.Path = configPath
	require.NoError(t, cfg.Load())

	// Create test storage with history
	storageDir := filepath.Join(tempDir, ".dsops", "rotation")
	_ = os.MkdirAll(storageDir, 0755)
	// Set environment variable to use test storage dir
	_ = os.Setenv("DSOPS_ROTATION_DIR", storageDir)
	defer func() { _ = os.Unsetenv("DSOPS_ROTATION_DIR") }()
	store := storage.NewFileStorage(storageDir)
	
	// Add test history entries
	now := time.Now()
	entries := []storage.HistoryEntry{
		{
			Timestamp:      now.Add(-2 * time.Hour),
			ServiceName:    "postgres-prod",
			CredentialType: "password",
			Action:         "rotate",
			Status:         "success",
			Duration:       2 * time.Second,
			Strategy:       "two-key",
			User:           "testuser",
		},
		{
			Timestamp:      now.Add(-25 * time.Hour),
			ServiceName:    "postgres-prod",
			CredentialType: "password",
			Action:         "rotate",
			Status:         "failed",
			Duration:       500 * time.Millisecond,
			Error:          "Connection timeout",
			Strategy:       "two-key",
			User:           "testuser",
		},
	}
	
	for _, entry := range entries {
		require.NoError(t, store.SaveHistory(&entry))
	}

	t.Run("show all history", func(t *testing.T) {
		cmd := NewRotationHistoryCmd(cfg)
		output := captureOutput(t, cmd, nil)
		
		assert.Contains(t, output, "postgres-prod")
		assert.Contains(t, output, "Success")
		assert.Contains(t, output, "Failed")
		assert.Contains(t, output, "Connection timeout")
	})

	t.Run("filter by status", func(t *testing.T) {
		cmd := NewRotationHistoryCmd(cfg)
		output := captureOutput(t, cmd, []string{"--status", "failed"})
		
		assert.Contains(t, output, "Failed")
		assert.Contains(t, output, "Connection timeout")
		assert.NotContains(t, output, "Success")
	})

	t.Run("limit results", func(t *testing.T) {
		cmd := NewRotationHistoryCmd(cfg)
		output := captureOutput(t, cmd, []string{"--limit", "1"})
		
		assert.Contains(t, output, "Showing 1 entries")
	})

	t.Run("json output", func(t *testing.T) {
		cmd := NewRotationHistoryCmd(cfg)
		output := captureOutput(t, cmd, []string{"--format", "json"})
		
		var result map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(output), &result))
		
		assert.Equal(t, float64(2), result["count"])
		entries := result["entries"].([]interface{})
		assert.Len(t, entries, 2)
	})
}

// captureOutput captures command output for testing
func captureOutput(t *testing.T, cmd *cobra.Command, args []string) string {
	t.Helper()
	
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	
	// Set args and execute
	if args != nil {
		cmd.SetArgs(args)
	}
	
	err := cmd.Execute()
	require.NoError(t, err)
	
	// Restore stdout and read output
	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	return buf.String()
}