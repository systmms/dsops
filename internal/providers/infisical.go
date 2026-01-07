package providers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/systmms/dsops/internal/providers/contracts"
	"github.com/systmms/dsops/pkg/provider"
)

// InfisicalProvider implements the provider interface for Infisical
type InfisicalProvider struct {
	name       string
	config     InfisicalConfig
	client     contracts.InfisicalClient
	tokenCache *TokenCache
}

// NewInfisicalProvider creates a new Infisical provider
func NewInfisicalProvider(name string, config map[string]interface{}) (*InfisicalProvider, error) {
	cfg, err := parseInfisicalConfig(config)
	if err != nil {
		return nil, err
	}

	client, err := newInfisicalHTTPClient(cfg)
	if err != nil {
		return nil, err
	}

	return &InfisicalProvider{
		name:       name,
		config:     cfg,
		client:     client,
		tokenCache: NewTokenCache(),
	}, nil
}

// NewInfisicalProviderWithClient creates an Infisical provider with a custom client.
// This is primarily for testing, allowing the HTTP client to be mocked.
func NewInfisicalProviderWithClient(name string, config map[string]interface{}, client contracts.InfisicalClient) *InfisicalProvider {
	cfg, _ := parseInfisicalConfig(config)
	return &InfisicalProvider{
		name:       name,
		config:     cfg,
		client:     client,
		tokenCache: NewTokenCache(),
	}
}

// Name returns the provider name
func (p *InfisicalProvider) Name() string {
	return p.name
}

// Resolve retrieves a secret from Infisical
func (p *InfisicalProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	// Parse the reference
	infRef, err := ParseInfisicalReference(ref.Key)
	if err != nil {
		return provider.SecretValue{}, fmt.Errorf("invalid infisical reference '%s': %w", ref.Key, err)
	}

	// Get or refresh token
	token, err := p.getToken(ctx)
	if err != nil {
		return provider.SecretValue{}, err
	}

	// Fetch the secret
	secret, err := p.client.GetSecret(ctx, token, infRef.Name, infRef.Version)
	if err != nil {
		if isInfisicalNotFoundError(err) {
			return provider.SecretValue{}, provider.NotFoundError{
				Provider: p.name,
				Key:      ref.Key,
			}
		}
		return provider.SecretValue{}, &InfisicalError{
			Op:      "fetch",
			Message: err.Error(),
			Err:     err,
		}
	}

	return provider.SecretValue{
		Value:     secret.SecretValue,
		Version:   strconv.Itoa(secret.Version),
		UpdatedAt: secret.UpdatedAt,
		Metadata: map[string]string{
			"provider":    p.name,
			"secret_key":  secret.SecretKey,
			"secret_type": secret.Type,
		},
	}, nil
}

// Describe returns metadata about an Infisical secret without retrieving its value
func (p *InfisicalProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	infRef, err := ParseInfisicalReference(ref.Key)
	if err != nil {
		return provider.Metadata{}, fmt.Errorf("invalid infisical reference '%s': %w", ref.Key, err)
	}

	token, err := p.getToken(ctx)
	if err != nil {
		return provider.Metadata{}, fmt.Errorf("failed to authenticate with infisical: %w", err)
	}

	secret, err := p.client.GetSecret(ctx, token, infRef.Name, infRef.Version)
	if err != nil {
		if isInfisicalNotFoundError(err) {
			return provider.Metadata{Exists: false}, nil
		}
		return provider.Metadata{}, fmt.Errorf("failed to describe infisical secret: %w", err)
	}

	return provider.Metadata{
		Exists:    true,
		Version:   strconv.Itoa(secret.Version),
		UpdatedAt: secret.UpdatedAt,
		Type:      secret.Type,
		Tags:      make(map[string]string),
	}, nil
}

