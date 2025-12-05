package notifications

import (
	"bytes"
	"fmt"
	"text/template"
	"time"
)

// TemplateData contains data for rendering notification templates.
type TemplateData struct {
	// Service is the service being rolled back.
	Service string

	// Environment is the environment.
	Environment string

	// Reason explains why the rollback was triggered.
	Reason string

	// TargetVersion is the version being rolled back to.
	TargetVersion string

	// FailedVersion is the version that failed.
	FailedVersion string

	// Trigger indicates what caused the rollback (automatic, manual).
	Trigger string

	// User is who initiated a manual rollback.
	User string

	// Duration is how long the rollback took.
	Duration time.Duration

	// Attempts is the number of attempts made.
	Attempts int

	// Error contains error details if the rollback failed.
	Error string

	// Timestamp is when the event occurred.
	Timestamp time.Time

	// Status is the rollback status (success, failed).
	Status string

	// NextSteps provides recommendations for what to do next.
	NextSteps string
}

// RollbackTemplates contains all rollback-specific message templates.
var RollbackTemplates = struct {
	Started   *template.Template
	Completed *template.Template
	Failed    *template.Template
}{
	Started:   template.Must(template.New("rollback_started").Parse(rollbackStartedTemplate)),
	Completed: template.Must(template.New("rollback_completed").Parse(rollbackCompletedTemplate)),
	Failed:    template.Must(template.New("rollback_failed").Parse(rollbackFailedTemplate)),
}

const rollbackStartedTemplate = `Rollback Started

Service:     {{.Service}}
Environment: {{.Environment}}
Trigger:     {{.Trigger}}
{{if .User}}Initiated by: {{.User}}{{end}}
Reason:      {{.Reason}}

Rolling back from {{.FailedVersion}} to {{.TargetVersion}}

A rollback operation has been initiated for this service.`

const rollbackCompletedTemplate = `Rollback Completed Successfully

Service:     {{.Service}}
Environment: {{.Environment}}
Duration:    {{.Duration}}
Attempts:    {{.Attempts}}
Trigger:     {{.Trigger}}
{{if .User}}Initiated by: {{.User}}{{end}}

Rolled back from {{.FailedVersion}} to {{.TargetVersion}}

Reason: {{.Reason}}

{{.NextSteps}}`

const rollbackFailedTemplate = `Rollback Failed

Service:     {{.Service}}
Environment: {{.Environment}}
Duration:    {{.Duration}}
Attempts:    {{.Attempts}}
Trigger:     {{.Trigger}}
{{if .User}}Initiated by: {{.User}}{{end}}

Failed to rollback from {{.FailedVersion}} to {{.TargetVersion}}

Reason: {{.Reason}}
Error:  {{.Error}}

{{.NextSteps}}`

// NextStepsSuccess provides recommendations after a successful rollback.
const NextStepsSuccess = `Next Steps:
- Service has been restored to the previous version
- Monitor service health for the next 15-30 minutes
- Investigate the root cause of the failed rotation
- Consider disabling automatic rotation until the issue is resolved
- Review rotation logs: dsops rotation history <service-name>`

// NextStepsFailure provides recommendations after a failed rollback.
const NextStepsFailure = `Next Steps:
- Manual intervention is required
- Service may be in an inconsistent state
- Check service logs and health metrics immediately
- Consider contacting the on-call team
- Review rollback logs: dsops rotation history <service-name>
- If service is down, initiate incident response procedures`

// RenderRollbackStarted renders the rollback started notification.
func RenderRollbackStarted(data TemplateData) (string, error) {
	return renderTemplate(RollbackTemplates.Started, data)
}

// RenderRollbackCompleted renders the rollback completed notification.
func RenderRollbackCompleted(data TemplateData) (string, error) {
	if data.NextSteps == "" {
		data.NextSteps = NextStepsSuccess
	}
	return renderTemplate(RollbackTemplates.Completed, data)
}

// RenderRollbackFailed renders the rollback failed notification.
func RenderRollbackFailed(data TemplateData) (string, error) {
	if data.NextSteps == "" {
		data.NextSteps = NextStepsFailure
	}
	return renderTemplate(RollbackTemplates.Failed, data)
}

// renderTemplate renders a template with the given data.
func renderTemplate(tmpl *template.Template, data TemplateData) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}
	return buf.String(), nil
}

// FormatDuration formats a duration for human reading.
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// GetNextSteps returns appropriate next steps based on the rollback result.
func GetNextSteps(success bool) string {
	if success {
		return NextStepsSuccess
	}
	return NextStepsFailure
}

// NewTemplateDataFromEvent creates TemplateData from a RotationEvent.
func NewTemplateDataFromEvent(event RotationEvent) TemplateData {
	data := TemplateData{
		Service:       event.Service,
		Environment:   event.Environment,
		TargetVersion: event.PreviousVersion,
		FailedVersion: event.NewVersion,
		User:          event.InitiatedBy,
		Duration:      event.Duration,
		Timestamp:     event.Timestamp,
	}

	// Extract metadata
	if event.Metadata != nil {
		if reason, ok := event.Metadata["reason"]; ok {
			data.Reason = reason
		}
		if trigger, ok := event.Metadata["trigger"]; ok {
			data.Trigger = trigger
		} else {
			data.Trigger = "automatic"
		}
		if attempts, ok := event.Metadata["attempts"]; ok {
			fmt.Sscanf(attempts, "%d", &data.Attempts)
		}
	}

	// Set status and next steps
	if event.Status == StatusRolledBack {
		data.Status = "success"
		data.NextSteps = NextStepsSuccess
	} else {
		data.Status = "failed"
		data.NextSteps = NextStepsFailure
		if event.Error != nil {
			data.Error = event.Error.Error()
		}
	}

	return data
}
