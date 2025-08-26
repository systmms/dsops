package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/pkg/provider"
	dserrors "github.com/systmms/dsops/internal/errors"
)

// AzureUnifiedProvider provides intelligent routing to different Azure providers
type AzureUnifiedProvider struct {
	name           string
	logger         *logging.Logger
	providers      map[string]provider.Provider
	defaultService string
}

// UnifiedAzureConfig holds configuration for the unified Azure provider
type UnifiedAzureConfig struct {
	TenantID           string
	ClientID           string
	ClientSecret       string
	UseManagedIdentity bool
	UserAssignedID     string
	DefaultService     string // Default service if not specified in reference
	
	// Service-specific configs
	KeyVault map[string]interface{}
	Identity map[string]interface{}
	// Future: AppConfig, Storage, etc.
}

// NewAzureUnifiedProvider creates a new unified Azure provider
func NewAzureUnifiedProvider(name string, configMap map[string]interface{}) (*AzureUnifiedProvider, error) {
	logger := logging.New(false, false)
	
	config := UnifiedAzureConfig{
		DefaultService:     "keyvault", // Default to Key Vault
		UseManagedIdentity: true,       // Default to managed identity
		KeyVault:           make(map[string]interface{}),
		Identity:           make(map[string]interface{}),
	}

	// Parse common configuration
	if tenantID, ok := configMap["tenant_id"].(string); ok {
		config.TenantID = tenantID
	}
	if clientID, ok := configMap["client_id"].(string); ok {
		config.ClientID = clientID
	}
	if clientSecret, ok := configMap["client_secret"].(string); ok {
		config.ClientSecret = clientSecret
	}
	if useMI, ok := configMap["use_managed_identity"].(bool); ok {
		config.UseManagedIdentity = useMI
	}
	if userAssignedID, ok := configMap["user_assigned_identity_id"].(string); ok {
		config.UserAssignedID = userAssignedID
	}
	if defaultService, ok := configMap["default_service"].(string); ok {
		config.DefaultService = defaultService
	}

	// Parse service-specific configs
	if kv, ok := configMap["keyvault"].(map[string]interface{}); ok {
		config.KeyVault = kv
	}
	if identity, ok := configMap["identity"].(map[string]interface{}); ok {
		config.Identity = identity
	}

	// Create sub-providers
	providers := make(map[string]provider.Provider)
	
	// Create Key Vault provider if vault_url is configured
	kvConfig := mergeAzureConfigs(getAzureCommonConfig(config), config.KeyVault)
	if vaultURL, exists := kvConfig["vault_url"]; exists && vaultURL != "" {
		kvProvider, err := NewAzureKeyVaultProvider(name+"-kv", kvConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Key Vault provider: %w", err)
		}
		providers["keyvault"] = kvProvider
		providers["kv"] = kvProvider      // Alias
		providers["vault"] = kvProvider   // Alias
		providers["secrets"] = kvProvider // Alias
	}

	// Create Identity provider
	identityConfig := mergeAzureConfigs(getAzureCommonConfig(config), config.Identity)
	identityProvider, err := NewAzureIdentityProvider(name+"-identity", identityConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Identity provider: %w", err)
	}
	providers["identity"] = identityProvider
	providers["auth"] = identityProvider     // Alias
	providers["token"] = identityProvider    // Alias
	providers["managed"] = identityProvider  // Alias

	return &AzureUnifiedProvider{
		name:           name,
		logger:         logger,
		providers:      providers,
		defaultService: config.DefaultService,
	}, nil
}

// getAzureCommonConfig extracts common Azure configuration
func getAzureCommonConfig(config UnifiedAzureConfig) map[string]interface{} {
	common := make(map[string]interface{})
	if config.TenantID != "" {
		common["tenant_id"] = config.TenantID
	}
	if config.ClientID != "" {
		common["client_id"] = config.ClientID
	}
	if config.ClientSecret != "" {
		common["client_secret"] = config.ClientSecret
	}
	common["use_managed_identity"] = config.UseManagedIdentity
	if config.UserAssignedID != "" {
		common["user_assigned_identity_id"] = config.UserAssignedID
	}
	return common
}

