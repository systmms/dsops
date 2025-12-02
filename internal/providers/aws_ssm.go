package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/pkg/provider"
	dserrors "github.com/systmms/dsops/internal/errors"
)

// SSMClientAPI defines the interface for AWS SSM Parameter Store operations
// This allows for mocking in tests
type SSMClientAPI interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	DescribeParameters(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error)
}

// AWSSSMProvider implements the Provider interface for AWS Systems Manager Parameter Store
type AWSSSMProvider struct {
	name   string
	client SSMClientAPI
	logger *logging.Logger
	config SSMConfig
}

// SSMConfig holds AWS SSM-specific configuration
type SSMConfig struct {
	Region          string
	Profile         string
	AssumeRole      string
	WithDecryption  bool
	ParameterPrefix string
}

// SSMProviderOption is a functional option for configuring SSM providers
type SSMProviderOption func(*AWSSSMProvider)

// WithSSMClient sets a custom SSM client (for testing)
func WithSSMClient(client SSMClientAPI) SSMProviderOption {
	return func(p *AWSSSMProvider) {
		p.client = client
	}
}

// NewAWSSSMProvider creates a new AWS SSM Parameter Store provider
func NewAWSSSMProvider(name string, configMap map[string]interface{}, opts ...SSMProviderOption) (*AWSSSMProvider, error) {
	logger := logging.New(false, false)

	config := SSMConfig{
		WithDecryption: true, // Default to decrypting SecureString parameters
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
	if decrypt, ok := configMap["with_decryption"].(bool); ok {
		config.WithDecryption = decrypt
	}
	if prefix, ok := configMap["parameter_prefix"].(string); ok {
		config.ParameterPrefix = prefix
	}

	p := &AWSSSMProvider{
		name:   name,
		logger: logger,
		config: config,
	}

	// Apply options (allows mock client injection)
	for _, opt := range opts {
		opt(p)
	}

	// If no client was provided via options, create real client
	if p.client == nil {
		client, err := createSSMClient(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create SSM client: %w", err)
		}
		p.client = client
	}

	return p, nil
}

// createSSMClient creates an AWS SSM client with the given configuration
func createSSMClient(config SSMConfig) (*ssm.Client, error) {
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

	// TODO: Add assume role support if config.AssumeRole is set

	return ssm.NewFromConfig(cfg), nil
}

// Name returns the provider name
func (p *AWSSSMProvider) Name() string {
	return p.name
}

// Resolve fetches a parameter from SSM Parameter Store
func (p *AWSSSMProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	// Apply prefix if configured
	parameterName := ref.Key
	if p.config.ParameterPrefix != "" {
		parameterName = p.config.ParameterPrefix + parameterName
	}

	p.logger.Debug("Fetching parameter from SSM: %s", logging.Secret(parameterName))

	// Get parameter
	input := &ssm.GetParameterInput{
		Name:           aws.String(parameterName),
		WithDecryption: aws.Bool(p.config.WithDecryption),
	}

	result, err := p.client.GetParameter(ctx, input)
	if err != nil {
		if isParameterNotFoundError(err) {
			return provider.SecretValue{}, dserrors.UserError{
				Message:    fmt.Sprintf("Parameter not found: %s", parameterName),
				Suggestion: "Check that the parameter exists and you have ssm:GetParameter permission",
				Details:    err.Error(),
			}
		}
		return provider.SecretValue{}, dserrors.UserError{
			Message:    "Failed to get parameter from SSM",
			Details:    err.Error(),
			Suggestion: getSSMErrorSuggestion(err),
		}
	}

	if result.Parameter == nil || result.Parameter.Value == nil {
		return provider.SecretValue{}, fmt.Errorf("parameter has no value")
	}

	// Build metadata
	metadata := map[string]string{
		"source": fmt.Sprintf("ssm:%s", parameterName),
		"type":   string(result.Parameter.Type),
	}

	if result.Parameter.Version != 0 {
		metadata["version"] = fmt.Sprintf("%d", result.Parameter.Version)
	}

	if result.Parameter.LastModifiedDate != nil {
		metadata["last_modified"] = result.Parameter.LastModifiedDate.String()
	}

	return provider.SecretValue{
		Value:    *result.Parameter.Value,
		Metadata: metadata,
	}, nil
}

// Describe returns metadata about a parameter without fetching its value
func (p *AWSSSMProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	// Apply prefix if configured
	parameterName := ref.Key
	if p.config.ParameterPrefix != "" {
		parameterName = p.config.ParameterPrefix + parameterName
	}

	// Describe parameter
	input := &ssm.DescribeParametersInput{
		ParameterFilters: []types.ParameterStringFilter{
			{
				Key:    aws.String("Name"),
				Values: []string{parameterName},
			},
		},
	}

	result, err := p.client.DescribeParameters(ctx, input)
	if err != nil {
		return provider.Metadata{}, fmt.Errorf("failed to describe parameter: %w", err)
	}

	if len(result.Parameters) == 0 {
		return provider.Metadata{
			Exists: false,
		}, nil
	}

	param := result.Parameters[0]
	metadata := provider.Metadata{
		Exists: true,
		Type:   string(param.Type),
		Tags: map[string]string{
			"source": fmt.Sprintf("ssm:%s", parameterName),
			"tier":   string(param.Tier),
		},
	}

	if param.Version != 0 {
		metadata.Version = fmt.Sprintf("%d", param.Version)
	}

	if param.LastModifiedDate != nil {
		metadata.UpdatedAt = *param.LastModifiedDate
	}

	return metadata, nil
}

// Capabilities returns the provider's capabilities
func (p *AWSSSMProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: true,
		SupportsMetadata:   true,
		SupportsWatching:   false,
		SupportsBinary:     false, // Parameter Store is text-only
		RequiresAuth:       true,
		AuthMethods:        []string{"iam", "profile", "role"},
	}
}

