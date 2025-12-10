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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/rotation/notifications"
)

// pagerdutyEvent represents a PagerDuty Events API v2 event structure.
type pagerdutyEvent struct {
	RoutingKey  string                 `json:"routing_key"`
	EventAction string                 `json:"event_action"`
	DedupKey    string                 `json:"dedup_key"`
	Payload     map[string]interface{} `json:"payload"`
}

// TestPagerDutyEvents_TriggerIncident tests creating a PagerDuty incident for a failed rotation.
func TestPagerDutyEvents_TriggerIncident(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var receivedEvents []pagerdutyEvent
	var mu sync.Mutex

	// Mock PagerDuty Events API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate HTTP method and headers
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v2/enqueue", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse and capture event
		var event pagerdutyEvent
		err := json.NewDecoder(r.Body).Decode(&event)
		require.NoError(t, err)

		mu.Lock()
		receivedEvents = append(receivedEvents, event)
		mu.Unlock()

		// Return success response (PagerDuty uses 202 Accepted)
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "success",
			"message":  "Event processed",
			"dedup_key": event.DedupKey,
		})
	}))
	defer server.Close()

	// Create PagerDuty provider with mock URL
	provider := notifications.NewPagerDutyProvider(notifications.PagerDutyConfig{
		IntegrationKey: "test-integration-key",
		Severity:       "error",
		AutoResolve:    true,
	})
	// Override API URL to use test server
	provider.SetAPIURL(server.URL + "/v2/enqueue")

	// Send failed rotation event
	ctx := context.Background()
	failedEvent := notifications.RotationEvent{
		Type:        notifications.EventTypeFailed,
		Service:     "payment-api",
		Environment: "production",
		Strategy:    "two-key",
		Status:      notifications.StatusFailure,
		Error:       fmt.Errorf("database connection timeout"),
		Duration:    30 * time.Second,
		Timestamp:   time.Now(),
		Metadata: map[string]string{
			"rotation_id": "rot-12345",
			"version":     "v2.1.0",
		},
	}

	err := provider.Send(ctx, failedEvent)
	require.NoError(t, err)

	// Validate event was received
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, receivedEvents, 1)

	event := receivedEvents[0]

	// Validate event structure
	assert.Equal(t, "test-integration-key", event.RoutingKey)
	assert.Equal(t, "trigger", event.EventAction)
	assert.Equal(t, "dsops-payment-api-production-rot-12345", event.DedupKey)

	// Validate payload
	require.NotNil(t, event.Payload)
	assert.Equal(t, "error", event.Payload["severity"])
	assert.Equal(t, "dsops-rotation", event.Payload["source"])
	assert.Contains(t, event.Payload["summary"], "dsops rotation failed")
	assert.Contains(t, event.Payload["summary"], "payment-api")
	assert.Contains(t, event.Payload["summary"], "production")

	// Validate custom details
	customDetails, ok := event.Payload["custom_details"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "payment-api", customDetails["service"])
	assert.Equal(t, "production", customDetails["environment"])
	assert.Equal(t, "failed", customDetails["event_type"])
	assert.Equal(t, "failure", customDetails["status"])
	assert.Equal(t, "two-key", customDetails["strategy"])
	assert.Equal(t, "database connection timeout", customDetails["error"])
	assert.Equal(t, "rot-12345", customDetails["rotation_id"])
	assert.Equal(t, "v2.1.0", customDetails["version"])
}