// Capabilities returns the provider's supported features
func (p *InfisicalProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: true,
		SupportsMetadata:   true,
		SupportsWatching:   false,
		SupportsBinary:     true,
		RequiresAuth:       true,
		AuthMethods:        []string{"machine_identity", "service_token", "api_key"},
	}
}

// Validate checks if the provider is properly configured and can authenticate
func (p *InfisicalProvider) Validate(ctx context.Context) error {
	_, err := p.getToken(ctx)
	if err != nil {
		return fmt.Errorf("infisical validation failed: %w", err)
	}
	return nil
}

// getToken returns a cached token or authenticates to get a new one
func (p *InfisicalProvider) getToken(ctx context.Context) (string, error) {
	// Check cache first
	if token, ok := p.tokenCache.Get(); ok {
		return token, nil
	}

	// Authenticate
	token, ttl, err := p.client.Authenticate(ctx)
	if err != nil {
		return "", &InfisicalError{
			Op:      "auth",
			Message: err.Error(),
			Err:     err,
		}
	}

	// Cache the token
	p.tokenCache.Set(token, ttl)

	return token, nil
}

// InfisicalReference represents a parsed Infisical secret reference
type InfisicalReference struct {
	Path    string // e.g., "folder/subfolder"
	Name    string // e.g., "SECRET_NAME"
	Version *int   // nil for latest
}

// ParseInfisicalReference parses an Infisical reference string
// Format: [path/]SECRET_NAME[@vN]
func ParseInfisicalReference(key string) (*InfisicalReference, error) {
	ref := &InfisicalReference{}

	// Check for version suffix
	if idx := strings.LastIndex(key, "@v"); idx != -1 {
		versionStr := key[idx+2:]
		version, err := strconv.Atoi(versionStr)
		if err == nil {
			ref.Version = &version
			key = key[:idx]
		}
	}

	// Split path and name
	if idx := strings.LastIndex(key, "/"); idx != -1 {
		ref.Path = key[:idx]
		ref.Name = key[idx+1:]
	} else {
		ref.Name = key
	}

	if ref.Name == "" {
		return nil, fmt.Errorf("infisical reference name cannot be empty")
	}

	return ref, nil
}

// parseInfisicalConfig parses configuration map into InfisicalConfig
func parseInfisicalConfig(config map[string]interface{}) (InfisicalConfig, error) {
	cfg := InfisicalConfig{
		Host:    DefaultInfisicalHost,
		Timeout: DefaultTimeout,
	}

	if config == nil {
		return cfg, nil
	}

	if host, ok := config["host"].(string); ok && host != "" {
		cfg.Host = host
	}

	if projectID, ok := config["project_id"].(string); ok {
		cfg.ProjectID = projectID
	}

	if env, ok := config["environment"].(string); ok {
		cfg.Environment = env
	}

	if timeout, ok := config["timeout"].(string); ok {
		if d, err := time.ParseDuration(timeout); err == nil {
			cfg.Timeout = d
		}
	}

	if caCert, ok := config["ca_cert"].(string); ok {
		cfg.CACert = caCert
	}

	if insecure, ok := config["insecure_skip_verify"].(bool); ok {
		cfg.InsecureSkipVerify = insecure
	}

	// Parse auth
	if auth, ok := config["auth"].(map[string]interface{}); ok {
		if method, ok := auth["method"].(string); ok {
			cfg.Auth.Method = method
		}
		if clientID, ok := auth["client_id"].(string); ok {
			cfg.Auth.ClientID = clientID
		}
		if clientSecret, ok := auth["client_secret"].(string); ok {
			cfg.Auth.ClientSecret = clientSecret
		}
		if serviceToken, ok := auth["service_token"].(string); ok {
			cfg.Auth.ServiceToken = serviceToken
		}
		if apiKey, ok := auth["api_key"].(string); ok {
			cfg.Auth.APIKey = apiKey
		}
	}

	return cfg, nil
}

// isInfisicalNotFoundError checks if an error indicates secret not found
func isInfisicalNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "404")
}