// Validate checks if the provider is properly configured and accessible
func (p *AWSSSMProvider) Validate(ctx context.Context) error {
	// Test by describing parameters (minimal permissions needed)
	input := &ssm.DescribeParametersInput{
		MaxResults: aws.Int32(1),
	}

	_, err := p.client.DescribeParameters(ctx, input)
	if err != nil {
		return dserrors.UserError{
			Message:    "Failed to connect to AWS SSM Parameter Store",
			Details:    err.Error(),
			Suggestion: getSSMErrorSuggestion(err),
		}
	}

	return nil
}

// isParameterNotFoundError checks if the error is a parameter not found error
func isParameterNotFoundError(err error) bool {
	return strings.Contains(err.Error(), "ParameterNotFound")
}

// getSSMErrorSuggestion provides helpful suggestions based on SSM errors
func getSSMErrorSuggestion(err error) string {
	errStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errStr, "accessdenied"):
		return "Check IAM permissions: ssm:GetParameter, ssm:DescribeParameters, and kms:Decrypt (for SecureString)"
	case strings.Contains(errStr, "parameternotfound"):
		return "Verify the parameter name and path. SSM parameters are case-sensitive"
	case strings.Contains(errStr, "invalidkeyid"):
		return "The KMS key for this SecureString parameter may not exist or you lack kms:Decrypt permission"
	case strings.Contains(errStr, "throttl"):
		return "Request was throttled. Consider adding exponential backoff or reducing request rate"
	case strings.Contains(errStr, "region"):
		return "Check that you're using the correct AWS region where the parameter is stored"
	default:
		return "Check AWS credentials, region, and IAM permissions for SSM Parameter Store"
	}
}

// NewAWSSSMProviderFactory creates an AWS SSM provider factory
func NewAWSSSMProviderFactory(name string, config map[string]interface{}) (provider.Provider, error) {
	return NewAWSSSMProvider(name, config)
}