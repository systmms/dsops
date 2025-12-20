package vault

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// Authenticate performs authentication with Vault based on the configured method
func (c *HTTPVaultClient) Authenticate(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If we already have a token, validate it
	if c.token != "" {
		if err := c.validateTokenLocked(ctx); err == nil {
			return nil // Token is still valid
		}
		// Token is invalid, clear it and re-authenticate
		c.token = ""
	}

	switch c.config.AuthMethod {
	case "token":
		return c.authenticateTokenLocked()
	case "userpass":
		return c.authenticateUserpassLocked(ctx)
	case "ldap":
		return c.authenticateLDAPLocked(ctx)
	case "aws":
		return c.authenticateAWSLocked(ctx)
	case "k8s", "kubernetes":
		return c.authenticateKubernetesLocked(ctx)
	default:
		return fmt.Errorf("unsupported auth method: %s", c.config.AuthMethod)
	}
}

// Read fetches a secret from Vault
func (c *HTTPVaultClient) Read(ctx context.Context, path string) (*VaultSecret, error) {
	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()

	if token == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	url := strings.TrimSuffix(c.config.Address, "/") + "/v1/" + strings.TrimPrefix(path, "/")

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", token)
	if c.config.Namespace != "" {
		req.Header.Set("X-Vault-Namespace", c.config.Namespace)
	}

	client := c.getHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 404 {
		return nil, nil // Secret not found
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vault returned status %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Data *VaultSecret `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Data, nil
}

// Close cleans up the client
func (c *HTTPVaultClient) Close() error {
	c.mu.Lock()
	c.token = ""
	c.mu.Unlock()
	return nil
}

// authenticateTokenLocked validates or sets the token
// Must be called with c.mu held
func (c *HTTPVaultClient) authenticateTokenLocked() error {
	if c.config.Token != "" {
		c.token = c.config.Token
		return nil
	}

	if token := os.Getenv("VAULT_TOKEN"); token != "" {
		c.token = token
		return nil
	}

	return fmt.Errorf("no vault token found in config or VAULT_TOKEN environment variable")
}

// authenticateUserpassLocked authenticates using username/password
// Must be called with c.mu held
func (c *HTTPVaultClient) authenticateUserpassLocked(ctx context.Context) error {
	password := c.config.UserpassPassword
	if password == "" {
		// Try environment variable
		password = os.Getenv("VAULT_USERPASS_PASSWORD")
	}
	if password == "" {
		return fmt.Errorf("no password found for userpass auth")
	}

	authData := map[string]interface{}{
		"password": password,
	}

	return c.performLoginLocked(ctx, fmt.Sprintf("auth/userpass/login/%s", c.config.UserpassUsername), authData)
}

// authenticateLDAPLocked authenticates using LDAP
// Must be called with c.mu held
func (c *HTTPVaultClient) authenticateLDAPLocked(ctx context.Context) error {
	password := c.config.LDAPPassword
	if password == "" {
		// Try environment variable
		password = os.Getenv("VAULT_LDAP_PASSWORD")
	}
	if password == "" {
		return fmt.Errorf("no password found for LDAP auth")
	}

	authData := map[string]interface{}{
		"password": password,
	}

	return c.performLoginLocked(ctx, fmt.Sprintf("auth/ldap/login/%s", c.config.LDAPUsername), authData)
}

// authenticateAWSLocked authenticates using AWS IAM
// Must be called with c.mu held
func (c *HTTPVaultClient) authenticateAWSLocked(ctx context.Context) error {
	// This is a simplified implementation
	// In practice, you'd need to generate AWS SigV4 signatures
	authData := map[string]interface{}{
		"role": c.config.AWSRole,
		// AWS auth requires iam_http_request_method, iam_request_url, iam_request_body, iam_request_headers
		// This would need proper AWS SigV4 implementation
	}

	return c.performLoginLocked(ctx, "auth/aws/login", authData)
}

// authenticateKubernetesLocked authenticates using Kubernetes service account
// Must be called with c.mu held
func (c *HTTPVaultClient) authenticateKubernetesLocked(ctx context.Context) error {
	// Read the service account token
	tokenPath := "/var/run/secrets/kubernetes.io/serviceaccount/token"
	if customPath := os.Getenv("VAULT_K8S_TOKEN_PATH"); customPath != "" {
		tokenPath = customPath
	}

	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		return fmt.Errorf("failed to read kubernetes token: %w", err)
	}

	authData := map[string]interface{}{
		"role": c.config.K8SRole,
		"jwt":  string(tokenBytes),
	}

	return c.performLoginLocked(ctx, "auth/kubernetes/login", authData)
}

// performLoginLocked handles the common login workflow
// Must be called with c.mu held
func (c *HTTPVaultClient) performLoginLocked(ctx context.Context, authPath string, authData map[string]interface{}) error {
	url := strings.TrimSuffix(c.config.Address, "/") + "/v1/" + strings.TrimPrefix(authPath, "/")

	jsonData, err := json.Marshal(authData)
	if err != nil {
		return fmt.Errorf("failed to marshal auth data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.Namespace != "" {
		req.Header.Set("X-Vault-Namespace", c.config.Namespace)
	}

	client := c.getHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make auth request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, string(body))
	}

	var authResp struct {
		Auth struct {
			ClientToken string `json:"client_token"`
		} `json:"auth"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("failed to decode auth response: %w", err)
	}

	if authResp.Auth.ClientToken == "" {
		return fmt.Errorf("no token received from vault")
	}

	c.token = authResp.Auth.ClientToken
	return nil
}

// validateTokenLocked checks if the current token is valid
// Must be called with c.mu held
func (c *HTTPVaultClient) validateTokenLocked(ctx context.Context) error {
	url := strings.TrimSuffix(c.config.Address, "/") + "/v1/auth/token/lookup-self"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create token validation request: %w", err)
	}

	req.Header.Set("X-Vault-Token", c.token)
	if c.config.Namespace != "" {
		req.Header.Set("X-Vault-Namespace", c.config.Namespace)
	}

	client := c.getHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to validate token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return fmt.Errorf("token validation failed with status %d", resp.StatusCode)
	}

	return nil
}

// getHTTPClient creates an HTTP client with appropriate TLS settings
func (c *HTTPVaultClient) getHTTPClient() *http.Client {
	client := &http.Client{
		Timeout: DefaultTimeout,
	}

	// Configure TLS
	if c.config.TLSSkip || c.config.CACert != "" || c.config.ClientCert != "" {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: c.config.TLSSkip,
		}

		// TODO: Add support for custom CA cert and client certificates
		// This would involve loading certificates from files and adding them to tlsConfig

		client.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	}

	return client
}