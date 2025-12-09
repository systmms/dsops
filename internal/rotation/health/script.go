package health

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// ScriptHealthConfig holds configuration for script health checks.
type ScriptHealthConfig struct {
	// Script is the path to the health check script.
	Script string

	// Timeout is the maximum time to wait for the script to complete.
	Timeout time.Duration

	// Environment holds additional environment variables to pass to the script.
	Environment map[string]string

	// Retry configures retry behavior on failure.
	Retry ScriptRetryConfig

	// WorkingDir is the working directory for the script.
	WorkingDir string
}

// ScriptRetryConfig configures retry behavior for script health checks.
type ScriptRetryConfig struct {
	// MaxAttempts is the maximum number of attempts.
	MaxAttempts int

	// Backoff is the initial backoff duration between retries.
	Backoff time.Duration

	// BackoffMultiplier is the multiplier for exponential backoff.
	BackoffMultiplier float64
}

// DefaultScriptHealthConfig returns the default script health configuration.
func DefaultScriptHealthConfig() ScriptHealthConfig {
	return ScriptHealthConfig{
		Timeout: 60 * time.Second,
		Retry: ScriptRetryConfig{
			MaxAttempts:       3,
			Backoff:           5 * time.Second,
			BackoffMultiplier: 2.0,
		},
	}
}

// ScriptHealthChecker performs health checks by executing custom scripts.
type ScriptHealthChecker struct {
	name   string
	config ScriptHealthConfig
}

// NewScriptHealthChecker creates a new script health checker.
func NewScriptHealthChecker(name string, config ScriptHealthConfig) *ScriptHealthChecker {
	return &ScriptHealthChecker{
		name:   name,
		config: config,
	}
}

// Name returns the health checker name.
func (c *ScriptHealthChecker) Name() string {
	return c.name
}

// Protocol returns the protocol type.
func (c *ScriptHealthChecker) Protocol() ProtocolType {
	return ProtocolScript
}

// Check performs a health check by executing the configured script.
func (c *ScriptHealthChecker) Check(ctx context.Context, service ServiceConfig) (HealthResult, error) {
	start := time.Now()
	result := HealthResult{
		Healthy:   true,
		Timestamp: start,
		Metadata:  make(map[string]interface{}),
	}

	// Validate script is configured
	if c.config.Script == "" {
		result.Healthy = false
		result.Message = "no script configured"
		result.Duration = time.Since(start)
		return result, fmt.Errorf("no script configured")
	}

	// Check script exists
	if _, err := os.Stat(c.config.Script); os.IsNotExist(err) {
		result.Healthy = false
		result.Message = fmt.Sprintf("script not found: %s", c.config.Script)
		result.Duration = time.Since(start)
		return result, err
	}

	// Execute with retry
	var lastErr error
	backoff := c.config.Retry.Backoff
	maxAttempts := c.config.Retry.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result.Metadata["attempt"] = attempt

		execResult, err := c.executeScript(ctx, service)
		if err == nil && execResult.Healthy {
			result.Healthy = true
			result.Message = execResult.Message
			result.Duration = time.Since(start)
			result.Metadata["stdout"] = execResult.Metadata["stdout"]
			result.Metadata["stderr"] = execResult.Metadata["stderr"]
			result.Metadata["exit_code"] = execResult.Metadata["exit_code"]
			return result, nil
		}

		lastErr = err
		result.Metadata["stdout"] = execResult.Metadata["stdout"]
		result.Metadata["stderr"] = execResult.Metadata["stderr"]
		result.Metadata["exit_code"] = execResult.Metadata["exit_code"]

		// Don't sleep on last attempt
		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				result.Healthy = false
				result.Message = "context cancelled during retry"
				result.Duration = time.Since(start)
				return result, ctx.Err()
			case <-time.After(backoff):
				backoff = time.Duration(float64(backoff) * c.config.Retry.BackoffMultiplier)
			}
		}
	}

	result.Healthy = false
	if lastErr != nil {
		result.Message = fmt.Sprintf("script failed after %d attempts: %v", maxAttempts, lastErr)
	} else {
		result.Message = fmt.Sprintf("script failed after %d attempts", maxAttempts)
	}
	result.Duration = time.Since(start)
	return result, lastErr
}

// executeScript runs the script once and returns the result.
func (c *ScriptHealthChecker) executeScript(ctx context.Context, service ServiceConfig) (HealthResult, error) {
	result := HealthResult{
		Healthy:   true,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Create timeout context
	timeout := c.config.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(timeoutCtx, c.config.Script)

	// Set working directory
	if c.config.WorkingDir != "" {
		cmd.Dir = c.config.WorkingDir
	}

	// Build environment
	cmd.Env = c.buildEnvironment(service)

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the script
	err := cmd.Run()

	result.Metadata["stdout"] = stdout.String()
	result.Metadata["stderr"] = stderr.String()

	if err != nil {
		// Check if it was a timeout
		if timeoutCtx.Err() == context.DeadlineExceeded {
			result.Healthy = false
			result.Message = fmt.Sprintf("script timed out after %v", timeout)
			result.Metadata["exit_code"] = -1
			return result, fmt.Errorf("script timed out")
		}

		// Get exit code
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Metadata["exit_code"] = exitErr.ExitCode()
		} else {
			result.Metadata["exit_code"] = -1
		}

		result.Healthy = false
		result.Message = fmt.Sprintf("script failed: %v", err)
		if stderr.Len() > 0 {
			result.Message = fmt.Sprintf("%s: %s", result.Message, stderr.String())
		}
		return result, err
	}

	result.Metadata["exit_code"] = 0
	result.Message = "script completed successfully"
	if stdout.Len() > 0 {
		result.Message = stdout.String()
	}
	return result, nil
}

// buildEnvironment builds the environment variables for the script.
func (c *ScriptHealthChecker) buildEnvironment(service ServiceConfig) []string {
	// Start with current environment
	env := os.Environ()

	// Add DSOPS_* variables
	dsopsVars := map[string]string{
		"DSOPS_SERVICE_NAME": service.Name,
		"DSOPS_SERVICE_TYPE": service.Type,
		"DSOPS_ENDPOINT":     service.Endpoint,
	}

	// Add service config values
	if service.Config != nil {
		if newVersion, ok := service.Config["new_version"].(string); ok {
			dsopsVars["DSOPS_NEW_VERSION"] = newVersion
		}
		if oldVersion, ok := service.Config["old_version"].(string); ok {
			dsopsVars["DSOPS_OLD_VERSION"] = oldVersion
		}
		if environment, ok := service.Config["environment"].(string); ok {
			dsopsVars["DSOPS_ENVIRONMENT"] = environment
		}
		if rotationID, ok := service.Config["rotation_id"].(string); ok {
			dsopsVars["DSOPS_ROTATION_ID"] = rotationID
		}
	}

	// Add custom environment variables
	for key, value := range c.config.Environment {
		dsopsVars[key] = value
	}

	// Append all variables
	for key, value := range dsopsVars {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	return env
}
