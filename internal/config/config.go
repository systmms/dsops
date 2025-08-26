package config

import (
	"fmt"
	"os"
	"strings"

	dserrors "github.com/systmms/dsops/internal/errors"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/internal/policy"
	"github.com/systmms/dsops/pkg/secretstore"
	"github.com/systmms/dsops/pkg/service"
	"gopkg.in/yaml.v3"
)

// Config holds the runtime configuration
type Config struct {
	Path           string
	Logger         *logging.Logger
	NonInteractive bool
	Definition     *Definition // New format with separated secret stores and services
}

// Definition represents the dsops.yaml structure with separated secret stores and services
type Definition struct {
	Version      int                      `yaml:"version"`
	SecretStores map[string]SecretStoreConfig `yaml:"secretStores,omitempty"`
	Services     map[string]ServiceConfig     `yaml:"services,omitempty"`
	Providers    map[string]ProviderConfig    `yaml:"providers,omitempty"` // Legacy compatibility
	Transforms   map[string][]string          `yaml:"transforms"`
	Envs         map[string]Environment       `yaml:"envs"`
	Templates    []Template                   `yaml:"templates"`
	Policies     *policy.PolicyConfig         `yaml:"policies,omitempty"`
}

// SecretStoreConfig holds secret store-specific configuration
type SecretStoreConfig struct {
	Type      string                 `yaml:"type"`
	TimeoutMs int                    `yaml:"timeout_ms,omitempty"`
	Config    map[string]interface{} `yaml:",inline"`
}

// ServiceConfig holds service-specific configuration for rotation targets
type ServiceConfig struct {
	Type      string                 `yaml:"type"`
	TimeoutMs int                    `yaml:"timeout_ms,omitempty"`
	Config    map[string]interface{} `yaml:",inline"`
}

// ProviderConfig holds provider-specific configuration (legacy compatibility)
type ProviderConfig struct {
	Type      string                 `yaml:"type"`
	TimeoutMs int                    `yaml:"timeout_ms,omitempty"` // Timeout in milliseconds (default: 30000)
	Config    map[string]interface{} `yaml:",inline"`
}

// Environment represents a named environment configuration
type Environment map[string]Variable

// Variable represents a single environment variable configuration with new reference types
type Variable struct {
	From      *Reference        `yaml:"from"`
	Literal   string            `yaml:"literal"`
	Transform string            `yaml:"transform"`
	Optional  bool              `yaml:"optional"`
	Metadata  map[string]string `yaml:"metadata,omitempty"`
}

// Reference represents either a legacy provider reference or a new URI reference
type Reference struct {
	// New URI format (primary)
	Store   string `yaml:"store,omitempty"`   // store:// reference
	Service string `yaml:"service,omitempty"` // svc:// reference

	// Legacy format (provider + key) - for backward compatibility
	Provider string `yaml:"provider,omitempty"`
	Key      string `yaml:"key,omitempty"`
	Version  string `yaml:"version,omitempty"`
}

// ProviderRef references a provider and key (legacy compatibility)
type ProviderRef struct {
	Provider string `yaml:"provider"`
	Key      string `yaml:"key"`
	Version  string `yaml:"version,omitempty"`
}

// Template represents an output template configuration
type Template struct {
	Name         string `yaml:"name"`
	Format       string `yaml:"format"`
	Env          string `yaml:"env"`
	Out          string `yaml:"out"`
	TemplatePath string `yaml:"template_path,omitempty"`
}

// Load reads and parses the dsops.yaml file
func (c *Config) Load() error {
	data, err := os.ReadFile(c.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return dserrors.ConfigError{
				Field:      "path",
				Value:      c.Path,
				Message:    "configuration file not found",
				Suggestion: "Run 'dsops init' to create a new configuration file",
			}
		}
		return dserrors.UserError{
			Message:    "Failed to read configuration file",
			Details:    err.Error(),
			Suggestion: "Check file permissions and path",
			Err:        err,
		}
	}

	// Parse configuration
	var def Definition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return dserrors.ConfigError{
			Message:    "invalid YAML syntax in configuration file",
			Suggestion: "Check for indentation errors, missing quotes, or invalid characters. Use a YAML validator",
		}
	}

	// Validate version
	if def.Version != 0 {
		return dserrors.ConfigError{
			Field:      "version",
			Value:      def.Version,
			Message:    "unsupported configuration version",
			Suggestion: "Set 'version: 0' at the top of your dsops.yaml file",
		}
	}

	c.Definition = &def
	return nil
}

