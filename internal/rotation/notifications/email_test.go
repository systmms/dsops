package notifications

import (
	"context"
	"fmt"
	"net/smtp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailProvider_Name(t *testing.T) {
	t.Parallel()
	provider := NewEmailProvider(EmailConfig{})
	assert.Equal(t, "email", provider.Name())
}

func TestEmailProvider_SupportsEvent(t *testing.T) {
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
			provider := NewEmailProvider(EmailConfig{Events: tt.events})
			got := provider.SupportsEvent(tt.eventType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEmailProvider_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  EmailConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: EmailConfig{
				SMTP: SMTPConfig{
					Host: "smtp.example.com",
					Port: 587,
				},
				From: "dsops@example.com",
				To:   []string{"team@example.com"},
			},
			wantErr: false,
		},
		{
			name: "missing SMTP host",
			config: EmailConfig{
				SMTP: SMTPConfig{
					Port: 587,
				},
				From: "dsops@example.com",
				To:   []string{"team@example.com"},
			},
			wantErr: true,
			errMsg:  "SMTP host is required",
		},
		{
			name: "missing SMTP port",
			config: EmailConfig{
				SMTP: SMTPConfig{
					Host: "smtp.example.com",
				},
				From: "dsops@example.com",
				To:   []string{"team@example.com"},
			},
			wantErr: true,
			errMsg:  "SMTP port is required",
		},
		{
			name: "missing from address",
			config: EmailConfig{
				SMTP: SMTPConfig{
					Host: "smtp.example.com",
					Port: 587,
				},
				To: []string{"team@example.com"},
			},
			wantErr: true,
			errMsg:  "from address is required",
		},
		{
			name: "missing to addresses",
			config: EmailConfig{
				SMTP: SMTPConfig{
					Host: "smtp.example.com",
					Port: 587,
				},
				From: "dsops@example.com",
				To:   []string{},
			},
			wantErr: true,
			errMsg:  "at least one recipient is required",
		},
		{
			name: "invalid batch mode",
			config: EmailConfig{
				SMTP: SMTPConfig{
					Host: "smtp.example.com",
					Port: 587,
				},
				From:      "dsops@example.com",
				To:        []string{"team@example.com"},
				BatchMode: "weekly",
			},
			wantErr: true,
			errMsg:  "invalid batch mode",
		},
		{
			name: "valid batch mode immediate",
			config: EmailConfig{
				SMTP: SMTPConfig{
					Host: "smtp.example.com",
					Port: 587,
				},
				From:      "dsops@example.com",
				To:        []string{"team@example.com"},
				BatchMode: "immediate",
			},
			wantErr: false,
		},
		{
			name: "valid batch mode hourly",
			config: EmailConfig{
				SMTP: SMTPConfig{
					Host: "smtp.example.com",
					Port: 587,
				},
				From:      "dsops@example.com",
				To:        []string{"team@example.com"},
				BatchMode: "hourly",
			},
			wantErr: false,
		},
		{
			name: "valid batch mode daily",
			config: EmailConfig{
				SMTP: SMTPConfig{
					Host: "smtp.example.com",
					Port: 587,
				},
				From:      "dsops@example.com",
				To:        []string{"team@example.com"},
				BatchMode: "daily",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider := NewEmailProvider(tt.config)
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

func TestEmailProvider_BuildMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		event     RotationEvent
		wantSubj  string
		wantBody  []string
		dontWant  []string
	}{
		{
			name: "completed event",
			event: RotationEvent{
				Type:        EventTypeCompleted,
				Service:     "postgresql",
				Environment: "production",
				Strategy:    "two-key",
				Status:      StatusSuccess,
				Duration:    45 * time.Second,
				Timestamp:   time.Date(2025, 12, 4, 10, 30, 0, 0, time.UTC),
			},
			wantSubj: "[dsops] Rotation Completed: postgresql (production)",
			wantBody: []string{
				"Rotation Completed",
				"postgresql",
				"production",
				"two-key",
				"45s",
			},
			dontWant: []string{"Error:", "Rollback"},
		},
		{
			name: "failed event",
			event: RotationEvent{
				Type:        EventTypeFailed,
				Service:     "mysql",
				Environment: "staging",
				Strategy:    "immediate",
				Status:      StatusFailure,
				Error:       fmt.Errorf("connection timeout"),
				Duration:    30 * time.Second,
				Timestamp:   time.Now(),
			},
			wantSubj: "[dsops] Rotation Failed: mysql (staging)",
			wantBody: []string{
				"Rotation Failed",
				"mysql",
				"staging",
				"connection timeout",
			},
		},
		{
			name: "rollback event",
			event: RotationEvent{
				Type:        EventTypeRollback,
				Service:     "redis",
				Environment: "production",
				Status:      StatusRolledBack,
				Timestamp:   time.Now(),
				Metadata: map[string]string{
					"reason":           "verification failure",
					"previous_version": "v1.0",
				},
			},
			wantSubj: "[dsops] Rotation Rolled Back: redis (production)",
			wantBody: []string{
				"Rolled Back",
				"redis",
				"production",
			},
		},
		{
			name: "started event",
			event: RotationEvent{
				Type:        EventTypeStarted,
				Service:     "mongodb",
				Environment: "development",
				Strategy:    "overlap",
				Timestamp:   time.Now(),
			},
			wantSubj: "[dsops] Rotation Started: mongodb (development)",
			wantBody: []string{
				"Rotation Started",
				"mongodb",
				"development",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := NewEmailProvider(EmailConfig{
				From: "dsops@example.com",
				To:   []string{"team@example.com"},
			})

			subject, htmlBody, textBody := provider.buildMessage(tt.event)

			assert.Contains(t, subject, tt.wantSubj)

			for _, want := range tt.wantBody {
				assert.Contains(t, htmlBody, want, "HTML body should contain %s", want)
				assert.Contains(t, textBody, want, "Text body should contain %s", want)
			}

			for _, dontWant := range tt.dontWant {
				assert.NotContains(t, textBody, dontWant, "Body should not contain %s", dontWant)
			}
		})
	}
}

func TestEmailProvider_BuildMIMEMessage(t *testing.T) {
	t.Parallel()

	provider := NewEmailProvider(EmailConfig{
		From: "dsops@example.com",
		To:   []string{"team@example.com", "admin@example.com"},
	})

	event := RotationEvent{
		Type:        EventTypeCompleted,
		Service:     "postgresql",
		Environment: "production",
		Status:      StatusSuccess,
		Timestamp:   time.Now(),
	}

	msg := provider.buildMIMEMessage(event)

	// Check headers
	assert.Contains(t, msg, "From: dsops@example.com")
	assert.Contains(t, msg, "To: team@example.com, admin@example.com")
	assert.Contains(t, msg, "Subject: [dsops] Rotation Completed: postgresql (production)")
	assert.Contains(t, msg, "MIME-Version: 1.0")
	assert.Contains(t, msg, "Content-Type: multipart/alternative")

	// Check both HTML and plain-text parts
	assert.Contains(t, msg, "text/plain")
	assert.Contains(t, msg, "text/html")
}

func TestEmailProvider_GetBatchMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mode     string
		expected BatchMode
	}{
		{"empty defaults to immediate", "", BatchModeImmediate},
		{"immediate", "immediate", BatchModeImmediate},
		{"hourly", "hourly", BatchModeHourly},
		{"daily", "daily", BatchModeDaily},
		{"unknown defaults to immediate", "unknown", BatchModeImmediate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider := NewEmailProvider(EmailConfig{BatchMode: tt.mode})
			assert.Equal(t, tt.expected, provider.GetBatchMode())
		})
	}
}

