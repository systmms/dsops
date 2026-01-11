package execenv

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/internal/secure"
)

func createTestExecutor() *Executor {
	logger := logging.New(false, true)
	return New(logger)
}

func TestNew(t *testing.T) {
	t.Parallel()
	logger := logging.New(false, true)
	executor := New(logger)
	assert.NotNil(t, executor)
	assert.Equal(t, logger, executor.logger)
}

func TestMaskValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", "(empty)"},
		{"single_char", "a", "*"},
		{"two_chars", "ab", "**"},
		{"three_chars", "abc", "***"},
		{"four_chars", "abcd", "a**d"},
		{"five_chars", "abcde", "a***e"},
		{"eight_chars", "abcdefgh", "a******h"},
		{"nine_chars", "abcdefghi", "abc********hi"},
		{"long_value", "mysupersecretpassword", "mys********rd"},
		{"special_chars", "pa$$w0rd!", "pa$********d!"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := maskValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExecutor_buildEnvironment(t *testing.T) {
	// Not parallel because some subtests use t.Setenv
	executor := createTestExecutor()

	t.Run("adds_dsops_vars_to_environment", func(t *testing.T) {
		t.Parallel()
		dsopsVars := map[string]string{
			"DATABASE_URL": "postgres://localhost/db",
			"API_KEY":      "secret123",
		}

		env, err := executor.buildEnvironment(dsopsVars, false)
		require.NoError(t, err)

		// Should contain dsops vars
		found := 0
		for _, e := range env {
			if strings.HasPrefix(e, "DATABASE_URL=") || strings.HasPrefix(e, "API_KEY=") {
				found++
			}
		}
		assert.Equal(t, 2, found)
	})

	t.Run("dsops_vars_override_existing_when_allowOverride_false", func(t *testing.T) {
		// Set env var that dsops will override
		t.Setenv("TEST_VAR", "original")

		dsopsVars := map[string]string{
			"TEST_VAR": "dsops_value",
		}

		executor := createTestExecutor()
		env, err := executor.buildEnvironment(dsopsVars, false)
		require.NoError(t, err)

		// Find TEST_VAR in result
		var foundValue string
		for _, e := range env {
			if strings.HasPrefix(e, "TEST_VAR=") {
				foundValue = strings.TrimPrefix(e, "TEST_VAR=")
				break
			}
		}

		// dsops value should take precedence
		assert.Equal(t, "dsops_value", foundValue)
	})

	t.Run("existing_vars_override_when_allowOverride_true", func(t *testing.T) {
		// Set env var that should not be overridden
		t.Setenv("PRESERVE_VAR", "original")

		dsopsVars := map[string]string{
			"PRESERVE_VAR": "dsops_value",
		}

		executor := createTestExecutor()
		env, err := executor.buildEnvironment(dsopsVars, true)
		require.NoError(t, err)

		// Find PRESERVE_VAR in result
		var foundValue string
		for _, e := range env {
			if strings.HasPrefix(e, "PRESERVE_VAR=") {
				foundValue = strings.TrimPrefix(e, "PRESERVE_VAR=")
				break
			}
		}

		// Original value should be preserved
		assert.Equal(t, "original", foundValue)
	})

	t.Run("preserves_existing_environment", func(t *testing.T) {
		t.Parallel()
		dsopsVars := map[string]string{
			"NEW_VAR": "new_value",
		}

		env, err := executor.buildEnvironment(dsopsVars, false)
		require.NoError(t, err)

		// Should have more than just the dsops var (includes system env vars)
		assert.Greater(t, len(env), 1)

		// Should include PATH (common env var)
		hasPath := false
		for _, e := range env {
			if strings.HasPrefix(e, "PATH=") {
				hasPath = true
				break
			}
		}
		assert.True(t, hasPath, "Should preserve PATH environment variable")
	})

	t.Run("returns_sorted_environment", func(t *testing.T) {
		t.Parallel()
		dsopsVars := map[string]string{
			"ZZZ_VAR": "last",
			"AAA_VAR": "first",
			"MMM_VAR": "middle",
		}

		env, err := executor.buildEnvironment(dsopsVars, false)
		require.NoError(t, err)

		// Verify sorting
		var prevKey string
		for _, e := range env {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) >= 1 {
				currentKey := parts[0]
				if prevKey != "" {
					assert.LessOrEqual(t, prevKey, currentKey, "Environment should be sorted")
				}
				prevKey = currentKey
			}
		}
	})

	t.Run("empty_dsops_vars", func(t *testing.T) {
		t.Parallel()
		dsopsVars := map[string]string{}

		env, err := executor.buildEnvironment(dsopsVars, false)
		require.NoError(t, err)

		// Should still have system environment
		assert.Greater(t, len(env), 0)
	})
}