// TestPagerDutyEvents_ResolveIncident tests auto-resolving an incident on successful rotation.
func TestPagerDutyEvents_ResolveIncident(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var receivedEvents []pagerdutyEvent
	var mu sync.Mutex

	// Mock PagerDuty Events API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event pagerdutyEvent
		_ = json.NewDecoder(r.Body).Decode(&event)

		mu.Lock()
		receivedEvents = append(receivedEvents, event)
		mu.Unlock()

		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "success",
			"dedup_key": event.DedupKey,
		})
	}))
	defer server.Close()

	// Create PagerDuty provider with auto-resolve enabled
	provider := notifications.NewPagerDutyProvider(notifications.PagerDutyConfig{
		IntegrationKey: "test-key",
		AutoResolve:    true,
	})
	provider.SetAPIURL(server.URL + "/v2/enqueue")

	ctx := context.Background()

	// Send failed event first (trigger)
	failedEvent := notifications.RotationEvent{
		Type:        notifications.EventTypeFailed,
		Service:     "database",
		Environment: "staging",
		Status:      notifications.StatusFailure,
		Timestamp:   time.Now(),
		Metadata: map[string]string{
			"rotation_id": "rot-999",
		},
	}

	err := provider.Send(ctx, failedEvent)
	require.NoError(t, err)

	// Send completed event (resolve)
	completedEvent := notifications.RotationEvent{
		Type:        notifications.EventTypeCompleted,
		Service:     "database",
		Environment: "staging",
		Status:      notifications.StatusSuccess,
		Duration:    45 * time.Second,
		Timestamp:   time.Now(),
		Metadata: map[string]string{
			"rotation_id": "rot-999",
		},
	}

	err = provider.Send(ctx, completedEvent)
	require.NoError(t, err)

	// Validate both events received
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, receivedEvents, 2)

	// Validate trigger event
	triggerEvent := receivedEvents[0]
	assert.Equal(t, "trigger", triggerEvent.EventAction)
	assert.Equal(t, "dsops-database-staging-rot-999", triggerEvent.DedupKey)

	// Validate resolve event
	resolveEvent := receivedEvents[1]
	assert.Equal(t, "resolve", resolveEvent.EventAction)
	assert.Equal(t, "dsops-database-staging-rot-999", resolveEvent.DedupKey)

	// Same dedup key means they're linked
	assert.Equal(t, triggerEvent.DedupKey, resolveEvent.DedupKey)
}

// TestPagerDutyEvents_AutoResolveDisabled tests that resolve events are not sent when auto-resolve is disabled.
func TestPagerDutyEvents_AutoResolveDisabled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var receivedEvents []pagerdutyEvent
	var mu sync.Mutex

	// Mock PagerDuty Events API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event pagerdutyEvent
		_ = json.NewDecoder(r.Body).Decode(&event)

		mu.Lock()
		receivedEvents = append(receivedEvents, event)
		mu.Unlock()

		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}))
	defer server.Close()

	// Create PagerDuty provider with auto-resolve DISABLED
	provider := notifications.NewPagerDutyProvider(notifications.PagerDutyConfig{
		IntegrationKey: "test-key",
		AutoResolve:    false, // Disabled
	})
	provider.SetAPIURL(server.URL + "/v2/enqueue")

	ctx := context.Background()

	// Send completed event (should not resolve)
	completedEvent := notifications.RotationEvent{
		Type:        notifications.EventTypeCompleted,
		Service:     "app",
		Environment: "prod",
		Status:      notifications.StatusSuccess,
		Timestamp:   time.Now(),
	}

	err := provider.Send(ctx, completedEvent)
	require.NoError(t, err)

	// Validate NO events received (resolve skipped)
	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, receivedEvents, 0, "No events should be sent when auto-resolve is disabled")
}

// TestPagerDutyEvents_CustomSeverityLevels tests that different severity levels are applied correctly.
func TestPagerDutyEvents_CustomSeverityLevels(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	severities := []string{"critical", "error", "warning", "info"}

	for _, severity := range severities {
		t.Run(severity, func(t *testing.T) {
			var receivedSeverity string
			var mu sync.Mutex

			// Mock PagerDuty Events API
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var event pagerdutyEvent
				_ = json.NewDecoder(r.Body).Decode(&event)

				mu.Lock()
				if sev, ok := event.Payload["severity"].(string); ok {
					receivedSeverity = sev
				}
				mu.Unlock()

				w.WriteHeader(http.StatusAccepted)
				_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
			}))
			defer server.Close()

			// Create provider with specific severity
			provider := notifications.NewPagerDutyProvider(notifications.PagerDutyConfig{
				IntegrationKey: "test-key",
				Severity:       severity,
			})
			provider.SetAPIURL(server.URL + "/v2/enqueue")

			// Send event
			ctx := context.Background()
			event := notifications.RotationEvent{
				Type:      notifications.EventTypeFailed,
				Service:   "test",
				Timestamp: time.Now(),
			}

			err := provider.Send(ctx, event)
			require.NoError(t, err)

			// Validate severity
			mu.Lock()
			defer mu.Unlock()
			assert.Equal(t, severity, receivedSeverity)
		})
	}
}

