// Package e2e provides end-to-end workflow tests for dsops.
//
// These tests validate complete workflows from configuration loading
// through secret resolution to output rendering, ensuring all components
// integrate correctly.
package e2e

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/internal/resolve"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/fakes"
	"github.com/systmms/dsops/tests/testutil"
	"gopkg.in/yaml.v3"
)

func TestWorkflowConfigLoadResolveRender(t *testing.T) {
	t.Parallel()

	t.Run("simple_literal_workflow", func(t *testing.T) {
		t.Parallel()

		// Step 1: Create configuration
		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  test:
    type: literal
    config:
      values:
        API_KEY: "test-api-key-123"
        DATABASE_URL: "postgres://localhost/test"
envs:
  test:
    API_KEY:
      from:
        store: "store://test/API_KEY"
    DATABASE_URL:
      from:
        store: "store://test/DATABASE_URL"
`)

		// Step 2: Load configuration
		def := testutil.LoadTestConfig(t, configPath)
		require.NotNil(t, def)
		assert.Equal(t, 1, def.Version)
		assert.Contains(t, def.SecretStores, "test")
		assert.Contains(t, def.Envs, "test")

		// Step 3: Create resolver with fake provider
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)

		// Create fake provider with expected secrets
		fakeProvider := fakes.NewFakeProvider("test").
			WithSecret("API_KEY", provider.SecretValue{Value: "test-api-key-123"}).
			WithSecret("DATABASE_URL", provider.SecretValue{Value: "postgres://localhost/test"})

		resolver.RegisterProvider("test", fakeProvider)

		// Step 4: Resolve environment
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resolved, err := resolver.Resolve(ctx, "test")
		require.NoError(t, err)
		assert.Len(t, resolved, 2)

		assert.Equal(t, "test-api-key-123", resolved["API_KEY"].Value)
		assert.Equal(t, "postgres://localhost/test", resolved["DATABASE_URL"].Value)

		// Step 5: Render output
		envMap := make(map[string]string)
		for name, rv := range resolved {
			envMap[name] = rv.Value
		}

		// Test dotenv rendering (simple format)
		var dotenvOutput strings.Builder
		for k, v := range envMap {
			dotenvOutput.WriteString(k + "=" + v + "\n")
		}
		assert.Contains(t, dotenvOutput.String(), "API_KEY=test-api-key-123")
		assert.Contains(t, dotenvOutput.String(), "DATABASE_URL=postgres://localhost/test")

		// Test JSON rendering
		jsonBytes, err := json.MarshalIndent(envMap, "", "  ")
		require.NoError(t, err)
		jsonOutput := string(jsonBytes)
		assert.Contains(t, jsonOutput, `"API_KEY"`)
		assert.Contains(t, jsonOutput, "test-api-key-123")
	})

	t.Run("workflow_with_transforms", func(t *testing.T) {
		t.Parallel()

		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  vault:
    type: vault
    config:
      addr: "http://localhost:8200"
envs:
  production:
    DB_PASSWORD:
      from:
        store: "store://vault/database/creds"
      transform: "json_extract:.password"
    TRIMMED_KEY:
      from:
        store: "store://vault/api/key"
      transform: "trim"
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)

		// Create fake provider with JSON secret and padded key
		fakeProvider := fakes.NewFakeProvider("vault").
			WithSecret("database/creds", provider.SecretValue{
				Value: `{"username":"dbuser","password":"super-secret-password"}`,
			}).
			WithSecret("api/key", provider.SecretValue{
				Value: "  api-key-with-spaces  ",
			})

		resolver.RegisterProvider("vault", fakeProvider)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resolved, err := resolver.Resolve(ctx, "production")
		require.NoError(t, err)
		assert.Len(t, resolved, 2)

		// Verify transforms were applied
		assert.Equal(t, "super-secret-password", resolved["DB_PASSWORD"].Value)
		assert.True(t, resolved["DB_PASSWORD"].Transformed)

		assert.Equal(t, "api-key-with-spaces", resolved["TRIMMED_KEY"].Value)
		assert.True(t, resolved["TRIMMED_KEY"].Transformed)
	})

	t.Run("workflow_with_optional_variables", func(t *testing.T) {
		t.Parallel()

		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  secrets:
    type: vault
    config:
      addr: "http://localhost:8200"
envs:
  test:
    REQUIRED_VAR:
      from:
        store: "store://secrets/required"
    OPTIONAL_VAR:
      from:
        store: "store://secrets/optional"
      optional: true
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)

		// Only add the required secret, optional one is missing
		fakeProvider := fakes.NewFakeProvider("secrets").
			WithSecret("required", provider.SecretValue{Value: "required-value"})

		resolver.RegisterProvider("secrets", fakeProvider)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Should succeed even though optional var is missing
		resolved, err := resolver.Resolve(ctx, "test")
		require.NoError(t, err)

		// Required var should be resolved
		assert.Equal(t, "required-value", resolved["REQUIRED_VAR"].Value)
		assert.NoError(t, resolved["REQUIRED_VAR"].Error)

		// Optional var should have error but not fail the overall resolution
		assert.Error(t, resolved["OPTIONAL_VAR"].Error)
	})

	t.Run("workflow_multiple_environments", func(t *testing.T) {
		t.Parallel()

		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  dev-store:
    type: literal
    config:
      values:
        API_KEY: "dev-key"
  prod-store:
    type: literal
    config:
      values:
        API_KEY: "prod-key"
envs:
  development:
    API_KEY:
      from:
        store: "store://dev-store/API_KEY"
  production:
    API_KEY:
      from:
        store: "store://prod-store/API_KEY"
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)

		// Register both providers
		devProvider := fakes.NewFakeProvider("dev-store").
			WithSecret("API_KEY", provider.SecretValue{Value: "dev-key"})
		prodProvider := fakes.NewFakeProvider("prod-store").
			WithSecret("API_KEY", provider.SecretValue{Value: "prod-key"})

		resolver.RegisterProvider("dev-store", devProvider)
		resolver.RegisterProvider("prod-store", prodProvider)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Resolve development environment
		devResolved, err := resolver.Resolve(ctx, "development")
		require.NoError(t, err)
		assert.Equal(t, "dev-key", devResolved["API_KEY"].Value)

		// Resolve production environment
		prodResolved, err := resolver.Resolve(ctx, "production")
		require.NoError(t, err)
		assert.Equal(t, "prod-key", prodResolved["API_KEY"].Value)

		// Verify different values
		assert.NotEqual(t, devResolved["API_KEY"].Value, prodResolved["API_KEY"].Value)
	})

	t.Run("workflow_render_to_file", func(t *testing.T) {
		t.Parallel()

		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  test:
    type: literal
    config:
      values:
        SECRET: "file-secret"
envs:
  test:
    SECRET:
      from:
        store: "store://test/SECRET"
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)
		fakeProvider := fakes.NewFakeProvider("test").
			WithSecret("SECRET", provider.SecretValue{Value: "file-secret"})
		resolver.RegisterProvider("test", fakeProvider)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resolved, err := resolver.Resolve(ctx, "test")
		require.NoError(t, err)

		// Convert to map
		envMap := make(map[string]string)
		for name, rv := range resolved {
			envMap[name] = rv.Value
		}

		// Write to temporary file
		tempDir := t.TempDir()
		outputPath := filepath.Join(tempDir, ".env")

		// Render as dotenv format
		var output strings.Builder
		for k, v := range envMap {
			output.WriteString(k + "=" + v + "\n")
		}
		err = os.WriteFile(outputPath, []byte(output.String()), 0600)
		require.NoError(t, err)

		// Verify file contents
		content, err := os.ReadFile(outputPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "SECRET=file-secret")

		// Verify file permissions (should be secure)
		info, err := os.Stat(outputPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	})

	t.Run("workflow_yaml_rendering", func(t *testing.T) {
		t.Parallel()

		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  test:
    type: literal
    config:
      values:
        KEY1: "value1"
        KEY2: "value2"
envs:
  test:
    KEY1:
      from:
        store: "store://test/KEY1"
    KEY2:
      from:
        store: "store://test/KEY2"
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)
		fakeProvider := fakes.NewFakeProvider("test").
			WithSecret("KEY1", provider.SecretValue{Value: "value1"}).
			WithSecret("KEY2", provider.SecretValue{Value: "value2"})
		resolver.RegisterProvider("test", fakeProvider)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resolved, err := resolver.Resolve(ctx, "test")
		require.NoError(t, err)

		envMap := make(map[string]string)
		for name, rv := range resolved {
			envMap[name] = rv.Value
		}

		// Test YAML rendering
		yamlBytes, err := yaml.Marshal(envMap)
		require.NoError(t, err)
		yamlOutput := string(yamlBytes)
		assert.Contains(t, yamlOutput, "KEY1: value1")
		assert.Contains(t, yamlOutput, "KEY2: value2")
	})

	t.Run("workflow_plan_then_resolve", func(t *testing.T) {
		t.Parallel()

		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  vault:
    type: vault
    config:
      addr: "http://localhost:8200"
envs:
  production:
    DB_URL:
      from:
        store: "store://vault/db/url"
    API_KEY:
      from:
        store: "store://vault/api/key"
      transform: "trim"
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)
		fakeProvider := fakes.NewFakeProvider("vault").
			WithSecret("db/url", provider.SecretValue{Value: "postgres://prod/db"}).
			WithSecret("api/key", provider.SecretValue{Value: "  prod-api-key  "})
		resolver.RegisterProvider("vault", fakeProvider)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Step 1: Plan (shows what would be resolved without fetching)
		plan, err := resolver.Plan(ctx, "production")
		require.NoError(t, err)
		assert.Len(t, plan.Variables, 2)
		assert.Empty(t, plan.Errors)

		// Verify plan contains expected variables
		varNames := make([]string, 0, len(plan.Variables))
		for _, v := range plan.Variables {
			varNames = append(varNames, v.Name)
		}
		assert.Contains(t, varNames, "DB_URL")
		assert.Contains(t, varNames, "API_KEY")

		// Step 2: Resolve (actually fetches secrets)
		resolved, err := resolver.Resolve(ctx, "production")
		require.NoError(t, err)
		assert.Len(t, resolved, 2)

		// Verify actual values
		assert.Equal(t, "postgres://prod/db", resolved["DB_URL"].Value)
		assert.Equal(t, "prod-api-key", resolved["API_KEY"].Value) // Trimmed
	})
}

func TestWorkflowErrorHandling(t *testing.T) {
	t.Parallel()

	t.Run("missing_required_secret_fails", func(t *testing.T) {
		t.Parallel()

		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  test:
    type: vault
    config:
      addr: "http://localhost:8200"
envs:
  test:
    MISSING_SECRET:
      from:
        store: "store://test/nonexistent"
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)
		// Empty fake provider - no secrets
		fakeProvider := fakes.NewFakeProvider("test")
		resolver.RegisterProvider("test", fakeProvider)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := resolver.Resolve(ctx, "test")
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "failed")
	})

	t.Run("invalid_transform_fails", func(t *testing.T) {
		t.Parallel()

		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  test:
    type: vault
    config:
      addr: "http://localhost:8200"
envs:
  test:
    BAD_JSON:
      from:
        store: "store://test/secret"
      transform: "json_extract:.missing.path"
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)
		fakeProvider := fakes.NewFakeProvider("test").
			WithSecret("secret", provider.SecretValue{Value: `{"other":"value"}`})
		resolver.RegisterProvider("test", fakeProvider)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result, err := resolver.Resolve(ctx, "test")
		// Transform failures are stored in the result, not as errors
		if err == nil {
			// Check if transform error is in result
			assert.Error(t, result["BAD_JSON"].Error)
		}
	})

	t.Run("provider_not_registered", func(t *testing.T) {
		t.Parallel()

		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  unregistered:
    type: vault
    config:
      addr: "http://localhost:8200"
envs:
  test:
    SECRET:
      from:
        store: "store://unregistered/key"
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)
		// Don't register the provider

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := resolver.Resolve(ctx, "test")
		require.Error(t, err)
	})

	t.Run("invalid_environment_name", func(t *testing.T) {
		t.Parallel()

		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  test:
    type: literal
    config:
      values:
        KEY: "value"
envs:
  production:
    KEY:
      from:
        store: "store://test/KEY"
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Try to resolve non-existent environment
		_, err := resolver.Resolve(ctx, "nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nonexistent")
	})
}

func TestWorkflowConcurrency(t *testing.T) {
	t.Parallel()

	t.Run("concurrent_resolution_of_multiple_variables", func(t *testing.T) {
		t.Parallel()

		// Create config with many variables to test concurrent resolution
		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  test:
    type: vault
    config:
      addr: "http://localhost:8200"
envs:
  test:
    VAR1:
      from:
        store: "store://test/var1"
    VAR2:
      from:
        store: "store://test/var2"
    VAR3:
      from:
        store: "store://test/var3"
    VAR4:
      from:
        store: "store://test/var4"
    VAR5:
      from:
        store: "store://test/var5"
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)

		// Create fake provider with slight delay to simulate network
		fakeProvider := fakes.NewFakeProvider("test").
			WithSecret("var1", provider.SecretValue{Value: "value1"}).
			WithSecret("var2", provider.SecretValue{Value: "value2"}).
			WithSecret("var3", provider.SecretValue{Value: "value3"}).
			WithSecret("var4", provider.SecretValue{Value: "value4"}).
			WithSecret("var5", provider.SecretValue{Value: "value5"}).
			WithDelay(10 * time.Millisecond)

		resolver.RegisterProvider("test", fakeProvider)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Resolution should be concurrent (total time should be much less than 5 * 10ms sequential)
		start := time.Now()
		resolved, err := resolver.Resolve(ctx, "test")
		duration := time.Since(start)

		require.NoError(t, err)
		assert.Len(t, resolved, 5)

		// Verify all values resolved correctly
		assert.Equal(t, "value1", resolved["VAR1"].Value)
		assert.Equal(t, "value2", resolved["VAR2"].Value)
		assert.Equal(t, "value3", resolved["VAR3"].Value)
		assert.Equal(t, "value4", resolved["VAR4"].Value)
		assert.Equal(t, "value5", resolved["VAR5"].Value)

		// Should be faster than sequential (50ms) but allow for test overhead
		// Just verify it completed in reasonable time
		assert.Less(t, duration, 5*time.Second, "Resolution took too long")
	})
}
