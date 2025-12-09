package health

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FakeHealthChecker is a test double for HealthChecker.
type FakeHealthChecker struct {
	name     string
	protocol ProtocolType
	results  []HealthResult
	errors   []error
	callIdx  int
	mu       sync.Mutex
}

// NewFakeHealthChecker creates a new fake health checker.
func NewFakeHealthChecker(name string, protocol ProtocolType) *FakeHealthChecker {
	return &FakeHealthChecker{
		name:     name,
		protocol: protocol,
	}
}

// SetResults sets the sequence of results to return.
func (f *FakeHealthChecker) SetResults(results []HealthResult, errs []error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results = results
	f.errors = errs
	f.callIdx = 0
}

func (f *FakeHealthChecker) Name() string {
	return f.name
}

func (f *FakeHealthChecker) Check(ctx context.Context, service ServiceConfig) (HealthResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.results) == 0 {
		return HealthResult{Healthy: true, Timestamp: time.Now()}, nil
	}

	idx := f.callIdx
	if idx >= len(f.results) {
		idx = len(f.results) - 1
	}
	f.callIdx++

	var err error
	if idx < len(f.errors) && f.errors[idx] != nil {
		err = f.errors[idx]
	}

	return f.results[idx], err
}

func (f *FakeHealthChecker) Protocol() ProtocolType {
	return f.protocol
}

// FakeRollbackTrigger tracks rollback calls for testing.
type FakeRollbackTrigger struct {
	TriggerCalled bool
	TriggerCount  int32
	LastService   string
	LastEnv       string
	LastReason    string
	mu            sync.Mutex
}

func (f *FakeRollbackTrigger) TriggerHealthCheckRollback(ctx context.Context, service, env, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.TriggerCalled = true
	atomic.AddInt32(&f.TriggerCount, 1)
	f.LastService = service
	f.LastEnv = env
	f.LastReason = reason
	return nil
}

func TestNewHealthMonitor(t *testing.T) {
	t.Parallel()

	config := DefaultMonitorConfig()
	monitor := NewHealthMonitor(config)

	assert.NotNil(t, monitor)
	assert.Equal(t, config.Interval, monitor.config.Interval)
	assert.Equal(t, config.Period, monitor.config.Period)
	assert.Equal(t, config.FailureThreshold, monitor.config.FailureThreshold)
}

func TestDefaultMonitorConfig(t *testing.T) {
	t.Parallel()

	config := DefaultMonitorConfig()

	assert.Equal(t, 30*time.Second, config.Interval)
	assert.Equal(t, 10*time.Minute, config.Period)
	assert.Equal(t, 3, config.FailureThreshold)
}

func TestHealthMonitor_RegisterChecker(t *testing.T) {
	t.Parallel()

	monitor := NewHealthMonitor(DefaultMonitorConfig())
	checker := NewFakeHealthChecker("test-checker", ProtocolSQL)

	monitor.RegisterChecker(checker)

	assert.Len(t, monitor.checkers, 1)
	assert.Equal(t, checker, monitor.checkers[0])
}

func TestHealthMonitor_StartMonitoring(t *testing.T) {
	t.Parallel()

	config := MonitorConfig{
		Interval:         10 * time.Millisecond,
		Period:           100 * time.Millisecond,
		FailureThreshold: 3,
	}
	monitor := NewHealthMonitor(config)

	checker := NewFakeHealthChecker("test-checker", ProtocolSQL)
	checker.SetResults([]HealthResult{
		{Healthy: true, Timestamp: time.Now()},
	}, nil)
	monitor.RegisterChecker(checker)

	ctx := context.Background()
	service := ServiceConfig{
		Name:     "test-service",
		Type:     "postgresql",
		Endpoint: "localhost:5432",
	}

	err := monitor.StartMonitoring(ctx, service, "production")
	require.NoError(t, err)

	// Verify monitoring is active
	assert.True(t, monitor.IsMonitoring("test-service", "production"))

	// Wait for monitoring period to complete
	time.Sleep(150 * time.Millisecond)

	// Verify monitoring has stopped
	assert.False(t, monitor.IsMonitoring("test-service", "production"))
}

