package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SlackConfig holds configuration for Slack webhook notifications.
type SlackConfig struct {
	// WebhookURL is the Slack incoming webhook URL.
	WebhookURL string

	// Channel is the Slack channel to post to (optional, uses webhook default).
	Channel string

	// Events specifies which rotation events trigger notifications.
	// If empty, all events are sent.
	Events []string

	// Mentions specifies who to mention for specific events.
	Mentions *SlackMentions
}

// SlackMentions defines who to mention for specific event types.
type SlackMentions struct {
	// OnFailure lists Slack handles to mention when rotation fails.
	OnFailure []string

	// OnRollback lists Slack handles to mention when rollback occurs.
	OnRollback []string
}

// SlackProvider sends rotation notifications to Slack via webhooks.
type SlackProvider struct {
	config SlackConfig
	client *http.Client
}

// NewSlackProvider creates a new Slack notification provider.
func NewSlackProvider(config SlackConfig) *SlackProvider {
	return &SlackProvider{
		config: config,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider name.
func (p *SlackProvider) Name() string {
	return "slack"
}

// SupportsEvent returns true if this provider handles the given event type.
func (p *SlackProvider) SupportsEvent(eventType EventType) bool {
	// If no events are configured, support all
	if len(p.config.Events) == 0 {
		return true
	}

	eventStr := string(eventType)
	for _, e := range p.config.Events {
		if strings.EqualFold(e, eventStr) {
			return true
		}
	}
	return false
}

// Validate checks if the provider configuration is valid.
func (p *SlackProvider) Validate(ctx context.Context) error {
	if p.config.WebhookURL == "" {
		return fmt.Errorf("webhook URL is required")
	}

	parsed, err := url.Parse(p.config.WebhookURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("invalid webhook URL: %s", p.config.WebhookURL)
	}

	return nil
}

// Send sends a notification to Slack for the given rotation event.
func (p *SlackProvider) Send(ctx context.Context, event RotationEvent) error {
	message := p.buildMessage(event)

	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Slack notification: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}

	return nil
}

// buildMessage creates a Block Kit formatted Slack message.
func (p *SlackProvider) buildMessage(event RotationEvent) map[string]interface{} {
	blocks := make([]map[string]interface{}, 0)

	// Header with emoji
	emoji := p.getEventEmoji(event.Type, event.Status)
	title := p.getEventTitle(event.Type, event.Status)
	blocks = append(blocks, map[string]interface{}{
		"type": "header",
		"text": map[string]interface{}{
			"type":  "plain_text",
			"text":  fmt.Sprintf("%s %s", emoji, title),
			"emoji": true,
		},
	})

	// Service and environment info
	blocks = append(blocks, map[string]interface{}{
		"type": "section",
		"fields": []map[string]interface{}{
			{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Service:*\n%s", event.Service),
			},
			{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Environment:*\n%s", event.Environment),
			},
		},
	})

	// Strategy and duration
	if event.Strategy != "" || event.Duration > 0 {
		fields := make([]map[string]interface{}, 0)
		if event.Strategy != "" {
			fields = append(fields, map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Strategy:*\n%s", event.Strategy),
			})
		}
		if event.Duration > 0 {
			fields = append(fields, map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Duration:*\n%s", event.Duration.Round(time.Millisecond)),
			})
		}
		if len(fields) > 0 {
			blocks = append(blocks, map[string]interface{}{
				"type":   "section",
				"fields": fields,
			})
		}
	}

	// Error details for failed events
	if event.Error != nil {
		blocks = append(blocks, map[string]interface{}{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf(":warning: *Error:*\n```%s```", event.Error.Error()),
			},
		})
	}

	// Add mentions for failure or rollback events
	mentions := p.getMentions(event)
	if mentions != "" {
		blocks = append(blocks, map[string]interface{}{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Attention:* %s", mentions),
			},
		})
	}

	// Context (timestamp)
	blocks = append(blocks, map[string]interface{}{
		"type": "context",
		"elements": []map[string]interface{}{
			{
				"type": "mrkdwn",
				"text": fmt.Sprintf("<!date^%d^{date_short_pretty} at {time}|%s>",
					event.Timestamp.Unix(), event.Timestamp.Format(time.RFC3339)),
			},
		},
	})

	// Divider
	blocks = append(blocks, map[string]interface{}{
		"type": "divider",
	})

	message := map[string]interface{}{
		"blocks": blocks,
	}

	// Add channel if specified
	if p.config.Channel != "" {
		message["channel"] = p.config.Channel
	}

	return message
}

// getEventEmoji returns the appropriate emoji for the event type.
func (p *SlackProvider) getEventEmoji(eventType EventType, status RotationStatus) string {
	switch eventType {
	case EventTypeStarted:
		return ":arrows_counterclockwise:"
	case EventTypeCompleted:
		if status == StatusSuccess {
			return ":white_check_mark:"
		}
		return ":warning:"
	case EventTypeFailed:
		return ":x:"
	case EventTypeRollback:
		return ":rewind:"
	default:
		return ":bell:"
	}
}

// getEventTitle returns a human-readable title for the event.
func (p *SlackProvider) getEventTitle(eventType EventType, status RotationStatus) string {
	switch eventType {
	case EventTypeStarted:
		return "Rotation Started"
	case EventTypeCompleted:
		if status == StatusSuccess {
			return "Rotation Completed Successfully"
		}
		return "Rotation Completed with Warnings"
	case EventTypeFailed:
		return "Rotation Failed"
	case EventTypeRollback:
		return "Rotation Rolled Back"
	default:
		return "Rotation Event"
	}
}

// getMentions returns a string of Slack mentions for the event.
func (p *SlackProvider) getMentions(event RotationEvent) string {
	if p.config.Mentions == nil {
		return ""
	}

	var mentions []string

	switch event.Type {
	case EventTypeFailed:
		mentions = p.config.Mentions.OnFailure
	case EventTypeRollback:
		mentions = p.config.Mentions.OnRollback
	}

	if len(mentions) == 0 {
		return ""
	}

	return strings.Join(mentions, " ")
}

// CreateSlackProvider creates a Slack provider from config notification settings.
func CreateSlackProvider(config *SlackNotificationConfig) (*SlackProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("slack config is nil")
	}

	slackConfig := SlackConfig{
		WebhookURL: config.WebhookURL,
		Channel:    config.Channel,
		Events:     config.Events,
	}

	if config.Mentions != nil {
		slackConfig.Mentions = &SlackMentions{
			OnFailure:  config.Mentions.OnFailure,
			OnRollback: config.Mentions.OnRollback,
		}
	}

	provider := NewSlackProvider(slackConfig)
	if err := provider.Validate(context.Background()); err != nil {
		return nil, err
	}

	return provider, nil
}

// SlackNotificationConfig mirrors the config package type for internal use.
type SlackNotificationConfig struct {
	WebhookURL string
	Channel    string
	Events     []string
	Mentions   *SlackMentionConfig
}

// SlackMentionConfig mirrors the config package mention type.
type SlackMentionConfig struct {
	OnFailure  []string
	OnRollback []string
}
