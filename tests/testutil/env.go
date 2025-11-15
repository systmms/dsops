package testutil

import (
	"os"
	"testing"
)

// SetupTestEnv sets environment variables for the duration of a test.
//
// The original environment is restored automatically when the test completes.
// This uses t.Cleanup() to ensure cleanup happens even if the test fails.
//
// Example usage:
//
//	SetupTestEnv(t, map[string]string{
//	    "AWS_REGION": "us-east-1",
//	    "VAULT_ADDR": "http://localhost:8200",
//	})
//
// Parameters:
//   - t: Testing context
//   - vars: Map of environment variable names to values
func SetupTestEnv(t *testing.T, vars map[string]string) {
	t.Helper()

	// Store original values for cleanup
	original := make(map[string]string)
	unset := make([]string, 0)

	for key, value := range vars {
		// Store original value
		if orig, ok := os.LookupEnv(key); ok {
			original[key] = orig
		} else {
			unset = append(unset, key)
		}

		// Set new value
		if err := os.Setenv(key, value); err != nil {
			t.Fatalf("Failed to set environment variable %s: %v", key, err)
		}
	}

	// Register cleanup to restore original environment
	t.Cleanup(func() {
		// Restore original values
		for key, value := range original {
			if err := os.Setenv(key, value); err != nil {
				t.Errorf("Failed to restore environment variable %s: %v", key, err)
			}
		}

		// Unset variables that weren't originally set
		for _, key := range unset {
			if err := os.Unsetenv(key); err != nil {
				t.Errorf("Failed to unset environment variable %s: %v", key, err)
			}
		}
	})
}

// CleanupTestFiles cleans up test files and directories.
//
// This is typically not needed when using t.TempDir(), but can be useful
// for cleaning up files created outside the temp directory.
//
// Example usage:
//
//	CleanupTestFiles(t, "output.txt", "temp_dir/")
//
// Parameters:
//   - t: Testing context
//   - paths: Files or directories to remove
func CleanupTestFiles(t *testing.T, paths ...string) {
	t.Helper()

	t.Cleanup(func() {
		for _, path := range paths {
			if err := os.RemoveAll(path); err != nil {
				t.Errorf("Failed to cleanup %s: %v", path, err)
			}
		}
	})
}

// WithEnv executes a function with temporary environment variables.
//
// This is useful for testing code that reads environment variables.
// The environment is restored automatically after the function returns.
//
// Example usage:
//
//	WithEnv(t, map[string]string{"DEBUG": "true"}, func() {
//	    // Code here sees DEBUG=true
//	    someFunction()
//	})
//	// DEBUG is restored to original value here
//
// Parameters:
//   - t: Testing context
//   - vars: Environment variables to set
//   - fn: Function to execute with the environment
func WithEnv(t *testing.T, vars map[string]string, fn func()) {
	t.Helper()

	// Store original values
	original := make(map[string]string)
	unset := make([]string, 0)

	for key, value := range vars {
		if orig, ok := os.LookupEnv(key); ok {
			original[key] = orig
		} else {
			unset = append(unset, key)
		}

		if err := os.Setenv(key, value); err != nil {
			t.Fatalf("Failed to set environment variable %s: %v", key, err)
		}
	}

	// Execute function
	defer func() {
		// Restore original values
		for key, value := range original {
			_ = os.Setenv(key, value)
		}

		// Unset variables that weren't originally set
		for _, key := range unset {
			_ = os.Unsetenv(key)
		}
	}()

	fn()
}
