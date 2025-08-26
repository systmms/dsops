package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/pkg/provider"
	dserrors "github.com/systmms/dsops/internal/errors"
)

// GCPSecretManagerProvider implements the Provider interface for Google Cloud Secret Manager
type GCPSecretManagerProvider struct {
	name       string
	client     *secretmanager.Client
	logger     *logging.Logger
	config     GCPSecretManagerConfig
	projectID  string
}

// GCPSecretManagerConfig holds GCP Secret Manager-specific configuration
type GCPSecretManagerConfig struct {
	ProjectID              string
	ServiceAccountKeyPath  string
	ImpersonateAccount     string
	Location               string // For regional secrets
	UsePlaintextNames      bool   // Use plaintext names instead of resource names
}

// NewGCPSecretManagerProvider creates a new GCP Secret Manager provider
func NewGCPSecretManagerProvider(name string, configMap map[string]interface{}) (*GCPSecretManagerProvider, error) {
	logger := logging.New(false, false)
	
	config := GCPSecretManagerConfig{
		Location:          "global",
		UsePlaintextNames: true, // Default to user-friendly names
	}

	// Parse configuration
	if projectID, ok := configMap["project_id"].(string); ok {
		config.ProjectID = projectID
	}
	if keyPath, ok := configMap["service_account_key_path"].(string); ok {
		config.ServiceAccountKeyPath = keyPath
	}
	if impersonate, ok := configMap["impersonate_service_account"].(string); ok {
		config.ImpersonateAccount = impersonate
	}
	if location, ok := configMap["location"].(string); ok {
		config.Location = location
	}
	if usePlaintext, ok := configMap["use_plaintext_names"].(bool); ok {
		config.UsePlaintextNames = usePlaintext
	}

	// Validate required configuration
	if config.ProjectID == "" {
		// Try to get from environment or metadata
		if projectID := getGCPProjectID(); projectID != "" {
			config.ProjectID = projectID
		} else {
			return nil, dserrors.ConfigError{
				Field:      "project_id",
				Message:    "project_id is required for GCP Secret Manager",
				Suggestion: "Set project_id in config or GOOGLE_CLOUD_PROJECT environment variable",
			}
		}
	}

	// Create GCP client
	client, err := createGCPSecretManagerClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP Secret Manager client: %w", err)
	}

	return &GCPSecretManagerProvider{
		name:      name,
		client:    client,
		logger:    logger,
		config:    config,
		projectID: config.ProjectID,
	}, nil
}

// createGCPSecretManagerClient creates a GCP Secret Manager client
func createGCPSecretManagerClient(config GCPSecretManagerConfig) (*secretmanager.Client, error) {
	ctx := context.Background()
	
	var clientOptions []option.ClientOption

	// Service account key file
	if config.ServiceAccountKeyPath != "" {
		// Expand home directory if needed
		if strings.HasPrefix(config.ServiceAccountKeyPath, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get home directory: %w", err)
			}
			config.ServiceAccountKeyPath = filepath.Join(home, config.ServiceAccountKeyPath[2:])
		}
		
		clientOptions = append(clientOptions, option.WithCredentialsFile(config.ServiceAccountKeyPath))
	}

	// Service account impersonation
	if config.ImpersonateAccount != "" {
		// Use the new impersonate package
		impersonatedCredentials, err := impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{
			TargetPrincipal: config.ImpersonateAccount,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create impersonated credentials: %w", err)
		}
		clientOptions = append(clientOptions, option.WithTokenSource(impersonatedCredentials))
	}

	return secretmanager.NewClient(ctx, clientOptions...)
}

// getGCPProjectID attempts to get the GCP project ID from various sources
func getGCPProjectID() string {
	// Try environment variables
	if projectID := os.Getenv("GOOGLE_CLOUD_PROJECT"); projectID != "" {
		return projectID
	}
	if projectID := os.Getenv("GCLOUD_PROJECT"); projectID != "" {
		return projectID
	}
	if projectID := os.Getenv("GCP_PROJECT"); projectID != "" {
		return projectID
	}
	
	// TODO: Could try to read from gcloud config or metadata service
	return ""
}

// Name returns the provider name
func (p *GCPSecretManagerProvider) Name() string {
	return p.name
}

// Resolve fetches a secret from GCP Secret Manager
func (p *GCPSecretManagerProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	secretName, version, jsonPath := p.parseReference(ref.Key)
	
	// Build the resource name
	resourceName := p.buildResourceName(secretName, version)
	
	p.logger.Debug("Accessing GCP secret: %s", logging.Secret(resourceName))

	// Access the secret version
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: resourceName,
	}

	result, err := p.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return provider.SecretValue{}, dserrors.UserError{
			Message:    fmt.Sprintf("Failed to access secret: %s", secretName),
			Details:    err.Error(),
			Suggestion: getGCPErrorSuggestion(err),
		}
	}

	if result.Payload == nil || result.Payload.Data == nil {
		return provider.SecretValue{}, fmt.Errorf("secret has no data")
	}

	secretData := string(result.Payload.Data)
	
	// Extract JSON field if specified
	if jsonPath != "" {
		extractedValue, err := extractJSONPath(secretData, jsonPath)
		if err != nil {
			return provider.SecretValue{}, dserrors.UserError{
				Message:    fmt.Sprintf("Failed to extract JSON path: %s", jsonPath),
				Details:    err.Error(),
				Suggestion: "Check that the secret contains valid JSON and the path exists",
			}
		}
		secretData = extractedValue
	}

	// Build metadata
	metadata := map[string]string{
		"source":     fmt.Sprintf("gcp-sm:%s", resourceName),
		"project_id": p.projectID,
	}

	if result.Name != "" {
		metadata["resource_name"] = result.Name
	}
	// Note: AccessSecretVersionResponse doesn't include CreateTime
	// Would need separate GetSecret call for creation time

	return provider.SecretValue{
		Value:    secretData,
		Metadata: metadata,
	}, nil
}

