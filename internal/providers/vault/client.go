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
	// If we already have a token, validate it
	if c.token != "" {
		if err := c.validateToken(ctx); err == nil {
			return nil // Token is still valid
		}
		// Token is invalid, clear it and re-authenticate
		c.token = ""
	}

	switch c.config.AuthMethod {
	case "token":
		return c.authenticateToken()
	case "userpass":
		return c.authenticateUserpass(ctx)
	case "ldap":
		return c.authenticateLDAP(ctx)
	case "aws":
		return c.authenticateAWS(ctx)
	case "k8s", "kubernetes":
		return c.authenticateKubernetes(ctx)
	default:
		return fmt.Errorf("unsupported auth method: %s", c.config.AuthMethod)
	}
}

// Read fetches a secret from Vault
func (c *HTTPVaultClient) Read(ctx context.Context, path string) (*VaultSecret, error) {
	if c.token == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	url := strings.TrimSuffix(c.config.Address, "/") + "/v1/" + strings.TrimPrefix(path, "/")
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", c.token)
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
	c.token = ""
	return nil
}

// authenticateToken validates or sets the token
func (c *HTTPVaultClient) authenticateToken() error {
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

// authenticateUserpass authenticates using username/password
func (c *HTTPVaultClient) authenticateUserpass(ctx context.Context) error {
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

	return c.performLogin(ctx, fmt.Sprintf("auth/userpass/login/%s", c.config.UserpassUsername), authData)
}

// authenticateLDAP authenticates using LDAP
func (c *HTTPVaultClient) authenticateLDAP(ctx context.Context) error {
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

	return c.performLogin(ctx, fmt.Sprintf("auth/ldap/login/%s", c.config.LDAPUsername), authData)
}

// authenticateAWS authenticates using AWS IAM
func (c *HTTPVaultClient) authenticateAWS(ctx context.Context) error {
	// This is a simplified implementation
	// In practice, you'd need to generate AWS SigV4 signatures
	authData := map[string]interface{}{
		"role": c.config.AWSRole,
		// AWS auth requires iam_http_request_method, iam_request_url, iam_request_body, iam_request_headers
		// This would need proper AWS SigV4 implementation
	}

	return c.performLogin(ctx, "auth/aws/login", authData)
}

// authenticateKubernetes authenticates using Kubernetes service account
func (c *HTTPVaultClient) authenticateKubernetes(ctx context.Context) error {
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

	return c.performLogin(ctx, "auth/kubernetes/login", authData)
}

// performLogin handles the common login workflow
func (c *HTTPVaultClient) performLogin(ctx context.Context, authPath string, authData map[string]interface{}) error {
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

// validateToken checks if the current token is valid
func (c *HTTPVaultClient) validateToken(ctx context.Context) error {
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