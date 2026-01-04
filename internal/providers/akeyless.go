package providers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/systmms/dsops/internal/providers/contracts"
	"github.com/systmms/dsops/pkg/provider"
)

// AkeylessProvider implements the provider interface for Akeyless
type AkeylessProvider struct {
	name       string
	config     AkeylessConfig
	client     contracts.AkeylessClient
	tokenCache *TokenCache
}

// NewAkeylessProvider creates a new Akeyless provider
func NewAkeylessProvider(name string, config map[string]interface{}) (*AkeylessProvider, error) {
	cfg, err := parseAkeylessConfig(config)
	if err != nil {
		return nil, err
	}

	client, err := newAkeylessSDKClient(cfg)
	if err != nil {
		return nil, err
	}

	return &AkeylessProvider{
		name:       name,
		config:     cfg,
		client:     client,
		tokenCache: NewTokenCache(),
	}, nil
}

// NewAkeylessProviderWithClient creates an Akeyless provider with a custom client.
// This is primarily for testing, allowing the SDK client to be mocked.
func NewAkeylessProviderWithClient(name string, config map[string]interface{}, client contracts.AkeylessClient) *AkeylessProvider {
	cfg, _ := parseAkeylessConfig(config)
	return &AkeylessProvider{
		name:       name,
		config:     cfg,
		client:     client,
		tokenCache: NewTokenCache(),
	}
}

// Name returns the provider name
func (p *AkeylessProvider) Name() string {
	return p.name
}

// Resolve retrieves a secret from Akeyless
func (p *AkeylessProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	// Parse the reference
	akRef, err := ParseAkeylessReference(ref.Key)
	if err != nil {
		return provider.SecretValue{}, fmt.Errorf("invalid akeyless reference '%s': %w", ref.Key, err)
	}

	// Get or refresh token
	token, err := p.getToken(ctx)
	if err != nil {
		return provider.SecretValue{}, err
	}

	// Fetch the secret
	secret, err := p.client.GetSecret(ctx, token, akRef.Path, akRef.Version)
	if err != nil {
		if isAkeylessNotFoundError(err) {
			return provider.SecretValue{}, provider.NotFoundError{
				Provider: p.name,
				Key:      ref.Key,
			}
		}
		return provider.SecretValue{}, &AkeylessError{
			Op:      "fetch",
			Path:    akRef.Path,
			Message: err.Error(),
			Err:     err,
		}
	}

	return provider.SecretValue{
		Value:     secret.Value,
		Version:   strconv.Itoa(secret.Version),
		UpdatedAt: secret.UpdatedAt,
		Metadata: map[string]string{
			"provider": p.name,
			"path":     secret.Path,
		},
	}, nil
}

// Describe returns metadata about an Akeyless secret without retrieving its value
func (p *AkeylessProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	akRef, err := ParseAkeylessReference(ref.Key)
	if err != nil {
		return provider.Metadata{}, fmt.Errorf("invalid akeyless reference '%s': %w", ref.Key, err)
	}

	token, err := p.getToken(ctx)
	if err != nil {
		return provider.Metadata{Exists: false}, nil
	}

	meta, err := p.client.DescribeItem(ctx, token, akRef.Path)
	if err != nil {
		if isAkeylessNotFoundError(err) {
			return provider.Metadata{Exists: false}, nil
		}
		return provider.Metadata{Exists: false}, nil
	}

	// Convert Akeyless tags ([]string) to map format
	// Akeyless tags can be in "key:value" format or just "tag" format
	tags := make(map[string]string)
	for _, tag := range meta.Tags {
		if idx := strings.Index(tag, ":"); idx > 0 {
			tags[tag[:idx]] = tag[idx+1:]
		} else {
			tags[tag] = ""
		}
	}

	return provider.Metadata{
		Exists:    true,
		Version:   strconv.Itoa(meta.Version),
		UpdatedAt: meta.LastModified,
		Type:      meta.ItemType,
		Tags:      tags,
	}, nil
}

// Capabilities returns the provider's supported features
func (p *AkeylessProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: true,
		SupportsMetadata:   true,
		SupportsWatching:   false,
		SupportsBinary:     true,
		RequiresAuth:       true,
		AuthMethods:        []string{"api_key", "aws_iam", "azure_ad", "gcp"},
	}
}

// Validate checks if the provider is properly configured and can authenticate
func (p *AkeylessProvider) Validate(ctx context.Context) error {
	_, err := p.getToken(ctx)
	if err != nil {
		return fmt.Errorf("akeyless validation failed: %w", err)
	}
	return nil
}

// getToken returns a cached token or authenticates to get a new one
func (p *AkeylessProvider) getToken(ctx context.Context) (string, error) {
	// Check cache first
	if token, ok := p.tokenCache.Get(); ok {
		return token, nil
	}

	// Authenticate
	token, ttl, err := p.client.Authenticate(ctx)
	if err != nil {
		return "", &AkeylessError{
			Op:      "auth",
			Message: err.Error(),
			Err:     err,
		}
	}

	// Cache the token
	p.tokenCache.Set(token, ttl)

	return token, nil
}

// AkeylessReference represents a parsed Akeyless secret reference
type AkeylessReference struct {
	Path    string // e.g., "/prod/database/password"
	Version *int   // nil for latest
}

// ParseAkeylessReference parses an Akeyless reference string
// Format: /path/to/secret[@vN]
func ParseAkeylessReference(key string) (*AkeylessReference, error) {
	ref := &AkeylessReference{}

	// Check for version suffix
	if idx := strings.LastIndex(key, "@v"); idx != -1 {
		versionStr := key[idx+2:]
		version, err := strconv.Atoi(versionStr)
		if err == nil {
			ref.Version = &version
			key = key[:idx]
		}
	}

	// Ensure path starts with /
	if !strings.HasPrefix(key, "/") {
		key = "/" + key
	}

	ref.Path = key

	if ref.Path == "/" {
		return nil, fmt.Errorf("akeyless reference path cannot be empty")
	}

	return ref, nil
}

// parseAkeylessConfig parses configuration map into AkeylessConfig
func parseAkeylessConfig(config map[string]interface{}) (AkeylessConfig, error) {
	cfg := AkeylessConfig{
		GatewayURL: DefaultAkeylessGateway,
		Timeout:    DefaultTimeout,
	}

	if config == nil {
		return cfg, nil
	}

	if accessID, ok := config["access_id"].(string); ok {
		cfg.AccessID = accessID
	}

	if gatewayURL, ok := config["gateway_url"].(string); ok && gatewayURL != "" {
		cfg.GatewayURL = gatewayURL
	}

	// Parse auth
	if auth, ok := config["auth"].(map[string]interface{}); ok {
		if method, ok := auth["method"].(string); ok {
			cfg.Auth.Method = method
		}
		if accessKey, ok := auth["access_key"].(string); ok {
			cfg.Auth.AccessKey = accessKey
		}
		if azureADObjectID, ok := auth["azure_ad_object_id"].(string); ok {
			cfg.Auth.AzureADObjectID = azureADObjectID
		}
		if gcpAudience, ok := auth["gcp_audience"].(string); ok {
			cfg.Auth.GCPAudience = gcpAudience
		}
	}

	return cfg, nil
}

// isAkeylessNotFoundError checks if an error indicates secret not found
func isAkeylessNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "itemNotFound")
}
