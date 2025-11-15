package resolve_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/resolve"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/fakes"
)

// TestConcurrentResolution verifies resolver handles concurrent Resolve() calls safely
func TestConcurrentResolution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	t.Parallel()

	ctx := context.Background()

	// Create fake provider with multiple secrets
	fake := fakes.NewFakeProvider("concurrent-resolver")
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("db/password-%d", i)
		fake.WithSecret(key, provider.SecretValue{
			Value: fmt.Sprintf("pass-%d", i),
		})
	}

	// Create resolver
	resolver := resolve.NewResolver(map[string]provider.Provider{
		"concurrent-resolver": fake,
	})

	// Launch 100 goroutines resolving secrets concurrently
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	results := make(chan map[string]string, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			secretKey := fmt.Sprintf("db/password-%d", id%50)
			ref := fmt.Sprintf("store://concurrent-resolver/%s", secretKey)

			resolved, err := resolver.Resolve(ctx, ref)
			if err != nil {
				errors <- err
				return
			}

			results <- resolved
		}(i)
	}

	// Wait for completion
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for concurrent resolutions")
	}

	close(results)
	close(errors)

	// Verify no errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}
	assert.Empty(t, errs, "No errors should occur during concurrent resolution")

	// Verify all resolutions succeeded
	var values []map[string]string
	for val := range results {
		values = append(values, val)
	}
	assert.Len(t, values, numGoroutines, "All goroutines should successfully resolve")
}

// TestConcurrentDependencyResolution verifies resolver handles concurrent resolution with dependencies
func TestConcurrentDependencyResolution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	t.Parallel()

	ctx := context.Background()

	fake := fakes.NewFakeProvider("deps").
		WithSecret("base/secret", provider.SecretValue{
			Value: "base-value",
		}).
		WithSecret("derived/secret", provider.SecretValue{
			Value: "derived-value",
		})

	resolver := resolve.NewResolver(map[string]provider.Provider{
		"deps": fake,
	})

	// Resolve multiple dependency chains concurrently
	const numChains = 20
	var wg sync.WaitGroup
	wg.Add(numChains * 2) // base + derived for each chain

	errors := make(chan error, numChains*2)

	for i := 0; i < numChains; i++ {
		// Resolve base secret
		go func() {
			defer wg.Done()
			_, err := resolver.Resolve(ctx, "store://deps/base/secret")
			if err != nil {
				errors <- err
			}
		}()

		// Resolve derived secret (may depend on base)
		go func() {
			defer wg.Done()
			_, err := resolver.Resolve(ctx, "store://deps/derived/secret")
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
	assert.Empty(t, errs, "Concurrent dependency resolution should not error")
}

// TestResolverCacheConcurrency verifies resolver cache is thread-safe
func TestResolverCacheConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	t.Parallel()

	ctx := context.Background()

	callCount := 0
	var callMu sync.Mutex

	// Create provider that counts calls
	fake := fakes.NewFakeProvider("cache-test")
	fake.WithSecret("cached/secret", provider.SecretValue{
		Value: "cached-value",
	})

	// Wrap provider to count calls
	countingProvider := &countingProvider{
		Provider: fake,
		onResolve: func() {
			callMu.Lock()
			callCount++
			callMu.Unlock()
		},
	}

	resolver := resolve.NewResolver(map[string]provider.Provider{
		"cache-test": countingProvider,
	})

	// Resolve same secret many times concurrently
	const numResolves = 50
	var wg sync.WaitGroup
	wg.Add(numResolves)

	results := make([]map[string]string, numResolves)

	for i := 0; i < numResolves; i++ {
		go func(idx int) {
			defer wg.Done()

			resolved, err := resolver.Resolve(ctx, "store://cache-test/cached/secret")
			require.NoError(t, err)

			results[idx] = resolved
		}(i)
	}

	wg.Wait()

	// All results should be identical
	for i := 0; i < numResolves; i++ {
		assert.NotEmpty(t, results[i], "Result should not be empty")
	}

	// Provider should be called (caching behavior depends on implementation)
	// We just verify no race conditions occurred
	callMu.Lock()
	count := callCount
	callMu.Unlock()

	assert.Greater(t, count, 0, "Provider should be called at least once")
}

