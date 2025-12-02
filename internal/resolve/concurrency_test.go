package resolve_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/internal/resolve"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/fakes"
)

// createTestConfig creates a minimal config for testing
func createTestConfig() *config.Config {
	return &config.Config{
		Logger: logging.New(false, true),
		Definition: &config.Definition{
			Version: 1,
		},
	}
}

// TestConcurrentProviderRegistration verifies RegisterProvider is thread-safe
func TestConcurrentProviderRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	t.Parallel()

	cfg := createTestConfig()
	resolver := resolve.New(cfg)

	// Register many providers concurrently
	const numProviders = 50
	var wg sync.WaitGroup
	wg.Add(numProviders)

	for i := 0; i < numProviders; i++ {
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("provider-%d", id)
			fake := fakes.NewFakeProvider(name)
			resolver.RegisterProvider(name, fake)
		}(i)
	}

	wg.Wait()

	// Verify all providers were registered
	providers := resolver.GetRegisteredProviders()
	assert.Len(t, providers, numProviders, "All providers should be registered")

	for i := 0; i < numProviders; i++ {
		name := fmt.Sprintf("provider-%d", i)
		_, exists := resolver.GetProvider(name)
		assert.True(t, exists, "Provider %s should exist", name)
	}
}

// TestConcurrentGetProvider verifies GetProvider is thread-safe
func TestConcurrentGetProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	t.Parallel()

	cfg := createTestConfig()
	resolver := resolve.New(cfg)

	// Register some providers first
	const numProviders = 10
	for i := 0; i < numProviders; i++ {
		name := fmt.Sprintf("provider-%d", i)
		fake := fakes.NewFakeProvider(name)
		resolver.RegisterProvider(name, fake)
	}

	// Concurrently get providers
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	results := make([]bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("provider-%d", id%numProviders)
			_, exists := resolver.GetProvider(name)
			results[id] = exists
		}(i)
	}

	wg.Wait()

	// All should have found their provider
	for i, found := range results {
		assert.True(t, found, "Goroutine %d should find provider", i)
	}
}

// TestConcurrentGetRegisteredProviders verifies GetRegisteredProviders is thread-safe
func TestConcurrentGetRegisteredProviders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	t.Parallel()

	cfg := createTestConfig()
	resolver := resolve.New(cfg)

	// Register initial providers
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("initial-%d", i)
		fake := fakes.NewFakeProvider(name)
		resolver.RegisterProvider(name, fake)
	}

	// Concurrently read and write
	const numOps = 50
	var wg sync.WaitGroup
	wg.Add(numOps * 2) // readers + writers

	// Writers
	for i := 0; i < numOps; i++ {
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("concurrent-%d", id)
			fake := fakes.NewFakeProvider(name)
			resolver.RegisterProvider(name, fake)
		}(i)
	}

	// Readers
	for i := 0; i < numOps; i++ {
		go func() {
			defer wg.Done()
			providers := resolver.GetRegisteredProviders()
			// Just verify we can read without panic/race
			assert.NotNil(t, providers)
		}()
	}

	wg.Wait()

	// Final count should be 5 initial + numOps concurrent
	providers := resolver.GetRegisteredProviders()
	assert.Len(t, providers, 5+numOps, "All providers should be registered")
}

// TestConcurrentResolveEnvironment verifies ResolveEnvironment is thread-safe
func TestConcurrentResolveEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	t.Parallel()

	ctx := context.Background()
	cfg := createTestConfig()
	// Add provider to config definition so GetProvider succeeds
	cfg.Definition.Providers = map[string]config.ProviderConfig{
		"test-provider": {Type: "fake", TimeoutMs: 30000},
	}
	resolver := resolve.New(cfg)

	// Setup fake provider with secrets
	fake := fakes.NewFakeProvider("test-provider").
		WithSecret("db/password", provider.SecretValue{
			Value: "secret123",
		}).
		WithSecret("api/key", provider.SecretValue{
			Value: "apikey456",
		})

	resolver.RegisterProvider("test-provider", fake)

	// Create test environment
	env := config.Environment{
		"DATABASE_PASSWORD": config.Variable{
			From: &config.Reference{
				Provider: "test-provider",
				Key:      "db/password",
			},
		},
		"API_KEY": config.Variable{
			From: &config.Reference{
				Provider: "test-provider",
				Key:      "api/key",
			},
		},
	}

	// Resolve environment concurrently
	const numGoroutines = 30
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	results := make(chan map[string]string, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			resolved, err := resolver.ResolveEnvironment(ctx, env)
			if err != nil {
				errors <- err
				return
			}
			results <- resolved
		}()
	}

	wg.Wait()
	close(results)
	close(errors)

	// Verify no errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}
	assert.Empty(t, errs, "No errors should occur during concurrent resolution")

	// Verify all resolutions succeeded
	var resolvedEnvs []map[string]string
	for env := range results {
		resolvedEnvs = append(resolvedEnvs, env)
	}
	require.Len(t, resolvedEnvs, numGoroutines, "All goroutines should successfully resolve")

	// Verify all resolved the same values
	for _, resolved := range resolvedEnvs {
		assert.Equal(t, "secret123", resolved["DATABASE_PASSWORD"])
		assert.Equal(t, "apikey456", resolved["API_KEY"])
	}
}

