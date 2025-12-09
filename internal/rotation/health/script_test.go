package health

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewScriptHealthChecker(t *testing.T) {
	t.Parallel()

	config := DefaultScriptHealthConfig()
	config.Script = "/path/to/script.sh"
	checker := NewScriptHealthChecker("test-script", config)

	assert.NotNil(t, checker)
	assert.Equal(t, "test-script", checker.Name())
	assert.Equal(t, ProtocolScript, checker.Protocol())
}

func TestDefaultScriptHealthConfig(t *testing.T) {
	t.Parallel()

	config := DefaultScriptHealthConfig()

	assert.Equal(t, 60*time.Second, config.Timeout)
	assert.Equal(t, 3, config.Retry.MaxAttempts)
	assert.Equal(t, 5*time.Second, config.Retry.Backoff)
	assert.Equal(t, 2.0, config.Retry.BackoffMultiplier)
}

func TestScriptHealthChecker_Check_Success(t *testing.T) {
	t.Parallel()

	// Create a temporary script
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "health.sh")
	err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho 'healthy'\nexit 0"), 0755)
	require.NoError(t, err)

	config := DefaultScriptHealthConfig()
	config.Script = scriptPath
	config.Retry.MaxAttempts = 1 // Disable retry for this test
	checker := NewScriptHealthChecker("test-script", config)

	ctx := context.Background()
	service := ServiceConfig{
		Name: "test-service",
		Type: "postgresql",
	}

	result, err := checker.Check(ctx, service)
	require.NoError(t, err)
	assert.True(t, result.Healthy)
	assert.Contains(t, result.Message, "healthy")
	assert.Equal(t, 0, result.Metadata["exit_code"])
}

func TestScriptHealthChecker_Check_Failure(t *testing.T) {
	t.Parallel()

	// Create a failing script
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "health.sh")
	err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho 'unhealthy' >&2\nexit 1"), 0755)
	require.NoError(t, err)

	config := DefaultScriptHealthConfig()
	config.Script = scriptPath
	config.Retry.MaxAttempts = 1
	checker := NewScriptHealthChecker("test-script", config)

	ctx := context.Background()
	service := ServiceConfig{
		Name: "test-service",
		Type: "postgresql",
	}

	result, err := checker.Check(ctx, service)
	assert.Error(t, err)
	assert.False(t, result.Healthy)
	assert.Equal(t, 1, result.Metadata["exit_code"])
	assert.Contains(t, result.Metadata["stderr"], "unhealthy")
}

func TestScriptHealthChecker_Check_Timeout(t *testing.T) {
	t.Parallel()

	// Create a slow script
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "health.sh")
	err := os.WriteFile(scriptPath, []byte("#!/bin/bash\nsleep 10\nexit 0"), 0755)
	require.NoError(t, err)

	config := ScriptHealthConfig{
		Script:  scriptPath,
		Timeout: 100 * time.Millisecond,
		Retry: ScriptRetryConfig{
			MaxAttempts: 1,
		},
	}
	checker := NewScriptHealthChecker("test-script", config)

	ctx := context.Background()
	service := ServiceConfig{
		Name: "test-service",
		Type: "postgresql",
	}

	result, err := checker.Check(ctx, service)
	assert.Error(t, err)
	assert.False(t, result.Healthy)
	assert.Contains(t, result.Message, "timed out")
}

func TestScriptHealthChecker_Check_NoScript(t *testing.T) {
	t.Parallel()

	config := DefaultScriptHealthConfig()
	// No script configured
	checker := NewScriptHealthChecker("test-script", config)

	ctx := context.Background()
	service := ServiceConfig{
		Name: "test-service",
		Type: "postgresql",
	}

	result, err := checker.Check(ctx, service)
	assert.Error(t, err)
	assert.False(t, result.Healthy)
	assert.Contains(t, result.Message, "no script configured")
}

func TestScriptHealthChecker_Check_ScriptNotFound(t *testing.T) {
	t.Parallel()

	config := DefaultScriptHealthConfig()
	config.Script = "/nonexistent/script.sh"
	config.Retry.MaxAttempts = 1
	checker := NewScriptHealthChecker("test-script", config)

	ctx := context.Background()
	service := ServiceConfig{
		Name: "test-service",
		Type: "postgresql",
	}

	result, err := checker.Check(ctx, service)
	assert.Error(t, err)
	assert.False(t, result.Healthy)
	assert.Contains(t, result.Message, "script not found")
}

