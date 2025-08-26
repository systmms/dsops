package providers

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/pkg/provider"
	dserrors "github.com/systmms/dsops/internal/errors"
)

// AWSSSOProvider implements the Provider interface for AWS IAM Identity Center (formerly AWS SSO)
type AWSSSOProvider struct {
	name      string
	ssoClient *sso.Client
	oidcClient *ssooidc.Client
	logger    *logging.Logger
	config    SSOConfig
	cache     *ssoCredentialCache
}

// SSOConfig holds AWS SSO-specific configuration
type SSOConfig struct {
	StartURL     string
	Region       string
	AccountID    string
	RoleName     string
	Profile      string
	CachePath    string // Optional: custom cache location
	RefreshToken bool   // Whether to refresh expired tokens
}

// ssoCredentialCache caches SSO credentials
type ssoCredentialCache struct {
	credentials *sso.GetRoleCredentialsOutput
	expiresAt   time.Time
	accessToken string
}

// ssoTokenCache represents the cached SSO token structure
type ssoTokenCache struct {
	StartURL    string    `json:"startUrl"`
	Region      string    `json:"region"`
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

// NewAWSSSOProvider creates a new AWS SSO provider
func NewAWSSSOProvider(name string, configMap map[string]interface{}) (*AWSSSOProvider, error) {
	logger := logging.New(false, false)
	
	config := SSOConfig{
		RefreshToken: true, // Default to auto-refresh
	}

	// Parse configuration
	if startURL, ok := configMap["start_url"].(string); ok {
		config.StartURL = startURL
	}
	if region, ok := configMap["region"].(string); ok {
		config.Region = region
	}
	if accountID, ok := configMap["account_id"].(string); ok {
		config.AccountID = accountID
	}
	if roleName, ok := configMap["role_name"].(string); ok {
		config.RoleName = roleName
	}
	if profile, ok := configMap["profile"].(string); ok {
		config.Profile = profile
	}
	if cachePath, ok := configMap["cache_path"].(string); ok {
		config.CachePath = cachePath
	}
	if refresh, ok := configMap["refresh_token"].(bool); ok {
		config.RefreshToken = refresh
	}

	// Validate required configuration
	if config.StartURL == "" {
		return nil, dserrors.ConfigError{
			Field:      "start_url",
			Message:    "start_url is required for SSO provider",
			Suggestion: "Provide your AWS SSO portal URL (e.g., https://my-sso-portal.awsapps.com/start)",
		}
	}
	if config.AccountID == "" {
		return nil, dserrors.ConfigError{
			Field:      "account_id",
			Message:    "account_id is required for SSO provider",
			Suggestion: "Provide the AWS account ID you want to access",
		}
	}
	if config.RoleName == "" {
		return nil, dserrors.ConfigError{
			Field:      "role_name",
			Message:    "role_name is required for SSO provider",
			Suggestion: "Provide the SSO role name (e.g., AdministratorAccess)",
		}
	}

	// Default cache path if not specified
	if config.CachePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		config.CachePath = filepath.Join(home, ".aws", "sso", "cache")
	}

	// Create AWS clients
	ssoClient, oidcClient, err := createSSOClients(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSO clients: %w", err)
	}

	return &AWSSSOProvider{
		name:       name,
		ssoClient:  ssoClient,
		oidcClient: oidcClient,
		logger:     logger,
		config:     config,
		cache:      &ssoCredentialCache{},
	}, nil
}

// createSSOClients creates AWS SSO and OIDC clients
func createSSOClients(config SSOConfig) (*sso.Client, *ssooidc.Client, error) {
	ctx := context.Background()
	
	// Build config options
	var configOpts []func(*awsconfig.LoadOptions) error

	// SSO requires region
	if config.Region != "" {
		configOpts = append(configOpts, awsconfig.WithRegion(config.Region))
	} else {
		// Try to determine region from start URL
		if strings.Contains(config.StartURL, "us-east-1") {
			configOpts = append(configOpts, awsconfig.WithRegion("us-east-1"))
		} else if strings.Contains(config.StartURL, "us-west-2") {
			configOpts = append(configOpts, awsconfig.WithRegion("us-west-2"))
		} else {
			// Default to us-east-1
			configOpts = append(configOpts, awsconfig.WithRegion("us-east-1"))
		}
	}

	if config.Profile != "" {
		configOpts = append(configOpts, awsconfig.WithSharedConfigProfile(config.Profile))
	}

	// Load AWS configuration
	cfg, err := awsconfig.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return sso.NewFromConfig(cfg), ssooidc.NewFromConfig(cfg), nil
}

// Name returns the provider name
func (p *AWSSSOProvider) Name() string {
	return p.name
}

// Resolve fetches temporary credentials from SSO
func (p *AWSSSOProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	// Check cache first
	if p.cache.credentials != nil && time.Now().Before(p.cache.expiresAt) {
		return p.getCredentialValue(ref.Key)
	}

	// Get or refresh SSO access token
	accessToken, err := p.getAccessToken(ctx)
	if err != nil {
		return provider.SecretValue{}, dserrors.UserError{
			Message:    "Failed to get SSO access token",
			Details:    err.Error(),
			Suggestion: "Run 'aws sso login' or 'dsops login " + p.name + "' to authenticate",
		}
	}

	// Get role credentials
	input := &sso.GetRoleCredentialsInput{
		AccountId:   aws.String(p.config.AccountID),
		RoleName:    aws.String(p.config.RoleName),
		AccessToken: aws.String(accessToken),
	}

	p.logger.Debug("Getting SSO role credentials for account %s, role %s", 
		logging.Secret(p.config.AccountID), logging.Secret(p.config.RoleName))

	result, err := p.ssoClient.GetRoleCredentials(ctx, input)
	if err != nil {
		return provider.SecretValue{}, dserrors.UserError{
			Message:    "Failed to get SSO role credentials",
			Details:    err.Error(),
			Suggestion: getSSOErrorSuggestion(err),
		}
	}

	// Cache the credentials
	p.cache.credentials = result
	p.cache.accessToken = accessToken
	if result.RoleCredentials != nil && result.RoleCredentials.Expiration != 0 {
		p.cache.expiresAt = time.Unix(result.RoleCredentials.Expiration/1000, 0)
	}

	return p.getCredentialValue(ref.Key)
}