func TestHealthMonitor_StopMonitoring(t *testing.T) {
	t.Parallel()

	config := MonitorConfig{
		Interval:         10 * time.Millisecond,
		Period:           1 * time.Second,
		FailureThreshold: 3,
	}
	monitor := NewHealthMonitor(config)

	checker := NewFakeHealthChecker("test-checker", ProtocolSQL)
	checker.SetResults([]HealthResult{
		{Healthy: true, Timestamp: time.Now()},
	}, nil)
	monitor.RegisterChecker(checker)

	ctx := context.Background()
	service := ServiceConfig{
		Name:     "test-service",
		Type:     "postgresql",
		Endpoint: "localhost:5432",
	}

	err := monitor.StartMonitoring(ctx, service, "production")
	require.NoError(t, err)
	assert.True(t, monitor.IsMonitoring("test-service", "production"))

	// Stop monitoring early
	monitor.StopMonitoring("test-service", "production")

	// Give goroutine time to clean up
	time.Sleep(50 * time.Millisecond)

	assert.False(t, monitor.IsMonitoring("test-service", "production"))
}

func TestHealthMonitor_FailureThreshold(t *testing.T) {
	t.Parallel()

	config := MonitorConfig{
		Interval:         10 * time.Millisecond,
		Period:           500 * time.Millisecond,
		FailureThreshold: 3,
	}
	monitor := NewHealthMonitor(config)

	rollbackTrigger := &FakeRollbackTrigger{}
	monitor.SetRollbackTrigger(rollbackTrigger)

	checker := NewFakeHealthChecker("test-checker", ProtocolSQL)
	// Return unhealthy results to trigger rollback
	checker.SetResults([]HealthResult{
		{Healthy: false, Message: "connection failed", Timestamp: time.Now()},
		{Healthy: false, Message: "connection failed", Timestamp: time.Now()},
		{Healthy: false, Message: "connection failed", Timestamp: time.Now()},
		{Healthy: false, Message: "connection failed", Timestamp: time.Now()},
	}, nil)
	monitor.RegisterChecker(checker)

	ctx := context.Background()
	service := ServiceConfig{
		Name:     "test-service",
		Type:     "postgresql",
		Endpoint: "localhost:5432",
	}

	err := monitor.StartMonitoring(ctx, service, "production")
	require.NoError(t, err)

	// Wait for enough health checks to trigger rollback
	time.Sleep(100 * time.Millisecond)

	// Verify rollback was triggered
	rollbackTrigger.mu.Lock()
	triggered := rollbackTrigger.TriggerCalled
	lastService := rollbackTrigger.LastService
	lastEnv := rollbackTrigger.LastEnv
	rollbackTrigger.mu.Unlock()

	assert.True(t, triggered, "rollback should have been triggered after failure threshold")
	assert.Equal(t, "test-service", lastService)
	assert.Equal(t, "production", lastEnv)
}

func TestHealthMonitor_FailureCounterResets(t *testing.T) {
	t.Parallel()

	config := MonitorConfig{
		Interval:         10 * time.Millisecond,
		Period:           200 * time.Millisecond,
		FailureThreshold: 3,
	}
	monitor := NewHealthMonitor(config)

	rollbackTrigger := &FakeRollbackTrigger{}
	monitor.SetRollbackTrigger(rollbackTrigger)

	checker := NewFakeHealthChecker("test-checker", ProtocolSQL)
	// Alternate between failures and successes (should not trigger rollback)
	checker.SetResults([]HealthResult{
		{Healthy: false, Timestamp: time.Now()},
		{Healthy: false, Timestamp: time.Now()},
		{Healthy: true, Timestamp: time.Now()}, // Reset counter
		{Healthy: false, Timestamp: time.Now()},
		{Healthy: false, Timestamp: time.Now()},
		{Healthy: true, Timestamp: time.Now()}, // Reset counter
	}, nil)
	monitor.RegisterChecker(checker)

	ctx := context.Background()
	service := ServiceConfig{
		Name:     "test-service",
		Type:     "postgresql",
		Endpoint: "localhost:5432",
	}

	err := monitor.StartMonitoring(ctx, service, "production")
	require.NoError(t, err)

	// Wait for monitoring to run through checks
	time.Sleep(150 * time.Millisecond)

	// Verify rollback was NOT triggered
	rollbackTrigger.mu.Lock()
	triggered := rollbackTrigger.TriggerCalled
	rollbackTrigger.mu.Unlock()

	assert.False(t, triggered, "rollback should not be triggered when failures are interrupted by success")
}

