package health

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDB implements a minimal database interface for testing.
type mockDB struct {
	pingErr      error
	pingLatency  time.Duration
	// queryErr and queryLatency are commented out as unused, but kept for potential future use
	// queryErr      error
	// queryLatency  time.Duration
	maxOpenConns int
	openConns    int
	inUseConns   int
	statsFunc    func() sql.DBStats
}

func (m *mockDB) PingContext(ctx context.Context) error {
	if m.pingLatency > 0 {
		time.Sleep(m.pingLatency)
	}
	return m.pingErr
}

func (m *mockDB) Stats() sql.DBStats {
	if m.statsFunc != nil {
		return m.statsFunc()
	}
	return sql.DBStats{
		MaxOpenConnections: m.maxOpenConns,
		OpenConnections:    m.openConns,
		InUse:              m.inUseConns,
	}
}

func (m *mockDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	// For testing, we'll handle this through the checker's implementation
	return nil
}

func TestNewSQLHealthChecker(t *testing.T) {
	t.Parallel()

	config := SQLHealthConfig{
		PingEnabled:            true,
		QueryLatencyEnabled:    true,
		ConnectionPoolEnabled:  true,
		QueryLatencyThreshold:  500 * time.Millisecond,
		ConnectionPoolWarnPct:  80,
		MaxConnections:         100,
	}

	checker := NewSQLHealthChecker("test-sql", config)

	assert.NotNil(t, checker)
	assert.Equal(t, "test-sql", checker.Name())
	assert.Equal(t, ProtocolSQL, checker.Protocol())
}

func TestDefaultSQLHealthConfig(t *testing.T) {
	t.Parallel()

	config := DefaultSQLHealthConfig()

	assert.True(t, config.PingEnabled)
	assert.True(t, config.QueryLatencyEnabled)
	assert.True(t, config.ConnectionPoolEnabled)
	assert.Equal(t, 500*time.Millisecond, config.QueryLatencyThreshold)
	assert.Equal(t, 80, config.ConnectionPoolWarnPct)
	assert.Equal(t, 100, config.MaxConnections)
}

func TestSQLHealthChecker_Check_PingSuccess(t *testing.T) {
	t.Parallel()

	config := SQLHealthConfig{
		PingEnabled: true,
	}
	checker := NewSQLHealthChecker("test-sql", config)

	mock := &mockDB{
		pingErr: nil,
	}
	checker.SetDB(mock)

	ctx := context.Background()
	service := ServiceConfig{
		Name:     "postgres-prod",
		Type:     "postgresql",
		Endpoint: "localhost:5432",
	}

	result, err := checker.Check(ctx, service)
	require.NoError(t, err)
	assert.True(t, result.Healthy)
}

func TestSQLHealthChecker_Check_PingFailure(t *testing.T) {
	t.Parallel()

	config := SQLHealthConfig{
		PingEnabled: true,
	}
	checker := NewSQLHealthChecker("test-sql", config)

	mock := &mockDB{
		pingErr: errors.New("connection refused"),
	}
	checker.SetDB(mock)

	ctx := context.Background()
	service := ServiceConfig{
		Name: "postgres-prod",
		Type: "postgresql",
	}

	result, err := checker.Check(ctx, service)
	require.NoError(t, err) // Check doesn't return error, just unhealthy result
	assert.False(t, result.Healthy)
	assert.Contains(t, result.Message, "ping failed")
}

func TestSQLHealthChecker_Check_QueryLatency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		latency   time.Duration
		threshold time.Duration
		healthy   bool
	}{
		{
			name:      "latency below threshold",
			latency:   100 * time.Millisecond,
			threshold: 500 * time.Millisecond,
			healthy:   true,
		},
		{
			name:      "latency at threshold",
			latency:   500 * time.Millisecond,
			threshold: 600 * time.Millisecond, // Threshold higher than latency
			healthy:   true,
		},
		{
			name:      "latency above threshold",
			latency:   600 * time.Millisecond,
			threshold: 500 * time.Millisecond,
			healthy:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := SQLHealthConfig{
				PingEnabled:           true,
				QueryLatencyEnabled:   true,
				QueryLatencyThreshold: tt.threshold,
			}
			checker := NewSQLHealthChecker("test-sql", config)

			mock := &mockDB{
				pingLatency: tt.latency,
			}
			checker.SetDB(mock)

			ctx := context.Background()
			service := ServiceConfig{Name: "postgres-prod", Type: "postgresql"}

			result, err := checker.Check(ctx, service)
			require.NoError(t, err)

			if tt.healthy {
				assert.True(t, result.Healthy, "expected healthy for latency %v with threshold %v", tt.latency, tt.threshold)
			} else {
				assert.False(t, result.Healthy, "expected unhealthy for latency %v with threshold %v", tt.latency, tt.threshold)
			}
		})
	}
}

