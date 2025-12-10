package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"
	"time"
)

// HTTPAPIAdapter implements the Adapter interface for HTTP/REST API services
type HTTPAPIAdapter struct {
	client *http.Client
}

// NewHTTPAPIAdapter creates a new HTTP API protocol adapter
func NewHTTPAPIAdapter() *HTTPAPIAdapter {
	return &HTTPAPIAdapter{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the adapter name
func (a *HTTPAPIAdapter) Name() string {
	return "HTTP API Protocol Adapter"
}

// Type returns the adapter type
func (a *HTTPAPIAdapter) Type() AdapterType {
	return AdapterTypeHTTPAPI
}

// Execute performs an HTTP API operation
func (a *HTTPAPIAdapter) Execute(ctx context.Context, operation Operation, config AdapterConfig) (*Result, error) {
	// Validate configuration
	if err := a.Validate(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Build request based on operation
	req, err := a.buildRequest(ctx, operation, config)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	
	// Add authentication
	if err := a.addAuthentication(req, config); err != nil {
		return nil, fmt.Errorf("failed to add authentication: %w", err)
	}
	
	// Execute request with retries
	resp, err := a.executeWithRetries(req, config)
	if err != nil {
		return &Result{
			Success: false,
			Error:   err.Error(),
		}, err
	}
	defer func() { _ = resp.Body.Close() }()
	
	// Parse response
	result, err := a.parseResponse(resp, operation)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to parse response: %v", err),
		}, err
	}
	
	return result, nil
}

// Validate checks if the configuration is valid
func (a *HTTPAPIAdapter) Validate(config AdapterConfig) error {
	// Check required connection fields
	if config.Connection == nil {
		return fmt.Errorf("connection configuration is required")
	}
	
	baseURL, exists := config.Connection["base_url"]
	if !exists || baseURL == "" {
		return fmt.Errorf("base_url is required in connection configuration")
	}
	
	// Validate auth configuration if present
	if config.Auth != nil {
		authType, exists := config.Auth["type"]
		if !exists {
			return fmt.Errorf("auth type is required when auth is configured")
		}
		
		switch authType {
		case "bearer", "api_key", "basic":
			// Valid auth types
		default:
			return fmt.Errorf("unsupported auth type: %s", authType)
		}
	}
	
	return nil
}

// Capabilities returns what this adapter can do
func (a *HTTPAPIAdapter) Capabilities() Capabilities {
	return Capabilities{
		SupportedActions: []string{"create", "verify", "rotate", "revoke", "list"},
		RequiredConfig:   []string{"base_url"},
		OptionalConfig:   []string{"auth_type", "auth_value", "headers", "timeout", "retries"},
		Features: map[string]bool{
			"authentication": true,
			"retries":        true,
			"templates":      true,
			"json_response":  true,
		},
	}
}

// buildRequest constructs an HTTP request based on the operation
func (a *HTTPAPIAdapter) buildRequest(ctx context.Context, operation Operation, config AdapterConfig) (*http.Request, error) {
	baseURL := config.Connection["base_url"]
	
	// Get endpoint template from service config
	endpointTemplate, err := a.getEndpointTemplate(operation, config)
	if err != nil {
		return nil, err
	}
	
	// Render endpoint with operation parameters
	endpoint, err := a.renderTemplate(endpointTemplate, operation)
	if err != nil {
		return nil, fmt.Errorf("failed to render endpoint template: %w", err)
	}
	
	// Build full URL
	url := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(endpoint, "/")
	
	// Determine HTTP method
	method := a.getHTTPMethod(operation)
	
	// Build request body if needed
	var body io.Reader
	if method != "GET" && method != "DELETE" {
		bodyData, err := a.buildRequestBody(operation, config)
		if err != nil {
			return nil, fmt.Errorf("failed to build request body: %w", err)
		}
		body = bytes.NewReader(bodyData)
	}
	
	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	
	// Add headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	
	// Add custom headers from config
	if headers, ok := config.ServiceConfig["headers"].(map[string]string); ok {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}
	
	return req, nil
}

// getEndpointTemplate retrieves the endpoint template for an operation
func (a *HTTPAPIAdapter) getEndpointTemplate(operation Operation, config AdapterConfig) (string, error) {
	// Look for endpoint configuration in service config
	endpoints, ok := config.ServiceConfig["endpoints"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("endpoints configuration not found")
	}
	
	// Find endpoint for this action
	endpointConfig, ok := endpoints[operation.Action].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("endpoint configuration not found for action %s", operation.Action)
	}
	
	path, ok := endpointConfig["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not found for action %s", operation.Action)
	}
	
	return path, nil
}

// renderTemplate renders a template string with operation data
func (a *HTTPAPIAdapter) renderTemplate(templateStr string, operation Operation) (string, error) {
	tmpl, err := template.New("endpoint").Parse(templateStr)
	if err != nil {
		return "", err
	}
	
	var buf bytes.Buffer
	data := map[string]interface{}{
		"Target":     operation.Target,
		"Action":     operation.Action,
		"Parameters": operation.Parameters,
		"Metadata":   operation.Metadata,
	}
	
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	
	return buf.String(), nil
}