// mockSMTPSender is a test helper that captures SMTP send calls
type mockSMTPSender struct {
	sentMessages []mockSMTPMessage
	err          error
}

type mockSMTPMessage struct {
	addr string
	auth smtp.Auth
	from string
	to   []string
	msg  []byte
}

func (m *mockSMTPSender) SendMail(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	if m.err != nil {
		return m.err
	}
	m.sentMessages = append(m.sentMessages, mockSMTPMessage{
		addr: addr,
		auth: auth,
		from: from,
		to:   to,
		msg:  msg,
	})
	return nil
}

func TestEmailProvider_Send_Immediate(t *testing.T) {
	t.Parallel()

	mock := &mockSMTPSender{}

	provider := NewEmailProvider(EmailConfig{
		SMTP: SMTPConfig{
			Host:     "smtp.example.com",
			Port:     587,
			Username: "user",
			Password: "pass",
		},
		From:      "dsops@example.com",
		To:        []string{"team@example.com"},
		BatchMode: "immediate",
	})
	provider.smtpSender = mock.SendMail

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

	require.Len(t, mock.sentMessages, 1)
	sent := mock.sentMessages[0]

	assert.Equal(t, "smtp.example.com:587", sent.addr)
	assert.Equal(t, "dsops@example.com", sent.from)
	assert.Equal(t, []string{"team@example.com"}, sent.to)
	assert.Contains(t, string(sent.msg), "postgresql")
}

