package rotation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/systmms/dsops/internal/dsopsdata"
	"github.com/systmms/dsops/internal/logging"
)

// ScriptRotator implements rotation via custom scripts
type ScriptRotator struct {
	logger     *logging.Logger
	repository *dsopsdata.Repository
}

// NewScriptRotator creates a new custom script rotation strategy
func NewScriptRotator(logger *logging.Logger) *ScriptRotator {
	return &ScriptRotator{
		logger:     logger,
		repository: nil,
	}
}

// SetRepository sets the dsops-data repository for schema-aware rotation
func (s *ScriptRotator) SetRepository(repository *dsopsdata.Repository) {
	s.repository = repository
}

// Name returns the strategy name
func (s *ScriptRotator) Name() string {
	return "script"
}

// SupportsSecret checks if this strategy can rotate the given secret
func (s *ScriptRotator) SupportsSecret(ctx context.Context, secret SecretInfo) bool {
	// Script strategy requires a script path in metadata
	_, hasScript := secret.Metadata["script_path"]
	if !hasScript {
		return false
	}

	// If we have schema information, check if rotation capability is available
	if err := s.validateCapability(secret, "rotate"); err != nil {
		s.logger.Debug("Rotation capability validation failed: %v", err)
		return false
	}

	return true
}

// hasCapability checks if a capability is present in the capabilities list
func (s *ScriptRotator) hasCapability(capabilities []string, capability string) bool {
	for _, cap := range capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}

// validateCapability checks if the credential kind supports a specific capability
func (s *ScriptRotator) validateCapability(secret SecretInfo, capability string) error {
	if s.repository == nil {
		return nil // Skip validation if no schema available
	}

	credKind := s.getCredentialKind(secret)
	if credKind == nil {
		return nil // Skip validation if credential kind not found
	}

	if !s.hasCapability(credKind.Capabilities, capability) {
		return fmt.Errorf("credential kind %s/%s does not support %s capability", 
			string(secret.SecretType), credKind.Name, capability)
	}

	return nil
}

// getCredentialKind returns the credential kind definition from the schema
func (s *ScriptRotator) getCredentialKind(secret SecretInfo) *dsopsdata.CredentialKind {
	if s.repository == nil {
		return nil
	}

	// Get service type from secret type or metadata
	serviceTypeName := string(secret.SecretType)
	if svcType, exists := secret.Metadata["service_type"]; exists {
		serviceTypeName = svcType
	}

	serviceType, exists := s.repository.GetServiceType(serviceTypeName)
	if !exists {
		return nil
	}

	// Get credential kind name from metadata, default to first available
	credentialKindName := "default"
	if ck, exists := secret.Metadata["credential_kind"]; exists {
		credentialKindName = ck
	}

	// Find the credential kind
	for i, credKind := range serviceType.Spec.CredentialKinds {
		if credKind.Name == credentialKindName {
			return &serviceType.Spec.CredentialKinds[i]
		}
	}

	// If named kind not found and we used default, try the first one
	if credentialKindName == "default" && len(serviceType.Spec.CredentialKinds) > 0 {
		return &serviceType.Spec.CredentialKinds[0]
	}

	return nil
}