// GetEnvironment returns the configuration for a specific environment
func (c *Config) GetEnvironment(name string) (Environment, error) {
	if c.Definition == nil {
		return nil, dserrors.UserError{
			Message:    "Configuration not loaded",
			Suggestion: "This is an internal error. Please report it",
		}
	}

	env, ok := c.Definition.Envs[name]
	if !ok {
		// Build a list of available environments
		var available []string
		for envName := range c.Definition.Envs {
			available = append(available, envName)
		}
		
		suggestion := "Check your dsops.yaml for available environments"
		if len(available) > 0 {
			suggestion = fmt.Sprintf("Available environments: %s", strings.Join(available, ", "))
		}
		
		return nil, dserrors.ConfigError{
			Field:      "environment",
			Value:      name,
			Message:    "environment not found",
			Suggestion: suggestion,
		}
	}

	return env, nil
}

// GetProvider returns the configuration for a provider (works with both secret stores and services)
func (c *Config) GetProvider(name string) (ProviderConfig, error) {
	if c.Definition == nil {
		return ProviderConfig{}, dserrors.UserError{
			Message:    "Configuration not loaded",
			Suggestion: "This is an internal error. Please report it",
		}
	}

	// Check secret stores first
	if store, ok := c.Definition.SecretStores[name]; ok {
		return ProviderConfig(store), nil
	}

	// Check services
	if service, ok := c.Definition.Services[name]; ok {
		return ProviderConfig(service), nil
	}

	// Build a list of available providers
	var available []string
	for storeName := range c.Definition.SecretStores {
		available = append(available, storeName)
	}
	for serviceName := range c.Definition.Services {
		available = append(available, serviceName)
	}
	
	suggestion := "Add the provider to the 'secretStores:' or 'services:' section of your dsops.yaml"
	if len(available) > 0 {
		suggestion = fmt.Sprintf("Available providers: %s. %s", strings.Join(available, ", "), suggestion)
	}
	
	return ProviderConfig{}, dserrors.ConfigError{
		Field:      "provider",
		Value:      name,
		Message:    "provider not found in configuration",
		Suggestion: suggestion,
	}
}

// GetProviderTimeout returns the timeout for a provider in milliseconds
func (p ProviderConfig) GetProviderTimeout() int {
	if p.TimeoutMs <= 0 {
		return 30000 // Default 30 seconds
	}
	return p.TimeoutMs
}

// GetPolicyEnforcer returns a policy enforcer for the configuration
func (c *Config) GetPolicyEnforcer() *policy.PolicyEnforcer {
	if c.Definition == nil || c.Definition.Policies == nil {
		return policy.NewPolicyEnforcer(nil) // No restrictions
	}
	return policy.NewPolicyEnforcer(c.Definition.Policies)
}

// HasPolicies returns true if policies are configured
func (c *Config) HasPolicies() bool {
	return c.Definition != nil && c.Definition.Policies != nil
}

// GetSecretStore returns the configuration for a specific secret store
func (c *Config) GetSecretStore(name string) (SecretStoreConfig, error) {
	if c.Definition == nil {
		return SecretStoreConfig{}, dserrors.UserError{
			Message:    "Configuration not loaded",
			Suggestion: "This is an internal error. Please report it",
		}
	}

	if store, ok := c.Definition.SecretStores[name]; ok {
		return store, nil
	}

	return SecretStoreConfig{}, fmt.Errorf("secret store %s not found", name)
}

