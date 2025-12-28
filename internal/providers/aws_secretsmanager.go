package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/systmms/dsops/pkg/provider"
)

// SecretsManagerClientAPI defines the interface for AWS Secrets Manager operations
// This allows for mocking in tests
type SecretsManagerClientAPI interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	DescribeSecret(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error)
	ListSecrets(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error)
	UpdateSecret(ctx context.Context, params *secretsmanager.UpdateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretOutput, error)
	UpdateSecretVersionStage(ctx context.Context, params *secretsmanager.UpdateSecretVersionStageInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretVersionStageOutput, error)
}

// AWSSecretsManagerProvider implements the provider interface for AWS Secrets Manager
type AWSSecretsManagerProvider struct {
	name     string
	client   SecretsManagerClientAPI
	region   string
	endpoint string // Optional custom endpoint for LocalStack or testing
}

// ProviderOption is a functional option for configuring providers
type ProviderOption func(*AWSSecretsManagerProvider)

// WithSecretsManagerClient sets a custom Secrets Manager client (for testing)
func WithSecretsManagerClient(client SecretsManagerClientAPI) ProviderOption {
	return func(p *AWSSecretsManagerProvider) {
		p.client = client
	}
}

// NewAWSSecretsManagerProvider creates a new AWS Secrets Manager provider
func NewAWSSecretsManagerProvider(name string, providerConfig map[string]interface{}, opts ...ProviderOption) (*AWSSecretsManagerProvider, error) {
	// Get region from config
	region := "us-east-1" // Default region
	if r, ok := providerConfig["region"].(string); ok && r != "" {
		region = r
	}

	// Get optional endpoint for LocalStack/testing
	var endpoint string
	if e, ok := providerConfig["endpoint"].(string); ok && e != "" {
		endpoint = e
	}

	// Get optional static credentials for LocalStack/testing
	var accessKeyID, secretAccessKey string
	if ak, ok := providerConfig["access_key_id"].(string); ok && ak != "" {
		accessKeyID = ak
	}
	if sk, ok := providerConfig["secret_access_key"].(string); ok && sk != "" {
		secretAccessKey = sk
	}

	p := &AWSSecretsManagerProvider{
		name:     name,
		region:   region,
		endpoint: endpoint,
	}

	// Apply options (allows mock client injection)
	for _, opt := range opts {
		opt(p)
	}

	// If no client was provided via options, create real client
	if p.client == nil {
		// Build config options
		var configOpts []func(*config.LoadOptions) error
		configOpts = append(configOpts, config.WithRegion(region))

		// Use static credentials if provided (for LocalStack/testing)
		if accessKeyID != "" && secretAccessKey != "" {
			configOpts = append(configOpts, config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
			))
		}

		// Load AWS config
		cfg, err := config.LoadDefaultConfig(context.Background(), configOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}

		// Create Secrets Manager client with optional custom endpoint
		var clientOpts []func(*secretsmanager.Options)
		if endpoint != "" {
			clientOpts = append(clientOpts, func(o *secretsmanager.Options) {
				o.BaseEndpoint = &endpoint
			})
		}
		p.client = secretsmanager.NewFromConfig(cfg, clientOpts...)
	}

	return p, nil
}

// Name returns the provider name
func (aws *AWSSecretsManagerProvider) Name() string {
	return aws.name
}

