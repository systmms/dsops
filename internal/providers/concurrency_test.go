package providers_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/fakes"
)

// TestConcurrentProviderAccess verifies providers handle concurrent Resolve() calls safely
func TestConcurrentProviderAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	t.Parallel()

	ctx := context.Background()

	// Create fake provider with multiple secrets
	fake := fakes.NewFakeProvider("concurrent-test")
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("secret/%d", i)
		fake.WithSecret(key, provider.SecretValue{
			Value: fmt.Sprintf("secret-value-%d", i),
		})
	}

	// Launch 100 goroutines accessing provider concurrently
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errors := make(chan error, numGoroutines)
	results := make(chan provider.SecretValue, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			key := fmt.Sprintf("secret/%d", id%10) // Access same secrets from multiple goroutines

			secret, err := fake.Resolve(ctx, provider.Reference{
				Provider: "concurrent-test",
				Key:      key,
			})

			if err != nil {
				errors <- err
				return
			}

			results <- secret
		}(i)
	}

	// Wait for all goroutines with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines completed
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for concurrent operations")
	}

	close(errors)
	close(results)

	// Verify no errors occurred
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}
	assert.Empty(t, errs, "No errors should occur during concurrent access")

	// Verify results
	var values []provider.SecretValue
	for val := range results {
		values = append(values, val)
	}
	assert.Len(t, values, numGoroutines, "All goroutines should successfully retrieve secrets")
}

