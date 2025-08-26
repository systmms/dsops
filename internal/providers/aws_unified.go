package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/pkg/provider"
	dserrors "github.com/systmms/dsops/internal/errors"
)

// AWSUnifiedProvider provides intelligent routing to different AWS secret providers
// based on the secret reference format
type AWSUnifiedProvider struct {
	name           string
	logger         *logging.Logger
	providers      map[string]provider.Provider
	defaultService string
}

// UnifiedAWSConfig holds configuration for the unified AWS provider
type UnifiedAWSConfig struct {
	Region         string
	Profile        string
	AssumeRole     string
	DefaultService string // Default service if not specified in reference
	
	// Service-specific configs
	SecretsManager map[string]interface{}
	SSM            map[string]interface{}
	STS            map[string]interface{}
	SSO            map[string]interface{}
}

// NewAWSUnifiedProvider creates a new unified AWS provider
func NewAWSUnifiedProvider(name string, configMap map[string]interface{}) (*AWSUnifiedProvider, error) {
	logger := logging.New(false, false)
	
	config := UnifiedAWSConfig{
		DefaultService: "secretsmanager", // Default to Secrets Manager
		SecretsManager: make(map[string]interface{}),
		SSM:            make(map[string]interface{}),
		STS:            make(map[string]interface{}),
		SSO:            make(map[string]interface{}),
	}

	// Parse common configuration
	if region, ok := configMap["region"].(string); ok {
		config.Region = region
	}
	if profile, ok := configMap["profile"].(string); ok {
		config.Profile = profile
	}
	if role, ok := configMap["assume_role"].(string); ok {
		config.AssumeRole = role
	}
	if defaultService, ok := configMap["default_service"].(string); ok {
		config.DefaultService = defaultService
	}

	// Parse service-specific configs
	if sm, ok := configMap["secretsmanager"].(map[string]interface{}); ok {
		config.SecretsManager = sm
	}
	if ssm, ok := configMap["ssm"].(map[string]interface{}); ok {
		config.SSM = ssm
	}
	if sts, ok := configMap["sts"].(map[string]interface{}); ok {
		config.STS = sts
	}
	if sso, ok := configMap["sso"].(map[string]interface{}); ok {
		config.SSO = sso
	}

	// Create sub-providers
	providers := make(map[string]provider.Provider)
	
	// Create Secrets Manager provider
	smConfig := mergeConfigs(getCommonConfig(config), config.SecretsManager)
	smProvider, err := NewAWSSecretsManagerProvider(name+"-sm", smConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Secrets Manager provider: %w", err)
	}
	providers["secretsmanager"] = smProvider
	providers["sm"] = smProvider // Alias

	// Create SSM provider
	ssmConfig := mergeConfigs(getCommonConfig(config), config.SSM)
	ssmProvider, err := NewAWSSSMProvider(name+"-ssm", ssmConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSM provider: %w", err)
	}
	providers["ssm"] = ssmProvider
	providers["parameter"] = ssmProvider // Alias

	// Create STS provider if configured
	if len(config.STS) > 0 || config.AssumeRole != "" {
		stsConfig := mergeConfigs(getCommonConfig(config), config.STS)
		if config.AssumeRole != "" && stsConfig["assume_role"] == nil {
			stsConfig["assume_role"] = config.AssumeRole
		}
		stsProvider, err := NewAWSSTSProvider(name+"-sts", stsConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create STS provider: %w", err)
		}
		providers["sts"] = stsProvider
		providers["credentials"] = stsProvider // Alias
	}

	// Create SSO provider if configured
	if len(config.SSO) > 0 {
		ssoConfig := mergeConfigs(getCommonConfig(config), config.SSO)
		ssoProvider, err := NewAWSSSOProvider(name+"-sso", ssoConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create SSO provider: %w", err)
		}
		providers["sso"] = ssoProvider
	}

	return &AWSUnifiedProvider{
		name:           name,
		logger:         logger,
		providers:      providers,
		defaultService: config.DefaultService,
	}, nil
}

// getCommonConfig extracts common AWS configuration
func getCommonConfig(config UnifiedAWSConfig) map[string]interface{} {
	common := make(map[string]interface{})
	if config.Region != "" {
		common["region"] = config.Region
	}
	if config.Profile != "" {
		common["profile"] = config.Profile
	}
	return common
}

// mergeConfigs merges two configuration maps
func mergeConfigs(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Copy base config
	for k, v := range base {
		result[k] = v
	}
	
	// Override with specific config
	for k, v := range override {
		result[k] = v
	}
	
	return result
}

// Name returns the provider name
func (p *AWSUnifiedProvider) Name() string {
	return p.name
}