// Resolve retrieves a secret from AWS Secrets Manager
func (aws *AWSSecretsManagerProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	// Parse the key format
	secretName, jsonPath := aws.parseKey(ref.Key)

	// Build the input
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	}

	// Add version if specified
	if ref.Version != "" && ref.Version != "latest" {
		if isVersionId(ref.Version) {
			input.VersionId = aws.String(ref.Version)
		} else {
			input.VersionStage = aws.String(ref.Version)
		}
	}

	// Get the secret
	result, err := aws.client.GetSecretValue(ctx, input)
	if err != nil {
		return provider.SecretValue{}, aws.handleError(err, secretName)
	}

	// Extract the secret value
	var secretString string
	if result.SecretString != nil {
		secretString = *result.SecretString
	} else if result.SecretBinary != nil {
		secretString = string(result.SecretBinary)
	} else {
		return provider.SecretValue{}, fmt.Errorf("secret '%s' has no value", secretName)
	}

	// Apply JSON path extraction if specified
	if jsonPath != "" {
		extracted, err := aws.extractJSONPath(secretString, jsonPath)
		if err != nil {
			return provider.SecretValue{}, fmt.Errorf("failed to extract JSON path '%s': %w", jsonPath, err)
		}
		secretString = extracted
	}

	// Build metadata
	metadata := map[string]string{
		"provider":    aws.name,
		"secret_name": secretName,
		"region":      aws.region,
	}
	if result.VersionId != nil {
		metadata["version_id"] = *result.VersionId
	}
	if len(result.VersionStages) > 0 {
		metadata["version_stage"] = result.VersionStages[0]
	}

	return provider.SecretValue{
		Value:     secretString,
		Version:   aws.getVersionString(result),
		UpdatedAt: aws.getUpdatedTime(result),
		Metadata:  metadata,
	}, nil
}

// Describe returns metadata about an AWS Secrets Manager secret
func (aws *AWSSecretsManagerProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	secretName, _ := aws.parseKey(ref.Key)

	input := &secretsmanager.DescribeSecretInput{
		SecretId: aws.String(secretName),
	}

	result, err := aws.client.DescribeSecret(ctx, input)
	if err != nil {
		if isNotFoundError(err) {
			return provider.Metadata{Exists: false}, nil
		}
		return provider.Metadata{}, aws.handleError(err, secretName)
	}

	return provider.Metadata{
		Exists:    true,
		Version:   aws.getLatestVersionId(result),
		UpdatedAt: aws.getLastChangedDate(result),
		Type:      "aws-secret",
		Tags: map[string]string{
			"provider":     aws.name,
			"secret_name":  secretName,
			"region":       aws.region,
			"kms_key_id":   aws.getKMSKeyId(result),
			"replica_regions": strings.Join(aws.getReplicaRegions(result), ","),
		},
	}, nil
}

// Capabilities returns AWS Secrets Manager provider capabilities
func (aws *AWSSecretsManagerProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: true,
		SupportsMetadata:   true,
		SupportsWatching:   false,
		SupportsBinary:     true,
		RequiresAuth:       true,
		AuthMethods:        []string{"aws-credentials", "iam-role", "environment-variables"},
	}
}

// Validate checks if AWS credentials are configured and accessible
func (aws *AWSSecretsManagerProvider) Validate(ctx context.Context) error {
	// Try to list secrets (with limit 1) to verify credentials
	input := &secretsmanager.ListSecretsInput{
		MaxResults: aws.Int32(1),
	}

	_, err := aws.client.ListSecrets(ctx, input)
	if err != nil {
		return provider.AuthError{
			Provider: aws.name,
			Message:  fmt.Sprintf("AWS authentication failed: %v", err),
		}
	}

	return nil
}

// Helper methods

// parseKey parses AWS SM key formats:
// - "secret-name" -> secret-name, ""
// - "secret-name#.field" -> secret-name, ".field"  
// - "secret/path" -> secret/path, ""
func (aws *AWSSecretsManagerProvider) parseKey(key string) (secretName, jsonPath string) {
	if idx := strings.Index(key, "#"); idx != -1 {
		return key[:idx], key[idx+1:]
	}
	return key, ""
}