// TestConcurrentProviderResolveAndDescribe verifies concurrent Resolve() and Describe() calls
func TestConcurrentProviderResolveAndDescribe(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	t.Parallel()

	ctx := context.Background()

	fake := fakes.NewFakeProvider("mixed-ops").
		WithSecret("test/secret", provider.SecretValue{
			Value: "test-value",
		}).
		WithMetadata("test/secret", provider.Metadata{
			Exists:    true,
			Version:   "v1",
			UpdatedAt: time.Now(),
		})

	const numOps = 50
	var wg sync.WaitGroup
	wg.Add(numOps * 2) // Resolve + Describe operations

	errors := make(chan error, numOps*2)

	// Launch Resolve() calls
	for i := 0; i < numOps; i++ {
		go func() {
			defer wg.Done()
			_, err := fake.Resolve(ctx, provider.Reference{
				Provider: "mixed-ops",
				Key:      "test/secret",
			})
			if err != nil {
				errors <- err
			}
		}()
	}

	// Launch Describe() calls concurrently
	for i := 0; i < numOps; i++ {
		go func() {
			defer wg.Done()
			_, err := fake.Describe(ctx, provider.Reference{
				Provider: "mixed-ops",
				Key:      "test/secret",
			})
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
	assert.Empty(t, errs, "No errors should occur during concurrent Resolve/Describe")
}

// TestProviderStateConcurrency verifies providers maintain consistent state under concurrent access
func TestProviderStateConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	t.Parallel()

	ctx := context.Background()

	fake := fakes.NewFakeProvider("state-test").
		WithSecret("counter", provider.SecretValue{
			Value: "0",
		})

	const numReads = 100
	var wg sync.WaitGroup
	wg.Add(numReads)

	results := make([]provider.SecretValue, numReads)

	// Read same secret concurrently from many goroutines
	for i := 0; i < numReads; i++ {
		go func(idx int) {
			defer wg.Done()

			secret, err := fake.Resolve(ctx, provider.Reference{
				Provider: "state-test",
				Key:      "counter",
			})

			require.NoError(t, err)
			results[idx] = secret
		}(i)
	}

	wg.Wait()

	// Verify all reads got consistent results
	for i := 0; i < numReads; i++ {
		assert.Equal(t, "0", results[i].Value, "All concurrent reads should return same value")
	}
}

// TestConcurrentProviderValidate verifies Validate() is safe to call concurrently
func TestConcurrentProviderValidate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	t.Parallel()

	ctx := context.Background()

	fake := fakes.NewFakeProvider("validate-test")

	const numValidations = 50
	var wg sync.WaitGroup
	wg.Add(numValidations)

	errors := make(chan error, numValidations)

	for i := 0; i < numValidations; i++ {
		go func() {
			defer wg.Done()
			if err := fake.Validate(ctx); err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Verify no errors occurred
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}
	assert.Empty(t, errs, "Concurrent Validate() calls should not error")
}

// TestRaceDetectorOnProviderMethods verifies race detector catches issues (must run with -race)
func TestRaceDetectorOnProviderMethods(t *testing.T) {
	// This test is designed to be run with: go test -race
	// It will fail if there are data races in provider implementations

	t.Parallel()

	ctx := context.Background()

	fake := fakes.NewFakeProvider("race-test").
		WithSecret("race/secret", provider.SecretValue{
			Value: "test-value",
		})

	// Concurrent access patterns that might trigger races
	const numOps = 20
	var wg sync.WaitGroup

	// Pattern 1: Read-Read races
	wg.Add(numOps)
	for i := 0; i < numOps; i++ {
		go func() {
			defer wg.Done()
			_, _ = fake.Resolve(ctx, provider.Reference{
				Provider: "race-test",
				Key:      "race/secret",
			})
		}()
	}

	// Pattern 2: Capabilities() concurrent access
	wg.Add(numOps)
	for i := 0; i < numOps; i++ {
		go func() {
			defer wg.Done()
			_ = fake.Capabilities()
		}()
	}

	// Pattern 3: Name() concurrent access
	wg.Add(numOps)
	for i := 0; i < numOps; i++ {
		go func() {
			defer wg.Done()
			_ = fake.Name()
		}()
	}

	wg.Wait()

	// If we reach here without race detector failures, test passes
	assert.True(t, true, "No races detected (run with -race flag)")
}

// TestConcurrentProviderWithErrors verifies error handling is thread-safe
func TestConcurrentProviderWithErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	t.Parallel()

	ctx := context.Background()

	fake := fakes.NewFakeProvider("error-test").
		WithError("fail/secret", fmt.Errorf("simulated error"))

	const numAttempts = 50
	var wg sync.WaitGroup
	wg.Add(numAttempts)

	errorCount := 0
	var mu sync.Mutex

	for i := 0; i < numAttempts; i++ {
		go func() {
			defer wg.Done()

			_, err := fake.Resolve(ctx, provider.Reference{
				Provider: "error-test",
				Key:      "fail/secret",
			})

			if err != nil {
				mu.Lock()
				errorCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// All attempts should have errored
	assert.Equal(t, numAttempts, errorCount, "All concurrent calls to failing secret should error")
}

// TestProviderTimeoutConcurrency verifies context cancellation works correctly under concurrent load
func TestProviderTimeoutConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	t.Parallel()

	// Create fake provider with artificial delay
	fake := fakes.NewFakeProvider("timeout-test").
		WithDelay(100 * time.Millisecond).
		WithSecret("slow/secret", provider.SecretValue{
			Value: "slow-value",
		})

	const numOps = 20
	var wg sync.WaitGroup
	wg.Add(numOps)

	timeouts := 0
	successes := 0
	var mu sync.Mutex

	for i := 0; i < numOps; i++ {
		go func(idx int) {
			defer wg.Done()

			// Vary timeout: some will timeout, some won't
			timeout := time.Duration(idx%3+1) * 50 * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			_, err := fake.Resolve(ctx, provider.Reference{
				Provider: "timeout-test",
				Key:      "slow/secret",
			})

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				if err == context.DeadlineExceeded {
					timeouts++
				}
			} else {
				successes++
			}
		}(i)
	}

	wg.Wait()

	// We should have mix of timeouts and successes
	total := timeouts + successes
	assert.Equal(t, numOps, total, "All operations should complete")
	assert.Greater(t, timeouts, 0, "Some operations should timeout")
	assert.Greater(t, successes, 0, "Some operations should succeed")
}