// parseReference parses GCP secret references
func (p *GCPSecretManagerProvider) parseReference(ref string) (secretName, version, jsonPath string) {
	version = "latest" // Default version
	
	// Check for JSON path extraction (secret-name#.json.path)
	if strings.Contains(ref, "#") {
		parts := strings.SplitN(ref, "#", 2)
		ref = parts[0]
		jsonPath = parts[1]
	}
	
	// Check for version specification (secret-name:version or secret-name@version)
	if strings.Contains(ref, ":") && !strings.HasPrefix(ref, "projects/") {
		parts := strings.SplitN(ref, ":", 2)
		secretName = parts[0]
		version = parts[1]
	} else if strings.Contains(ref, "@") {
		parts := strings.SplitN(ref, "@", 2)
		secretName = parts[0]
		version = parts[1]
	} else {
		secretName = ref
	}
	
	return secretName, version, jsonPath
}

// buildResourceName builds the full GCP resource name
func (p *GCPSecretManagerProvider) buildResourceName(secretName, version string) string {
	// If it's already a full resource name, use it as-is
	if strings.HasPrefix(secretName, "projects/") {
		if strings.Contains(secretName, "/versions/") {
			return secretName
		}
		return fmt.Sprintf("%s/versions/%s", secretName, version)
	}
	
	// Build from project ID and secret name
	return fmt.Sprintf("projects/%s/secrets/%s/versions/%s", p.projectID, secretName, version)
}

// extractJSONPath extracts a value from JSON using a simple path
func extractJSONPath(jsonStr, path string) (string, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	
	// Simple path extraction (e.g., .field.subfield)
	path = strings.TrimPrefix(path, ".")
	parts := strings.Split(path, ".")
	
	current := data
	for _, part := range parts {
		if part == "" {
			continue
		}
		
		switch v := current.(type) {
		case map[string]interface{}:
			var exists bool
			current, exists = v[part]
			if !exists {
				return "", fmt.Errorf("path not found: %s", part)
			}
		case []interface{}:
			// Handle array index
			if index, err := strconv.Atoi(part); err == nil && index < len(v) {
				current = v[index]
			} else {
				return "", fmt.Errorf("invalid array index: %s", part)
			}
		default:
			return "", fmt.Errorf("cannot traverse path at: %s", part)
		}
	}
	
	// Convert result to string
	switch v := current.(type) {
	case string:
		return v, nil
	case nil:
		return "", nil
	default:
		// Convert to JSON for complex types
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to marshal result: %w", err)
		}
		return string(jsonBytes), nil
	}
}

// Describe returns metadata about a secret without fetching its value
func (p *GCPSecretManagerProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	secretName, _, _ := p.parseReference(ref.Key)
	
	// Build secret resource name (without version)
	var resourceName string
	if strings.HasPrefix(secretName, "projects/") {
		resourceName = secretName
	} else {
		resourceName = fmt.Sprintf("projects/%s/secrets/%s", p.projectID, secretName)
	}

	req := &secretmanagerpb.GetSecretRequest{
		Name: resourceName,
	}

	result, err := p.client.GetSecret(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return provider.Metadata{
				Exists: false,
			}, nil
		}
		return provider.Metadata{}, fmt.Errorf("failed to describe secret: %w", err)
	}

	metadata := provider.Metadata{
		Exists: true,
		Type:   "gcp-secret",
		Tags: map[string]string{
			"source":     fmt.Sprintf("gcp-sm:%s", resourceName),
			"project_id": p.projectID,
		},
	}

	if result.CreateTime != nil {
		metadata.UpdatedAt = result.CreateTime.AsTime()
	}

	// Add labels as tags
	for key, value := range result.Labels {
		metadata.Tags["label."+key] = value
	}

	return metadata, nil
}

// Capabilities returns the provider's capabilities
func (p *GCPSecretManagerProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: true,
		SupportsMetadata:   true,
		SupportsWatching:   false,
		SupportsBinary:     true,
		RequiresAuth:       true,
		AuthMethods:        []string{"service_account", "application_default", "impersonation"},
	}
}

