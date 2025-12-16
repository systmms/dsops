// Package integration provides integration tests for dsops.
package integration

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/rotation/health"
	"github.com/systmms/dsops/tests/testutil"
)

// TestHealthMonitor_PostgreSQL tests health monitoring against a real PostgreSQL instance.
func TestHealthMonitor_PostgreSQL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start PostgreSQL container
	env := testutil.StartDockerEnv(t, []string{"postgres"})
	pg := env.PostgresClient()

	// Create health monitor with fast intervals for testing
	config := health.MonitorConfig{
		Interval:         100 * time.Millisecond,
		Period:           1 * time.Second,
		FailureThreshold: 3,
	}
	monitor := health.NewHealthMonitor(config)

	// Create SQL health checker - use the underlying db from PostgresClient
	sqlConfig := health.DefaultSQLHealthConfig()
	checker := health.NewSQLHealthChecker("postgres-health", sqlConfig)
	// Note: PostgresTestClient wraps *sql.DB internally; we use a new connection for the health checker

	connStr := "host=127.0.0.1 port=5432 user=test password=test-password dbname=testdb sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	checker.SetDBConn(db)
	monitor.RegisterChecker(checker)

	// Start monitoring
	ctx := context.Background()
	service := health.ServiceConfig{
		Name:     "postgres-test",
		Type:     "postgresql",
		Endpoint: connStr,
	}

	err = monitor.StartMonitoring(ctx, service, "test")
	require.NoError(t, err)

	// Verify monitoring is active
	assert.True(t, monitor.IsMonitoring("postgres-test", "test"))

	// Wait for at least one health check
	time.Sleep(200 * time.Millisecond)

	// Check status is healthy
	status := monitor.GetStatus("postgres-test", "test")
	assert.Equal(t, health.StatusHealthy, status)

	// Stop monitoring
	monitor.StopMonitoring("postgres-test", "test")
	time.Sleep(50 * time.Millisecond)

	assert.False(t, monitor.IsMonitoring("postgres-test", "test"))

	// Use pg to keep it referenced
	_ = pg
}

// TestSQLHealthChecker_RealPostgres tests SQL health checker against real PostgreSQL.
func TestSQLHealthChecker_RealPostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start PostgreSQL container
	env := testutil.StartDockerEnv(t, []string{"postgres"})
	_ = env.PostgresClient() // Ensure connection is ready

	// Connect to database
	connStr := "host=127.0.0.1 port=5432 user=test password=test-password dbname=testdb sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	// Create health checker with all checks enabled
	config := health.SQLHealthConfig{
		PingEnabled:           true,
		QueryLatencyEnabled:   true,
		ConnectionPoolEnabled: true,
		QueryLatencyThreshold: 1 * time.Second, // Generous threshold for test
		ConnectionPoolWarnPct: 80,
		MaxConnections:        10,
	}
	checker := health.NewSQLHealthChecker("postgres-health", config)
	checker.SetDBConn(db)

	ctx := context.Background()
	service := health.ServiceConfig{
		Name:     "postgres-test",
		Type:     "postgresql",
		Endpoint: connStr,
	}

	// Run health check
	result, err := checker.Check(ctx, service)
	require.NoError(t, err)

	// Verify health check results
	assert.True(t, result.Healthy)
	assert.NotZero(t, result.Duration)
	assert.NotZero(t, result.Timestamp)

	// Check metadata
	assert.NotNil(t, result.Metadata)
	if result.Metadata != nil {
		// Should have connection pool info
		_, hasPoolInfo := result.Metadata["open_connections"]
		assert.True(t, hasPoolInfo || result.Metadata["pool_status"] != nil)
	}
}

// testRollbackTrigger is a test implementation of RollbackTrigger.
// Commented out as unused, but kept for potential future use
//type testRollbackTrigger struct {
//	onTrigger func(ctx context.Context, service, env, reason string) error
//}
//
//func (t *testRollbackTrigger) TriggerHealthCheckRollback(ctx context.Context, service, env, reason string) error {
//	if t.onTrigger != nil {
//		return t.onTrigger(ctx, service, env, reason)
//	}
//	return nil
//}
