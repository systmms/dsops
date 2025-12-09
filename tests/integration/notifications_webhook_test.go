// Package integration provides integration tests for dsops.
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/rotation/notifications"
)

// TestWebhookRetry_ExponentialBackoff tests webhook retry with exponential backoff timing.
func TestWebhookRetry_ExponentialBackoff(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var requestTimes []time.Time
	var requestCount int32
	var mu sync.Mutex

	// Mock webhook server that fails first 2 attempts, succeeds on 3rd
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestTimes = append(requestTimes, time.Now())
		mu.Unlock()

		count := atomic.AddInt32(&requestCount, 1)

		// Fail first 2 requests, succeed on 3rd
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create webhook with exponential backoff
	provider := notifications.NewWebhookProvider(notifications.WebhookConfig{
		URL:    server.URL,
		Method: "POST",
		Retry: &notifications.RetryConfig{
			MaxAttempts: 3,
			Backoff:     "exponential",
			InitialWait: 500 * time.Millisecond, // Use shorter waits for testing
		},
	})

	// Send event
	ctx := context.Background()
	event := notifications.RotationEvent{
		Type:        notifications.EventTypeFailed,
		Service:     "test-service",
		Environment: "test",
		Strategy:    "two-key",
		Status:      notifications.StatusFailure,
		Timestamp:   time.Now(),
	}

	err := provider.Send(ctx, event)
	require.NoError(t, err)

	// Validate retry count
	assert.Equal(t, int32(3), requestCount)

	// Validate exponential backoff timing
	// Expected: 0ms (attempt 1), ~500ms (attempt 2), ~1000ms (attempt 3)
	// Exponential: 2^(attempt-1) * initial = 2^0 * 500ms = 500ms, 2^1 * 500ms = 1000ms
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, requestTimes, 3)

	delay1 := requestTimes[1].Sub(requestTimes[0])
	delay2 := requestTimes[2].Sub(requestTimes[1])

	// Allow 100ms tolerance for scheduling
	assert.InDelta(t, float64(500*time.Millisecond), float64(delay1), float64(100*time.Millisecond),
		"First retry should wait ~500ms (exponential: 2^0 * 500ms)")
	assert.InDelta(t, float64(1000*time.Millisecond), float64(delay2), float64(100*time.Millisecond),
		"Second retry should wait ~1000ms (exponential: 2^1 * 500ms)")
}

// TestWebhookRetry_LinearBackoff tests webhook retry with linear backoff timing.
func TestWebhookRetry_LinearBackoff(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var requestTimes []time.Time
	var requestCount int32
	var mu sync.Mutex

	// Mock webhook server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestTimes = append(requestTimes, time.Now())
		mu.Unlock()

		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Create webhook with linear backoff
	provider := notifications.NewWebhookProvider(notifications.WebhookConfig{
		URL:    server.URL,
		Method: "POST",
		Retry: &notifications.RetryConfig{
			MaxAttempts: 3,
			Backoff:     "linear",
			InitialWait: 300 * time.Millisecond, // Use shorter waits for testing
		},
	})

	// Send event
	ctx := context.Background()
	event := notifications.RotationEvent{
		Type:      notifications.EventTypeFailed,
		Service:   "test-service",
		Timestamp: time.Now(),
	}

	err := provider.Send(ctx, event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook failed after 3 attempts")

	// Validate retry count
	assert.Equal(t, int32(3), requestCount)

	// Validate linear backoff timing
	// Expected: attempt 1 * 300ms = 300ms, attempt 2 * 300ms = 600ms
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, requestTimes, 3)

	delay1 := requestTimes[1].Sub(requestTimes[0])
	delay2 := requestTimes[2].Sub(requestTimes[1])

	assert.InDelta(t, float64(300*time.Millisecond), float64(delay1), float64(100*time.Millisecond),
		"First retry should wait ~300ms (linear: 1 * 300ms)")
	assert.InDelta(t, float64(600*time.Millisecond), float64(delay2), float64(100*time.Millisecond),
		"Second retry should wait ~600ms (linear: 2 * 300ms)")
}

