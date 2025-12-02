package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/systmms/dsops/internal/config"
	"gopkg.in/yaml.v3"
)

// TestFixture provides convenient access to test fixtures.
//
// This helper loads pre-defined test data from the tests/fixtures/ directory.
// It caches loaded fixtures to avoid re-reading files.
//
// Example usage:
//
//	fixtures := NewTestFixture(t)
//	cfg := fixtures.LoadConfig("simple.yaml")
//	secrets := fixtures.LoadJSON("vault-secrets.json")
type TestFixture struct {
	baseDir string
	cache   map[string][]byte
	t       *testing.T
}

// NewTestFixture creates a new TestFixture helper.
//
// The base directory is automatically determined from the project root.
func NewTestFixture(t *testing.T) *TestFixture {
	t.Helper()

	// Find project root by looking for go.mod
	baseDir := findProjectRoot(t)
	fixturesDir := filepath.Join(baseDir, "tests", "fixtures")

	return &TestFixture{
		baseDir: fixturesDir,
		cache:   make(map[string][]byte),
		t:       t,
	}
}

// LoadConfig loads a configuration fixture by name.
//
// The name should be relative to tests/fixtures/configs/
// Example: LoadConfig("simple.yaml")
//
// Returns a parsed config.Definition or fails the test.
func (f *TestFixture) LoadConfig(name string) *config.Definition {
	f.t.Helper()

	path := filepath.Join(f.baseDir, "configs", name)
	data, err := f.loadFile(path)
	if err != nil {
		f.t.Fatalf("Failed to load config fixture %s: %v", name, err)
	}

	var def config.Definition
	if err := yaml.Unmarshal(data, &def); err != nil {
		f.t.Fatalf("Failed to parse config fixture %s: %v", name, err)
	}

	return &def
}

// LoadYAML loads a YAML fixture and returns it as a map.
//
// The name should be relative to tests/fixtures/
// Example: LoadYAML("services/postgresql.yaml")
//
// Returns a map[string]any or fails the test.
func (f *TestFixture) LoadYAML(name string) map[string]any {
	f.t.Helper()

	path := filepath.Join(f.baseDir, name)
	data, err := f.loadFile(path)
	if err != nil {
		f.t.Fatalf("Failed to load YAML fixture %s: %v", name, err)
	}

	var result map[string]any
	if err := yaml.Unmarshal(data, &result); err != nil {
		f.t.Fatalf("Failed to parse YAML fixture %s: %v", name, err)
	}

	return result
}

// LoadJSON loads a JSON fixture and returns it as a map.
//
// The name should be relative to tests/fixtures/
// Example: LoadJSON("secrets/vault-secrets.json")
//
// Returns a map[string]any or fails the test.
func (f *TestFixture) LoadJSON(name string) map[string]any {
	f.t.Helper()

	path := filepath.Join(f.baseDir, name)
	data, err := f.loadFile(path)
	if err != nil {
		f.t.Fatalf("Failed to load JSON fixture %s: %v", name, err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		f.t.Fatalf("Failed to parse JSON fixture %s: %v", name, err)
	}

	return result
}

// LoadFile loads a raw file fixture.
//
// The name should be relative to tests/fixtures/
// Returns the file contents as bytes or fails the test.
func (f *TestFixture) LoadFile(name string) []byte {
	f.t.Helper()

	path := filepath.Join(f.baseDir, name)
	data, err := f.loadFile(path)
	if err != nil {
		f.t.Fatalf("Failed to load file fixture %s: %v", name, err)
	}

	return data
}

// ConfigPath returns the absolute path to a config fixture.
//
// Useful when you need the path itself rather than loading the config.
// Example: ConfigPath("multi-provider.yaml")
func (f *TestFixture) ConfigPath(name string) string {
	f.t.Helper()

	return filepath.Join(f.baseDir, "configs", name)
}

// SecretPath returns the absolute path to a secret fixture.
//
// Example: SecretPath("vault-secrets.json")
func (f *TestFixture) SecretPath(name string) string {
	f.t.Helper()

	return filepath.Join(f.baseDir, "secrets", name)
}

// ServicePath returns the absolute path to a service fixture.
//
// Example: ServicePath("postgresql.yaml")
func (f *TestFixture) ServicePath(name string) string {
	f.t.Helper()

	return filepath.Join(f.baseDir, "services", name)
}

// loadFile loads a file with caching.
func (f *TestFixture) loadFile(path string) ([]byte, error) {
	// Check cache first
	if data, ok := f.cache[path]; ok {
		return data, nil
	}

	// Load from disk
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Cache for future use
	f.cache[path] = data

	return data, nil
}

// findProjectRoot finds the project root directory by looking for go.mod.
//
// This allows fixtures to be loaded regardless of where tests are executed from.
func findProjectRoot(t *testing.T) string {
	t.Helper()

	// Start from current directory and walk up
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	for {
		// Check if go.mod exists
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding go.mod
			t.Fatal("Could not find project root (go.mod not found)")
		}
		dir = parent
	}
}
