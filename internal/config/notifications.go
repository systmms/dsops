package config

// NotificationConfig holds configuration for rotation notifications.
type NotificationConfig struct {
	// Slack configuration for Slack webhook notifications.
	Slack *SlackNotificationConfig `yaml:"slack,omitempty"`

	// Email configuration for SMTP email notifications.
	Email *EmailNotificationConfig `yaml:"email,omitempty"`

	// PagerDuty configuration for PagerDuty incident notifications.
	PagerDuty *PagerDutyNotificationConfig `yaml:"pagerduty,omitempty"`

	// Webhooks configuration for custom webhook notifications.
	Webhooks []WebhookNotificationConfig `yaml:"webhooks,omitempty"`
}

// SlackNotificationConfig holds Slack webhook configuration for rotation events.
type SlackNotificationConfig struct {
	// WebhookURL is the Slack incoming webhook URL.
	// Can be a secret reference like "store://vault/slack/webhook".
	WebhookURL string `yaml:"webhook_url"`

	// Channel is the Slack channel to post to (optional, uses webhook default).
	Channel string `yaml:"channel,omitempty"`

	// Events specifies which rotation events trigger notifications.
	// Valid values: started, completed, failed, rollback.
	// If empty, all events are sent.
	Events []string `yaml:"events,omitempty"`

	// Mentions specifies who to mention for specific events.
	Mentions *SlackMentions `yaml:"mentions,omitempty"`
}

// SlackMentions defines who to mention for specific event types.
type SlackMentions struct {
	// OnFailure lists Slack handles to mention when rotation fails.
	// Examples: ["@oncall", "@platform-team"]
	OnFailure []string `yaml:"on_failure,omitempty"`

	// OnRollback lists Slack handles to mention when rollback occurs.
	OnRollback []string `yaml:"on_rollback,omitempty"`
}

// EmailNotificationConfig holds SMTP email configuration for rotation events.
type EmailNotificationConfig struct {
	// SMTP server configuration.
	SMTP SMTPConfig `yaml:"smtp"`

	// From is the sender email address.
	From string `yaml:"from"`

	// To is the list of recipient email addresses.
	To []string `yaml:"to"`

	// Events specifies which rotation events trigger notifications.
	Events []string `yaml:"events,omitempty"`

	// BatchMode controls email batching: immediate, hourly, daily.
	BatchMode string `yaml:"batch_mode,omitempty"`
}

// SMTPConfig holds SMTP server configuration.
type SMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
	TLS      bool   `yaml:"tls,omitempty"`
}

// PagerDutyNotificationConfig holds PagerDuty configuration for rotation events.
type PagerDutyNotificationConfig struct {
	// IntegrationKey is the PagerDuty Events API integration key.
	// Can be a secret reference like "store://vault/pagerduty/integration-key".
	IntegrationKey string `yaml:"integration_key"`

	// ServiceID is the PagerDuty service ID (optional).
	ServiceID string `yaml:"service_id,omitempty"`

	// Severity is the default incident severity: critical, error, warning, info.
	Severity string `yaml:"severity,omitempty"`

	// Events specifies which rotation events trigger notifications.
	Events []string `yaml:"events,omitempty"`

	// AutoResolve indicates whether to auto-resolve incidents on success.
	AutoResolve bool `yaml:"auto_resolve,omitempty"`
}

// WebhookNotificationConfig holds configuration for custom webhook notifications.
type WebhookNotificationConfig struct {
	// Name is a human-readable name for this webhook.
	Name string `yaml:"name"`

	// URL is the webhook endpoint URL.
	URL string `yaml:"url"`

	// Method is the HTTP method to use (default: POST).
	Method string `yaml:"method,omitempty"`

	// Headers are additional HTTP headers to include.
	Headers map[string]string `yaml:"headers,omitempty"`

	// Events specifies which rotation events trigger notifications.
	Events []string `yaml:"events,omitempty"`

	// PayloadTemplate is a Go template for the request body.
	// If empty, a default JSON payload is used.
	PayloadTemplate string `yaml:"payload_template,omitempty"`

	// Retry configuration.
	Retry *WebhookRetryConfig `yaml:"retry,omitempty"`

	// Timeout in seconds (default: 10).
	TimeoutSeconds int `yaml:"timeout,omitempty"`
}

// WebhookRetryConfig holds retry configuration for webhooks.
type WebhookRetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts (default: 3).
	MaxAttempts int `yaml:"max_attempts,omitempty"`

	// Backoff strategy: linear, exponential (default: exponential).
	Backoff string `yaml:"backoff,omitempty"`
}

// RollbackConfig holds configuration for automatic rollback behavior.
type RollbackConfig struct {
	// Automatic enables automatic rollback on verification failure.
	Automatic bool `yaml:"automatic,omitempty"`

	// OnVerificationFailure triggers rollback when verification fails.
	OnVerificationFailure bool `yaml:"on_verification_failure,omitempty"`

	// OnHealthCheckFailure triggers rollback when health checks fail.
	OnHealthCheckFailure bool `yaml:"on_health_check_failure,omitempty"`

	// TimeoutSeconds is the maximum time for rollback operation (default: 30).
	TimeoutSeconds int `yaml:"timeout,omitempty"`

	// MaxRetries is the number of times to retry rollback if it fails (default: 2).
	MaxRetries int `yaml:"max_retries,omitempty"`

	// Notifications lists notification channels for rollback events.
	Notifications []string `yaml:"notifications,omitempty"`
}