// GetService returns the configuration for a specific service
func (c *Config) GetService(name string) (ServiceConfig, error) {
	if c.Definition == nil {
		return ServiceConfig{}, dserrors.UserError{
			Message:    "Configuration not loaded",
			Suggestion: "This is an internal error. Please report it",
		}
	}

	if service, ok := c.Definition.Services[name]; ok {
		return service, nil
	}

	return ServiceConfig{}, fmt.Errorf("service %s not found", name)
}

// Reference methods

// IsLegacyFormat returns true if this reference uses the old provider+key format
func (r *Reference) IsLegacyFormat() bool {
	return r.Provider != "" || r.Key != ""
}

// IsStoreReference returns true if this references a secret store
func (r *Reference) IsStoreReference() bool {
	return r.Store != ""
}

// IsServiceReference returns true if this references a service
func (r *Reference) IsServiceReference() bool {
	return r.Service != ""
}

// ToSecretRef converts a Reference to a SecretRef (if it's a store reference)
func (r *Reference) ToSecretRef() (secretstore.SecretRef, error) {
	if r.IsStoreReference() {
		return secretstore.ParseSecretRef(r.Store)
	}
	
	if r.IsLegacyFormat() {
		// Convert legacy format to new SecretRef
		return secretstore.SecretRef{
			Store:   r.Provider,
			Path:    r.Key,
			Version: r.Version,
			Options: map[string]string{},
		}, nil
	}

	return secretstore.SecretRef{}, fmt.Errorf("reference is not a secret store reference")
}

// ToServiceRef converts a Reference to a ServiceRef (if it's a service reference)
func (r *Reference) ToServiceRef() (service.ServiceRef, error) {
	if r.IsServiceReference() {
		return service.ParseServiceRef(r.Service)
	}

	return service.ServiceRef{}, fmt.Errorf("reference is not a service reference")
}

// ToLegacyProviderRef converts a Reference to legacy ProviderRef format
func (r *Reference) ToLegacyProviderRef() ProviderRef {
	if r.IsLegacyFormat() {
		return ProviderRef{
			Provider: r.Provider,
			Key:      r.Key,
			Version:  r.Version,
		}
	}

	if r.IsStoreReference() {
		// Try to parse the store reference and convert to legacy format
		if ref, err := secretstore.ParseSecretRef(r.Store); err == nil {
			return ProviderRef{
				Provider: ref.Store,
				Key:      ref.Path,
				Version:  ref.Version,
			}
		}
	}

	// Default fallback
	return ProviderRef{}
}

// GetEffectiveProvider returns the effective provider name for this reference
func (r *Reference) GetEffectiveProvider() string {
	if r.IsLegacyFormat() {
		return r.Provider
	}
	
	if r.IsStoreReference() {
		if ref, err := secretstore.ParseSecretRef(r.Store); err == nil {
			return ref.Store
		}
	}
	
	if r.IsServiceReference() {
		if ref, err := service.ParseServiceRef(r.Service); err == nil {
			return ref.Type
		}
	}

	return ""
}

// ConvertLegacyReference converts a legacy ProviderRef to a store:// URI
func ConvertLegacyReference(ref *ProviderRef) string {
	if ref == nil {
		return ""
	}
	
	storeURI := fmt.Sprintf("store://%s/%s", ref.Provider, ref.Key)
	if ref.Version != "" {
		storeURI += "?version=" + ref.Version
	}
	
	return storeURI
}

// ListAllProviders returns all configured providers (secret stores + services + legacy providers)
func (c *Config) ListAllProviders() map[string]ProviderConfig {
	if c.Definition == nil {
		return make(map[string]ProviderConfig)
	}

	providers := make(map[string]ProviderConfig)

	// Add secret stores
	for name, store := range c.Definition.SecretStores {
		providers[name] = ProviderConfig(store)
	}

	// Add services
	for name, service := range c.Definition.Services {
		providers[name] = ProviderConfig(service)
	}

	// Add legacy providers (for backward compatibility)
	for name, provider := range c.Definition.Providers {
		providers[name] = provider
	}

	return providers
}