// TestPagerDutyEvents_DedupKeyConsistency tests that dedup keys are consistent for the same rotation.
func TestPagerDutyEvents_DedupKeyConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var dedupKeys []string
	var mu sync.Mutex

	// Mock PagerDuty Events API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event pagerdutyEvent
		_ = json.NewDecoder(r.Body).Decode(&event)

		mu.Lock()
		dedupKeys = append(dedupKeys, event.DedupKey)
		mu.Unlock()

		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}))
	defer server.Close()

	provider := notifications.NewPagerDutyProvider(notifications.PagerDutyConfig{
		IntegrationKey: "test-key",
	})
	provider.SetAPIURL(server.URL + "/v2/enqueue")

	ctx := context.Background()

	// Send multiple events for the same rotation
	baseEvent := notifications.RotationEvent{
		Service:     "api-gateway",
		Environment: "production",
		Timestamp:   time.Now(),
		Metadata: map[string]string{
			"rotation_id": "rot-abc123",
		},
	}

	// Send different event types
	events := []notifications.RotationEvent{
		{Type: notifications.EventTypeFailed, Service: baseEvent.Service, Environment: baseEvent.Environment, Metadata: baseEvent.Metadata, Timestamp: time.Now()},
		{Type: notifications.EventTypeRollback, Service: baseEvent.Service, Environment: baseEvent.Environment, Metadata: baseEvent.Metadata, Timestamp: time.Now()},
		{Type: notifications.EventTypeStarted, Service: baseEvent.Service, Environment: baseEvent.Environment, Metadata: baseEvent.Metadata, Timestamp: time.Now()},
	}

	for _, event := range events {
		err := provider.Send(ctx, event)
		require.NoError(t, err)
	}

	// Validate all dedup keys are identical
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, dedupKeys, 3)

	expectedKey := "dsops-api-gateway-production-rot-abc123"
	for i, key := range dedupKeys {
		assert.Equal(t, expectedKey, key, "Dedup key %d should match", i)
	}
}

// TestPagerDutyEvents_DifferentRotationIDs tests that different rotation IDs produce different dedup keys.
func TestPagerDutyEvents_DifferentRotationIDs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var dedupKeys []string
	var mu sync.Mutex

	// Mock PagerDuty Events API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event pagerdutyEvent
		_ = json.NewDecoder(r.Body).Decode(&event)

		mu.Lock()
		dedupKeys = append(dedupKeys, event.DedupKey)
		mu.Unlock()

		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}))
	defer server.Close()

	provider := notifications.NewPagerDutyProvider(notifications.PagerDutyConfig{
		IntegrationKey: "test-key",
	})
	provider.SetAPIURL(server.URL + "/v2/enqueue")

	ctx := context.Background()

	// Send events with different rotation IDs
	rotationIDs := []string{"rot-001", "rot-002", "rot-003"}

	for _, rotID := range rotationIDs {
		event := notifications.RotationEvent{
			Type:        notifications.EventTypeFailed,
			Service:     "database",
			Environment: "prod",
			Timestamp:   time.Now(),
			Metadata: map[string]string{
				"rotation_id": rotID,
			},
		}

		err := provider.Send(ctx, event)
		require.NoError(t, err)
	}

	// Validate all dedup keys are different
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, dedupKeys, 3)

	assert.Equal(t, "dsops-database-prod-rot-001", dedupKeys[0])
	assert.Equal(t, "dsops-database-prod-rot-002", dedupKeys[1])
	assert.Equal(t, "dsops-database-prod-rot-003", dedupKeys[2])

	// Ensure they're all unique
	uniqueKeys := make(map[string]bool)
	for _, key := range dedupKeys {
		uniqueKeys[key] = true
	}
	assert.Len(t, uniqueKeys, 3, "All dedup keys should be unique")
}

// TestPagerDutyEvents_ErrorHandling tests error handling for various HTTP status codes.
func TestPagerDutyEvents_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "success 200",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "success 202 accepted",
			statusCode: http.StatusAccepted,
			wantErr:    false,
		},
		{
			name:       "bad request 400",
			statusCode: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "PagerDuty returned status 400",
		},
		{
			name:       "unauthorized 401",
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
			errMsg:     "PagerDuty returned status 401",
		},
		{
			name:       "rate limited 429",
			statusCode: http.StatusTooManyRequests,
			wantErr:    true,
			errMsg:     "PagerDuty returned status 429",
		},
		{
			name:       "server error 500",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
			errMsg:     "PagerDuty returned status 500",
		},
		{
			name:       "service unavailable 503",
			statusCode: http.StatusServiceUnavailable,
			wantErr:    true,
			errMsg:     "PagerDuty returned status 503",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock PagerDuty Events API
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.statusCode >= 200 && tt.statusCode < 300 {
					_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
				} else {
					_ = json.NewEncoder(w).Encode(map[string]string{
						"status":  "error",
						"message": fmt.Sprintf("Error %d", tt.statusCode),
					})
				}
			}))
			defer server.Close()

			provider := notifications.NewPagerDutyProvider(notifications.PagerDutyConfig{
				IntegrationKey: "test-key",
			})
			provider.SetAPIURL(server.URL + "/v2/enqueue")

			// Send event
			ctx := context.Background()
			event := notifications.RotationEvent{
				Type:      notifications.EventTypeFailed,
				Service:   "test",
				Timestamp: time.Now(),
			}

			err := provider.Send(ctx, event)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestPagerDutyEvents_MetadataInCustomDetails tests that event metadata is included in custom_details.