// extractJSONPath extracts a value from JSON using a simple path
func (aws *AWSSecretsManagerProvider) extractJSONPath(jsonStr, path string) (string, error) {
	if !strings.HasPrefix(path, ".") {
		return "", fmt.Errorf("JSON path must start with '.'")
	}

	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	// Remove leading dot and split path
	path = strings.TrimPrefix(path, ".")
	parts := strings.Split(path, ".")

	current := data
	for _, part := range parts {
		if part == "" {
			continue
		}

		switch v := current.(type) {
		case map[string]interface{}:
			if val, exists := v[part]; exists {
				current = val
			} else {
				return "", fmt.Errorf("field '%s' not found in JSON", part)
			}
		default:
			return "", fmt.Errorf("cannot navigate into non-object at path '%s'", part)
		}
	}

	// Convert result to string
	switch v := current.(type) {
	case string:
		return v, nil
	case float64:
		return fmt.Sprintf("%.0f", v), nil
	case bool:
		return fmt.Sprintf("%t", v), nil
	case nil:
		return "", nil
	default:
		// For complex objects, return as JSON
		bytes, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to marshal result: %w", err)
		}
		return string(bytes), nil
	}
}

// handleError converts AWS errors to provider errors
func (aws *AWSSecretsManagerProvider) handleError(err error, secretName string) error {
	if isNotFoundError(err) {
		return &provider.NotFoundError{
			Provider: aws.name,
			Key:      secretName,
		}
	}

	// Check for authentication/authorization errors
	if isAuthError(err) {
		return provider.AuthError{
			Provider: aws.name,
			Message:  fmt.Sprintf("AWS authentication/authorization failed: %v", err),
		}
	}

	return fmt.Errorf("AWS Secrets Manager error: %w", err)
}

// Utility functions for AWS types

func (aws *AWSSecretsManagerProvider) getVersionString(result *secretsmanager.GetSecretValueOutput) string {
	if result.VersionId != nil {
		return *result.VersionId
	}
	if len(result.VersionStages) > 0 {
		return result.VersionStages[0]
	}
	return "latest"
}

func (aws *AWSSecretsManagerProvider) getUpdatedTime(result *secretsmanager.GetSecretValueOutput) time.Time {
	if result.CreatedDate != nil {
		return *result.CreatedDate
	}
	return time.Now()
}

func (aws *AWSSecretsManagerProvider) getLatestVersionId(result *secretsmanager.DescribeSecretOutput) string {
	for _, version := range result.VersionIdsToStages {
		for _, stage := range version {
			if stage == "AWSCURRENT" {
				return version[0] // Return the version ID
			}
		}
	}
	return "latest"
}

func (aws *AWSSecretsManagerProvider) getLastChangedDate(result *secretsmanager.DescribeSecretOutput) time.Time {
	if result.LastChangedDate != nil {
		return *result.LastChangedDate
	}
	if result.CreatedDate != nil {
		return *result.CreatedDate
	}
	return time.Now()
}

// Rotation support implementation

// CreateNewVersion creates a new version of a secret in AWS Secrets Manager
func (aws *AWSSecretsManagerProvider) CreateNewVersion(ctx context.Context, ref provider.Reference, newValue []byte, meta map[string]string) (string, error) {
	secretName := ref.Key

	// Update the secret value which creates a new version
	input := &secretsmanager.UpdateSecretInput{
		SecretId:     &secretName,
		SecretString: stringPtr(string(newValue)),
	}

	// Add description if provided in metadata
	if description, exists := meta["description"]; exists {
		input.Description = &description
	}

	result, err := aws.client.UpdateSecret(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to create new secret version: %w", err)
	}

	if result.VersionId != nil {
		return *result.VersionId, nil
	}

	return "latest", nil
}

