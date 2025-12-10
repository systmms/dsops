package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/internal/rotation/storage"
	"gopkg.in/yaml.v3"
)

func TestRotationRollbackCommand(t *testing.T) {
	t.Parallel()

	t.Run("command has correct flags", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{Logger: logging.New(false, true)}
		cmd := NewRotationRollbackCmd(cfg)

		// Check required flags exist
		serviceFlag := cmd.Flags().Lookup("service")
		require.NotNil(t, serviceFlag, "service flag should exist")

		envFlag := cmd.Flags().Lookup("env")
		require.NotNil(t, envFlag, "env flag should exist")

		reasonFlag := cmd.Flags().Lookup("reason")
		require.NotNil(t, reasonFlag, "reason flag should exist")

		// Check optional flags exist
		versionFlag := cmd.Flags().Lookup("version")
		require.NotNil(t, versionFlag, "version flag should exist")

		forceFlag := cmd.Flags().Lookup("force")
		require.NotNil(t, forceFlag, "force flag should exist")
		assert.Equal(t, "f", forceFlag.Shorthand, "force flag should have shorthand f")

		dryRunFlag := cmd.Flags().Lookup("dry-run")
		require.NotNil(t, dryRunFlag, "dry-run flag should exist")

		verboseFlag := cmd.Flags().Lookup("verbose")
		require.NotNil(t, verboseFlag, "verbose flag should exist")
		assert.Equal(t, "v", verboseFlag.Shorthand, "verbose flag should have shorthand v")
	})

	t.Run("missing service flag returns error", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{Logger: logging.New(false, true)}
		cmd := NewRotationRollbackCmd(cfg)
		cmd.SetArgs([]string{"--env", "production", "--reason", "test"})

		err := cmd.Execute()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service")
	})

	t.Run("missing env flag returns error", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{Logger: logging.New(false, true)}
		cmd := NewRotationRollbackCmd(cfg)
		cmd.SetArgs([]string{"--service", "postgres", "--reason", "test"})

		err := cmd.Execute()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "env")
	})

	t.Run("missing reason flag returns error", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{Logger: logging.New(false, true)}
		cmd := NewRotationRollbackCmd(cfg)
		cmd.SetArgs([]string{"--service", "postgres", "--env", "production"})

		err := cmd.Execute()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reason")
	})

	t.Run("command use and short description", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{Logger: logging.New(false, true)}
		cmd := NewRotationRollbackCmd(cfg)

		assert.Equal(t, "rollback", cmd.Use)
		assert.NotEmpty(t, cmd.Short)
		assert.Contains(t, cmd.Short, "Rollback")
	})

	t.Run("command examples are provided in long description", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{Logger: logging.New(false, true)}
		cmd := NewRotationRollbackCmd(cfg)

		assert.NotEmpty(t, cmd.Long)
		assert.Contains(t, cmd.Long, "--service")
		assert.Contains(t, cmd.Long, "--env")
		assert.Contains(t, cmd.Long, "--reason")
		assert.Contains(t, cmd.Long, "--dry-run")
		assert.Contains(t, cmd.Long, "--force")
	})
}

func TestRollbackDryRun(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv

	// Create test config
	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	// Create temp directory for test
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	// Write config file
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

	// Create test storage with status and history
	storageDir := filepath.Join(tempDir, ".dsops", "rotation")
	require.NoError(t, os.MkdirAll(storageDir, 0755))
	t.Setenv("DSOPS_ROTATION_DIR", storageDir)

	store := storage.NewFileStorage(storageDir)

	// Add rotation status
	status := &storage.RotationStatus{
		ServiceName:   "postgres-prod",
		Status:        "failed",
		LastRotation:  time.Now().Add(-1 * time.Hour),
		LastResult:    "failed",
		RotationCount: 5,
		Metadata: map[string]string{
			"current_version": "v2.0.0",
		},
	}
	require.NoError(t, store.SaveStatus(status))

	// Add history entry with version info
	historyEntry := &storage.HistoryEntry{
		Timestamp:      time.Now().Add(-1 * time.Hour),
		ServiceName:    "postgres-prod",
		CredentialType: "password",
		Action:         "rotate",
		Status:         "failed",
		OldVersion:     "v1.0.0",
		NewVersion:     "v2.0.0",
	}
	require.NoError(t, store.SaveHistory(historyEntry))

	t.Run("dry-run shows plan without executing", func(t *testing.T) {
		cmd := NewRotationRollbackCmd(cfg)

		// Capture output
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{
			"--service", "postgres-prod",
			"--env", "production",
			"--reason", "testing dry-run",
			"--dry-run",
		})

		err := cmd.Execute()
		require.NoError(t, err)

		_ = w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		output := buf.String()

		// Should show plan
		assert.Contains(t, output, "Rollback Plan")
		assert.Contains(t, output, "postgres-prod")
		assert.Contains(t, output, "production")
		assert.Contains(t, output, "testing dry-run")

		// Should indicate dry-run
		assert.Contains(t, output, "DRY RUN")
		assert.Contains(t, output, "No changes made")
	})

	t.Run("dry-run with verbose shows additional details", func(t *testing.T) {
		cmd := NewRotationRollbackCmd(cfg)

		// Capture output
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{
			"--service", "postgres-prod",
			"--env", "production",
			"--reason", "testing verbose",
			"--dry-run",
			"--verbose",
		})

		err := cmd.Execute()
		require.NoError(t, err)

		_ = w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		output := buf.String()

		// Should show service status details
		assert.Contains(t, output, "Service Status")
	})
}