// TestWebhookRetry_FixedBackoff tests webhook retry with fixed backoff timing.
func TestWebhookRetry_FixedBackoff(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var requestTimes []time.Time
	var requestCount int32
	var mu sync.Mutex

	// Mock webhook server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestTimes = append(requestTimes, time.Now())
		mu.Unlock()

		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Create webhook with fixed backoff
	provider := notifications.NewWebhookProvider(notifications.WebhookConfig{
		URL:    server.URL,
		Method: "POST",
		Retry: &notifications.RetryConfig{
			MaxAttempts: 3,
			Backoff:     "fixed",
			InitialWait: 400 * time.Millisecond,
		},
	})

	// Send event
	ctx := context.Background()
	event := notifications.RotationEvent{
		Type:      notifications.EventTypeFailed,
		Service:   "test-service",
		Timestamp: time.Now(),
	}

	err := provider.Send(ctx, event)
	require.Error(t, err)

	// Validate retry count
	assert.Equal(t, int32(3), requestCount)

	// Validate fixed backoff timing - all delays should be the same
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, requestTimes, 3)

	delay1 := requestTimes[1].Sub(requestTimes[0])
	delay2 := requestTimes[2].Sub(requestTimes[1])

	assert.InDelta(t, float64(400*time.Millisecond), float64(delay1), float64(100*time.Millisecond),
		"First retry should wait ~400ms (fixed)")
	assert.InDelta(t, float64(400*time.Millisecond), float64(delay2), float64(100*time.Millisecond),
		"Second retry should wait ~400ms (fixed)")
}

// TestWebhookRetry_SuccessFirstAttempt tests that no retries occur on immediate success.
func TestWebhookRetry_SuccessFirstAttempt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var requestCount int32

	// Mock webhook server that always succeeds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create webhook provider
	provider := notifications.NewWebhookProvider(notifications.WebhookConfig{
		URL:    server.URL,
		Method: "POST",
		Retry: &notifications.RetryConfig{
			MaxAttempts: 3,
			Backoff:     "exponential",
			InitialWait: 1 * time.Second,
		},
	})

	// Send event
	ctx := context.Background()
	event := notifications.RotationEvent{
		Type:      notifications.EventTypeCompleted,
		Service:   "test-service",
		Status:    notifications.StatusSuccess,
		Timestamp: time.Now(),
	}

	startTime := time.Now()
	err := provider.Send(ctx, event)
	duration := time.Since(startTime)

	require.NoError(t, err)

	// Validate exactly 1 request (no retries)
	assert.Equal(t, int32(1), requestCount)

	// Should complete quickly (no retry delays)
	assert.Less(t, duration, 200*time.Millisecond, "Should complete immediately without retries")
}

// TestWebhookRetry_MaxAttemptsExhausted tests behavior when all retry attempts fail.
func TestWebhookRetry_MaxAttemptsExhausted(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var requestCount int32

	// Mock webhook server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Create webhook with 5 max attempts
	provider := notifications.NewWebhookProvider(notifications.WebhookConfig{
		URL:    server.URL,
		Method: "POST",
		Retry: &notifications.RetryConfig{
			MaxAttempts: 5,
			Backoff:     "fixed",
			InitialWait: 100 * time.Millisecond, // Fast for testing
		},
	})

	// Send event
	ctx := context.Background()
	event := notifications.RotationEvent{
		Type:      notifications.EventTypeFailed,
		Service:   "test-service",
		Timestamp: time.Now(),
	}

	err := provider.Send(ctx, event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook failed after 5 attempts")

	// Validate exactly max_attempts requests
	assert.Equal(t, int32(5), requestCount)
}

// TestWebhookRetry_ContextCancellation tests that context cancellation stops retries.
func TestWebhookRetry_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var requestCount int32

	// Mock webhook server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Create webhook with long backoff
	provider := notifications.NewWebhookProvider(notifications.WebhookConfig{
		URL:    server.URL,
		Method: "POST",
		Retry: &notifications.RetryConfig{
			MaxAttempts: 5,
			Backoff:     "fixed",
			InitialWait: 2 * time.Second, // Long wait to test cancellation
		},
	})

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Send event in goroutine
	errChan := make(chan error, 1)
	go func() {
		event := notifications.RotationEvent{
			Type:      notifications.EventTypeFailed,
			Service:   "test-service",
			Timestamp: time.Now(),
		}
		errChan <- provider.Send(ctx, event)
	}()

	// Wait for first attempt to fail, then cancel
	time.Sleep(200 * time.Millisecond)
	cancel()

	// Wait for Send to return
	err := <-errChan
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")

	// Should have only 1 or 2 requests (cancelled before completing retries)
	count := atomic.LoadInt32(&requestCount)
	assert.LessOrEqual(t, count, int32(2), "Should cancel before all retries")
}