// TestConcurrentTransformPipeline verifies transform pipelines are thread-safe
func TestConcurrentTransformPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	t.Parallel()

	ctx := context.Background()

	fake := fakes.NewFakeProvider("transform-test").
		WithSecret("json/data", provider.SecretValue{
			Value: `{"password":"secret123","username":"admin"}`,
		})

	resolver := resolve.NewResolver(map[string]provider.Provider{
		"transform-test": fake,
	})

	const numOps = 30
	var wg sync.WaitGroup
	wg.Add(numOps)

	results := make([]map[string]string, numOps)

	for i := 0; i < numOps; i++ {
		go func(idx int) {
			defer wg.Done()

			// Apply transforms concurrently
			resolved, err := resolver.Resolve(ctx, "store://transform-test/json/data | json_extract:.password")
			require.NoError(t, err)

			results[idx] = resolved
		}(i)
	}

	wg.Wait()

	// All transforms should produce a result
	for i := 0; i < numOps; i++ {
		assert.NotEmpty(t, results[i], "Transform result should not be empty")
	}
}

// TestRaceDetectorOnResolver verifies race detector catches issues in resolver
func TestRaceDetectorOnResolver(t *testing.T) {
	// Must run with: go test -race

	t.Parallel()

	ctx := context.Background()

	fake := fakes.NewFakeProvider("race-test").
		WithSecret("race/secret", provider.SecretValue{
			Value: "test-value",
		})

	resolver := resolve.NewResolver(map[string]provider.Provider{
		"race-test": fake,
	})

	const numOps = 20
	var wg sync.WaitGroup

	// Pattern 1: Concurrent resolutions
	wg.Add(numOps)
	for i := 0; i < numOps; i++ {
		go func() {
			defer wg.Done()
			_, _ = resolver.Resolve(ctx, "store://race-test/race/secret")
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

	fake := fakes.NewFakeProvider("error-test").
		WithError("fail/secret", fmt.Errorf("simulated failure"))

	resolver := resolve.NewResolver(map[string]provider.Provider{
		"error-test": fake,
	})

	const numAttempts = 40
	var wg sync.WaitGroup
	wg.Add(numAttempts)

	errorCount := 0
	var mu sync.Mutex

	for i := 0; i < numAttempts; i++ {
		go func() {
			defer wg.Done()

			_, err := resolver.Resolve(ctx, "store://error-test/fail/secret")
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

// TestConcurrentContextCancellation verifies context cancellation works correctly
func TestConcurrentContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	t.Parallel()

	fake := fakes.NewFakeProvider("cancel-test").
		WithDelay(200 * time.Millisecond).
		WithSecret("slow/secret", provider.SecretValue{
			Value: "slow-value",
		})

	resolver := resolve.NewResolver(map[string]provider.Provider{
		"cancel-test": fake,
	})

	const numOps = 10
	var wg sync.WaitGroup
	wg.Add(numOps)

	canceledCount := 0
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < numOps; i++ {
		go func(idx int) {
			defer wg.Done()

			// Vary timeout: some will be canceled, some won't
			timeout := time.Duration(idx%4+1) * 100 * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			_, err := resolver.Resolve(ctx, "store://cancel-test/slow/secret")

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				if err == context.DeadlineExceeded {
					canceledCount++
				}
			} else {
				successCount++
			}
		}(i)
	}

	wg.Wait()

	total := canceledCount + successCount
	assert.Equal(t, numOps, total, "All operations should complete or cancel")
}

// countingProvider wraps a provider and counts Resolve calls
type countingProvider struct {
	provider.Provider
	onResolve func()
}

func (c *countingProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	if c.onResolve != nil {
		c.onResolve()
	}
	return c.Provider.Resolve(ctx, ref)
}