// getAccessToken retrieves or loads the SSO access token
func (p *AWSSSOProvider) getAccessToken(ctx context.Context) (string, error) {
	// Try to load from cache
	token, err := p.loadCachedToken()
	if err == nil && token != nil && time.Now().Before(token.ExpiresAt) {
		return token.AccessToken, nil
	}

	// Token expired or not found
	if p.config.RefreshToken && token != nil {
		// TODO: Implement token refresh via OIDC
		// For now, return error requiring re-login
		return "", fmt.Errorf("SSO token expired, please re-authenticate")
	}

	return "", fmt.Errorf("no valid SSO token found")
}

// loadCachedToken loads the SSO token from cache
func (p *AWSSSOProvider) loadCachedToken() (*ssoTokenCache, error) {
	// Calculate cache file name (SHA1 of start URL)
	hash := fmt.Sprintf("%x", sha1.Sum([]byte(p.config.StartURL)))
	cacheFile := filepath.Join(p.config.CachePath, hash+".json")

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, err
	}

	var token ssoTokenCache
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}

	// Validate token matches our config
	if token.StartURL != p.config.StartURL {
		return nil, fmt.Errorf("cached token start URL mismatch")
	}

	return &token, nil
}

// getCredentialValue extracts the requested credential component
func (p *AWSSSOProvider) getCredentialValue(key string) (provider.SecretValue, error) {
	if p.cache.credentials == nil || p.cache.credentials.RoleCredentials == nil {
		return provider.SecretValue{}, fmt.Errorf("no credentials available")
	}

	creds := p.cache.credentials.RoleCredentials
	var value string

	switch key {
	case "access_key_id", "AccessKeyId", "AWS_ACCESS_KEY_ID":
		value = *creds.AccessKeyId
	case "secret_access_key", "SecretAccessKey", "AWS_SECRET_ACCESS_KEY":
		value = *creds.SecretAccessKey
	case "session_token", "SessionToken", "AWS_SESSION_TOKEN":
		value = *creds.SessionToken
	case "expiration":
		value = time.Unix(creds.Expiration/1000, 0).Format(time.RFC3339)
	case "credentials", "all":
		// Return all credentials as JSON
		credsMap := map[string]string{
			"AccessKeyId":     *creds.AccessKeyId,
			"SecretAccessKey": *creds.SecretAccessKey,
			"SessionToken":    *creds.SessionToken,
			"Expiration":      time.Unix(creds.Expiration/1000, 0).Format(time.RFC3339),
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
		"source":     fmt.Sprintf("sso:%s/%s", p.config.AccountID, p.config.RoleName),
		"expires_at": time.Unix(creds.Expiration/1000, 0).Format(time.RFC3339),
	}

	return provider.SecretValue{
		Value:    value,
		Metadata: metadata,
	}, nil
}

// Describe returns metadata about the SSO provider
func (p *AWSSSOProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	return provider.Metadata{
		Exists: true,
		Type:   "sso-credentials",
		Tags: map[string]string{
			"start_url":  p.config.StartURL,
			"account_id": p.config.AccountID,
			"role_name":  p.config.RoleName,
		},
	}, nil
}

// Capabilities returns the provider's capabilities
func (p *AWSSSOProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: false,
		SupportsMetadata:   true,
		SupportsWatching:   false,
		SupportsBinary:     false,
		RequiresAuth:       true,
		AuthMethods:        []string{"sso", "browser"},
	}
}

// Validate checks if the provider is properly configured and accessible
func (p *AWSSSOProvider) Validate(ctx context.Context) error {
	// Check if we can load a token
	token, err := p.loadCachedToken()
	if err != nil || token == nil {
		return dserrors.UserError{
			Message:    "No SSO session found",
			Suggestion: "Run 'aws sso login --sso-session " + p.name + "' to authenticate",
			Details:    "SSO requires browser-based authentication",
		}
	}

	// Check if token is expired
	if time.Now().After(token.ExpiresAt) {
		return dserrors.UserError{
			Message:    "SSO session expired",
			Suggestion: "Run 'aws sso login' to re-authenticate",
			Details:    fmt.Sprintf("Token expired at %s", token.ExpiresAt.Format(time.RFC3339)),
		}
	}

	return nil
}

// getSSOErrorSuggestion provides helpful suggestions based on SSO errors
func getSSOErrorSuggestion(err error) string {
	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "UnauthorizedException"):
		return "Your SSO session may have expired. Run 'aws sso login' to re-authenticate"
	case strings.Contains(errStr, "AccessDeniedException"):
		return "You don't have permission to assume this role. Check with your SSO administrator"
	case strings.Contains(errStr, "ResourceNotFoundException"):
		return "The specified account or role was not found. Verify your configuration"
	case strings.Contains(errStr, "TooManyRequestsException"):
		return "Request was throttled. Wait a moment and try again"
	default:
		return "Check your SSO configuration and ensure you're logged in via 'aws sso login'"
	}
}

// NewAWSSSOProviderFactory creates an AWS SSO provider factory
func NewAWSSSOProviderFactory(name string, config map[string]interface{}) (provider.Provider, error) {
	return NewAWSSSOProvider(name, config)
}