// TestWebhookRetry_CustomPayloadAndHeaders tests that custom payloads and headers work with retries.
func TestWebhookRetry_CustomPayloadAndHeaders(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var receivedHeaders http.Header
	var receivedBody string
	var requestCount int32
	var mu sync.Mutex

	// Mock webhook server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		// Capture headers and body from first request
		if count == 1 {
			mu.Lock()
			receivedHeaders = r.Header.Clone()
			bodyBytes, _ := io.ReadAll(r.Body)
			receivedBody = string(bodyBytes)
			mu.Unlock()
		}

		// Fail first request, succeed on second
		if count < 2 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create webhook with custom template and headers
	customTemplate := `{"message": "Rotation {{.Type}} for {{.Service}}", "env": "{{.Environment}}"}`
	provider := notifications.NewWebhookProvider(notifications.WebhookConfig{
		URL:             server.URL,
		Method:          "POST",
		PayloadTemplate: customTemplate,
		Headers: map[string]string{
			"Authorization": "Bearer test-token",
			"X-Custom-Header": "custom-value",
		},
		Retry: &notifications.RetryConfig{
			MaxAttempts: 2,
			Backoff:     "fixed",
			InitialWait: 200 * time.Millisecond,
		},
	})

	// Send event
	ctx := context.Background()
	event := notifications.RotationEvent{
		Type:        notifications.EventTypeFailed,
		Service:     "payment-api",
		Environment: "production",
		Status:      notifications.StatusFailure,
		Timestamp:   time.Now(),
	}

	err := provider.Send(ctx, event)
	require.NoError(t, err)

	// Validate request count
	assert.Equal(t, int32(2), requestCount)

	// Validate custom headers
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "Bearer test-token", receivedHeaders.Get("Authorization"))
	assert.Equal(t, "custom-value", receivedHeaders.Get("X-Custom-Header"))
	assert.Equal(t, "application/json", receivedHeaders.Get("Content-Type"))

	// Validate custom payload
	assert.Contains(t, receivedBody, `"message": "Rotation failed for payment-api"`)
	assert.Contains(t, receivedBody, `"env": "production"`)
}

// TestWebhookRetry_DifferentHTTPMethods tests that different HTTP methods work with retries.
func TestWebhookRetry_DifferentHTTPMethods(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	methods := []string{"POST", "PUT", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			var receivedMethod string
			var requestCount int32

			// Mock webhook server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count := atomic.AddInt32(&requestCount, 1)
				receivedMethod = r.Method

				// Fail first request, succeed on second
				if count < 2 {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			// Create webhook with specific method
			provider := notifications.NewWebhookProvider(notifications.WebhookConfig{
				URL:    server.URL,
				Method: method,
				Retry: &notifications.RetryConfig{
					MaxAttempts: 2,
					Backoff:     "fixed",
					InitialWait: 100 * time.Millisecond,
				},
			})

			// Send event
			ctx := context.Background()
			event := notifications.RotationEvent{
				Type:      notifications.EventTypeCompleted,
				Service:   "test-service",
				Timestamp: time.Now(),
			}

			err := provider.Send(ctx, event)
			require.NoError(t, err)

			// Validate method
			assert.Equal(t, method, receivedMethod)
			assert.Equal(t, int32(2), requestCount)
		})
	}
}

// TestWebhookRetry_DefaultPayloadStructure tests the default JSON payload structure.
func TestWebhookRetry_DefaultPayloadStructure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var receivedPayload map[string]interface{}
	var mu sync.Mutex

	// Mock webhook server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		var payload map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err == nil {
			receivedPayload = payload
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create webhook with default payload
	provider := notifications.NewWebhookProvider(notifications.WebhookConfig{
		URL:    server.URL,
		Method: "POST",
	})

	// Send event with all fields
	ctx := context.Background()
	event := notifications.RotationEvent{
		Type:        notifications.EventTypeFailed,
		Service:     "database",
		Environment: "staging",
		Strategy:    "two-key",
		Status:      notifications.StatusFailure,
		Error:       fmt.Errorf("connection timeout"),
		Duration:    5 * time.Second,
		Timestamp:   time.Now(),
		Metadata: map[string]string{
			"rotation_id": "rot-123",
			"version":     "v2",
		},
	}

	err := provider.Send(ctx, event)
	require.NoError(t, err)

	// Validate payload structure
	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, receivedPayload)

	assert.Equal(t, "failed", receivedPayload["event"])
	assert.Equal(t, "database", receivedPayload["service"])
	assert.Equal(t, "staging", receivedPayload["environment"])
	assert.Equal(t, "failure", receivedPayload["status"])
	assert.Equal(t, "two-key", receivedPayload["strategy"])
	assert.Equal(t, "connection timeout", receivedPayload["error"])
	assert.Equal(t, 5.0, receivedPayload["duration_seconds"])
	assert.NotEmpty(t, receivedPayload["timestamp"])

	// Validate metadata
	metadata, ok := receivedPayload["metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "rot-123", metadata["rotation_id"])
	assert.Equal(t, "v2", metadata["version"])
}