func TestScriptHealthChecker_Check_Retry(t *testing.T) {
	t.Parallel()

	// Create a script that fails first but would succeed on retry
	tmpDir := t.TempDir()
	counterFile := filepath.Join(tmpDir, "counter")
	err := os.WriteFile(counterFile, []byte("0"), 0644)
	require.NoError(t, err)

	// This script increments a counter and fails on first two attempts
	scriptPath := filepath.Join(tmpDir, "health.sh")
	script := `#!/bin/bash
counter=$(cat "` + counterFile + `")
counter=$((counter + 1))
echo $counter > "` + counterFile + `"
if [ $counter -lt 3 ]; then
  echo "attempt $counter failed"
  exit 1
fi
echo "success on attempt $counter"
exit 0`
	err = os.WriteFile(scriptPath, []byte(script), 0755)
	require.NoError(t, err)

	config := ScriptHealthConfig{
		Script:  scriptPath,
		Timeout: 5 * time.Second,
		Retry: ScriptRetryConfig{
			MaxAttempts:       3,
			Backoff:           10 * time.Millisecond,
			BackoffMultiplier: 1.0,
		},
	}
	checker := NewScriptHealthChecker("test-script", config)

	ctx := context.Background()
	service := ServiceConfig{
		Name: "test-service",
		Type: "postgresql",
	}

	result, err := checker.Check(ctx, service)
	require.NoError(t, err)
	assert.True(t, result.Healthy)
	assert.Equal(t, 3, result.Metadata["attempt"])
}

func TestScriptHealthChecker_Check_Environment(t *testing.T) {
	t.Parallel()

	// Create a script that outputs environment variables
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "health.sh")
	script := `#!/bin/bash
echo "SERVICE_NAME=$DSOPS_SERVICE_NAME"
echo "SERVICE_TYPE=$DSOPS_SERVICE_TYPE"
echo "NEW_VERSION=$DSOPS_NEW_VERSION"
echo "CUSTOM_VAR=$CUSTOM_VAR"
exit 0`
	err := os.WriteFile(scriptPath, []byte(script), 0755)
	require.NoError(t, err)

	config := ScriptHealthConfig{
		Script:  scriptPath,
		Timeout: 5 * time.Second,
		Environment: map[string]string{
			"CUSTOM_VAR": "custom-value",
		},
		Retry: ScriptRetryConfig{
			MaxAttempts: 1,
		},
	}
	checker := NewScriptHealthChecker("test-script", config)

	ctx := context.Background()
	service := ServiceConfig{
		Name: "test-service",
		Type: "postgresql",
		Config: map[string]interface{}{
			"new_version": "v2.0.0",
			"old_version": "v1.0.0",
		},
	}

	result, err := checker.Check(ctx, service)
	require.NoError(t, err)
	assert.True(t, result.Healthy)

	stdout := result.Metadata["stdout"].(string)
	assert.Contains(t, stdout, "SERVICE_NAME=test-service")
	assert.Contains(t, stdout, "SERVICE_TYPE=postgresql")
	assert.Contains(t, stdout, "NEW_VERSION=v2.0.0")
	assert.Contains(t, stdout, "CUSTOM_VAR=custom-value")
}

func TestScriptHealthChecker_Check_CaptureStderr(t *testing.T) {
	t.Parallel()

	// Create a script that writes to stderr
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "health.sh")
	err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho 'stdout message'\necho 'stderr message' >&2\nexit 0"), 0755)
	require.NoError(t, err)

	config := DefaultScriptHealthConfig()
	config.Script = scriptPath
	config.Retry.MaxAttempts = 1
	checker := NewScriptHealthChecker("test-script", config)

	ctx := context.Background()
	service := ServiceConfig{
		Name: "test-service",
		Type: "postgresql",
	}

	result, err := checker.Check(ctx, service)
	require.NoError(t, err)
	assert.True(t, result.Healthy)
	assert.Contains(t, result.Metadata["stdout"], "stdout message")
	assert.Contains(t, result.Metadata["stderr"], "stderr message")
}

func TestScriptHealthChecker_Check_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Create a slow script
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "health.sh")
	err := os.WriteFile(scriptPath, []byte("#!/bin/bash\nsleep 10\nexit 0"), 0755)
	require.NoError(t, err)

	config := ScriptHealthConfig{
		Script:  scriptPath,
		Timeout: 5 * time.Second,
		Retry: ScriptRetryConfig{
			MaxAttempts:       3,
			Backoff:           1 * time.Second,
			BackoffMultiplier: 1.0,
		},
	}
	checker := NewScriptHealthChecker("test-script", config)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	service := ServiceConfig{
		Name: "test-service",
		Type: "postgresql",
	}

	result, err := checker.Check(ctx, service)
	// Should handle context cancellation
	assert.False(t, result.Healthy)
	if err != nil {
		// Either context error or timeout error is acceptable
		assert.True(t, err == context.DeadlineExceeded || err == context.Canceled ||
			result.Message == "context cancelled during retry" ||
			result.Message == "script timed out")
	}
}
