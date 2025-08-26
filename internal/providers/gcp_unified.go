package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/pkg/provider"
	dserrors "github.com/systmms/dsops/internal/errors"
)

// GCPUnifiedProvider provides intelligent routing to different GCP secret providers
type GCPUnifiedProvider struct {
	name           string
	logger         *logging.Logger
	providers      map[string]provider.Provider
	defaultService string
}

// UnifiedGCPConfig holds configuration for the unified GCP provider
type UnifiedGCPConfig struct {
	ProjectID              string
	ServiceAccountKeyPath  string
	ImpersonateAccount     string
	DefaultService         string // Default service if not specified in reference
	
	// Service-specific configs
	SecretManager map[string]interface{}
	// Future: KMS, Config, etc.
}

// NewGCPUnifiedProvider creates a new unified GCP provider
func NewGCPUnifiedProvider(name string, configMap map[string]interface{}) (*GCPUnifiedProvider, error) {
	logger := logging.New(false, false)
	
	config := UnifiedGCPConfig{
		DefaultService: "secretmanager", // Default to Secret Manager
		SecretManager:  make(map[string]interface{}),
	}

	// Parse common configuration
	if projectID, ok := configMap["project_id"].(string); ok {
		config.ProjectID = projectID
	}
	if keyPath, ok := configMap["service_account_key_path"].(string); ok {
		config.ServiceAccountKeyPath = keyPath
	}
	if impersonate, ok := configMap["impersonate_service_account"].(string); ok {
		config.ImpersonateAccount = impersonate
	}
	if defaultService, ok := configMap["default_service"].(string); ok {
		config.DefaultService = defaultService
	}

	// Parse service-specific configs
	if sm, ok := configMap["secretmanager"].(map[string]interface{}); ok {
		config.SecretManager = sm
	}

	// Create sub-providers
	providers := make(map[string]provider.Provider)
	
	// Create Secret Manager provider
	smConfig := mergeGCPConfigs(getGCPCommonConfig(config), config.SecretManager)
	smProvider, err := NewGCPSecretManagerProvider(name+"-sm", smConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Secret Manager provider: %w", err)
	}
	providers["secretmanager"] = smProvider
	providers["sm"] = smProvider     // Alias
	providers["secrets"] = smProvider // Alias

	return &GCPUnifiedProvider{
		name:           name,
		logger:         logger,
		providers:      providers,
		defaultService: config.DefaultService,
	}, nil
}

// getGCPCommonConfig extracts common GCP configuration
func getGCPCommonConfig(config UnifiedGCPConfig) map[string]interface{} {
	common := make(map[string]interface{})
	if config.ProjectID != "" {
		common["project_id"] = config.ProjectID
	}
	if config.ServiceAccountKeyPath != "" {
		common["service_account_key_path"] = config.ServiceAccountKeyPath
	}
	if config.ImpersonateAccount != "" {
		common["impersonate_service_account"] = config.ImpersonateAccount
	}
	return common
}

// mergeGCPConfigs merges two configuration maps
func mergeGCPConfigs(base, override map[string]interface{}) map[string]interface{} {
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
func (p *GCPUnifiedProvider) Name() string {
	return p.name
}

// Resolve intelligently routes to the appropriate GCP provider
func (p *GCPUnifiedProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	service, key := p.parseGCPReference(ref.Key)
	
	// Get the appropriate provider
	subProvider, exists := p.providers[service]
	if !exists {
		return provider.SecretValue{}, dserrors.UserError{
			Message:    fmt.Sprintf("Unknown GCP service: %s", service),
			Suggestion: fmt.Sprintf("Available services: %s", p.getAvailableGCPServices()),
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

// parseGCPReference parses GCP references and routes to appropriate service
func (p *GCPUnifiedProvider) parseGCPReference(ref string) (service, key string) {
	// Check for explicit service prefix
	if strings.HasPrefix(ref, "sm:") || strings.HasPrefix(ref, "secretmanager:") {
		parts := strings.SplitN(ref, ":", 2)
		return "secretmanager", parts[1]
	}
	
	if strings.HasPrefix(ref, "secrets:") {
		parts := strings.SplitN(ref, ":", 2)
		return "secretmanager", parts[1]
	}
	
	// Auto-detect based on format
	if strings.HasPrefix(ref, "projects/") {
		// Full GCP resource name format
		if strings.Contains(ref, "/secrets/") {
			return "secretmanager", ref
		}
		// Future: Could detect other services like KMS, Config, etc.
	}
	
	// Check for version specification patterns (secret:version or secret@version)
	if strings.Contains(ref, ":") && !strings.HasPrefix(ref, "projects/") {
		// Likely Secret Manager with version
		return "secretmanager", ref
	}
	
	if strings.Contains(ref, "@") {
		// Likely Secret Manager with version
		return "secretmanager", ref
	}
	
	// Check for JSON path extraction
	if strings.Contains(ref, "#") {
		// Likely Secret Manager with JSON extraction
		return "secretmanager", ref
	}
	
	// Default to configured service
	return p.defaultService, ref
}

// getAvailableGCPServices returns comma-separated list of available services
func (p *GCPUnifiedProvider) getAvailableGCPServices() string {
	services := make([]string, 0, len(p.providers))
	seen := make(map[string]bool)
	
	for service := range p.providers {
		if !seen[service] {
			services = append(services, service)
			seen[service] = true
		}
	}
	return strings.Join(services, ", ")
}

// Describe returns metadata about the secret
func (p *GCPUnifiedProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	service, key := p.parseGCPReference(ref.Key)
	
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
func (p *GCPUnifiedProvider) Capabilities() provider.Capabilities {
	// Return union of all sub-provider capabilities
	caps := provider.Capabilities{
		SupportsVersioning: true,  // Secret Manager supports versioning
		SupportsMetadata:   true,  // All GCP providers support metadata
		SupportsWatching:   false, // None support watching yet
		SupportsBinary:     true,  // Secret Manager supports binary
		RequiresAuth:       true,  // All require authentication
		AuthMethods:        []string{"service_account", "application_default", "impersonation"},
	}
	
	return caps
}

// Validate checks if all sub-providers are properly configured
func (p *GCPUnifiedProvider) Validate(ctx context.Context) error {
	var errors []string
	
	for service, subProvider := range p.providers {
		if err := subProvider.Validate(ctx); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", service, err))
		}
	}
	
	if len(errors) > 0 {
		return dserrors.UserError{
			Message:    "One or more GCP services failed validation",
			Details:    strings.Join(errors, "\n"),
			Suggestion: "Check GCP credentials and permissions for each service",
		}
	}
	
	return nil
}

// NewGCPUnifiedProviderFactory creates a GCP unified provider factory
func NewGCPUnifiedProviderFactory(name string, config map[string]interface{}) (provider.Provider, error) {
	return NewGCPUnifiedProvider(name, config)
}