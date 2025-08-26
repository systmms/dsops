package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/pkg/provider"
	dserrors "github.com/systmms/dsops/internal/errors"
)

// AWSSTSProvider implements the Provider interface for AWS STS (Security Token Service)
type AWSSTSProvider struct {
	name   string
	client *sts.Client
	logger *logging.Logger
	config STSConfig
	cache  *stsCredentialCache
}

// STSConfig holds AWS STS-specific configuration
type STSConfig struct {
	Region         string
	Profile        string
	AssumeRole     string
	RoleSessionName string
	ExternalID     string
	Duration       int32 // in seconds
	SerialNumber   string // For MFA
	TokenCode      string // For MFA
	Policy         string // Session policy JSON
	Tags           map[string]string
}

// stsCredentialCache caches temporary credentials
type stsCredentialCache struct {
	credentials *sts.AssumeRoleOutput
	expiresAt   time.Time
}

// NewAWSSTSProvider creates a new AWS STS provider
func NewAWSSTSProvider(name string, configMap map[string]interface{}) (*AWSSTSProvider, error) {
	logger := logging.New(false, false)
	
	config := STSConfig{
		RoleSessionName: fmt.Sprintf("dsops-%d", time.Now().Unix()),
		Duration:       3600, // Default 1 hour
	}

	// Parse configuration
	if region, ok := configMap["region"].(string); ok {
		config.Region = region
	}
	if profile, ok := configMap["profile"].(string); ok {
		config.Profile = profile
	}
	if role, ok := configMap["assume_role"].(string); ok {
		config.AssumeRole = role
	}
	if sessionName, ok := configMap["role_session_name"].(string); ok {
		config.RoleSessionName = sessionName
	}
	if externalID, ok := configMap["external_id"].(string); ok {
		config.ExternalID = externalID
	}
	if duration, ok := configMap["duration"].(int); ok {
		config.Duration = int32(duration)
	}
	if serialNumber, ok := configMap["mfa_serial_number"].(string); ok {
		config.SerialNumber = serialNumber
	}
	if tokenCode, ok := configMap["mfa_token_code"].(string); ok {
		config.TokenCode = tokenCode
	}
	if policy, ok := configMap["session_policy"].(string); ok {
		config.Policy = policy
	}
	if tags, ok := configMap["tags"].(map[string]interface{}); ok {
		config.Tags = make(map[string]string)
		for k, v := range tags {
			if strVal, ok := v.(string); ok {
				config.Tags[k] = strVal
			}
		}
	}

	// Validate required configuration
	if config.AssumeRole == "" {
		return nil, dserrors.ConfigError{
			Field:      "assume_role",
			Message:    "assume_role is required for STS provider",
			Suggestion: "Provide the ARN of the role to assume",
		}
	}

	// Create AWS client
	client, err := createSTSClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create STS client: %w", err)
	}

	return &AWSSTSProvider{
		name:   name,
		client: client,
		logger: logger,
		config: config,
		cache:  &stsCredentialCache{},
	}, nil
}

// createSTSClient creates an AWS STS client with the given configuration
func createSTSClient(config STSConfig) (*sts.Client, error) {
	ctx := context.Background()
	
	// Build config options
	var configOpts []func(*awsconfig.LoadOptions) error

	if config.Region != "" {
		configOpts = append(configOpts, awsconfig.WithRegion(config.Region))
	}

	if config.Profile != "" {
		configOpts = append(configOpts, awsconfig.WithSharedConfigProfile(config.Profile))
	}

	// Load AWS configuration
	cfg, err := awsconfig.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return sts.NewFromConfig(cfg), nil
}

// Name returns the provider name
func (p *AWSSTSProvider) Name() string {
	return p.name
}

// Resolve fetches temporary credentials from STS
func (p *AWSSTSProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	// Check cache first
	if p.cache.credentials != nil && time.Now().Before(p.cache.expiresAt) {
		return p.getCredentialValue(ref.Key)
	}

	// Assume role to get new credentials
	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(p.config.AssumeRole),
		RoleSessionName: aws.String(p.config.RoleSessionName),
		DurationSeconds: aws.Int32(p.config.Duration),
	}

	if p.config.ExternalID != "" {
		input.ExternalId = aws.String(p.config.ExternalID)
	}

	if p.config.SerialNumber != "" && p.config.TokenCode != "" {
		input.SerialNumber = aws.String(p.config.SerialNumber)
		input.TokenCode = aws.String(p.config.TokenCode)
	}

	if p.config.Policy != "" {
		input.Policy = aws.String(p.config.Policy)
	}

	if len(p.config.Tags) > 0 {
		var tags []types.Tag
		for k, v := range p.config.Tags {
			tags = append(tags, types.Tag{
				Key:   aws.String(k),
				Value: aws.String(v),
			})
		}
		input.Tags = tags
	}

	p.logger.Debug("Assuming role: %s", logging.Secret(p.config.AssumeRole))

	result, err := p.client.AssumeRole(ctx, input)
	if err != nil {
		return provider.SecretValue{}, dserrors.UserError{
			Message:    "Failed to assume role",
			Details:    err.Error(),
			Suggestion: getSTSErrorSuggestion(err),
		}
	}

	// Cache the credentials
	p.cache.credentials = result
	p.cache.expiresAt = *result.Credentials.Expiration

	return p.getCredentialValue(ref.Key)
}