// mergeAzureConfigs merges two configuration maps
func mergeAzureConfigs(base, override map[string]interface{}) map[string]interface{} {
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
func (p *AzureUnifiedProvider) Name() string {
	return p.name
}

// Resolve intelligently routes to the appropriate Azure provider
func (p *AzureUnifiedProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	service, key := p.parseAzureReference(ref.Key)
	
	// Get the appropriate provider
	subProvider, exists := p.providers[service]
	if !exists {
		return provider.SecretValue{}, dserrors.UserError{
			Message:    fmt.Sprintf("Unknown Azure service: %s", service),
			Suggestion: fmt.Sprintf("Available services: %s", p.getAvailableAzureServices()),
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

// parseAzureReference parses Azure references and routes to appropriate service
func (p *AzureUnifiedProvider) parseAzureReference(ref string) (service, key string) {
	// Check for explicit service prefix
	if strings.HasPrefix(ref, "kv:") || strings.HasPrefix(ref, "keyvault:") {
		parts := strings.SplitN(ref, ":", 2)
		return "keyvault", parts[1]
	}
	
	if strings.HasPrefix(ref, "vault:") || strings.HasPrefix(ref, "secrets:") {
		parts := strings.SplitN(ref, ":", 2)
		return "keyvault", parts[1]
	}
	
	if strings.HasPrefix(ref, "identity:") || strings.HasPrefix(ref, "auth:") {
		parts := strings.SplitN(ref, ":", 2)
		return "identity", parts[1]
	}
	
	if strings.HasPrefix(ref, "token:") || strings.HasPrefix(ref, "managed:") {
		parts := strings.SplitN(ref, ":", 2)
		return "identity", parts[1]
	}
	
	// Auto-detect based on format
	lowerRef := strings.ToLower(ref)
	
	// Token/authentication patterns suggest Identity
	if strings.Contains(lowerRef, "scope") || strings.Contains(lowerRef, ".default") ||
	   lowerRef == "access_token" || lowerRef == "token" || lowerRef == "expires_at" {
		return "identity", ref
	}
	
	// HTTPS URL patterns for Key Vault
	if strings.HasPrefix(ref, "https://") && strings.Contains(ref, ".vault.azure.net/") {
		return "keyvault", ref
	}
	
	// Secret name patterns (typically Key Vault)
	if strings.Contains(ref, "/") || strings.Contains(ref, "#") {
		// Version or JSON extraction patterns
		return "keyvault", ref
	}
	
	// Default to configured service
	return p.defaultService, ref
}

// getAvailableAzureServices returns comma-separated list of available services
func (p *AzureUnifiedProvider) getAvailableAzureServices() string {
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
func (p *AzureUnifiedProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	service, key := p.parseAzureReference(ref.Key)
	
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
func (p *AzureUnifiedProvider) Capabilities() provider.Capabilities {
	// Return union of all sub-provider capabilities
	caps := provider.Capabilities{
		SupportsVersioning: true,  // Key Vault supports versioning
		SupportsMetadata:   true,  // All Azure providers support metadata
		SupportsWatching:   false, // None support watching yet
		SupportsBinary:     true,  // Key Vault supports binary
		RequiresAuth:       true,  // All require authentication
		AuthMethods:        []string{"managed_identity", "service_principal", "azure_cli", "default_credential"},
	}
	
	return caps
}

// Validate checks if all sub-providers are properly configured
func (p *AzureUnifiedProvider) Validate(ctx context.Context) error {
	var errors []string
	
	for service, subProvider := range p.providers {
		if err := subProvider.Validate(ctx); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", service, err))
		}
	}
	
	if len(errors) > 0 {
		return dserrors.UserError{
			Message:    "One or more Azure services failed validation",
			Details:    strings.Join(errors, "\n"),
			Suggestion: "Check Azure credentials and permissions for each service",
		}
	}
	
	return nil
}

// NewAzureUnifiedProviderFactory creates an Azure unified provider factory
func NewAzureUnifiedProviderFactory(name string, config map[string]interface{}) (provider.Provider, error) {
	return NewAzureUnifiedProvider(name, config)
}