// DeprecateVersion marks an old version as deprecated by removing it from AWSCURRENT stage
func (aws *AWSSecretsManagerProvider) DeprecateVersion(ctx context.Context, ref provider.Reference, version string) error {
	secretName := ref.Key

	// AWS Secrets Manager automatically moves the old version to AWSPENDING when a new version
	// becomes AWSCURRENT. We can optionally remove AWSPENDING stage to fully deprecate.
	
	input := &secretsmanager.UpdateSecretVersionStageInput{
		SecretId:     &secretName,
		VersionStage: stringPtr("AWSPENDING"),
		RemoveFromVersionId: &version,
	}

	_, err := aws.client.UpdateSecretVersionStage(ctx, input)
	if err != nil {
		// Don't fail if the version is already deprecated or doesn't exist
		if strings.Contains(err.Error(), "InvalidVersionStage") || 
		   strings.Contains(err.Error(), "InvalidParameterValue") {
			return nil
		}
		return fmt.Errorf("failed to deprecate secret version: %w", err)
	}

	return nil
}

// GetRotationMetadata returns metadata about rotation capabilities for a secret
func (aws *AWSSecretsManagerProvider) GetRotationMetadata(ctx context.Context, ref provider.Reference) (provider.RotationMetadata, error) {
	secretName := ref.Key

	// Describe the secret to get metadata
	input := &secretsmanager.DescribeSecretInput{
		SecretId: &secretName,
	}

	result, err := aws.client.DescribeSecret(ctx, input)
	if err != nil {
		return provider.RotationMetadata{}, fmt.Errorf("failed to describe secret: %w", err)
	}

	metadata := provider.RotationMetadata{
		SupportsRotation:   true,
		SupportsVersioning: true,
		MaxValueLength:     65536, // AWS Secrets Manager limit
		MinValueLength:     1,
		Constraints: map[string]string{
			"provider": "aws-secretsmanager",
			"region":   aws.region,
		},
	}

	// Set last rotated time
	if result.LastChangedDate != nil {
		metadata.LastRotated = result.LastChangedDate
	}

	// Check if automatic rotation is configured
	if result.RotationEnabled != nil && *result.RotationEnabled {
		if result.RotationLambdaARN != nil {
			metadata.Constraints["automatic_rotation"] = "enabled"
			metadata.Constraints["rotation_lambda"] = *result.RotationLambdaARN
		}
		
		if result.RotationRules != nil && result.RotationRules.AutomaticallyAfterDays != nil {
			days := *result.RotationRules.AutomaticallyAfterDays
			metadata.RotationInterval = fmt.Sprintf("%dd", days)
			
			if metadata.LastRotated != nil {
				nextRotation := metadata.LastRotated.AddDate(0, 0, int(days))
				metadata.NextRotation = &nextRotation
			}
		}
	}

	return metadata, nil
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}

func (aws *AWSSecretsManagerProvider) getKMSKeyId(result *secretsmanager.DescribeSecretOutput) string {
	if result.KmsKeyId != nil {
		return *result.KmsKeyId
	}
	return ""
}

func (aws *AWSSecretsManagerProvider) getReplicaRegions(result *secretsmanager.DescribeSecretOutput) []string {
	var regions []string
	for _, replica := range result.ReplicationStatus {
		if replica.Region != nil {
			regions = append(regions, *replica.Region)
		}
	}
	return regions
}

// Error checking utilities

func isNotFoundError(err error) bool {
	var resourceNotFound *types.ResourceNotFoundException
	return errors.As(err, &resourceNotFound)
}

func isAuthError(err error) bool {
	// Check for common auth-related errors by string matching
	errStr := err.Error()
	return strings.Contains(errStr, "AccessDenied") ||
		strings.Contains(errStr, "UnauthorizedOperation") ||
		strings.Contains(errStr, "InvalidUserID") ||
		strings.Contains(errStr, "Forbidden")
}

func isVersionId(version string) bool {
	// AWS version IDs are UUIDs
	return len(version) == 36 && strings.Count(version, "-") == 4
}

// Helper functions

func (aws *AWSSecretsManagerProvider) String(s string) *string {
	return &s
}

func (aws *AWSSecretsManagerProvider) Int32(i int32) *int32 {
	return &i
}