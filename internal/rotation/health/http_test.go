package health

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHTTPClient implements HTTPClient for testing.
type mockHTTPClient struct {
	response *http.Response
	err      error
	latency  time.Duration
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.latency > 0 {
		time.Sleep(m.latency)
	}
	return m.response, m.err
}

func newMockResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestNewHTTPHealthChecker(t *testing.T) {
	t.Parallel()

	config := DefaultHTTPHealthConfig()
	checker := NewHTTPHealthChecker("test-http", config)

	assert.NotNil(t, checker)
	assert.Equal(t, "test-http", checker.Name())
	assert.Equal(t, ProtocolHTTP, checker.Protocol())
}

func TestDefaultHTTPHealthConfig(t *testing.T) {
	t.Parallel()

	config := DefaultHTTPHealthConfig()

	assert.True(t, config.ResponseTimeEnabled)
	assert.Equal(t, 5*time.Second, config.ResponseTimeThreshold)
	assert.True(t, config.ErrorRateEnabled)
	assert.Contains(t, config.ExpectedStatusCodes, 200)
	assert.Equal(t, 10*time.Second, config.Timeout)
}

func TestHTTPHealthChecker_Check_Success(t *testing.T) {
	t.Parallel()

	config := DefaultHTTPHealthConfig()
	checker := NewHTTPHealthChecker("test-http", config)

	mock := &mockHTTPClient{
		response: newMockResponse(200, "OK"),
	}
	checker.SetClient(mock)

	ctx := context.Background()
	service := ServiceConfig{
		Name:     "api-service",
		Type:     "http",
		Endpoint: "http://localhost:8080/health",
	}

	result, err := checker.Check(ctx, service)
	require.NoError(t, err)
	assert.True(t, result.Healthy)
	assert.Equal(t, 200, result.Metadata["status_code"])
}

func TestHTTPHealthChecker_Check_UnexpectedStatusCode(t *testing.T) {
	t.Parallel()

	config := DefaultHTTPHealthConfig()
	checker := NewHTTPHealthChecker("test-http", config)

	mock := &mockHTTPClient{
		response: newMockResponse(500, "Internal Server Error"),
	}
	checker.SetClient(mock)

	ctx := context.Background()
	service := ServiceConfig{
		Name:     "api-service",
		Type:     "http",
		Endpoint: "http://localhost:8080/health",
	}

	result, err := checker.Check(ctx, service)
	require.NoError(t, err)
	assert.False(t, result.Healthy)
	assert.Contains(t, result.Message, "unexpected status code 500")
}

func TestHTTPHealthChecker_Check_SlowResponse(t *testing.T) {
	t.Parallel()

	config := HTTPHealthConfig{
		ResponseTimeEnabled:   true,
		ResponseTimeThreshold: 50 * time.Millisecond,
		ErrorRateEnabled:      true,
		ExpectedStatusCodes:   []int{200},
		Timeout:               1 * time.Second,
	}
	checker := NewHTTPHealthChecker("test-http", config)

	mock := &mockHTTPClient{
		response: newMockResponse(200, "OK"),
		latency:  100 * time.Millisecond,
	}
	checker.SetClient(mock)

	ctx := context.Background()
	service := ServiceConfig{
		Name:     "api-service",
		Type:     "http",
		Endpoint: "http://localhost:8080/health",
	}

	result, err := checker.Check(ctx, service)
	require.NoError(t, err)
	assert.False(t, result.Healthy)
	assert.Contains(t, result.Message, "response time")
	assert.Contains(t, result.Message, "exceeds threshold")
}

func TestHTTPHealthChecker_Check_ConnectionError(t *testing.T) {
	t.Parallel()

	config := DefaultHTTPHealthConfig()
	checker := NewHTTPHealthChecker("test-http", config)

	mock := &mockHTTPClient{
		err: &mockError{"connection refused"},
	}
	checker.SetClient(mock)

	ctx := context.Background()
	service := ServiceConfig{
		Name:     "api-service",
		Type:     "http",
		Endpoint: "http://localhost:8080/health",
	}

	result, err := checker.Check(ctx, service)
	require.NoError(t, err) // Should not return error, just unhealthy result
	assert.False(t, result.Healthy)
	assert.Contains(t, result.Message, "request failed")
}

func TestHTTPHealthChecker_Check_NoEndpoint(t *testing.T) {
	t.Parallel()

	config := DefaultHTTPHealthConfig()
	checker := NewHTTPHealthChecker("test-http", config)

	ctx := context.Background()
	service := ServiceConfig{
		Name: "api-service",
		Type: "http",
		// No endpoint
	}

	result, err := checker.Check(ctx, service)
	assert.Error(t, err)
	assert.False(t, result.Healthy)
	assert.Contains(t, result.Message, "no endpoint")
}

func TestHTTPHealthChecker_Check_RateLimitHeaders(t *testing.T) {
	t.Parallel()

	config := DefaultHTTPHealthConfig()
	checker := NewHTTPHealthChecker("test-http", config)

	resp := newMockResponse(200, "OK")
	resp.Header.Set("X-RateLimit-Remaining", "100")
	resp.Header.Set("X-RateLimit-Reset", "1609459200")

	mock := &mockHTTPClient{
		response: resp,
	}
	checker.SetClient(mock)

	ctx := context.Background()
	service := ServiceConfig{
		Name:     "api-service",
		Type:     "http",
		Endpoint: "http://localhost:8080/health",
	}

	result, err := checker.Check(ctx, service)
	require.NoError(t, err)
	assert.True(t, result.Healthy)
	assert.Equal(t, "100", result.Metadata["rate_limit_remaining"])
	assert.Equal(t, "1609459200", result.Metadata["rate_limit_reset"])
}

func TestHTTPHealthChecker_Check_CustomHeaders(t *testing.T) {
	t.Parallel()

	config := DefaultHTTPHealthConfig()
	config.Headers = map[string]string{
		"Authorization": "Bearer test-token",
		"X-Custom":      "custom-value",
	}
	checker := NewHTTPHealthChecker("test-http", config)

	// Use a capturing mock that records the request
	captureMock := &capturingHTTPClient{
		response: newMockResponse(200, "OK"),
	}
	checker.SetClient(captureMock)

	ctx := context.Background()
	service := ServiceConfig{
		Name:     "api-service",
		Type:     "http",
		Endpoint: "http://localhost:8080/health",
	}

	result, err := checker.Check(ctx, service)
	require.NoError(t, err)
	assert.True(t, result.Healthy)

	// Verify headers were set
	if captureMock.lastRequest != nil {
		assert.Equal(t, "Bearer test-token", captureMock.lastRequest.Header.Get("Authorization"))
		assert.Equal(t, "custom-value", captureMock.lastRequest.Header.Get("X-Custom"))
	}
}

type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}

type capturingHTTPClient struct {
	response    *http.Response
	err         error
	latency     time.Duration
	lastRequest *http.Request
}

func (c *capturingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	c.lastRequest = req
	if c.latency > 0 {
		time.Sleep(c.latency)
	}
	return c.response, c.err
}
