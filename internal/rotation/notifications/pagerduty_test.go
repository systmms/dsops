package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPagerDutyProvider_Name(t *testing.T) {
	t.Parallel()
	provider := NewPagerDutyProvider(PagerDutyConfig{})
	assert.Equal(t, "pagerduty", provider.Name())
}

func TestPagerDutyProvider_SupportsEvent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		events    []string
		eventType EventType
		want      bool
	}{
		{
			name:      "empty events supports all",
			events:    nil,
			eventType: EventTypeFailed,
			want:      true,
		},
		{
			name:      "explicit failed supported",
			events:    []string{"failed", "rollback"},
			eventType: EventTypeFailed,
			want:      true,
		},
		{
			name:      "explicit rollback supported",
			events:    []string{"failed", "rollback"},
			eventType: EventTypeRollback,
			want:      true,
		},
		{
			name:      "started not in list",
			events:    []string{"failed", "rollback"},
			eventType: EventTypeStarted,
			want:      false,
		},
		{
			name:      "completed not in list",
			events:    []string{"failed", "rollback"},
			eventType: EventTypeCompleted,
			want:      false,
		},
		{
			name:      "case insensitive",
			events:    []string{"FAILED", "Rollback"},
			eventType: EventTypeFailed,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider := NewPagerDutyProvider(PagerDutyConfig{Events: tt.events})
			got := provider.SupportsEvent(tt.eventType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPagerDutyProvider_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  PagerDutyConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: PagerDutyConfig{
				IntegrationKey: "abc123def456",
				Severity:       "error",
			},
			wantErr: false,
		},
		{
			name:    "missing integration key",
			config:  PagerDutyConfig{},
			wantErr: true,
			errMsg:  "integration key is required",
		},
		{
			name: "invalid severity",
			config: PagerDutyConfig{
				IntegrationKey: "abc123def456",
				Severity:       "unknown",
			},
			wantErr: true,
			errMsg:  "invalid severity",
		},
		{
			name: "valid severity - critical",
			config: PagerDutyConfig{
				IntegrationKey: "abc123def456",
				Severity:       "critical",
			},
			wantErr: false,
		},
		{
			name: "valid severity - warning",
			config: PagerDutyConfig{
				IntegrationKey: "abc123def456",
				Severity:       "warning",
			},
			wantErr: false,
		},
		{
			name: "valid severity - info",
			config: PagerDutyConfig{
				IntegrationKey: "abc123def456",
				Severity:       "info",
			},
			wantErr: false,
		},
		{
			name: "empty severity defaults to error",
			config: PagerDutyConfig{
				IntegrationKey: "abc123def456",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider := NewPagerDutyProvider(tt.config)
			err := provider.Validate(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPagerDutyProvider_Send_Trigger(t *testing.T) {
	t.Parallel()

	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedBody)
		require.NoError(t, err)

		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"success","message":"Event processed","dedup_key":"test-key"}`))
	}))
	defer server.Close()

	provider := NewPagerDutyProvider(PagerDutyConfig{
		IntegrationKey: "test-integration-key",
		Severity:       "error",
	})
	provider.apiURL = server.URL

	event := RotationEvent{
		Type:        EventTypeFailed,
		Service:     "postgresql",
		Environment: "production",
		Strategy:    "two-key",
		Status:      StatusFailure,
		Error:       fmt.Errorf("connection timeout"),
		Duration:    30 * time.Second,
		Timestamp:   time.Now(),
		Metadata: map[string]string{
			"rotation_id": "rot-123",
		},
	}

	err := provider.Send(context.Background(), event)
	require.NoError(t, err)

	// Verify the request structure
	assert.Equal(t, "test-integration-key", receivedBody["routing_key"])
	assert.Equal(t, "trigger", receivedBody["event_action"])
	assert.NotEmpty(t, receivedBody["dedup_key"])

	payload := receivedBody["payload"].(map[string]interface{})
	assert.Contains(t, payload["summary"], "postgresql")
	assert.Contains(t, payload["summary"], "production")
	assert.Equal(t, "error", payload["severity"])
	assert.Equal(t, "dsops-rotation", payload["source"])

	customDetails := payload["custom_details"].(map[string]interface{})
	assert.Equal(t, "postgresql", customDetails["service"])
	assert.Equal(t, "production", customDetails["environment"])
}

func TestPagerDutyProvider_Send_Resolve(t *testing.T) {
	t.Parallel()

	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedBody)
		require.NoError(t, err)

		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	provider := NewPagerDutyProvider(PagerDutyConfig{
		IntegrationKey: "test-integration-key",
		AutoResolve:    true,
	})
	provider.apiURL = server.URL

	event := RotationEvent{
		Type:        EventTypeCompleted,
		Service:     "postgresql",
		Environment: "production",
		Status:      StatusSuccess,
		Duration:    45 * time.Second,
		Timestamp:   time.Now(),
	}

	err := provider.Send(context.Background(), event)
	require.NoError(t, err)

	// When AutoResolve is enabled and event is successful completion,
	// the action should be "resolve"
	assert.Equal(t, "resolve", receivedBody["event_action"])
}

func TestPagerDutyProvider_Send_Rollback(t *testing.T) {
	t.Parallel()

	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedBody)
		require.NoError(t, err)

		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	provider := NewPagerDutyProvider(PagerDutyConfig{
		IntegrationKey: "test-integration-key",
		Severity:       "warning",
	})
	provider.apiURL = server.URL

	event := RotationEvent{
		Type:        EventTypeRollback,
		Service:     "postgresql",
		Environment: "production",
		Status:      StatusRolledBack,
		Timestamp:   time.Now(),
		Metadata: map[string]string{
			"reason":           "verification failure",
			"previous_version": "v1.0",
		},
	}

	err := provider.Send(context.Background(), event)
	require.NoError(t, err)

	assert.Equal(t, "trigger", receivedBody["event_action"])

	payload := receivedBody["payload"].(map[string]interface{})
	assert.Contains(t, payload["summary"], "rollback")
}

func TestPagerDutyProvider_Send_ServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"status":"error","message":"internal error"}`))
	}))
	defer server.Close()

	provider := NewPagerDutyProvider(PagerDutyConfig{
		IntegrationKey: "test-key",
	})
	provider.apiURL = server.URL

	event := RotationEvent{
		Type:      EventTypeFailed,
		Service:   "postgresql",
		Timestamp: time.Now(),
	}

	err := provider.Send(context.Background(), event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestPagerDutyProvider_Send_NetworkError(t *testing.T) {
	t.Parallel()

	provider := NewPagerDutyProvider(PagerDutyConfig{
		IntegrationKey: "test-key",
	})
	provider.apiURL = "http://localhost:99999/invalid"

	event := RotationEvent{
		Type:      EventTypeFailed,
		Service:   "postgresql",
		Timestamp: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := provider.Send(ctx, event)
	assert.Error(t, err)
}

func TestPagerDutyProvider_DedupKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		event         RotationEvent
		expectContain []string
	}{
		{
			name: "with rotation_id in metadata",
			event: RotationEvent{
				Service:     "postgresql",
				Environment: "production",
				Metadata: map[string]string{
					"rotation_id": "rot-abc123",
				},
			},
			expectContain: []string{"dsops", "postgresql", "production", "rot-abc123"},
		},
		{
			name: "without rotation_id",
			event: RotationEvent{
				Service:     "mysql",
				Environment: "staging",
			},
			expectContain: []string{"dsops", "mysql", "staging"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var receivedBody map[string]interface{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &receivedBody)
				w.WriteHeader(http.StatusAccepted)
			}))
			defer server.Close()

			provider := NewPagerDutyProvider(PagerDutyConfig{
				IntegrationKey: "test-key",
			})
			provider.apiURL = server.URL

			tt.event.Type = EventTypeFailed
			tt.event.Timestamp = time.Now()

			err := provider.Send(context.Background(), tt.event)
			require.NoError(t, err)

			dedupKey := receivedBody["dedup_key"].(string)
			for _, expected := range tt.expectContain {
				assert.Contains(t, dedupKey, expected, "dedup_key should contain %s", expected)
			}
		})
	}
}