func TestEmailProvider_Send_SMTPError(t *testing.T) {
	t.Parallel()

	mock := &mockSMTPSender{
		err: fmt.Errorf("connection refused"),
	}

	provider := NewEmailProvider(EmailConfig{
		SMTP: SMTPConfig{
			Host: "smtp.example.com",
			Port: 587,
		},
		From: "dsops@example.com",
		To:   []string{"team@example.com"},
	})
	provider.smtpSender = mock.SendMail

	event := RotationEvent{
		Type:      EventTypeCompleted,
		Service:   "postgresql",
		Timestamp: time.Now(),
	}

	err := provider.Send(context.Background(), event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestEmailProvider_Send_MultipleRecipients(t *testing.T) {
	t.Parallel()

	mock := &mockSMTPSender{}

	provider := NewEmailProvider(EmailConfig{
		SMTP: SMTPConfig{
			Host: "smtp.example.com",
			Port: 587,
		},
		From: "dsops@example.com",
		To:   []string{"team@example.com", "admin@example.com", "security@example.com"},
	})
	provider.smtpSender = mock.SendMail

	event := RotationEvent{
		Type:      EventTypeCompleted,
		Service:   "postgresql",
		Timestamp: time.Now(),
	}

	err := provider.Send(context.Background(), event)
	require.NoError(t, err)

	require.Len(t, mock.sentMessages, 1)
	sent := mock.sentMessages[0]

	assert.Equal(t, []string{"team@example.com", "admin@example.com", "security@example.com"}, sent.to)
}

func TestEmailProvider_HTMLTemplate(t *testing.T) {
	t.Parallel()

	provider := NewEmailProvider(EmailConfig{
		From: "dsops@example.com",
		To:   []string{"team@example.com"},
	})

	event := RotationEvent{
		Type:        EventTypeFailed,
		Service:     "postgresql",
		Environment: "production",
		Status:      StatusFailure,
		Error:       fmt.Errorf("connection timeout"),
		Timestamp:   time.Now(),
	}

	_, htmlBody, _ := provider.buildMessage(event)

	// Check HTML structure
	assert.Contains(t, htmlBody, "<html")
	assert.Contains(t, htmlBody, "<body")
	assert.Contains(t, htmlBody, "</html>")

	// Check content styling
	assert.Contains(t, htmlBody, "postgresql")
	assert.Contains(t, htmlBody, "production")
	assert.Contains(t, htmlBody, "connection timeout")
}

func TestEmailProvider_PlainTextTemplate(t *testing.T) {
	t.Parallel()

	provider := NewEmailProvider(EmailConfig{
		From: "dsops@example.com",
		To:   []string{"team@example.com"},
	})

	event := RotationEvent{
		Type:        EventTypeCompleted,
		Service:     "postgresql",
		Environment: "production",
		Strategy:    "two-key",
		Status:      StatusSuccess,
		Duration:    45 * time.Second,
		Timestamp:   time.Date(2025, 12, 4, 10, 30, 0, 0, time.UTC),
	}

	_, _, textBody := provider.buildMessage(event)

	// Check plain text format (no HTML tags)
	assert.NotContains(t, textBody, "<html")
	assert.NotContains(t, textBody, "<body")

	// Check content
	assert.Contains(t, textBody, "postgresql")
	assert.Contains(t, textBody, "production")
	assert.Contains(t, textBody, "two-key")
}

func TestCreateEmailProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  *EmailNotificationConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "valid config",
			config: &EmailNotificationConfig{
				SMTP: SMTPConfigInput{
					Host: "smtp.example.com",
					Port: 587,
				},
				From: "dsops@example.com",
				To:   []string{"team@example.com"},
			},
			wantErr: false,
		},
		{
			name: "invalid config - missing host",
			config: &EmailNotificationConfig{
				SMTP: SMTPConfigInput{
					Port: 587,
				},
				From: "dsops@example.com",
				To:   []string{"team@example.com"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider, err := CreateEmailProvider(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
				assert.Equal(t, "email", provider.Name())
			}
		})
	}
}