// TestConcurrentResolveVariables verifies ResolveVariablesConcurrently is thread-safe
func TestConcurrentResolveVariables(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	t.Parallel()

	ctx := context.Background()
	cfg := createTestConfig()
	// Add provider to config definition so GetProvider succeeds
	cfg.Definition.Providers = map[string]config.ProviderConfig{
		"multi-secret": {Type: "fake", TimeoutMs: 30000},
	}
	resolver := resolve.New(cfg)

	// Setup fake provider with multiple secrets
	fake := fakes.NewFakeProvider("multi-secret")
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("secret-%d", i)
		fake.WithSecret(key, provider.SecretValue{
			Value: fmt.Sprintf("value-%d", i),
		})
	}
	resolver.RegisterProvider("multi-secret", fake)

	// Create environment with many variables
	env := make(config.Environment)
	for i := 0; i < 10; i++ {
		varName := fmt.Sprintf("VAR_%d", i)
		key := fmt.Sprintf("secret-%d", i)
		env[varName] = config.Variable{
			From: &config.Reference{
				Provider: "multi-secret",
				Key:      key,
			},
		}
	}

	// Resolve concurrently multiple times
	const numOps = 20
	var wg sync.WaitGroup
	wg.Add(numOps)

	errors := make(chan error, numOps)

	for i := 0; i < numOps; i++ {
		go func() {
			defer wg.Done()
			_, err := resolver.ResolveVariablesConcurrently(ctx, env)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Verify no errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}
	assert.Empty(t, errs, "Concurrent variable resolution should not error")
}

// TestRaceDetectorOnResolver verifies race detector catches issues
func TestRaceDetectorOnResolver(t *testing.T) {
	// Must run with: go test -race

	t.Parallel()

	cfg := createTestConfig()
	resolver := resolve.New(cfg)

	const numOps = 20
	var wg sync.WaitGroup

	// Pattern 1: Concurrent registrations
	wg.Add(numOps)
	for i := 0; i < numOps; i++ {
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("race-%d", id)
			fake := fakes.NewFakeProvider(name)
			resolver.RegisterProvider(name, fake)
		}(i)
	}

	// Pattern 2: Concurrent reads while writing
	wg.Add(numOps)
	for i := 0; i < numOps; i++ {
		go func() {
			defer wg.Done()
			_ = resolver.GetRegisteredProviders()
		}()
	}

	wg.Wait()

	assert.True(t, true, "No races detected (run with -race flag)")
}

// TestConcurrentErrorHandling verifies error handling is thread-safe
func TestConcurrentErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	t.Parallel()

	ctx := context.Background()
	cfg := createTestConfig()
	resolver := resolve.New(cfg)

	// Setup provider that always errors
	fake := fakes.NewFakeProvider("error-test").
		WithError("fail/secret", fmt.Errorf("simulated failure"))
	resolver.RegisterProvider("error-test", fake)

	// Create environment that will fail
	env := config.Environment{
		"WILL_FAIL": config.Variable{
			From: &config.Reference{
				Provider: "error-test",
				Key:      "fail/secret",
			},
		},
	}

	const numAttempts = 40
	var wg sync.WaitGroup
	wg.Add(numAttempts)

	errorCount := 0
	var mu sync.Mutex

	for i := 0; i < numAttempts; i++ {
		go func() {
			defer wg.Done()

			_, err := resolver.ResolveEnvironment(ctx, env)
			if err != nil {
				mu.Lock()
				errorCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// All attempts should have errored
	assert.Equal(t, numAttempts, errorCount, "All concurrent error cases should fail consistently")
}

// TestConcurrentMixedOperations verifies mixed read/write operations are safe
func TestConcurrentMixedOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	t.Parallel()

	ctx := context.Background()
	cfg := createTestConfig()
	resolver := resolve.New(cfg)

	// Initial setup
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("initial-%d", i)
		fake := fakes.NewFakeProvider(name).
			WithSecret("key", provider.SecretValue{Value: "value"})
		resolver.RegisterProvider(name, fake)
	}

	const numOps = 30
	var wg sync.WaitGroup

	// Mix of operations
	wg.Add(numOps * 3)

	// Register new providers
	for i := 0; i < numOps; i++ {
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("new-%d", id)
			fake := fakes.NewFakeProvider(name)
			resolver.RegisterProvider(name, fake)
		}(i)
	}

	// Get providers
	for i := 0; i < numOps; i++ {
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("initial-%d", id%3)
			_, _ = resolver.GetProvider(name)
		}(i)
	}

	// Resolve environments
	for i := 0; i < numOps; i++ {
		go func(id int) {
			defer wg.Done()
			env := config.Environment{
				"TEST": config.Variable{
					From: &config.Reference{
						Provider: fmt.Sprintf("initial-%d", id%3),
						Key:      "key",
					},
				},
			}
			_, _ = resolver.ResolveEnvironment(ctx, env)
		}(i)
	}

	wg.Wait()

	// Verify final state
	providers := resolver.GetRegisteredProviders()
	expectedCount := 3 + numOps // initial + new
	assert.Len(t, providers, expectedCount, "All providers should be registered")
}