func TestHealthMonitor_GetStatus(t *testing.T) {
	t.Parallel()

	config := MonitorConfig{
		Interval:         20 * time.Millisecond,
		Period:           500 * time.Millisecond,
		FailureThreshold: 3,
	}
	monitor := NewHealthMonitor(config)

	checker := NewFakeHealthChecker("test-checker", ProtocolSQL)
	checker.SetResults([]HealthResult{
		{Healthy: true, Timestamp: time.Now()},
	}, nil)
	monitor.RegisterChecker(checker)

	ctx := context.Background()
	service := ServiceConfig{
		Name:     "test-service",
		Type:     "postgresql",
		Endpoint: "localhost:5432",
	}

	// Before monitoring starts, status should be unknown
	status := monitor.GetStatus("test-service", "production")
	assert.Equal(t, StatusUnknown, status)

	err := monitor.StartMonitoring(ctx, service, "production")
	require.NoError(t, err)

	// Wait for at least one check
	time.Sleep(50 * time.Millisecond)

	// After successful check, status should be healthy
	status = monitor.GetStatus("test-service", "production")
	assert.Equal(t, StatusHealthy, status)
}

func TestHealthMonitor_MultipleServices(t *testing.T) {
	t.Parallel()

	config := MonitorConfig{
		Interval:         10 * time.Millisecond,
		Period:           200 * time.Millisecond,
		FailureThreshold: 3,
	}
	monitor := NewHealthMonitor(config)

	checker := NewFakeHealthChecker("test-checker", ProtocolSQL)
	checker.SetResults([]HealthResult{
		{Healthy: true, Timestamp: time.Now()},
	}, nil)
	monitor.RegisterChecker(checker)

	ctx := context.Background()

	service1 := ServiceConfig{Name: "service-1", Type: "postgresql"}
	service2 := ServiceConfig{Name: "service-2", Type: "postgresql"}

	err := monitor.StartMonitoring(ctx, service1, "production")
	require.NoError(t, err)

	err = monitor.StartMonitoring(ctx, service2, "production")
	require.NoError(t, err)

	assert.True(t, monitor.IsMonitoring("service-1", "production"))
	assert.True(t, monitor.IsMonitoring("service-2", "production"))

	// Stop one service
	monitor.StopMonitoring("service-1", "production")
	time.Sleep(20 * time.Millisecond)

	assert.False(t, monitor.IsMonitoring("service-1", "production"))
	assert.True(t, monitor.IsMonitoring("service-2", "production"))
}

func TestHealthMonitor_AlreadyMonitoring(t *testing.T) {
	t.Parallel()

	config := MonitorConfig{
		Interval:         10 * time.Millisecond,
		Period:           1 * time.Second,
		FailureThreshold: 3,
	}
	monitor := NewHealthMonitor(config)

	checker := NewFakeHealthChecker("test-checker", ProtocolSQL)
	monitor.RegisterChecker(checker)

	ctx := context.Background()
	service := ServiceConfig{Name: "test-service", Type: "postgresql"}

	err := monitor.StartMonitoring(ctx, service, "production")
	require.NoError(t, err)

	// Try to start monitoring again - should return error
	err = monitor.StartMonitoring(ctx, service, "production")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already monitoring")

	monitor.StopMonitoring("test-service", "production")
}

func TestHealthMonitor_NoCheckers(t *testing.T) {
	t.Parallel()

	monitor := NewHealthMonitor(DefaultMonitorConfig())

	ctx := context.Background()
	service := ServiceConfig{Name: "test-service", Type: "postgresql"}

	err := monitor.StartMonitoring(ctx, service, "production")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no health checkers")
}

func TestHealthMonitor_CheckerError(t *testing.T) {
	t.Parallel()

	config := MonitorConfig{
		Interval:         10 * time.Millisecond,
		Period:           200 * time.Millisecond,
		FailureThreshold: 3,
	}
	monitor := NewHealthMonitor(config)

	rollbackTrigger := &FakeRollbackTrigger{}
	monitor.SetRollbackTrigger(rollbackTrigger)

	checker := NewFakeHealthChecker("test-checker", ProtocolSQL)
	// Return errors (should count as failures)
	checker.SetResults([]HealthResult{
		{Healthy: false, Timestamp: time.Now()},
		{Healthy: false, Timestamp: time.Now()},
		{Healthy: false, Timestamp: time.Now()},
	}, []error{
		errors.New("connection error"),
		errors.New("connection error"),
		errors.New("connection error"),
	})
	monitor.RegisterChecker(checker)

	ctx := context.Background()
	service := ServiceConfig{Name: "test-service", Type: "postgresql"}

	err := monitor.StartMonitoring(ctx, service, "production")
	require.NoError(t, err)

	// Wait for enough checks to trigger rollback
	time.Sleep(100 * time.Millisecond)

	rollbackTrigger.mu.Lock()
	triggered := rollbackTrigger.TriggerCalled
	rollbackTrigger.mu.Unlock()

	assert.True(t, triggered, "errors should count as failures and trigger rollback")
}