// Resolve intelligently routes to the appropriate AWS provider
func (p *AWSUnifiedProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	service, key := p.parseReference(ref.Key)
	
	// Get the appropriate provider
	subProvider, exists := p.providers[service]
	if !exists {
		return provider.SecretValue{}, dserrors.UserError{
			Message:    fmt.Sprintf("Unknown AWS service: %s", service),
			Suggestion: fmt.Sprintf("Available services: %s", p.getAvailableServices()),
			Details:    fmt.Sprintf("Reference: %s", ref.Key),
		}
	}

	// Create new reference with parsed key
	subRef := provider.Reference{
		Provider: ref.Provider,
		Key:      key,
	}

	p.logger.Debug("Routing to %s provider with key: %s", service, logging.Secret(key))
	
	// Delegate to sub-provider
	return subProvider.Resolve(ctx, subRef)
}

// parseReference parses AWS references and routes to appropriate service
func (p *AWSUnifiedProvider) parseReference(ref string) (service, key string) {
	// Check for explicit service prefix
	if strings.HasPrefix(ref, "sm:") || strings.HasPrefix(ref, "secretsmanager:") {
		parts := strings.SplitN(ref, ":", 2)
		return "secretsmanager", parts[1]
	}
	
	if strings.HasPrefix(ref, "ssm:") || strings.HasPrefix(ref, "parameter:") {
		parts := strings.SplitN(ref, ":", 2)
		return "ssm", parts[1]
	}
	
	if strings.HasPrefix(ref, "sts:") || strings.HasPrefix(ref, "credentials:") {
		parts := strings.SplitN(ref, ":", 2)
		return "sts", parts[1]
	}
	
	if strings.HasPrefix(ref, "sso:") {
		parts := strings.SplitN(ref, ":", 2)
		return "sso", parts[1]
	}
	
	// Auto-detect based on format
	if strings.HasPrefix(ref, "/") {
		// Path format suggests SSM Parameter Store
		return "ssm", ref
	}
	
	if strings.Contains(ref, "arn:aws:") {
		// ARN format - could be various services
		if strings.Contains(ref, ":secretsmanager:") {
			return "secretsmanager", ref
		}
		if strings.Contains(ref, ":ssm:") {
			return "ssm", ref
		}
		if strings.Contains(ref, ":iam:") && strings.Contains(ref, ":role/") {
			return "sts", ref
		}
	}
	
	// Credential-specific keys suggest STS/SSO
	lowerRef := strings.ToLower(ref)
	if lowerRef == "access_key_id" || lowerRef == "secret_access_key" || 
	   lowerRef == "session_token" || lowerRef == "credentials" {
		if _, exists := p.providers["sts"]; exists {
			return "sts", ref
		}
		if _, exists := p.providers["sso"]; exists {
			return "sso", ref
		}
	}
	
	// Default to configured service
	return p.defaultService, ref
}

// getAvailableServices returns comma-separated list of available services
func (p *AWSUnifiedProvider) getAvailableServices() string {
	services := make([]string, 0, len(p.providers))
	for service := range p.providers {
		services = append(services, service)
	}
	return strings.Join(services, ", ")
}

// Describe returns metadata about the secret
func (p *AWSUnifiedProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	service, key := p.parseReference(ref.Key)
	
	subProvider, exists := p.providers[service]
	if !exists {
		return provider.Metadata{
			Exists: false,
		}, nil
	}

	subRef := provider.Reference{
		Provider: ref.Provider,
		Key:      key,
	}

	return subProvider.Describe(ctx, subRef)
}

// Capabilities returns the unified provider's capabilities
func (p *AWSUnifiedProvider) Capabilities() provider.Capabilities {
	// Return union of all sub-provider capabilities
	caps := provider.Capabilities{
		SupportsVersioning: true,  // Secrets Manager and SSM support versioning
		SupportsMetadata:   true,  // All AWS providers support metadata
		SupportsWatching:   false, // None support watching
		SupportsBinary:     true,  // Secrets Manager supports binary
		RequiresAuth:       true,  // All require authentication
		AuthMethods:        []string{"iam", "profile", "role", "sso", "mfa"},
	}
	
	return caps
}

// Validate checks if all sub-providers are properly configured
func (p *AWSUnifiedProvider) Validate(ctx context.Context) error {
	var errors []string
	
	for service, subProvider := range p.providers {
		if err := subProvider.Validate(ctx); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", service, err))
		}
	}
	
	if len(errors) > 0 {
		return dserrors.UserError{
			Message:    "One or more AWS services failed validation",
			Details:    strings.Join(errors, "\n"),
			Suggestion: "Check AWS credentials and permissions for each service",
		}
	}
	
	return nil
}

// NewAWSUnifiedProviderFactory creates an AWS unified provider factory
func NewAWSUnifiedProviderFactory(name string, config map[string]interface{}) (provider.Provider, error) {
	return NewAWSUnifiedProvider(name, config)
}