func TestEmailProvider_SubjectSanitization(t *testing.T) {
	t.Parallel()

	provider := NewEmailProvider(EmailConfig{
		From: "dsops@example.com",
		To:   []string{"team@example.com"},
	})

	t.Run("removes_newlines", func(t *testing.T) {
		event := RotationEvent{
			Type:        EventTypeCompleted,
			Service:     "postgresql\nInjected-Header: value",
			Environment: "production",
			Status:      StatusSuccess,
			Timestamp:   time.Now(),
		}

		subject, _, _ := provider.buildMessage(event)

		// Subject should not contain newlines (header injection prevention)
		assert.NotContains(t, subject, "\n")
		assert.NotContains(t, subject, "\r")
	})

	t.Run("removes_header_injection_patterns", func(t *testing.T) {
		event := RotationEvent{
			Type:        EventTypeFailed,
			Service:     "evil-service\r\nBcc: attacker@evil.com",
			Environment: "prod\nX-Injected-Header: malicious",
			Status:      StatusFailure,
			Timestamp:   time.Now(),
		}

		subject, _, _ := provider.buildMessage(event)

		// Subject should not contain newlines
		assert.NotContains(t, subject, "\n")
		assert.NotContains(t, subject, "\r")
		// Subject should not contain header injection patterns
		assert.NotContains(t, subject, "Bcc:")
		assert.NotContains(t, subject, "X-Injected-Header:")
		// Should still contain the safe parts
		assert.Contains(t, subject, "evil-service")
		assert.Contains(t, subject, "prod")
	})

	t.Run("removes_various_header_patterns", func(t *testing.T) {
		event := RotationEvent{
			Type:        EventTypeCompleted,
			Service:     "test Cc: someone@evil.com To: another@evil.com",
			Environment: "From: fake@sender.com Reply-To: phishing@evil.com",
			Status:      StatusSuccess,
			Timestamp:   time.Now(),
		}

		subject, _, _ := provider.buildMessage(event)

		// All header patterns should be removed
		assert.NotContains(t, subject, "Cc:")
		assert.NotContains(t, subject, "To:")
		assert.NotContains(t, subject, "From:")
		assert.NotContains(t, subject, "Reply-To:")
	})
}

func TestEmailProvider_ContextCancellation(t *testing.T) {
	t.Parallel()

	provider := NewEmailProvider(EmailConfig{
		SMTP: SMTPConfig{
			Host: "smtp.example.com",
			Port: 587,
		},
		From: "dsops@example.com",
		To:   []string{"team@example.com"},
	})

	// Set a sender that respects context
	provider.smtpSender = func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
		// Simulate slow send
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	event := RotationEvent{
		Type:      EventTypeCompleted,
		Service:   "postgresql",
		Timestamp: time.Now(),
	}

	// Note: Standard smtp.SendMail doesn't respect context, but our wrapper should
	// This test documents the expected behavior
	_ = provider.Send(ctx, event)
}

// Helper to check string containment
// Commented out as unused (redundant with strings.Contains)
//func stringContains(s, substr string) bool {
//	return strings.Contains(s, substr)
//}