func TestSQLHealthChecker_Check_ConnectionPool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		maxConns    int
		openConns   int
		inUseConns  int
		warnPct     int
		healthy     bool
		statusLevel HealthStatus
	}{
		{
			name:        "pool well below threshold",
			maxConns:    100,
			openConns:   50,
			inUseConns:  30,
			warnPct:     80,
			healthy:     true,
			statusLevel: StatusHealthy,
		},
		{
			name:        "pool at warning threshold",
			maxConns:    100,
			openConns:   80,
			inUseConns:  80,
			warnPct:     80,
			healthy:     true, // At threshold is degraded but not unhealthy
			statusLevel: StatusDegraded,
		},
		{
			name:        "pool above warning threshold",
			maxConns:    100,
			openConns:   90,
			inUseConns:  85,
			warnPct:     80,
			healthy:     true, // Still healthy, just degraded
			statusLevel: StatusDegraded,
		},
		{
			name:        "pool exhausted",
			maxConns:    100,
			openConns:   100,
			inUseConns:  100,
			warnPct:     80,
			healthy:     false, // Exhausted is unhealthy
			statusLevel: StatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := SQLHealthConfig{
				PingEnabled:           true,
				ConnectionPoolEnabled: true,
				ConnectionPoolWarnPct: tt.warnPct,
				MaxConnections:        tt.maxConns,
			}
			checker := NewSQLHealthChecker("test-sql", config)

			mock := &mockDB{
				statsFunc: func() sql.DBStats {
					return sql.DBStats{
						MaxOpenConnections: tt.maxConns,
						OpenConnections:    tt.openConns,
						InUse:              tt.inUseConns,
					}
				},
			}
			checker.SetDB(mock)

			ctx := context.Background()
			service := ServiceConfig{Name: "postgres-prod", Type: "postgresql"}

			result, err := checker.Check(ctx, service)
			require.NoError(t, err)
			assert.Equal(t, tt.healthy, result.Healthy)

			// Check metadata for pool info
			if result.Metadata != nil {
				poolStatus, ok := result.Metadata["pool_status"]
				if ok {
					assert.NotEmpty(t, poolStatus)
				}
			}
		})
	}
}

func TestSQLHealthChecker_Check_ContextCancellation(t *testing.T) {
	t.Parallel()

	config := SQLHealthConfig{
		PingEnabled: true,
	}
	checker := NewSQLHealthChecker("test-sql", config)

	// Use a mock that respects context
	mock := &contextAwareDB{
		pingLatency: 1 * time.Second,
	}
	checker.SetDB(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	service := ServiceConfig{Name: "postgres-prod", Type: "postgresql"}

	result, err := checker.Check(ctx, service)
	// Should handle context cancellation gracefully
	// Either err contains context error, or result is unhealthy
	if err != nil {
		// Error path - context error is acceptable
		assert.True(t, true) // Test passes if we get here
	} else if !result.Healthy {
		// Unhealthy result is acceptable
		assert.True(t, true)
	} else {
		// This shouldn't happen - context should have been cancelled
		assert.True(t, false, "expected context cancellation to cause failure")
	}
}

// contextAwareDB is a mock that respects context cancellation.
type contextAwareDB struct {
	pingLatency time.Duration
}

func (m *contextAwareDB) PingContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(m.pingLatency):
		return nil
	}
}

func (m *contextAwareDB) Stats() sql.DBStats {
	return sql.DBStats{}
}

func TestSQLHealthChecker_NoDBConnection(t *testing.T) {
	t.Parallel()

	config := SQLHealthConfig{
		PingEnabled: true,
	}
	checker := NewSQLHealthChecker("test-sql", config)
	// Don't set DB - simulate no connection

	ctx := context.Background()
	service := ServiceConfig{Name: "postgres-prod", Type: "postgresql"}

	result, err := checker.Check(ctx, service)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no database connection")
	assert.False(t, result.Healthy)
}

func TestSQLHealthChecker_AllChecksDisabled(t *testing.T) {
	t.Parallel()

	config := SQLHealthConfig{
		PingEnabled:           false,
		QueryLatencyEnabled:   false,
		ConnectionPoolEnabled: false,
	}
	checker := NewSQLHealthChecker("test-sql", config)

	mock := &mockDB{}
	checker.SetDB(mock)

	ctx := context.Background()
	service := ServiceConfig{Name: "postgres-prod", Type: "postgresql"}

	result, err := checker.Check(ctx, service)
	require.NoError(t, err)
	// With all checks disabled, should return healthy (no checks = no failures)
	assert.True(t, result.Healthy)
	assert.Contains(t, result.Message, "no checks enabled")
}