// getHTTPMethod determines the HTTP method for an operation
func (a *HTTPAPIAdapter) getHTTPMethod(operation Operation) string {
	// Default mapping
	methodMap := map[string]string{
		"create": "POST",
		"verify": "GET",
		"rotate": "PUT",
		"revoke": "DELETE",
		"list":   "GET",
	}
	
	if method, ok := methodMap[operation.Action]; ok {
		return method
	}
	
	return "POST" // Default
}

// buildRequestBody creates the request body for an operation
func (a *HTTPAPIAdapter) buildRequestBody(operation Operation, config AdapterConfig) ([]byte, error) {
	// Get body template from service config
	bodyTemplate, err := a.getBodyTemplate(operation, config)
	if err != nil {
		// If no template, use operation parameters directly
		return json.Marshal(operation.Parameters)
	}
	
	// Render body template
	body, err := a.renderTemplate(bodyTemplate, operation)
	if err != nil {
		return nil, fmt.Errorf("failed to render body template: %w", err)
	}
	
	// Validate JSON
	var jsonData interface{}
	if err := json.Unmarshal([]byte(body), &jsonData); err != nil {
		return nil, fmt.Errorf("invalid JSON in rendered body: %w", err)
	}
	
	return []byte(body), nil
}

// getBodyTemplate retrieves the body template for an operation
func (a *HTTPAPIAdapter) getBodyTemplate(operation Operation, config AdapterConfig) (string, error) {
	endpoints, ok := config.ServiceConfig["endpoints"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("endpoints configuration not found")
	}
	
	endpointConfig, ok := endpoints[operation.Action].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("endpoint configuration not found for action %s", operation.Action)
	}
	
	body, ok := endpointConfig["body"].(string)
	if !ok {
		return "", fmt.Errorf("body template not found for action %s", operation.Action)
	}
	
	return body, nil
}

// addAuthentication adds authentication to the request
func (a *HTTPAPIAdapter) addAuthentication(req *http.Request, config AdapterConfig) error {
	if config.Auth == nil {
		return nil // No authentication required
	}
	
	authType := config.Auth["type"]
	authValue, hasValue := config.Auth["value"]
	
	if !hasValue {
		return fmt.Errorf("auth value is required")
	}
	
	switch authType {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+authValue)
		
	case "api_key":
		// Check if API key should go in header or query param
		if location, ok := config.Auth["location"]; ok && location == "query" {
			q := req.URL.Query()
			paramName := config.Auth["param_name"]
			if paramName == "" {
				paramName = "api_key"
			}
			q.Set(paramName, authValue)
			req.URL.RawQuery = q.Encode()
		} else {
			// Default to header
			headerName := config.Auth["header_name"]
			if headerName == "" {
				headerName = "X-API-Key"
			}
			req.Header.Set(headerName, authValue)
		}
		
	case "basic":
		username := config.Auth["username"]
		password := authValue
		req.SetBasicAuth(username, password)
		
	default:
		return fmt.Errorf("unsupported auth type: %s", authType)
	}
	
	return nil
}

// executeWithRetries executes the request with retry logic
func (a *HTTPAPIAdapter) executeWithRetries(req *http.Request, config AdapterConfig) (*http.Response, error) {
	maxRetries := 3
	if config.Retries > 0 {
		maxRetries = config.Retries
	}
	
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Clone request for retry
		reqClone := req.Clone(req.Context())
		if req.Body != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("failed to get request body for retry: %w", err)
			}
			reqClone.Body = body
		}
		
		resp, err := a.client.Do(reqClone)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		
		// Check if response indicates success
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}
		
		// Read error body for better error message
		bodyBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		
		lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
		
		// Don't retry client errors (4xx)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			break
		}
		
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	
	return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries, lastErr)
}

// parseResponse parses the HTTP response into a Result
func (a *HTTPAPIAdapter) parseResponse(resp *http.Response, operation Operation) (*Result, error) {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	result := &Result{
		Success: resp.StatusCode >= 200 && resp.StatusCode < 300,
		Data:    make(map[string]interface{}),
		Metadata: map[string]string{
			"status_code": fmt.Sprintf("%d", resp.StatusCode),
			"action":      operation.Action,
			"target":      operation.Target,
		},
	}
	
	// Try to parse JSON response
	if len(bodyBytes) > 0 {
		var jsonData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &jsonData); err == nil {
			result.Data = jsonData
		} else {
			// If not JSON, store as string
			result.Data["response"] = string(bodyBytes)
		}
	}
	
	// Add response headers to metadata
	for key, values := range resp.Header {
		if len(values) > 0 {
			result.Metadata["header_"+strings.ToLower(key)] = values[0]
		}
	}
	
	return result, nil
}