// getCredentialValue extracts the requested credential component
func (p *AWSSTSProvider) getCredentialValue(key string) (provider.SecretValue, error) {
	if p.cache.credentials == nil || p.cache.credentials.Credentials == nil {
		return provider.SecretValue{}, fmt.Errorf("no credentials available")
	}

	creds := p.cache.credentials.Credentials
	var value string

	switch key {
	case "access_key_id", "AccessKeyId", "AWS_ACCESS_KEY_ID":
		value = *creds.AccessKeyId
	case "secret_access_key", "SecretAccessKey", "AWS_SECRET_ACCESS_KEY":
		value = *creds.SecretAccessKey
	case "session_token", "SessionToken", "AWS_SESSION_TOKEN":
		value = *creds.SessionToken
	case "expiration":
		value = creds.Expiration.Format(time.RFC3339)
	case "assumed_role_arn":
		if p.cache.credentials.AssumedRoleUser != nil {
			value = *p.cache.credentials.AssumedRoleUser.Arn
		}
	case "credentials", "all":
		// Return all credentials as JSON
		credsMap := map[string]string{
			"AccessKeyId":     *creds.AccessKeyId,
			"SecretAccessKey": *creds.SecretAccessKey,
			"SessionToken":    *creds.SessionToken,
			"Expiration":      creds.Expiration.Format(time.RFC3339),
		}
		jsonData, err := json.Marshal(credsMap)
		if err != nil {
			return provider.SecretValue{}, fmt.Errorf("failed to marshal credentials: %w", err)
		}
		value = string(jsonData)
	default:
		return provider.SecretValue{}, dserrors.UserError{
			Message:    fmt.Sprintf("Unknown credential field: %s", key),
			Suggestion: "Use one of: access_key_id, secret_access_key, session_token, expiration, credentials",
		}
	}

	metadata := map[string]string{
		"source":     fmt.Sprintf("sts:%s", p.config.AssumeRole),
		"expires_at": creds.Expiration.Format(time.RFC3339),
	}

	return provider.SecretValue{
		Value:    value,
		Metadata: metadata,
	}, nil
}

// Describe returns metadata about the STS provider
func (p *AWSSTSProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	return provider.Metadata{
		Exists: true,
		Type:   "sts-credentials",
		Tags: map[string]string{
			"role_arn":     p.config.AssumeRole,
			"session_name": p.config.RoleSessionName,
		},
	}, nil
}

// Capabilities returns the provider's capabilities
func (p *AWSSTSProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: false,
		SupportsMetadata:   true,
		SupportsWatching:   false,
		SupportsBinary:     false,
		RequiresAuth:       true,
		AuthMethods:        []string{"iam", "profile", "mfa"},
	}
}

// Validate checks if the provider is properly configured and accessible
func (p *AWSSTSProvider) Validate(ctx context.Context) error {
	// Get caller identity to validate credentials
	input := &sts.GetCallerIdentityInput{}
	
	_, err := p.client.GetCallerIdentity(ctx, input)
	if err != nil {
		return dserrors.UserError{
			Message:    "Failed to validate AWS credentials",
			Details:    err.Error(),
			Suggestion: "Check AWS credentials and permissions to call sts:GetCallerIdentity",
		}
	}

	return nil
}

// getSTSErrorSuggestion provides helpful suggestions based on STS errors
func getSTSErrorSuggestion(err error) string {
	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "AccessDenied"):
		return "Check that you have permission to assume the role and the trust policy allows your principal"
	case strings.Contains(errStr, "InvalidParameterValue"):
		return "Check the role ARN format and external ID if provided"
	case strings.Contains(errStr, "TokenRefreshRequired"):
		return "MFA token has expired. Provide a new token code"
	case strings.Contains(errStr, "RegionDisabled"):
		return "The specified region is disabled for your account"
	default:
		return "Check AWS credentials, role ARN, and IAM permissions"
	}
}


// NewAWSSTSProviderFactory creates an AWS STS provider factory
func NewAWSSTSProviderFactory(name string, config map[string]interface{}) (provider.Provider, error) {
	return NewAWSSTSProvider(name, config)
}