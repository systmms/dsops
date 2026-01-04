package providers

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/systmms/dsops/internal/providers/contracts"
)

// infisicalHTTPClient implements InfisicalClient using HTTP
type infisicalHTTPClient struct {
	httpClient  *http.Client
	host        string
	projectID   string
	environment string
	auth        InfisicalAuth
}

// newInfisicalHTTPClient creates a new HTTP client for Infisical
func newInfisicalHTTPClient(cfg InfisicalConfig) (*infisicalHTTPClient, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{},
	}

	// Configure custom CA if provided
	if cfg.CACert != "" {
		caCert, err := os.ReadFile(cfg.CACert)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}

		transport.TLSClientConfig.RootCAs = caCertPool
	}

	// Configure insecure skip verify if needed
	if cfg.InsecureSkipVerify {
		transport.TLSClientConfig.InsecureSkipVerify = true
	}

	return &infisicalHTTPClient{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   cfg.Timeout,
		},
		host:        cfg.Host,
		projectID:   cfg.ProjectID,
		environment: cfg.Environment,
		auth:        cfg.Auth,
	}, nil
}

// Authenticate obtains an access token from Infisical
func (c *infisicalHTTPClient) Authenticate(ctx context.Context) (string, time.Duration, error) {
	switch c.auth.Method {
	case "machine_identity", "":
		return c.authenticateMachineIdentity(ctx)
	case "service_token":
		// Service tokens don't need authentication - they are the token
		return c.auth.ServiceToken, 24 * time.Hour, nil
	case "api_key":
		// API keys don't need authentication - they are used directly
		return c.auth.APIKey, 24 * time.Hour, nil
	default:
		return "", 0, fmt.Errorf("unsupported authentication method: %s", c.auth.Method)
	}
}

// authenticateMachineIdentity authenticates using Universal Auth (Machine Identity)
func (c *infisicalHTTPClient) authenticateMachineIdentity(ctx context.Context) (string, time.Duration, error) {
	url := fmt.Sprintf("%s/api/v1/auth/universal-auth/login", c.host)

	body := map[string]string{
		"clientId":     c.auth.ClientID,
		"clientSecret": c.auth.ClientSecret,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal auth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", 0, &InfisicalError{
			Op:         "auth",
			StatusCode: resp.StatusCode,
			Message:    string(bodyBytes),
		}
	}

	var authResp struct {
		AccessToken          string `json:"accessToken"`
		ExpiresIn            int    `json:"expiresIn"`
		AccessTokenMaxTTL    int    `json:"accessTokenMaxTTL"`
		TokenType            string `json:"tokenType"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", 0, fmt.Errorf("failed to decode auth response: %w", err)
	}

	ttl := time.Duration(authResp.ExpiresIn) * time.Second
	if ttl == 0 {
		ttl = 30 * time.Second // Default TTL
	}

	return authResp.AccessToken, ttl, nil
}

// GetSecret retrieves a single secret by name
func (c *infisicalHTTPClient) GetSecret(ctx context.Context, token, secretName string, version *int) (*contracts.InfisicalSecret, error) {
	url := fmt.Sprintf("%s/api/v3/secrets/%s", c.host, secretName)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	q := req.URL.Query()
	q.Add("workspaceId", c.projectID)
	q.Add("environment", c.environment)
	if version != nil {
		q.Add("version", fmt.Sprintf("%d", *version))
	}
	req.URL.RawQuery = q.Encode()

	// Set auth header
	c.setAuthHeader(req, token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrInfisicalSecretNotFound
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, &InfisicalError{
			Op:         "fetch",
			StatusCode: resp.StatusCode,
			Message:    string(bodyBytes),
		}
	}

	var secretResp struct {
		Secret struct {
			ID            string    `json:"_id"`
			SecretKey     string    `json:"secretKey"`
			SecretValue   string    `json:"secretValue"`
			Version       int       `json:"version"`
			Type          string    `json:"type"`
			CreatedAt     time.Time `json:"createdAt"`
			UpdatedAt     time.Time `json:"updatedAt"`
			SecretComment string    `json:"secretComment"`
			Tags          []string  `json:"tags"`
		} `json:"secret"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&secretResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &contracts.InfisicalSecret{
		SecretKey:     secretResp.Secret.SecretKey,
		SecretValue:   secretResp.Secret.SecretValue,
		Version:       secretResp.Secret.Version,
		Type:          secretResp.Secret.Type,
		CreatedAt:     secretResp.Secret.CreatedAt,
		UpdatedAt:     secretResp.Secret.UpdatedAt,
		SecretComment: secretResp.Secret.SecretComment,
		Tags:          secretResp.Secret.Tags,
	}, nil
}

// ListSecrets lists all secrets (for doctor validation)
func (c *infisicalHTTPClient) ListSecrets(ctx context.Context, token string) ([]string, error) {
	url := fmt.Sprintf("%s/api/v3/secrets", c.host)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	q := req.URL.Query()
	q.Add("workspaceId", c.projectID)
	q.Add("environment", c.environment)
	req.URL.RawQuery = q.Encode()

	// Set auth header
	c.setAuthHeader(req, token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, &InfisicalError{
			Op:         "list",
			StatusCode: resp.StatusCode,
			Message:    string(bodyBytes),
		}
	}

	var listResp struct {
		Secrets []struct {
			SecretKey string `json:"secretKey"`
		} `json:"secrets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	names := make([]string, len(listResp.Secrets))
	for i, s := range listResp.Secrets {
		names[i] = s.SecretKey
	}

	return names, nil
}

// setAuthHeader sets the appropriate authorization header
func (c *infisicalHTTPClient) setAuthHeader(req *http.Request, token string) {
	switch c.auth.Method {
	case "service_token":
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	case "api_key":
		req.Header.Set("X-API-Key", token)
	default:
		// Machine identity uses Bearer token
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}
}

// Ensure infisicalHTTPClient implements contracts.InfisicalClient
var _ contracts.InfisicalClient = (*infisicalHTTPClient)(nil)
