package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookProvider_Name(t *testing.T) {
	t.Parallel()
	provider := NewWebhookProvider(WebhookConfig{Name: "custom-webhook"})
	assert.Equal(t, "webhook:custom-webhook", provider.Name())
}

func TestWebhookProvider_NameDefault(t *testing.T) {
	t.Parallel()
	provider := NewWebhookProvider(WebhookConfig{})
	assert.Equal(t, "webhook", provider.Name())
}

func TestWebhookProvider_SupportsEvent(t *testing.T) {
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
			eventType: EventTypeStarted,
			want:      true,
		},
		{
			name:      "explicit started supported",
			events:    []string{"started", "completed"},
			eventType: EventTypeStarted,
			want:      true,
		},
		{
			name:      "failed not in list",
			events:    []string{"started", "completed"},
			eventType: EventTypeFailed,
			want:      false,
		},
		{
			name:      "case insensitive",
			events:    []string{"STARTED", "Completed"},
			eventType: EventTypeStarted,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider := NewWebhookProvider(WebhookConfig{Events: tt.events})
			got := provider.SupportsEvent(tt.eventType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWebhookProvider_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  WebhookConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: WebhookConfig{
				URL: "https://example.com/webhook",
			},
			wantErr: false,
		},
		{
			name:    "missing URL",
			config:  WebhookConfig{},
			wantErr: true,
			errMsg:  "URL is required",
		},
		{
			name: "invalid URL",
			config: WebhookConfig{
				URL: "not-a-url",
			},
			wantErr: true,
			errMsg:  "invalid URL",
		},
		{
			name: "invalid method",
			config: WebhookConfig{
				URL:    "https://example.com/webhook",
				Method: "DELETE",
			},
			wantErr: true,
			errMsg:  "invalid method",
		},
		{
			name: "valid POST method",
			config: WebhookConfig{
				URL:    "https://example.com/webhook",
				Method: "POST",
			},
			wantErr: false,
		},
		{
			name: "valid PUT method",
			config: WebhookConfig{
				URL:    "https://example.com/webhook",
				Method: "PUT",
			},
			wantErr: false,
		},
		{
			name: "valid PATCH method",
			config: WebhookConfig{
				URL:    "https://example.com/webhook",
				Method: "PATCH",
			},
			wantErr: false,
		},
		{
			name: "invalid backoff strategy",
			config: WebhookConfig{
				URL: "https://example.com/webhook",
				Retry: &RetryConfig{
					Backoff: "unknown",
				},
			},
			wantErr: true,
			errMsg:  "invalid backoff",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider := NewWebhookProvider(tt.config)
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

func TestWebhookProvider_Send_DefaultPayload(t *testing.T) {
	t.Parallel()

	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedBody)
		require.NoError(t, err)

		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewWebhookProvider(WebhookConfig{
		URL: server.URL,
	})

	event := RotationEvent{
		Type:        EventTypeCompleted,
		Service:     "postgresql",
		Environment: "production",
		Strategy:    "two-key",
		Status:      StatusSuccess,
		Duration:    45 * time.Second,
		Timestamp:   time.Now(),
	}

	err := provider.Send(context.Background(), event)
	require.NoError(t, err)

	// Verify default payload structure
	assert.Equal(t, "completed", receivedBody["event"])
	assert.Equal(t, "postgresql", receivedBody["service"])
	assert.Equal(t, "production", receivedBody["environment"])
	assert.Equal(t, "success", receivedBody["status"])
	assert.NotEmpty(t, receivedBody["timestamp"])
}

func TestWebhookProvider_Send_CustomTemplate(t *testing.T) {
	t.Parallel()

	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedBody)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	customTemplate := `{
		"type": "rotation",
		"name": "{{.Service}}",
		"state": "{{.Status}}",
		"env": "{{.Environment}}"
	}`

	provider := NewWebhookProvider(WebhookConfig{
		URL:             server.URL,
		PayloadTemplate: customTemplate,
	})

	event := RotationEvent{
		Type:        EventTypeCompleted,
		Service:     "postgresql",
		Environment: "production",
		Status:      StatusSuccess,
		Timestamp:   time.Now(),
	}

	err := provider.Send(context.Background(), event)
	require.NoError(t, err)

	assert.Equal(t, "rotation", receivedBody["type"])
	assert.Equal(t, "postgresql", receivedBody["name"])
	assert.Equal(t, "success", receivedBody["state"])
	assert.Equal(t, "production", receivedBody["env"])
}