func TestRollbackWithForce(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv

	// Create test config
	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	// Create temp directory for test
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	// Write config file
	configData := &config.Definition{
		Version: 0,
		Services: map[string]config.ServiceConfig{
			"postgres-test": {
				Type: "postgresql",
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

	// Create test storage with status and history
	storageDir := filepath.Join(tempDir, ".dsops", "rotation")
	require.NoError(t, os.MkdirAll(storageDir, 0755))
	t.Setenv("DSOPS_ROTATION_DIR", storageDir)

	store := storage.NewFileStorage(storageDir)

	// Add rotation status
	status := &storage.RotationStatus{
		ServiceName:  "postgres-test",
		Status:       "failed",
		LastRotation: time.Now().Add(-1 * time.Hour),
		LastResult:   "failed",
		Metadata: map[string]string{
			"current_version": "v2.0.0",
		},
	}
	require.NoError(t, store.SaveStatus(status))

	// Add history entry
	historyEntry := &storage.HistoryEntry{
		Timestamp:      time.Now().Add(-1 * time.Hour),
		ServiceName:    "postgres-test",
		CredentialType: "password",
		Action:         "rotate",
		Status:         "success",
		OldVersion:     "v1.0.0",
		NewVersion:     "v2.0.0",
	}
	require.NoError(t, store.SaveHistory(historyEntry))

	t.Run("force flag skips confirmation and executes", func(t *testing.T) {
		cmd := NewRotationRollbackCmd(cfg)

		// Capture output
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{
			"--service", "postgres-test",
			"--env", "test",
			"--reason", "testing force flag",
			"--force",
		})

		err := cmd.Execute()
		require.NoError(t, err)

		_ = w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		output := buf.String()

		// Should execute and complete
		assert.Contains(t, output, "Executing rollback")
		assert.Contains(t, output, "Rollback completed successfully")
		assert.Contains(t, output, "postgres-test")
	})
}

func TestRollbackReasonCaptured(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv

	// Create test config
	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	// Create temp directory for test
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	// Write config file
	configData := &config.Definition{
		Version: 0,
		Services: map[string]config.ServiceConfig{
			"test-service": {Type: "postgresql"},
		},
		Envs: map[string]config.Environment{"test": {}},
	}

	configBytes, _ := yaml.Marshal(configData)
	require.NoError(t, os.WriteFile(configPath, configBytes, 0644))

	cfg.Path = configPath
	require.NoError(t, cfg.Load())

	// Create test storage
	storageDir := filepath.Join(tempDir, ".dsops", "rotation")
	require.NoError(t, os.MkdirAll(storageDir, 0755))
	t.Setenv("DSOPS_ROTATION_DIR", storageDir)

	store := storage.NewFileStorage(storageDir)

	// Setup required data
	status := &storage.RotationStatus{
		ServiceName:  "test-service",
		Status:       "active",
		LastRotation: time.Now().Add(-1 * time.Hour),
		Metadata:     map[string]string{"current_version": "v2.0.0"},
	}
	require.NoError(t, store.SaveStatus(status))

	historyEntry := &storage.HistoryEntry{
		Timestamp:   time.Now().Add(-1 * time.Hour),
		ServiceName: "test-service",
		OldVersion:  "v1.0.0",
		NewVersion:  "v2.0.0",
	}
	require.NoError(t, store.SaveHistory(historyEntry))

	t.Run("reason is displayed in rollback plan", func(t *testing.T) {
		cmd := NewRotationRollbackCmd(cfg)

		// Capture output
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		customReason := "Production incident #12345 requires immediate rollback"
		cmd.SetArgs([]string{
			"--service", "test-service",
			"--env", "production",
			"--reason", customReason,
			"--dry-run",
		})

		err := cmd.Execute()
		require.NoError(t, err)

		_ = w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		output := buf.String()

		// Reason should be visible in the plan
		assert.Contains(t, output, customReason)
	})
}

func TestRollbackServiceNotFound(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv

	// Create test config
	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	// Create temp directory for test
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	// Write config file with different services
	configData := &config.Definition{
		Version: 0,
		Services: map[string]config.ServiceConfig{
			"existing-service": {Type: "postgresql"},
		},
		Envs: map[string]config.Environment{"test": {}},
	}

	configBytes, _ := yaml.Marshal(configData)
	require.NoError(t, os.WriteFile(configPath, configBytes, 0644))

	cfg.Path = configPath
	require.NoError(t, cfg.Load())

	// Create empty storage directory
	storageDir := filepath.Join(tempDir, ".dsops", "rotation")
	require.NoError(t, os.MkdirAll(storageDir, 0755))
	t.Setenv("DSOPS_ROTATION_DIR", storageDir)

	t.Run("error when service has no status", func(t *testing.T) {
		cmd := NewRotationRollbackCmd(cfg)

		cmd.SetArgs([]string{
			"--service", "non-existent-service",
			"--env", "production",
			"--reason", "testing error case",
			"--force",
		})

		err := cmd.Execute()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Cannot find service")
	})
}

func TestRollbackVersionFlags(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv

	// Create test config
	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	configData := &config.Definition{
		Version:  0,
		Services: map[string]config.ServiceConfig{"versioned-service": {Type: "postgresql"}},
		Envs:     map[string]config.Environment{"test": {}},
	}

	configBytes, _ := yaml.Marshal(configData)
	require.NoError(t, os.WriteFile(configPath, configBytes, 0644))

	cfg.Path = configPath
	require.NoError(t, cfg.Load())

	storageDir := filepath.Join(tempDir, ".dsops", "rotation")
	require.NoError(t, os.MkdirAll(storageDir, 0755))
	t.Setenv("DSOPS_ROTATION_DIR", storageDir)

	store := storage.NewFileStorage(storageDir)

	status := &storage.RotationStatus{
		ServiceName:  "versioned-service",
		Status:       "active",
		LastRotation: time.Now().Add(-1 * time.Hour),
		Metadata:     map[string]string{"current_version": "v3.0.0"},
	}
	require.NoError(t, store.SaveStatus(status))

	// Add multiple history entries
	for i, version := range []string{"v1.0.0", "v2.0.0"} {
		historyEntry := &storage.HistoryEntry{
			Timestamp:   time.Now().Add(-time.Duration(i+1) * time.Hour),
			ServiceName: "versioned-service",
			OldVersion:  version,
			NewVersion:  "v" + string(rune('2'+i)) + ".0.0",
		}
		require.NoError(t, store.SaveHistory(historyEntry))
	}

	t.Run("specific version flag is respected in plan", func(t *testing.T) {
		cmd := NewRotationRollbackCmd(cfg)

		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{
			"--service", "versioned-service",
			"--env", "production",
			"--version", "v1.5.0",
			"--reason", "rollback to specific version",
			"--dry-run",
		})

		err := cmd.Execute()
		require.NoError(t, err)

		_ = w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		output := buf.String()

		// Should show the specific version
		assert.Contains(t, output, "v1.5.0")
		assert.Contains(t, output, "Target Version")
	})
}

func TestGetUsername(t *testing.T) {
	t.Parallel()

	t.Run("returns USER env var when set", func(t *testing.T) {
		originalUser := os.Getenv("USER")
		originalUsername := os.Getenv("USERNAME")
		defer func() {
			if originalUser != "" {
				_ = os.Setenv("USER", originalUser)
			}
			if originalUsername != "" {
				_ = os.Setenv("USERNAME", originalUsername)
			}
		}()

		_ = os.Setenv("USER", "testuser")
		_ = os.Unsetenv("USERNAME")

		result := getUsername()
		assert.Equal(t, "testuser", result)
	})

	t.Run("returns USERNAME when USER not set", func(t *testing.T) {
		originalUser := os.Getenv("USER")
		originalUsername := os.Getenv("USERNAME")
		defer func() {
			if originalUser != "" {
				_ = os.Setenv("USER", originalUser)
			}
			if originalUsername != "" {
				_ = os.Setenv("USERNAME", originalUsername)
			}
		}()

		_ = os.Unsetenv("USER")
		_ = os.Setenv("USERNAME", "windowsuser")

		result := getUsername()
		assert.Equal(t, "windowsuser", result)
	})

	t.Run("returns unknown when neither set", func(t *testing.T) {
		originalUser := os.Getenv("USER")
		originalUsername := os.Getenv("USERNAME")
		defer func() {
			if originalUser != "" {
				_ = os.Setenv("USER", originalUser)
			}
			if originalUsername != "" {
				_ = os.Setenv("USERNAME", originalUsername)
			}
		}()

		_ = os.Unsetenv("USER")
		_ = os.Unsetenv("USERNAME")

		result := getUsername()
		assert.Equal(t, "unknown", result)
	})
}