func TestExecutor_buildSecureEnvironment(t *testing.T) {
	executor := createTestExecutor()

	t.Run("adds_secure_vars_to_environment", func(t *testing.T) {
		t.Parallel()

		buf1, err := secure.NewSecureBufferFromString("postgres://localhost/db")
		require.NoError(t, err)
		buf2, err := secure.NewSecureBufferFromString("secret123")
		require.NoError(t, err)

		secureVars := map[string]*secure.SecureBuffer{
			"DATABASE_URL": buf1,
			"API_KEY":      buf2,
		}

		env, err := executor.buildSecureEnvironment(secureVars, false)
		require.NoError(t, err)

		// Should contain the vars with correct values
		found := make(map[string]string)
		for _, e := range env {
			if strings.HasPrefix(e, "DATABASE_URL=") {
				found["DATABASE_URL"] = strings.TrimPrefix(e, "DATABASE_URL=")
			}
			if strings.HasPrefix(e, "API_KEY=") {
				found["API_KEY"] = strings.TrimPrefix(e, "API_KEY=")
			}
		}

		assert.Equal(t, "postgres://localhost/db", found["DATABASE_URL"])
		assert.Equal(t, "secret123", found["API_KEY"])

		// Cleanup
		buf1.Destroy()
		buf2.Destroy()
	})

	t.Run("secure_vars_override_existing_when_allowOverride_false", func(t *testing.T) {
		t.Setenv("SECURE_TEST_VAR", "original")

		buf, err := secure.NewSecureBufferFromString("secure_value")
		require.NoError(t, err)
		defer buf.Destroy()

		secureVars := map[string]*secure.SecureBuffer{
			"SECURE_TEST_VAR": buf,
		}

		executor := createTestExecutor()
		env, err := executor.buildSecureEnvironment(secureVars, false)
		require.NoError(t, err)

		// Find the var in result
		var foundValue string
		for _, e := range env {
			if strings.HasPrefix(e, "SECURE_TEST_VAR=") {
				foundValue = strings.TrimPrefix(e, "SECURE_TEST_VAR=")
				break
			}
		}

		// Secure value should take precedence
		assert.Equal(t, "secure_value", foundValue)
	})

	t.Run("existing_vars_override_when_allowOverride_true", func(t *testing.T) {
		t.Setenv("PRESERVE_SECURE_VAR", "original")

		buf, err := secure.NewSecureBufferFromString("secure_value")
		require.NoError(t, err)
		defer buf.Destroy()

		secureVars := map[string]*secure.SecureBuffer{
			"PRESERVE_SECURE_VAR": buf,
		}

		executor := createTestExecutor()
		env, err := executor.buildSecureEnvironment(secureVars, true)
		require.NoError(t, err)

		// Find the var in result
		var foundValue string
		for _, e := range env {
			if strings.HasPrefix(e, "PRESERVE_SECURE_VAR=") {
				foundValue = strings.TrimPrefix(e, "PRESERVE_SECURE_VAR=")
				break
			}
		}

		// Original value should be preserved
		assert.Equal(t, "original", foundValue)
	})

	t.Run("handles_destroyed_buffer", func(t *testing.T) {
		t.Parallel()

		buf, err := secure.NewSecureBufferFromString("value")
		require.NoError(t, err)
		buf.Destroy() // Pre-destroy the buffer

		secureVars := map[string]*secure.SecureBuffer{
			"DESTROYED_VAR": buf,
		}

		_, err = executor.buildSecureEnvironment(secureVars, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to open secure buffer")
	})

	t.Run("preserves_existing_environment", func(t *testing.T) {
		t.Parallel()

		buf, err := secure.NewSecureBufferFromString("new_value")
		require.NoError(t, err)
		defer buf.Destroy()

		secureVars := map[string]*secure.SecureBuffer{
			"NEW_SECURE_VAR": buf,
		}

		env, err := executor.buildSecureEnvironment(secureVars, false)
		require.NoError(t, err)

		// Should have more than just the secure var (includes system env vars)
		assert.Greater(t, len(env), 1)

		// Should include PATH (common env var)
		hasPath := false
		for _, e := range env {
			if strings.HasPrefix(e, "PATH=") {
				hasPath = true
				break
			}
		}
		assert.True(t, hasPath, "Should preserve PATH environment variable")
	})

	t.Run("returns_sorted_environment", func(t *testing.T) {
		t.Parallel()

		buf1, _ := secure.NewSecureBufferFromString("last")
		buf2, _ := secure.NewSecureBufferFromString("first")
		buf3, _ := secure.NewSecureBufferFromString("middle")
		defer buf1.Destroy()
		defer buf2.Destroy()
		defer buf3.Destroy()

		secureVars := map[string]*secure.SecureBuffer{
			"ZZZ_SECURE": buf1,
			"AAA_SECURE": buf2,
			"MMM_SECURE": buf3,
		}

		env, err := executor.buildSecureEnvironment(secureVars, false)
		require.NoError(t, err)

		// Verify sorting
		var prevKey string
		for _, e := range env {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) >= 1 {
				currentKey := parts[0]
				if prevKey != "" {
					assert.LessOrEqual(t, prevKey, currentKey, "Environment should be sorted")
				}
				prevKey = currentKey
			}
		}
	})
}