// buildSchemaMetadata creates schema metadata for script input
func (s *ScriptRotator) buildSchemaMetadata(secret SecretInfo) *SchemaMetadata {
	if s.repository == nil {
		return nil
	}

	serviceTypeName := string(secret.SecretType)
	if svcType, exists := secret.Metadata["service_type"]; exists {
		serviceTypeName = svcType
	}

	credentialKindName := "default"
	if ck, exists := secret.Metadata["credential_kind"]; exists {
		credentialKindName = ck
	}

	credKind := s.getCredentialKind(secret)
	if credKind == nil {
		return &SchemaMetadata{
			ServiceType:    serviceTypeName,
			CredentialKind: credentialKindName,
		}
	}

	metadata := &SchemaMetadata{
		ServiceType:    serviceTypeName,
		CredentialKind: credKind.Name,
		Capabilities:   credKind.Capabilities,
		Constraints: &CredentialConstraints{
			MaxActive:        credKind.Constraints.MaxActive,
			TTL:              credKind.Constraints.TTL,
			Format:           credKind.Constraints.Format,
		},
	}

	// Add service instance metadata if available
	if instanceID, exists := secret.Metadata["instance_id"]; exists {
		if instance, exists := s.repository.GetServiceInstance(serviceTypeName, instanceID); exists {
			metadata.ServiceInstance = &ServiceInstanceMeta{
				Type:     instance.Metadata.Type,
				ID:       instance.Metadata.ID,
				Name:     instance.Metadata.Name,
				Endpoint: instance.Spec.Endpoint,
				Auth:     instance.Spec.Auth,
				Config:   instance.Spec.Config,
			}
		}
	}

	return metadata
}

// ScriptInput represents the JSON input passed to the script
type ScriptInput struct {
	Action            string                 `json:"action"`
	SecretInfo        SecretInfo             `json:"secret_info"`
	DryRun            bool                   `json:"dry_run"`
	Force             bool                   `json:"force"`
	NewValue          *NewSecretValue        `json:"new_value,omitempty"`
	Config            map[string]interface{} `json:"config,omitempty"`
	Environment       map[string]string      `json:"environment,omitempty"`
	SchemaMetadata    *SchemaMetadata        `json:"schema_metadata,omitempty"`
}

// SchemaMetadata contains information from dsops-data schemas
type SchemaMetadata struct {
	ServiceType       string              `json:"service_type,omitempty"`
	CredentialKind    string              `json:"credential_kind,omitempty"`
	Capabilities      []string            `json:"capabilities,omitempty"`
	Constraints       *CredentialConstraints `json:"constraints,omitempty"`
	ServiceInstance   *ServiceInstanceMeta   `json:"service_instance,omitempty"`
}

// CredentialConstraints mirrors the dsops-data constraints structure
type CredentialConstraints struct {
	MaxActive           interface{} `json:"maxActive,omitempty"`           // Can be int or "unlimited"
	TTL                 string      `json:"ttl,omitempty"`
	Format              string      `json:"format,omitempty"`
	RotationRequired    bool        `json:"rotationRequired,omitempty"`
	MaxKeys             int         `json:"maxKeys,omitempty"`
	Renewable           bool        `json:"renewable,omitempty"`
	Managed             bool        `json:"managed,omitempty"`
	RequiresMFA         bool        `json:"requiresMFA,omitempty"`
}

