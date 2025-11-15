package testutil

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// AssertSecretRedacted verifies that a secret value does not appear in a string.
//
// This is a specialized assertion for security testing. It checks that the
// secret value is not present in the output, and that the [REDACTED] marker
// is present instead.
//
// Example usage:
//
//	output := someOperation()
//	AssertSecretRedacted(t, output, "password123")
//
// Parameters:
//   - t: Testing context
//   - output: The string to check (log output, error message, etc.)
//   - secretValue: The secret that should be redacted
func AssertSecretRedacted(t *testing.T, output, secretValue string) {
	t.Helper()

	// Secret value must not appear
	assert.NotContains(t, output, secretValue,
		"Secret value %q should be redacted, but appears in output", secretValue)

	// [REDACTED] marker should appear
	assert.Contains(t, output, "[REDACTED]",
		"Expected [REDACTED] marker when secret is used")
}

// AssertFileContents verifies that a file exists and contains expected content.
//
// This is a convenience wrapper for file content assertions.
//
// Example usage:
//
//	AssertFileContents(t, "output.txt", "expected content")
//
// Parameters:
//   - t: Testing context
//   - path: Path to the file
//   - expected: Expected file contents
func AssertFileContents(t *testing.T, path string, expected string) {
	t.Helper()

	// Check file exists
	assert.FileExists(t, path, "File should exist: %s", path)

	// Read file contents
	data, err := os.ReadFile(path)
	assert.NoError(t, err, "Failed to read file %s", path)

	// Compare contents
	actual := string(data)
	assert.Equal(t, expected, actual, "File contents mismatch for %s", path)
}

// AssertFileContainsAll verifies that a file contains all specified substrings.
//
// Example usage:
//
//	AssertFileContainsAll(t, "config.yaml", []string{"version: 1", "secretStores:"})
//
// Parameters:
//   - t: Testing context
//   - path: Path to the file
//   - substrings: Substrings that must all be present
func AssertFileContainsAll(t *testing.T, path string, substrings []string) {
	t.Helper()

	// Check file exists
	assert.FileExists(t, path, "File should exist: %s", path)

	// Read file contents
	data, err := os.ReadFile(path)
	assert.NoError(t, err, "Failed to read file %s", path)

	actual := string(data)

	// Check each substring
	for _, substr := range substrings {
		assert.Contains(t, actual, substr,
			"File %s should contain %q", path, substr)
	}
}

// AssertNoSecretLeak verifies that multiple secret values are redacted in output.
//
// This is useful for testing that all secrets in a configuration are properly
// redacted in logs or error messages.
//
// Example usage:
//
//	secrets := []string{"password123", "api-key-456", "token-789"}
//	AssertNoSecretLeak(t, logOutput, secrets)
//
// Parameters:
//   - t: Testing context
//   - output: The string to check
//   - secrets: List of secret values that should all be redacted
func AssertNoSecretLeak(t *testing.T, output string, secrets []string) {
	t.Helper()

	for _, secret := range secrets {
		assert.NotContains(t, output, secret,
			"Secret %q should be redacted, but appears in output", secret)
	}

	// Verify [REDACTED] appears at least once
	assert.Contains(t, output, "[REDACTED]",
		"Expected at least one [REDACTED] marker in output")
}

// AssertErrorContains verifies that an error occurred and contains a substring.
//
// This is a convenience wrapper for error assertion with message checking.
//
// Example usage:
//
//	err := someOperation()
//	AssertErrorContains(t, err, "connection failed")
//
// Parameters:
//   - t: Testing context
//   - err: The error to check
//   - substr: Substring that should appear in the error message
func AssertErrorContains(t *testing.T, err error, substr string) {
	t.Helper()

	assert.Error(t, err, "Expected an error to occur")
	if err != nil {
		assert.Contains(t, err.Error(), substr,
			"Error message should contain %q", substr)
	}
}

// AssertLinesContain verifies that specific lines are present in multi-line output.
//
// This is useful for testing command output or log files line-by-line.
//
// Example usage:
//
//	output := "line1\nline2\nline3"
//	AssertLinesContain(t, output, []string{"line1", "line3"})
//
// Parameters:
//   - t: Testing context
//   - output: Multi-line string
//   - expectedLines: Lines that should be present (partial match)
func AssertLinesContain(t *testing.T, output string, expectedLines []string) {
	t.Helper()

	lines := strings.Split(output, "\n")

	for _, expected := range expectedLines {
		found := false
		for _, line := range lines {
			if strings.Contains(line, expected) {
				found = true
				break
			}
		}

		assert.True(t, found,
			"Expected to find line containing %q in output", expected)
	}
}

// AssertCommandSuccess verifies that a command executed successfully.
//
// This checks that the error is nil and optionally that the output
// contains expected content.
//
// Example usage:
//
//	output, err := exec.Command("dsops", "plan").CombinedOutput()
//	AssertCommandSuccess(t, err, string(output), "DATABASE_URL")
//
// Parameters:
//   - t: Testing context
//   - err: Error from command execution
//   - output: Command output (stdout/stderr combined)
//   - expectedInOutput: Optional substring to check in output (empty string skips)
func AssertCommandSuccess(t *testing.T, err error, output string, expectedInOutput string) {
	t.Helper()

	assert.NoError(t, err, "Command should execute successfully. Output:\n%s", output)

	if expectedInOutput != "" {
		assert.Contains(t, output, expectedInOutput,
			"Command output should contain %q", expectedInOutput)
	}
}