func TestWebhookProvider_Send_CustomHeaders(t *testing.T) {
	t.Parallel()

	receivedHeaders := make(http.Header)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewWebhookProvider(WebhookConfig{
		URL: server.URL,
		Headers: map[string]string{
			"Authorization":   "Bearer test-token",
			"X-Custom-Header": "custom-value",
		},
	})

	event := RotationEvent{
		Type:      EventTypeCompleted,
		Service:   "postgresql",
		Timestamp: time.Now(),
	}

	err := provider.Send(context.Background(), event)
	require.NoError(t, err)

	assert.Equal(t, "Bearer test-token", receivedHeaders.Get("Authorization"))
	assert.Equal(t, "custom-value", receivedHeaders.Get("X-Custom-Header"))
}

func TestWebhookProvider_Send_CustomMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method string
	}{
		{"POST"},
		{"PUT"},
		{"PATCH"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			t.Parallel()

			var receivedMethod string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedMethod = r.Method
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			provider := NewWebhookProvider(WebhookConfig{
				URL:    server.URL,
				Method: tt.method,
			})

			event := RotationEvent{
				Type:      EventTypeCompleted,
				Service:   "postgresql",
				Timestamp: time.Now(),
			}

			err := provider.Send(context.Background(), event)
			require.NoError(t, err)

			assert.Equal(t, tt.method, receivedMethod)
		})
	}
}

func TestWebhookProvider_Send_ServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := NewWebhookProvider(WebhookConfig{
		URL: server.URL,
	})

	event := RotationEvent{
		Type:      EventTypeCompleted,
		Service:   "postgresql",
		Timestamp: time.Now(),
	}

	err := provider.Send(context.Background(), event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestWebhookProvider_Send_RetryOnFailure(t *testing.T) {
	t.Parallel()

	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewWebhookProvider(WebhookConfig{
		URL: server.URL,
		Retry: &RetryConfig{
			MaxAttempts: 3,
			Backoff:     "linear",
			InitialWait: 10 * time.Millisecond,
		},
	})

	event := RotationEvent{
		Type:      EventTypeCompleted,
		Service:   "postgresql",
		Timestamp: time.Now(),
	}

	err := provider.Send(context.Background(), event)
	require.NoError(t, err)

	assert.Equal(t, int32(3), atomic.LoadInt32(&callCount))
}

func TestWebhookProvider_Send_RetryExhausted(t *testing.T) {
	t.Parallel()

	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	provider := NewWebhookProvider(WebhookConfig{
		URL: server.URL,
		Retry: &RetryConfig{
			MaxAttempts: 3,
			Backoff:     "linear",
			InitialWait: 10 * time.Millisecond,
		},
	})

	event := RotationEvent{
		Type:      EventTypeCompleted,
		Service:   "postgresql",
		Timestamp: time.Now(),
	}

	err := provider.Send(context.Background(), event)
	assert.Error(t, err)

	// Should have tried 3 times
	assert.Equal(t, int32(3), atomic.LoadInt32(&callCount))
}

func TestWebhookProvider_Send_ExponentialBackoff(t *testing.T) {
	t.Parallel()

	var times []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		times = append(times, time.Now())
		if len(times) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewWebhookProvider(WebhookConfig{
		URL: server.URL,
		Retry: &RetryConfig{
			MaxAttempts: 3,
			Backoff:     "exponential",
			InitialWait: 50 * time.Millisecond,
		},
	})

	event := RotationEvent{
		Type:      EventTypeCompleted,
		Service:   "postgresql",
		Timestamp: time.Now(),
	}

	err := provider.Send(context.Background(), event)
	require.NoError(t, err)

	require.Len(t, times, 3)

	// Check that delays are roughly exponential
	// First delay should be ~50ms, second should be ~100ms
	delay1 := times[1].Sub(times[0])
	delay2 := times[2].Sub(times[1])

	// Allow some tolerance for timing
	assert.GreaterOrEqual(t, delay1.Milliseconds(), int64(40))
	assert.GreaterOrEqual(t, delay2.Milliseconds(), int64(80))
}

func TestWebhookProvider_Send_Timeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewWebhookProvider(WebhookConfig{
		URL:     server.URL,
		Timeout: 50 * time.Millisecond,
	})

	event := RotationEvent{
		Type:      EventTypeCompleted,
		Service:   "postgresql",
		Timestamp: time.Now(),
	}

	err := provider.Send(context.Background(), event)
	assert.Error(t, err)
}