// ServiceInstanceMeta contains relevant service instance metadata
type ServiceInstanceMeta struct {
	Type        string                 `json:"type"`
	ID          string                 `json:"id"`
	Name        string                 `json:"name,omitempty"`
	Endpoint    string                 `json:"endpoint"`
	Auth        string                 `json:"auth"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// ScriptOutput represents the expected JSON output from the script
type ScriptOutput struct {
	Success      bool                   `json:"success"`
	Message      string                 `json:"message,omitempty"`
	NewSecretRef *SecretReference       `json:"new_secret_ref,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Warnings     []string               `json:"warnings,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Rotate performs rotation via custom script
func (s *ScriptRotator) Rotate(ctx context.Context, request RotationRequest) (*RotationResult, error) {
	auditTrail := []AuditEntry{
		{
			Timestamp: time.Now(),
			Action:    "script_rotation_started",
			Component: "script_rotator",
			Status:    "info",
			Message:   "Starting script rotation",
			Details: map[string]interface{}{
				"secret_key": logging.Secret(request.Secret.Key),
				"dry_run":    request.DryRun,
			},
		},
	}

	// Get script path from metadata
	scriptPath, ok := request.Secret.Metadata["script_path"]
	if !ok {
		return &RotationResult{
			Secret:     request.Secret,
			Status:     StatusFailed,
			Error:      "script_path not found in secret metadata",
			AuditTrail: auditTrail,
		}, fmt.Errorf("script_path not found in secret metadata")
	}

	s.logger.Info("Starting script rotation for %s using %s", logging.Secret(request.Secret.Key), scriptPath)

	// Check if rotate capability is available
	if err := s.validateCapability(request.Secret, "rotate"); err != nil {
		return &RotationResult{
			Secret:     request.Secret,
			Status:     StatusFailed,
			Error:      err.Error(),
			AuditTrail: auditTrail,
		}, err
	}

	// Prepare script input with schema metadata
	scriptInput := ScriptInput{
		Action:         "rotate",
		SecretInfo:     request.Secret,
		DryRun:         request.DryRun,
		Force:          request.Force,
		NewValue:       request.NewValue,
		Config:         request.Config,
		Environment:    s.buildEnvironment(request),
		SchemaMetadata: s.buildSchemaMetadata(request.Secret),
	}

	// Execute script
	output, err := s.executeScript(ctx, scriptPath, scriptInput)
	if err != nil {
		auditTrail = append(auditTrail, AuditEntry{
			Timestamp: time.Now(),
			Action:    "script_execution_failed",
			Component: "script_rotator",
			Status:    "error",
			Message:   "Failed to execute script",
			Error:     err.Error(),
		})

		return &RotationResult{
			Secret:     request.Secret,
			Status:     StatusFailed,
			Error:      fmt.Sprintf("script execution failed: %v", err),
			AuditTrail: auditTrail,
		}, err
	}

	// Process output
	if !output.Success {
		auditTrail = append(auditTrail, AuditEntry{
			Timestamp: time.Now(),
			Action:    "script_rotation_failed",
			Component: "script_rotator",
			Status:    "error",
			Message:   "Script returned failure",
			Error:     output.Error,
		})

		return &RotationResult{
			Secret:     request.Secret,
			Status:     StatusFailed,
			Error:      output.Error,
			Warnings:   output.Warnings,
			AuditTrail: auditTrail,
		}, fmt.Errorf("script returned failure: %s", output.Error)
	}

	// Build result
	rotatedAt := time.Now()
	auditTrail = append(auditTrail, AuditEntry{
		Timestamp: rotatedAt,
		Action:    "script_rotation_completed",
		Component: "script_rotator",
		Status:    "info",
		Message:   "Script rotation completed successfully",
		Details:   output.Metadata,
	})

	s.logger.Info("Successfully completed script rotation for %s", logging.Secret(request.Secret.Key))

	return &RotationResult{
		Secret:       request.Secret,
		Status:       StatusCompleted,
		NewSecretRef: output.NewSecretRef,
		RotatedAt:    &rotatedAt,
		Warnings:     output.Warnings,
		AuditTrail:   auditTrail,
	}, nil
}

// Verify performs verification via custom script
func (s *ScriptRotator) Verify(ctx context.Context, request VerificationRequest) error {
	scriptPath, ok := request.Secret.Metadata["script_path"]
	if !ok {
		return fmt.Errorf("script_path not found in secret metadata")
	}

	// Check if verify capability is available
	if err := s.validateCapability(request.Secret, "verify"); err != nil {
		s.logger.Debug("Verification capability not available: %v, skipping", err)
		return nil // Skip verification gracefully
	}

	// Allow separate verification script
	if verifyScript, ok := request.Secret.Metadata["script_verify_path"]; ok {
		scriptPath = verifyScript
	}

	s.logger.Debug("Verifying via script for %s", logging.Secret(request.Secret.Key))

	scriptInput := ScriptInput{
		Action:         "verify",
		SecretInfo:     request.Secret,
		Config: map[string]interface{}{
			"new_secret_ref": request.NewSecretRef,
			"tests":          request.Tests,
		},
		SchemaMetadata: s.buildSchemaMetadata(request.Secret),
	}

	output, err := s.executeScript(ctx, scriptPath, scriptInput)
	if err != nil {
		return fmt.Errorf("script verification failed: %w", err)
	}

	if !output.Success {
		return fmt.Errorf("script verification failed: %s", output.Error)
	}

	return nil
}

// Rollback performs rollback via custom script
func (s *ScriptRotator) Rollback(ctx context.Context, request RollbackRequest) error {
	scriptPath, ok := request.Secret.Metadata["script_path"]
	if !ok {
		return fmt.Errorf("script_path not found in secret metadata")
	}

	// Check if rollback capability is available (using "revoke" as proxy)
	if err := s.validateCapability(request.Secret, "revoke"); err != nil {
		// Note: using "revoke" capability as proxy for rollback since rollback isn't in the schema yet
		return fmt.Errorf("rollback not supported: %w", err)
	}

	s.logger.Info("Rolling back via script for %s", logging.Secret(request.Secret.Key))

	scriptInput := ScriptInput{
		Action:         "rollback",
		SecretInfo:     request.Secret,
		Config: map[string]interface{}{
			"old_secret_ref": request.OldSecretRef,
			"reason":         request.Reason,
		},
		SchemaMetadata: s.buildSchemaMetadata(request.Secret),
	}

	output, err := s.executeScript(ctx, scriptPath, scriptInput)
	if err != nil {
		return fmt.Errorf("script rollback failed: %w", err)
	}

	if !output.Success {
		return fmt.Errorf("script rollback failed: %s", output.Error)
	}

	return nil
}

// GetStatus performs status check via custom script
func (s *ScriptRotator) GetStatus(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
	scriptPath, ok := secret.Metadata["script_path"]
	if !ok {
		// Check capabilities if available
		if s.repository != nil {
			credKind := s.getCredentialKind(secret)
			if credKind != nil {
				canRotate := s.hasCapability(credKind.Capabilities, "rotate")
				reason := "Script rotation available"
				if !canRotate {
					reason = "Credential kind does not support rotation"
				}
				return &RotationStatusInfo{
					Status:    StatusPending,
					CanRotate: canRotate,
					Reason:    reason,
				}, nil
			}
		}
		
		// Default fallback
		return &RotationStatusInfo{
			Status:    StatusPending,
			CanRotate: true,
			Reason:    "Script rotation available",
		}, nil
	}

	// Allow separate status script
	if statusScript, ok := secret.Metadata["script_status_path"]; ok {
		scriptPath = statusScript
	}

	scriptInput := ScriptInput{
		Action:         "status",
		SecretInfo:     secret,
		SchemaMetadata: s.buildSchemaMetadata(secret),
	}

	output, err := s.executeScript(ctx, scriptPath, scriptInput)
	if err != nil {
		// Don't fail completely if status check fails, but consider capabilities
		defaultCanRotate := true
		defaultReason := "Status check failed, assuming rotation possible"
		
		if s.repository != nil {
			credKind := s.getCredentialKind(secret)
			if credKind != nil && !s.hasCapability(credKind.Capabilities, "rotate") {
				defaultCanRotate = false
				defaultReason = "Credential kind does not support rotation"
			}
		}
		
		s.logger.Warn("Failed to get script status for %s: %v", logging.Secret(secret.Key), err)
		return &RotationStatusInfo{
			Status:    StatusPending,
			CanRotate: defaultCanRotate,
			Reason:    defaultReason,
		}, nil
	}

	// Extract status info from metadata
	statusInfo := &RotationStatusInfo{
		Status:    StatusPending,
		CanRotate: true,
	}

	if output.Metadata != nil {
		if status, ok := output.Metadata["status"].(string); ok {
			statusInfo.Status = RotationStatus(status)
		}
		if canRotate, ok := output.Metadata["can_rotate"].(bool); ok {
			statusInfo.CanRotate = canRotate
		}
		if reason, ok := output.Metadata["reason"].(string); ok {
			statusInfo.Reason = reason
		}
		if lastRotated, ok := output.Metadata["last_rotated"].(string); ok {
			if t, err := time.Parse(time.RFC3339, lastRotated); err == nil {
				statusInfo.LastRotated = &t
			}
		}
		if nextRotation, ok := output.Metadata["next_rotation"].(string); ok {
			if t, err := time.Parse(time.RFC3339, nextRotation); err == nil {
				statusInfo.NextRotation = &t
			}
		}
		if version, ok := output.Metadata["rotation_version"].(string); ok {
			statusInfo.RotationVersion = version
		}
	}

	// Override with capability check if needed
	if s.repository != nil && statusInfo.CanRotate {
		credKind := s.getCredentialKind(secret)
		if credKind != nil && !s.hasCapability(credKind.Capabilities, "rotate") {
			statusInfo.CanRotate = false
			statusInfo.Reason = "Credential kind does not support rotation"
		}
	}

	return statusInfo, nil
}

// executeScript runs the script with JSON input and parses JSON output
func (s *ScriptRotator) executeScript(ctx context.Context, scriptPath string, input ScriptInput) (*ScriptOutput, error) {
	// Resolve script path
	scriptPath = os.ExpandEnv(scriptPath)
	if !filepath.IsAbs(scriptPath) {
		// Try to find script relative to working directory
		if cwd, err := os.Getwd(); err == nil {
			scriptPath = filepath.Join(cwd, scriptPath)
		}
	}

	// Check if script exists
	if _, err := os.Stat(scriptPath); err != nil {
		return nil, fmt.Errorf("script not found: %s: %w", scriptPath, err)
	}

	// Marshal input
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal script input: %w", err)
	}

	// Create command
	cmd := exec.CommandContext(ctx, scriptPath)
	cmd.Stdin = bytes.NewReader(inputJSON)
	
	// Set environment variables
	cmd.Env = os.Environ()
	for k, v := range input.Environment {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute script
	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		if stderrStr != "" {
			return nil, fmt.Errorf("script execution failed: %w\nstderr: %s", err, stderrStr)
		}
		return nil, fmt.Errorf("script execution failed: %w", err)
	}

	// Log stderr if present (may contain warnings/debug info)
	if stderrStr := stderr.String(); stderrStr != "" {
		s.logger.Debug("Script stderr output: %s", stderrStr)
	}

	// Parse output
	var output ScriptOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		// If JSON parsing fails, try to provide helpful error
		stdoutStr := stdout.String()
		if strings.TrimSpace(stdoutStr) == "" {
			return nil, fmt.Errorf("script produced no output")
		}
		return nil, fmt.Errorf("failed to parse script output as JSON: %w\nOutput: %s", err, stdoutStr)
	}

	return &output, nil
}

// buildEnvironment creates environment variables for the script
func (s *ScriptRotator) buildEnvironment(request RotationRequest) map[string]string {
	env := make(map[string]string)

	// Add standard environment variables
	env["DSOPS_ACTION"] = "rotate"
	env["DSOPS_SECRET_KEY"] = request.Secret.Key
	env["DSOPS_SECRET_PROVIDER"] = request.Secret.Provider
	env["DSOPS_SECRET_TYPE"] = string(request.Secret.SecretType)
	env["DSOPS_DRY_RUN"] = fmt.Sprintf("%t", request.DryRun)
	env["DSOPS_FORCE"] = fmt.Sprintf("%t", request.Force)

	// Add metadata as environment variables
	for k, v := range request.Secret.Metadata {
		envKey := fmt.Sprintf("DSOPS_META_%s", strings.ToUpper(strings.ReplaceAll(k, "-", "_")))
		env[envKey] = v
	}

	// Add config as environment variables
	for k, v := range request.Config {
		envKey := fmt.Sprintf("DSOPS_CONFIG_%s", strings.ToUpper(strings.ReplaceAll(k, "-", "_")))
		env[envKey] = fmt.Sprintf("%v", v)
	}

	return env
}