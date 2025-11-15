// Package testutil provides test utilities and helpers for dsops tests.
//
// This package contains shared test infrastructure including configuration builders,
// logger helpers, fixture loaders, and Docker environment management.
package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/systmms/dsops/internal/config"
	"gopkg.in/yaml.v3"
)

// TestConfigBuilder provides a fluent API for building test configurations.
//
// This builder allows programmatic creation of dsops.yaml configurations
// for testing without manually writing YAML strings. It handles cleanup
// of temporary files automatically.
//
// Example usage:
//
//	builder := NewTestConfig(t).
//	    WithSecretStore("vault", "vault", map[string]any{
//	        "addr": "http://localhost:8200",
//	    }).
//	    WithEnv("test", map[string]config.Variable{
//	        "DATABASE_URL": {From: &config.Reference{Store: "store://vault/db/url"}},
//	    })
//	defer builder.Cleanup()
//
//	cfg := builder.Build()
type TestConfigBuilder struct {
	config       *config.Definition
	tempDir      string
	cleanupFuncs []func()
	t            *testing.T
}

// NewTestConfig creates a new TestConfigBuilder.
//
// The builder starts with a minimal valid configuration (version: 1).
// Use builder methods to add secret stores, services, and environments.
func NewTestConfig(t *testing.T) *TestConfigBuilder {
	t.Helper()

	tempDir := t.TempDir() // Auto-cleanup by testing framework

	return &TestConfigBuilder{
		config: &config.Definition{
			Version:      1,
			SecretStores: make(map[string]config.SecretStoreConfig),
			Services:     make(map[string]config.ServiceConfig),
			Envs:         make(map[string]config.Environment),
			Transforms:   make(map[string][]string),
		},
		tempDir:      tempDir,
		cleanupFuncs: []func(){},
		t:            t,
	}
}

// WithSecretStore adds a secret store configuration.
//
// Parameters:
//   - name: The secret store name (used in references)
//   - storeType: The store type (e.g., "vault", "aws.secretsmanager", "literal")
//   - cfg: Store-specific configuration fields
//
// Returns the builder for method chaining.
func (b *TestConfigBuilder) WithSecretStore(name, storeType string, cfg map[string]any) *TestConfigBuilder {
	b.t.Helper()

	storeConfig := config.SecretStoreConfig{
		Type:   storeType,
		Config: cfg,
	}

	b.config.SecretStores[name] = storeConfig
	return b
}

// WithService adds a service configuration.
//
// Parameters:
//   - name: The service name (used in rotation references)
//   - serviceType: The service type (e.g., "postgresql", "mongodb", "stripe")
//   - cfg: Service-specific configuration fields
//
// Returns the builder for method chaining.
func (b *TestConfigBuilder) WithService(name, serviceType string, cfg map[string]any) *TestConfigBuilder {
	b.t.Helper()

	serviceConfig := config.ServiceConfig{
		Type:   serviceType,
		Config: cfg,
	}

	b.config.Services[name] = serviceConfig
	return b
}

// WithProvider adds a legacy provider configuration.
//
// This method exists for backward compatibility testing. New tests should
// use WithSecretStore instead.
//
// Returns the builder for method chaining.
func (b *TestConfigBuilder) WithProvider(name, providerType string, cfg map[string]any) *TestConfigBuilder {
	b.t.Helper()

	if b.config.Providers == nil {
		b.config.Providers = make(map[string]config.ProviderConfig)
	}

	providerConfig := config.ProviderConfig{
		Type:   providerType,
		Config: cfg,
	}

	b.config.Providers[name] = providerConfig
	return b
}

// WithEnv adds an environment configuration.
//
// Parameters:
//   - name: The environment name (e.g., "test", "production")
//   - vars: Map of variable names to Variable configurations
//
// Returns the builder for method chaining.
func (b *TestConfigBuilder) WithEnv(name string, vars map[string]config.Variable) *TestConfigBuilder {
	b.t.Helper()

	b.config.Envs[name] = config.Environment(vars)
	return b
}

// WithTransform adds a transform pipeline configuration.
//
// Parameters:
//   - name: The transform name
//   - steps: Transform step names
//
// Returns the builder for method chaining.
func (b *TestConfigBuilder) WithTransform(name string, steps []string) *TestConfigBuilder {
	b.t.Helper()

	b.config.Transforms[name] = steps
	return b
}

// Build returns the built configuration Definition.
//
// This returns the in-memory configuration object. Use Write() or
// WriteYAML() if you need a file on disk.
func (b *TestConfigBuilder) Build() *config.Definition {
	b.t.Helper()

	return b.config
}

// Write writes the configuration to a temporary file and returns the path.
//
// The file is created in a temporary directory and will be cleaned up
// automatically by the testing framework.
//
// Returns the absolute path to the written configuration file.
func (b *TestConfigBuilder) Write() string {
	b.t.Helper()

	path := filepath.Join(b.tempDir, "dsops.yaml")
	if err := b.WriteYAML(path); err != nil {
		b.t.Fatalf("Failed to write test config: %v", err)
	}

	return path
}

// WriteYAML writes the configuration to a specific path.
//
// Parameters:
//   - path: Absolute or relative path to write the YAML file
//
// Returns an error if writing fails.
func (b *TestConfigBuilder) WriteYAML(path string) error {
	b.t.Helper()

	data, err := yaml.Marshal(b.config)
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	return nil
}

// Cleanup runs all registered cleanup functions.
//
// This is called automatically by t.Cleanup() when using t.TempDir(),
// but can also be called manually with defer if needed.
func (b *TestConfigBuilder) Cleanup() {
	b.t.Helper()

	for _, cleanup := range b.cleanupFuncs {
		cleanup()
	}
}

// WriteTestConfig is a convenience function for writing a YAML string to a file.
//
// This is useful for tests that have hand-written YAML test cases.
// The file is created in a temporary directory and cleaned up automatically.
//
// Parameters:
//   - t: Testing context
//   - yamlContent: Raw YAML configuration string
//
// Returns the absolute path to the written configuration file.
//
// Example:
//
//	path := WriteTestConfig(t, `
//	version: 1
//	secretStores:
//	  test:
//	    type: literal
//	    values:
//	      API_KEY: "test-key"
//	`)
func WriteTestConfig(t *testing.T, yamlContent string) string {
	t.Helper()

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "dsops.yaml")

	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	return path
}

// LoadTestConfig loads a configuration from a file path.
//
// This is a convenience wrapper around config loading for tests.
// It handles errors by failing the test immediately.
//
// Parameters:
//   - t: Testing context
//   - path: Path to the configuration file
//
// Returns the loaded Configuration Definition.
func LoadTestConfig(t *testing.T, path string) *config.Definition {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var def config.Definition
	if err := yaml.Unmarshal(data, &def); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	return &def
}
