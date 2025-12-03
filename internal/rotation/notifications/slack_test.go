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

func TestSlackProvider_Name(t *testing.T) {
	t.Parallel()
	provider := NewSlackProvider(SlackConfig{})
	assert.Equal(t, "slack", provider.Name())
}

func TestSlackProvider_SupportsEvent(t *testing.T) {
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
			name:      "explicit completed supported",
			events:    []string{"started", "completed"},
			eventType: EventTypeCompleted,
			want:      true,
		},
		{
			name:      "failed not in list",
			events:    []string{"started", "completed"},
			eventType: EventTypeFailed,
			want:      false,
		},
		{
			name:      "rollback not in list",
			events:    []string{"started", "completed"},
			eventType: EventTypeRollback,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider := NewSlackProvider(SlackConfig{Events: tt.events})
			got := provider.SupportsEvent(tt.eventType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSlackProvider_Send_Success(t *testing.T) {
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
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	provider := NewSlackProvider(SlackConfig{
		WebhookURL: server.URL,
		Channel:    "#rotation-alerts",
	})

	event := RotationEvent{
		Type:        EventTypeCompleted,
		Service:     "postgresql",
		Environment: "production",
		Strategy:    "two-key",
		Status:      StatusSuccess,
		Duration:    5 * time.Second,
		Timestamp:   time.Now(),
	}

	err := provider.Send(context.Background(), event)
	require.NoError(t, err)

	// Verify message was sent with Block Kit format
	assert.NotNil(t, receivedBody["blocks"])
}

func TestSlackProvider_Send_Failure(t *testing.T) {
	t.Parallel()

	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		_ = json.Unmarshal(body, &receivedBody)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	provider := NewSlackProvider(SlackConfig{
		WebhookURL: server.URL,
		Mentions: &SlackMentions{
			OnFailure: []string{"@oncall", "@platform-team"},
		},
	})

	event := RotationEvent{
		Type:        EventTypeFailed,
		Service:     "postgresql",
		Environment: "production",
		Strategy:    "two-key",
		Status:      StatusFailure,
		Error:       fmt.Errorf("connection timeout"),
		Duration:    30 * time.Second,
		Timestamp:   time.Now(),
	}

	err := provider.Send(context.Background(), event)
	require.NoError(t, err)

	// Verify mentions are included for failure
	blocks := receivedBody["blocks"].([]interface{})
	found := false
	for _, block := range blocks {
		b := block.(map[string]interface{})
		if b["type"] == "section" {
			if text, ok := b["text"].(map[string]interface{}); ok {
				if textContent, ok := text["text"].(string); ok {
					if contains(textContent, "@oncall") || contains(textContent, "@platform-team") {
						found = true
						break
					}
				}
			}
		}
	}
	assert.True(t, found, "Mentions should be included in failure messages")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsLoop(s, substr))
}

func containsLoop(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSlackProvider_Send_ServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	provider := NewSlackProvider(SlackConfig{
		WebhookURL: server.URL,
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

func TestSlackProvider_Send_NetworkError(t *testing.T) {
	t.Parallel()

	provider := NewSlackProvider(SlackConfig{
		WebhookURL: "http://localhost:99999/invalid",
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

func TestSlackProvider_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  SlackConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: SlackConfig{
				WebhookURL: "https://hooks.slack.com/services/xxx/yyy/zzz",
			},
			wantErr: false,
		},
		{
			name:    "missing webhook URL",
			config:  SlackConfig{},
			wantErr: true,
		},
		{
			name: "invalid URL",
			config: SlackConfig{
				WebhookURL: "not-a-url",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider := NewSlackProvider(tt.config)
			err := provider.Validate(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSlackProvider_BlockKitFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		eventType EventType
		status    RotationStatus
		wantEmoji string
	}{
		{
			name:      "started event",
			eventType: EventTypeStarted,
			status:    StatusSuccess,
			wantEmoji: ":arrows_counterclockwise:",
		},
		{
			name:      "completed success",
			eventType: EventTypeCompleted,
			status:    StatusSuccess,
			wantEmoji: ":white_check_mark:",
		},
		{
			name:      "failed event",
			eventType: EventTypeFailed,
			status:    StatusFailure,
			wantEmoji: ":x:",
		},
		{
			name:      "rollback event",
			eventType: EventTypeRollback,
			status:    StatusRolledBack,
			wantEmoji: ":rewind:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var receivedBody map[string]interface{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &receivedBody)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			provider := NewSlackProvider(SlackConfig{WebhookURL: server.URL})

			event := RotationEvent{
				Type:        tt.eventType,
				Service:     "test-service",
				Environment: "test-env",
				Status:      tt.status,
				Timestamp:   time.Now(),
			}

			err := provider.Send(context.Background(), event)
			require.NoError(t, err)

			// Check for emoji in header
			blocks := receivedBody["blocks"].([]interface{})
			headerFound := false
			for _, block := range blocks {
				b := block.(map[string]interface{})
				if b["type"] == "header" {
					text := b["text"].(map[string]interface{})
					textContent := text["text"].(string)
					if contains(textContent, tt.wantEmoji) {
						headerFound = true
						break
					}
				}
			}
			assert.True(t, headerFound, "Expected emoji %s in header", tt.wantEmoji)
		})
	}
}
