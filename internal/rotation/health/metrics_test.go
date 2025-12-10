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

func TestInitMetrics(t *testing.T) {
	// Note: InitMetrics uses sync.Once, so it can only be called once per test run
	// We test the behavior after initialization
	InitMetrics()

	assert.True(t, IsMetricsRegistered())
	assert.NotNil(t, GetRotationStartedTotal())
	assert.NotNil(t, GetRotationCompletedTotal())
	assert.NotNil(t, GetRotationDuration())
	assert.NotNil(t, GetRollbackTotal())
	assert.NotNil(t, GetHealthCheckDuration())
	assert.NotNil(t, GetHealthCheckStatus())
}

func TestRotationMetrics_RecordRotationStarted(t *testing.T) {
	InitMetrics()

	metrics := NewRotationMetrics()
	metrics.RecordRotationStarted("postgres-prod", "production", "two-key")

	// Verify no panic and counter exists
	counter := GetRotationStartedTotal()
	assert.NotNil(t, counter)
}

func TestRotationMetrics_RecordRotationCompleted(t *testing.T) {
	InitMetrics()

	metrics := NewRotationMetrics()
	metrics.RecordRotationCompleted("postgres-prod", "production", "success", 45.5)

	// Verify no panic and metrics exist
	counter := GetRotationCompletedTotal()
	assert.NotNil(t, counter)

	histogram := GetRotationDuration()
	assert.NotNil(t, histogram)
}

func TestRotationMetrics_RecordRollback(t *testing.T) {
	InitMetrics()

	metrics := NewRotationMetrics()
	metrics.RecordRollback("postgres-prod", "automatic")
	metrics.RecordRollback("postgres-prod", "manual")

	counter := GetRollbackTotal()
	assert.NotNil(t, counter)
}

func TestRotationMetrics_RecordHealthCheck(t *testing.T) {
	InitMetrics()

	metrics := NewRotationMetrics()
	metrics.RecordHealthCheck("postgres-prod", "connection", true, 0.05)
	metrics.RecordHealthCheck("postgres-prod", "query_latency", false, 0.5)

	histogram := GetHealthCheckDuration()
	assert.NotNil(t, histogram)

	gauge := GetHealthCheckStatus()
	assert.NotNil(t, gauge)
}

func TestDefaultMetricsServerConfig(t *testing.T) {
	t.Parallel()

	config := DefaultMetricsServerConfig()

	assert.False(t, config.Enabled)
	assert.Equal(t, 9090, config.Port)
	assert.Equal(t, "/metrics", config.Path)
	assert.Equal(t, 5*time.Second, config.ReadTimeout)
	assert.Equal(t, 10*time.Second, config.WriteTimeout)
}

func TestNewMetricsServer(t *testing.T) {
	t.Parallel()

	config := DefaultMetricsServerConfig()
	server := NewMetricsServer(config)

	assert.NotNil(t, server)
	assert.Equal(t, config, server.config)
}

func TestMetricsServer_StartDisabled(t *testing.T) {
	t.Parallel()

	config := DefaultMetricsServerConfig()
	config.Enabled = false
	server := NewMetricsServer(config)

	err := server.Start()
	assert.NoError(t, err)
	assert.Empty(t, server.Addr())
}

func TestMetricsServer_StartEnabled(t *testing.T) {
	// Initialize metrics first
	InitMetrics()

	config := MetricsServerConfig{
		Enabled:      true,
		Port:         0, // Use random port
		Path:         "/metrics",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Use a specific port for testing
	config.Port = 19090 // Use high port to avoid conflicts

	server := NewMetricsServer(config)

	err := server.Start()
	require.NoError(t, err)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Try to fetch metrics
	resp, err := http.Get("http://localhost:19090/metrics")
	if err != nil {
		// Port might be in use, skip test
		t.Skipf("skipping test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Verify some metrics are present
	bodyStr := string(body)
	assert.True(t, strings.Contains(bodyStr, "dsops_") || strings.Contains(bodyStr, "go_"),
		"expected prometheus metrics in response")

	// Stop server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = server.Stop(ctx)
	assert.NoError(t, err)
}

func TestMetricsServer_HealthEndpoint(t *testing.T) {
	InitMetrics()

	config := MetricsServerConfig{
		Enabled:      true,
		Port:         19091,
		Path:         "/metrics",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	server := NewMetricsServer(config)

	err := server.Start()
	require.NoError(t, err)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Try to fetch health endpoint
	resp, err := http.Get("http://localhost:19091/health")
	if err != nil {
		t.Skipf("skipping test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "OK", string(body))

	// Stop server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = server.Stop(ctx)
	assert.NoError(t, err)
}

func TestMetricsServer_StopNilServer(t *testing.T) {
	t.Parallel()

	config := DefaultMetricsServerConfig()
	server := NewMetricsServer(config)

	ctx := context.Background()
	err := server.Stop(ctx)
	assert.NoError(t, err)
}
