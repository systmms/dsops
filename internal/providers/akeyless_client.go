package providers

import (
	"context"
	"fmt"
	"time"

	akeyless "github.com/akeylesslabs/akeyless-go/v3"

	"github.com/systmms/dsops/internal/providers/contracts"
)

// akeylessSDKClient implements AkeylessClient using the official SDK
type akeylessSDKClient struct {
	apiClient *akeyless.APIClient
	config    AkeylessConfig
}

// newAkeylessSDKClient creates a new SDK client for Akeyless
func newAkeylessSDKClient(cfg AkeylessConfig) (*akeylessSDKClient, error) {
	configuration := akeyless.NewConfiguration()
	configuration.Servers = []akeyless.ServerConfiguration{
		{URL: cfg.GatewayURL},
	}

	apiClient := akeyless.NewAPIClient(configuration)

	return &akeylessSDKClient{
		apiClient: apiClient,
		config:    cfg,
	}, nil
}

// Authenticate obtains an access token from Akeyless
func (c *akeylessSDKClient) Authenticate(ctx context.Context) (string, time.Duration, error) {
	switch c.config.Auth.Method {
	case "api_key", "":
		return c.authenticateAPIKey(ctx)
	case "aws_iam":
		return c.authenticateAWSIAM(ctx)
	case "azure_ad":
		return c.authenticateAzureAD(ctx)
	case "gcp":
		return c.authenticateGCP(ctx)
	default:
		return "", 0, fmt.Errorf("unsupported authentication method: %s", c.config.Auth.Method)
	}
}

// authenticateAPIKey authenticates using API Key (access_id + access_key)
func (c *akeylessSDKClient) authenticateAPIKey(ctx context.Context) (string, time.Duration, error) {
	authBody := akeyless.NewAuthWithDefaults()
	authBody.SetAccessId(c.config.AccessID)
	authBody.SetAccessKey(c.config.Auth.AccessKey)

	authRes, _, err := c.apiClient.V2Api.Auth(ctx).Body(*authBody).Execute()
	if err != nil {
		return "", 0, fmt.Errorf("api key authentication failed: %w", err)
	}

	token := authRes.GetToken()
	// Akeyless tokens typically last 30 minutes, but we'll use 25 to be safe
	ttl := 25 * time.Minute

	return token, ttl, nil
}

// authenticateAWSIAM authenticates using AWS IAM
func (c *akeylessSDKClient) authenticateAWSIAM(ctx context.Context) (string, time.Duration, error) {
	authBody := akeyless.NewAuthWithDefaults()
	authBody.SetAccessId(c.config.AccessID)
	authBody.SetAccessType("aws_iam")

	authRes, _, err := c.apiClient.V2Api.Auth(ctx).Body(*authBody).Execute()
	if err != nil {
		return "", 0, fmt.Errorf("aws iam authentication failed: %w", err)
	}

	token := authRes.GetToken()
	ttl := 25 * time.Minute

	return token, ttl, nil
}

// authenticateAzureAD authenticates using Azure AD
func (c *akeylessSDKClient) authenticateAzureAD(ctx context.Context) (string, time.Duration, error) {
	authBody := akeyless.NewAuthWithDefaults()
	authBody.SetAccessId(c.config.AccessID)
	authBody.SetAccessType("azure_ad")
	if c.config.Auth.AzureADObjectID != "" {
		// CloudId is used for Azure AD object ID
		authBody.SetCloudId(c.config.Auth.AzureADObjectID)
	}

	authRes, _, err := c.apiClient.V2Api.Auth(ctx).Body(*authBody).Execute()
	if err != nil {
		return "", 0, fmt.Errorf("azure ad authentication failed: %w", err)
	}

	token := authRes.GetToken()
	ttl := 25 * time.Minute

	return token, ttl, nil
}

// authenticateGCP authenticates using GCP
func (c *akeylessSDKClient) authenticateGCP(ctx context.Context) (string, time.Duration, error) {
	authBody := akeyless.NewAuthWithDefaults()
	authBody.SetAccessId(c.config.AccessID)
	authBody.SetAccessType("gcp")
	if c.config.Auth.GCPAudience != "" {
		authBody.SetGcpAudience(c.config.Auth.GCPAudience)
	}

	authRes, _, err := c.apiClient.V2Api.Auth(ctx).Body(*authBody).Execute()
	if err != nil {
		return "", 0, fmt.Errorf("gcp authentication failed: %w", err)
	}

	token := authRes.GetToken()
	ttl := 25 * time.Minute

	return token, ttl, nil
}

// GetSecret retrieves a secret by path
func (c *akeylessSDKClient) GetSecret(ctx context.Context, token, path string, version *int) (*contracts.AkeylessSecret, error) {
	body := akeyless.NewGetSecretValue([]string{path})
	body.SetToken(token)
	if version != nil {
		body.SetVersion(int32(*version))
	}

	res, _, err := c.apiClient.V2Api.GetSecretValue(ctx).Body(*body).Execute()
	if err != nil {
		return nil, err
	}

	// GetSecretValue returns a map of path -> value
	secretMap := res
	value, ok := secretMap[path]
	if !ok {
		return nil, ErrAkeylessSecretNotFound
	}

	return &contracts.AkeylessSecret{
		Path:      path,
		Value:     value,
		Version:   1, // The SDK doesn't return version info in GetSecretValue
		UpdatedAt: time.Now(),
	}, nil
}

// DescribeItem gets metadata about a secret
func (c *akeylessSDKClient) DescribeItem(ctx context.Context, token, path string) (*contracts.AkeylessMetadata, error) {
	body := akeyless.NewDescribeItem(path)
	body.SetToken(token)

	res, _, err := c.apiClient.V2Api.DescribeItem(ctx).Body(*body).Execute()
	if err != nil {
		return nil, err
	}

	var lastModified time.Time
	if res.ModificationDate != nil {
		lastModified = *res.ModificationDate
	} else if res.LastVersion != nil {
		lastModified = time.Now() // Fallback to now if no timestamp
	}

	// Calculate version from LastVersion or count of ItemVersions
	version := 0
	if res.LastVersion != nil {
		version = int(*res.LastVersion)
	} else if res.ItemVersions != nil {
		version = len(*res.ItemVersions)
	}

	// Convert tags from pointer
	var tags []string
	if res.ItemTags != nil {
		tags = *res.ItemTags
	}

	return &contracts.AkeylessMetadata{
		Path:         path,
		ItemType:     res.GetItemType(),
		Version:      version,
		LastModified: lastModified,
		Tags:         tags,
	}, nil
}

// ListItems lists secrets at a path
func (c *akeylessSDKClient) ListItems(ctx context.Context, token, path string) ([]string, error) {
	body := akeyless.NewListItems()
	body.SetPath(path)
	body.SetToken(token)

	res, _, err := c.apiClient.V2Api.ListItems(ctx).Body(*body).Execute()
	if err != nil {
		return nil, err
	}

	items := res.GetItems()
	paths := make([]string, len(items))
	for i, item := range items {
		paths[i] = item.GetItemName()
	}

	return paths, nil
}

// Ensure akeylessSDKClient implements contracts.AkeylessClient
var _ contracts.AkeylessClient = (*akeylessSDKClient)(nil)