// Validate checks if the provider is properly configured and accessible
func (p *GCPSecretManagerProvider) Validate(ctx context.Context) error {
	// Test by listing secrets (minimal permissions needed)
	req := &secretmanagerpb.ListSecretsRequest{
		Parent:   fmt.Sprintf("projects/%s", p.projectID),
		PageSize: 1, // Just test access
	}

	iter := p.client.ListSecrets(ctx, req)
	_, err := iter.Next()
	if err != nil && err != iterator.Done {
		return dserrors.UserError{
			Message:    "Failed to connect to GCP Secret Manager",
			Details:    err.Error(),
			Suggestion: getGCPErrorSuggestion(err),
		}
	}

	return nil
}

// getGCPErrorSuggestion provides helpful suggestions based on GCP errors
func getGCPErrorSuggestion(err error) string {
	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "PermissionDenied"):
		return "Check IAM permissions: secretmanager.secrets.get, secretmanager.versions.access"
	case strings.Contains(errStr, "NotFound"):
		return "Verify the secret name and project ID. Check that the secret exists"
	case strings.Contains(errStr, "Unauthenticated"):
		return "Check authentication: set GOOGLE_APPLICATION_CREDENTIALS or run 'gcloud auth application-default login'"
	case strings.Contains(errStr, "InvalidArgument"):
		return "Check the secret name format and version specification"
	case strings.Contains(errStr, "ResourceExhausted"):
		return "Request was throttled. Consider adding exponential backoff"
	case strings.Contains(errStr, "project"):
		return "Check that the project ID is correct and the project exists"
	default:
		return "Check GCP credentials, project ID, and IAM permissions for Secret Manager"
	}
}

// Rotation support implementation

// CreateNewVersion creates a new version of a secret in GCP Secret Manager
func (p *GCPSecretManagerProvider) CreateNewVersion(ctx context.Context, ref provider.Reference, newValue []byte, meta map[string]string) (string, error) {
	secretName, _, _ := p.parseReference(ref.Key)
	
	// Build the full secret name
	fullSecretName := fmt.Sprintf("projects/%s/secrets/%s", p.config.ProjectID, secretName)
	
	// Create new secret version
	req := &secretmanagerpb.AddSecretVersionRequest{
		Parent: fullSecretName,
		Payload: &secretmanagerpb.SecretPayload{
			Data: newValue,
		},
	}
	
	result, err := p.client.AddSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create new secret version: %w", err)
	}
	
	// Extract version number from the response name
	// Format: projects/PROJECT/secrets/SECRET/versions/VERSION
	parts := strings.Split(result.Name, "/")
	if len(parts) >= 6 {
		return parts[5], nil // Return just the version number
	}
	
	return "latest", nil
}

// DeprecateVersion marks an old version as disabled in GCP Secret Manager
func (p *GCPSecretManagerProvider) DeprecateVersion(ctx context.Context, ref provider.Reference, version string) error {
	secretName, _, _ := p.parseReference(ref.Key)
	
	// Build the full version name
	versionName := fmt.Sprintf("projects/%s/secrets/%s/versions/%s", p.config.ProjectID, secretName, version)
	
	// Disable the secret version
	req := &secretmanagerpb.DisableSecretVersionRequest{
		Name: versionName,
	}
	
	_, err := p.client.DisableSecretVersion(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to disable secret version: %w", err)
	}
	
	return nil
}

// GetRotationMetadata returns metadata about rotation capabilities for a secret
func (p *GCPSecretManagerProvider) GetRotationMetadata(ctx context.Context, ref provider.Reference) (provider.RotationMetadata, error) {
	secretName, _, _ := p.parseReference(ref.Key)
	
	// Build the full secret name
	fullSecretName := fmt.Sprintf("projects/%s/secrets/%s", p.config.ProjectID, secretName)
	
	// Get secret metadata
	req := &secretmanagerpb.GetSecretRequest{
		Name: fullSecretName,
	}
	
	secret, err := p.client.GetSecret(ctx, req)
	if err != nil {
		return provider.RotationMetadata{}, fmt.Errorf("failed to get secret metadata: %w", err)
	}
	
	metadata := provider.RotationMetadata{
		SupportsRotation:   true,
		SupportsVersioning: true,
		MaxValueLength:     65536, // GCP Secret Manager limit
		MinValueLength:     1,
		Constraints: map[string]string{
			"provider":   "gcp-secretmanager",
			"project_id": p.config.ProjectID,
		},
	}
	
	// Set creation time as last rotated (GCP doesn't track rotation explicitly)
	if secret.CreateTime != nil {
		createTime := secret.CreateTime.AsTime()
		metadata.LastRotated = &createTime
	}
	
	// Check for labels that might indicate rotation policy
	if secret.Labels != nil {
		if rotationInterval, exists := secret.Labels["rotation_interval"]; exists {
			metadata.RotationInterval = rotationInterval
		}
		if rotationPolicy, exists := secret.Labels["rotation_policy"]; exists {
			metadata.Constraints["rotation_policy"] = rotationPolicy
		}
	}
	
	return metadata, nil
}

// NewGCPSecretManagerProviderFactory creates a GCP Secret Manager provider factory
func NewGCPSecretManagerProviderFactory(name string, config map[string]interface{}) (provider.Provider, error) {
	return NewGCPSecretManagerProvider(name, config)
}