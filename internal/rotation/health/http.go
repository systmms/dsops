package health

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPHealthConfig holds configuration for HTTP health checks.
type HTTPHealthConfig struct {
	// ResponseTimeEnabled enables response time monitoring.
	ResponseTimeEnabled bool

	// ResponseTimeThreshold is the maximum acceptable response time.
	ResponseTimeThreshold time.Duration

	// ErrorRateEnabled enables error rate monitoring.
	ErrorRateEnabled bool

	// ExpectedStatusCodes are the HTTP status codes considered healthy.
	ExpectedStatusCodes []int

	// Timeout is the HTTP request timeout.
	Timeout time.Duration

	// Headers are custom headers to include in the health check request.
	Headers map[string]string
}

// DefaultHTTPHealthConfig returns the default HTTP health configuration.
func DefaultHTTPHealthConfig() HTTPHealthConfig {
	return HTTPHealthConfig{
		ResponseTimeEnabled:   true,
		ResponseTimeThreshold: 5 * time.Second,
		ErrorRateEnabled:      true,
		ExpectedStatusCodes:   []int{200, 201, 202, 204},
		Timeout:               10 * time.Second,
	}
}

// HTTPClient is the interface for making HTTP requests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// HTTPHealthChecker performs health checks on HTTP endpoints.
type HTTPHealthChecker struct {
	name   string
	config HTTPHealthConfig
	client HTTPClient
}

// NewHTTPHealthChecker creates a new HTTP health checker.
func NewHTTPHealthChecker(name string, config HTTPHealthConfig) *HTTPHealthChecker {
	return &HTTPHealthChecker{
		name:   name,
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// SetClient sets a custom HTTP client for testing.
func (c *HTTPHealthChecker) SetClient(client HTTPClient) {
	c.client = client
}

// Name returns the health checker name.
func (c *HTTPHealthChecker) Name() string {
	return c.name
}

// Protocol returns the protocol type.
func (c *HTTPHealthChecker) Protocol() ProtocolType {
	return ProtocolHTTP
}

// Check performs a health check on the HTTP endpoint.
func (c *HTTPHealthChecker) Check(ctx context.Context, service ServiceConfig) (HealthResult, error) {
	start := time.Now()
	result := HealthResult{
		Healthy:   true,
		Timestamp: start,
		Metadata:  make(map[string]interface{}),
	}

	// Build the health check URL
	endpoint := service.Endpoint
	if endpoint == "" {
		result.Healthy = false
		result.Message = "no endpoint configured"
		result.Duration = time.Since(start)
		return result, fmt.Errorf("no endpoint configured for service %s", service.Name)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		result.Healthy = false
		result.Message = fmt.Sprintf("failed to create request: %v", err)
		result.Duration = time.Since(start)
		return result, err
	}

	// Add custom headers
	for key, value := range c.config.Headers {
		req.Header.Set(key, value)
	}

	// Make request
	resp, err := c.client.Do(req)
	if err != nil {
		result.Healthy = false
		result.Message = fmt.Sprintf("request failed: %v", err)
		result.Duration = time.Since(start)
		result.Metadata["error"] = err.Error()
		return result, nil // Return result without error to let caller decide
	}
	defer resp.Body.Close()

	// Discard body to allow connection reuse
	_, _ = io.Copy(io.Discard, resp.Body)

	responseTime := time.Since(start)
	result.Duration = responseTime
	result.Metadata["status_code"] = resp.StatusCode
	result.Metadata["response_time_ms"] = responseTime.Milliseconds()

	var messages []string

	// Check response time
	if c.config.ResponseTimeEnabled && responseTime > c.config.ResponseTimeThreshold {
		result.Healthy = false
		messages = append(messages, fmt.Sprintf("response time %v exceeds threshold %v",
			responseTime, c.config.ResponseTimeThreshold))
	}

	// Check status code
	if c.config.ErrorRateEnabled {
		statusOK := false
		for _, code := range c.config.ExpectedStatusCodes {
			if resp.StatusCode == code {
				statusOK = true
				break
			}
		}
		if !statusOK {
			result.Healthy = false
			messages = append(messages, fmt.Sprintf("unexpected status code %d", resp.StatusCode))
		}
	}

	// Check rate limit headers if present
	if rateLimitRemaining := resp.Header.Get("X-RateLimit-Remaining"); rateLimitRemaining != "" {
		result.Metadata["rate_limit_remaining"] = rateLimitRemaining
	}
	if rateLimitReset := resp.Header.Get("X-RateLimit-Reset"); rateLimitReset != "" {
		result.Metadata["rate_limit_reset"] = rateLimitReset
	}

	if len(messages) > 0 {
		result.Message = fmt.Sprintf("%v", messages)
	} else if result.Healthy {
		result.Message = fmt.Sprintf("healthy: status %d in %v", resp.StatusCode, responseTime)
	}

	return result, nil
}
