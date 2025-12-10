package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/systmms/dsops/internal/rotation/gradual"
)

// EndpointProvider discovers instances by querying an HTTP endpoint.
// The endpoint should return a JSON array of instances.
type EndpointProvider struct {
	httpClient *http.Client
}

// EndpointResponse represents the expected JSON response from the discovery endpoint.
type EndpointResponse struct {
	Instances []EndpointInstance `json:"instances"`
}

// EndpointInstance represents an instance in the endpoint response.
type EndpointInstance struct {
	ID       string            `json:"id"`
	Endpoint string            `json:"endpoint,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

// NewEndpointProvider creates a new endpoint discovery provider.
func NewEndpointProvider() *EndpointProvider {
	return &EndpointProvider{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider name.
func (p *EndpointProvider) Name() string {
	return "endpoint"
}

// Discover returns the list of instances from an HTTP endpoint.
// The endpoint should return JSON in the format:
//
//	{
//	  "instances": [
//	    {"id": "instance-1", "endpoint": "http://...", "labels": {...}},
//	    {"id": "instance-2", "endpoint": "http://...", "labels": {...}}
//	  ]
//	}
func (p *EndpointProvider) Discover(ctx context.Context, configIface interface{}) ([]gradual.Instance, error) {
	config, ok := configIface.(Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type for endpoint discovery: expected Config, got %T", configIface)
	}

	if config.Endpoint == "" {
		return nil, fmt.Errorf("endpoint URL is required for endpoint discovery")
	}

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", config.Endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "dsops/1.0")

	// Make request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("endpoint returned non-200 status: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse JSON response
	var endpointResp EndpointResponse
	if err := json.Unmarshal(body, &endpointResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Convert to gradual.Instance format
	instances := make([]gradual.Instance, 0, len(endpointResp.Instances))
	for _, inst := range endpointResp.Instances {
		if inst.ID == "" {
			continue // Skip instances without ID
		}

		instances = append(instances, gradual.Instance{
			ID:       inst.ID,
			Labels:   inst.Labels,
			Endpoint: inst.Endpoint,
		})
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("endpoint returned no valid instances")
	}

	return instances, nil
}

// Validate checks if the configuration is valid for endpoint discovery.
func (p *EndpointProvider) Validate(configIface interface{}) error {
	config, ok := configIface.(Config)
	if !ok {
		return fmt.Errorf("invalid config type for endpoint discovery: expected Config, got %T", configIface)
	}

	if config.Type != "endpoint" && config.Type != "" {
		return fmt.Errorf("invalid discovery type for endpoint provider: %s", config.Type)
	}

	if config.Endpoint == "" {
		return fmt.Errorf("endpoint URL is required")
	}

	// Validate URL format
	if config.Endpoint[:4] != "http" {
		return fmt.Errorf("endpoint must be a valid HTTP or HTTPS URL")
	}

	return nil
}