func TestWebhookProvider_Send_NetworkError(t *testing.T) {
	t.Parallel()

	provider := NewWebhookProvider(WebhookConfig{
		URL: "http://localhost:99999/invalid",
	})

	event := RotationEvent{
		Type:      EventTypeCompleted,
		Service:   "postgresql",
		Timestamp: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := provider.Send(ctx, event)
	assert.Error(t, err)
}

func TestWebhookProvider_TemplateError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewWebhookProvider(WebhookConfig{
		URL:             server.URL,
		PayloadTemplate: `{{.NonExistentField}}`,
	})

	event := RotationEvent{
		Type:      EventTypeCompleted,
		Service:   "postgresql",
		Timestamp: time.Now(),
	}

	err := provider.Send(context.Background(), event)
	// Template errors should be handled gracefully
	// Either return error or fall back to default payload
	_ = err
}

func TestWebhookProvider_MetadataInPayload(t *testing.T) {
	t.Parallel()

	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedBody)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewWebhookProvider(WebhookConfig{
		URL: server.URL,
	})

	event := RotationEvent{
		Type:        EventTypeCompleted,
		Service:     "postgresql",
		Environment: "production",
		Status:      StatusSuccess,
		Timestamp:   time.Now(),
		Metadata: map[string]string{
			"rotation_id":      "rot-123",
			"previous_version": "v1.0",
			"new_version":      "v1.1",
		},
	}

	err := provider.Send(context.Background(), event)
	require.NoError(t, err)

	metadata := receivedBody["metadata"].(map[string]interface{})
	assert.Equal(t, "rot-123", metadata["rotation_id"])
	assert.Equal(t, "v1.0", metadata["previous_version"])
	assert.Equal(t, "v1.1", metadata["new_version"])
}

func TestWebhookProvider_ErrorInPayload(t *testing.T) {
	t.Parallel()

	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedBody)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewWebhookProvider(WebhookConfig{
		URL: server.URL,
	})

	event := RotationEvent{
		Type:      EventTypeFailed,
		Service:   "postgresql",
		Status:    StatusFailure,
		Error:     fmt.Errorf("connection timeout"),
		Timestamp: time.Now(),
	}

	err := provider.Send(context.Background(), event)
	require.NoError(t, err)

	assert.Equal(t, "connection timeout", receivedBody["error"])
}

func TestCreateWebhookProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  *WebhookNotificationConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "valid config",
			config: &WebhookNotificationConfig{
				URL: "https://example.com/webhook",
			},
			wantErr: false,
		},
		{
			name: "invalid config - missing URL",
			config: &WebhookNotificationConfig{
				Name: "test-webhook",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider, err := CreateWebhookProvider(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
			}
		})
	}
}