func TestPagerDutyEvents_MetadataInCustomDetails(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var receivedCustomDetails map[string]interface{}
	var mu sync.Mutex

	// Mock PagerDuty Events API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event pagerdutyEvent
		_ = json.NewDecoder(r.Body).Decode(&event)

		mu.Lock()
		if details, ok := event.Payload["custom_details"].(map[string]interface{}); ok {
			receivedCustomDetails = details
		}
		mu.Unlock()

		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}))
	defer server.Close()

	provider := notifications.NewPagerDutyProvider(notifications.PagerDutyConfig{
		IntegrationKey: "test-key",
	})
	provider.SetAPIURL(server.URL + "/v2/enqueue")

	// Send event with rich metadata
	ctx := context.Background()
	event := notifications.RotationEvent{
		Type:        notifications.EventTypeFailed,
		Service:     "payment-gateway",
		Environment: "production",
		Strategy:    "two-key",
		Status:      notifications.StatusFailure,
		Error:       fmt.Errorf("connection pool exhausted"),
		Duration:    120 * time.Second,
		Timestamp:   time.Now(),
		Metadata: map[string]string{
			"rotation_id":      "rot-xyz789",
			"previous_version": "v1.2.3",
			"new_version":      "v1.2.4",
			"triggered_by":     "scheduler",
			"attempt":          "3",
		},
	}

	err := provider.Send(ctx, event)
	require.NoError(t, err)

	// Validate custom details include all metadata
	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, receivedCustomDetails)

	// Core fields
	assert.Equal(t, "payment-gateway", receivedCustomDetails["service"])
	assert.Equal(t, "production", receivedCustomDetails["environment"])
	assert.Equal(t, "two-key", receivedCustomDetails["strategy"])
	assert.Equal(t, "connection pool exhausted", receivedCustomDetails["error"])

	// Metadata fields
	assert.Equal(t, "rot-xyz789", receivedCustomDetails["rotation_id"])
	assert.Equal(t, "v1.2.3", receivedCustomDetails["previous_version"])
	assert.Equal(t, "v1.2.4", receivedCustomDetails["new_version"])
	assert.Equal(t, "scheduler", receivedCustomDetails["triggered_by"])
	assert.Equal(t, "3", receivedCustomDetails["attempt"])
}

// TestPagerDutyEvents_RequestFormat tests that the HTTP request format is correct.
func TestPagerDutyEvents_RequestFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var receivedMethod string
	var receivedPath string
	var receivedContentType string
	var receivedBody []byte
	var mu sync.Mutex

	// Mock PagerDuty Events API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		receivedContentType = r.Header.Get("Content-Type")
		receivedBody, _ = io.ReadAll(r.Body)
		mu.Unlock()

		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}))
	defer server.Close()

	provider := notifications.NewPagerDutyProvider(notifications.PagerDutyConfig{
		IntegrationKey: "test-key-12345",
	})
	provider.SetAPIURL(server.URL + "/v2/enqueue")

	// Send event
	ctx := context.Background()
	event := notifications.RotationEvent{
		Type:        notifications.EventTypeFailed,
		Service:     "api",
		Environment: "prod",
		Timestamp:   time.Now(),
	}

	err := provider.Send(ctx, event)
	require.NoError(t, err)

	// Validate request format
	mu.Lock()
	defer mu.Unlock()

	assert.Equal(t, "POST", receivedMethod)
	assert.Equal(t, "/v2/enqueue", receivedPath)
	assert.Equal(t, "application/json", receivedContentType)

	// Validate JSON structure
	var payload map[string]interface{}
	err = json.Unmarshal(receivedBody, &payload)
	require.NoError(t, err)

	assert.Equal(t, "test-key-12345", payload["routing_key"])
	assert.Equal(t, "trigger", payload["event_action"])
	assert.NotEmpty(t, payload["dedup_key"])
	assert.NotNil(t, payload["payload"])
}