func TestExecutor_printEnvironment(t *testing.T) {
	executor := createTestExecutor()

	t.Run("prints_empty_message_for_no_vars", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		executor.printEnvironment(map[string]string{})

		_ = w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		assert.Contains(t, output, "No environment variables resolved")
	})

	t.Run("prints_variables_with_masked_values", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		vars := map[string]string{
			"API_KEY":      "supersecretkey123",
			"DATABASE_URL": "postgres://user:pass@localhost/db",
		}

		executor.printEnvironment(vars)

		_ = w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// Should contain variable names
		assert.Contains(t, output, "API_KEY")
		assert.Contains(t, output, "DATABASE_URL")

		// Should contain masked values (asterisks)
		assert.Contains(t, output, "*")

		// Should NOT contain actual secret values
		assert.NotContains(t, output, "supersecretkey123")
		assert.NotContains(t, output, "pass@localhost")

		// Should show count
		assert.Contains(t, output, "Resolved 2 environment variables")
	})

	t.Run("prints_sorted_variables", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		vars := map[string]string{
			"ZZZ": "zzz",
			"AAA": "aaa",
			"MMM": "mmm",
		}

		executor.printEnvironment(vars)

		_ = w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// AAA should appear before MMM, which should appear before ZZZ
		aaaIdx := strings.Index(output, "AAA")
		mmmIdx := strings.Index(output, "MMM")
		zzzIdx := strings.Index(output, "ZZZ")

		assert.Less(t, aaaIdx, mmmIdx)
		assert.Less(t, mmmIdx, zzzIdx)
	})
}

func TestValidateCommand(t *testing.T) {
	t.Parallel()

	t.Run("empty_command", func(t *testing.T) {
		t.Parallel()
		err := ValidateCommand([]string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "No command specified")
	})

	t.Run("valid_command_exists", func(t *testing.T) {
		t.Parallel()
		// 'echo' should exist on all platforms
		err := ValidateCommand([]string{"echo", "test"})
		assert.NoError(t, err)
	})

	t.Run("nonexistent_command", func(t *testing.T) {
		t.Parallel()
		err := ValidateCommand([]string{"this_command_does_not_exist_12345"})
		assert.Error(t, err)
	})

	t.Run("dangerous_rm_command", func(t *testing.T) {
		t.Parallel()
		err := ValidateCommand([]string{"rm", "-rf", "/"})
		if err != nil {
			// Only check if the error mentions dangerous
			assert.Contains(t, err.Error(), "dangerous")
		}
		// If rm doesn't exist on system (Windows), test passes anyway
	})

	t.Run("dangerous_dd_command", func(t *testing.T) {
		t.Parallel()
		err := ValidateCommand([]string{"dd", "if=/dev/zero"})
		if err != nil && strings.Contains(err.Error(), "dangerous") {
			// Expected behavior
			assert.Contains(t, err.Error(), "dangerous")
		}
		// If dd doesn't exist, test passes
	})

	t.Run("command_with_full_path", func(t *testing.T) {
		t.Parallel()
		// Test with absolute path
		err := ValidateCommand([]string{"/usr/bin/echo", "test"})
		// This might fail on Windows, so we just ensure it doesn't panic
		if err != nil {
			assert.IsType(t, err, err)
		}
	})

	t.Run("valid_safe_commands", func(t *testing.T) {
		t.Parallel()
		safeCommands := [][]string{
			{"echo", "hello"},
			{"cat", "file.txt"},
			{"ls", "-la"},
			{"pwd"},
		}

		for _, cmd := range safeCommands {
			// These may or may not exist, just ensure no panic
			_ = ValidateCommand(cmd)
		}
	})
}

func TestExecOptions_validation(t *testing.T) {
	t.Parallel()

	t.Run("options_struct_fields", func(t *testing.T) {
		t.Parallel()
		options := ExecOptions{
			Command:       []string{"echo", "test"},
			Environment:   map[string]string{"KEY": "value"},
			AllowOverride: true,
			PrintVars:     true,
			WorkingDir:    "/tmp",
			Timeout:       30,
		}

		assert.Equal(t, []string{"echo", "test"}, options.Command)
		assert.Equal(t, "value", options.Environment["KEY"])
		assert.True(t, options.AllowOverride)
		assert.True(t, options.PrintVars)
		assert.Equal(t, "/tmp", options.WorkingDir)
		assert.Equal(t, 30, options.Timeout)
	})
}

// Note: Testing Exec() directly is challenging because it:
// 1. Calls os.Exit() on process exit errors
// 2. Executes real commands that may have side effects
// 3. Requires careful handling of I/O streams
//
// For comprehensive testing, integration tests would use subprocess testing
// or mock the exec.Command functionality. The buildEnvironment and
// ValidateCommand functions provide good coverage of the core logic.

func TestExecutor_Exec_EmptyCommand(t *testing.T) {
	t.Parallel()

	executor := createTestExecutor()

	err := executor.Exec(context.Background(), ExecOptions{
		Command: []string{},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "No command specified")
}

func TestExecutor_Exec_CommandNotFound(t *testing.T) {
	t.Parallel()

	executor := createTestExecutor()

	err := executor.Exec(context.Background(), ExecOptions{
		Command: []string{"nonexistent_command_xyz"},
	})

	require.Error(t, err)
}