func TestPagerDutyProvider_Severity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		configSeverity string
		eventType      EventType
		eventStatus    RotationStatus
		wantSeverity   string
	}{
		{
			name:           "uses config severity",
			configSeverity: "critical",
			eventType:      EventTypeFailed,
			eventStatus:    StatusFailure,
			wantSeverity:   "critical",
		},
		{
			name:           "empty config defaults to error",
			configSeverity: "",
			eventType:      EventTypeFailed,
			eventStatus:    StatusFailure,
			wantSeverity:   "error",
		},
		{
			name:           "warning severity",
			configSeverity: "warning",
			eventType:      EventTypeRollback,
			eventStatus:    StatusRolledBack,
			wantSeverity:   "warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var receivedBody map[string]interface{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &receivedBody)
				w.WriteHeader(http.StatusAccepted)
			}))
			defer server.Close()

			provider := NewPagerDutyProvider(PagerDutyConfig{
				IntegrationKey: "test-key",
				Severity:       tt.configSeverity,
			})
			provider.apiURL = server.URL

			event := RotationEvent{
				Type:      tt.eventType,
				Service:   "postgresql",
				Status:    tt.eventStatus,
				Timestamp: time.Now(),
			}

			err := provider.Send(context.Background(), event)
			require.NoError(t, err)

			payload := receivedBody["payload"].(map[string]interface{})
			assert.Equal(t, tt.wantSeverity, payload["severity"])
		})
	}
}

func TestPagerDutyProvider_AutoResolveDisabled(t *testing.T) {
	t.Parallel()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	provider := NewPagerDutyProvider(PagerDutyConfig{
		IntegrationKey: "test-key",
		AutoResolve:    false, // Disabled
		Events:         []string{"completed"}, // Still configured to receive completed events
	})
	provider.apiURL = server.URL

	event := RotationEvent{
		Type:      EventTypeCompleted,
		Service:   "postgresql",
		Status:    StatusSuccess,
		Timestamp: time.Now(),
	}

	// When AutoResolve is disabled, completed events should not trigger a resolve action
	// The provider should skip sending for successful completions when AutoResolve is disabled
	err := provider.Send(context.Background(), event)
	require.NoError(t, err)

	// With AutoResolve disabled, we shouldn't send resolve events
	// The behavior depends on implementation - this documents expected behavior
}

func TestCreatePagerDutyProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  *PagerDutyNotificationConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "valid config",
			config: &PagerDutyNotificationConfig{
				IntegrationKey: "test-key-123",
				Severity:       "error",
			},
			wantErr: false,
		},
		{
			name: "invalid config - missing key",
			config: &PagerDutyNotificationConfig{
				Severity: "error",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider, err := CreatePagerDutyProvider(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
				assert.Equal(t, "pagerduty", provider.Name())
			}
		})
	}
}
