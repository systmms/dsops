package health

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SQLHealthConfig holds configuration for SQL health checks.
type SQLHealthConfig struct {
	// PingEnabled enables basic ping check.
	PingEnabled bool

	// QueryLatencyEnabled enables query latency check.
	QueryLatencyEnabled bool

	// QueryLatencyThreshold is the maximum acceptable query latency.
	QueryLatencyThreshold time.Duration

	// ConnectionPoolEnabled enables connection pool monitoring.
	ConnectionPoolEnabled bool

	// ConnectionPoolWarnPct is the percentage threshold for connection pool warning.
	ConnectionPoolWarnPct int

	// MaxConnections is the maximum number of connections allowed.
	MaxConnections int
}

// DefaultSQLHealthConfig returns the default SQL health configuration.
func DefaultSQLHealthConfig() SQLHealthConfig {
	return SQLHealthConfig{
		PingEnabled:           true,
		QueryLatencyEnabled:   true,
		ConnectionPoolEnabled: true,
		QueryLatencyThreshold: 500 * time.Millisecond,
		ConnectionPoolWarnPct: 80,
		MaxConnections:        100,
	}
}

// SQLPinger is the interface for pinging a database.
type SQLPinger interface {
	PingContext(ctx context.Context) error
	Stats() sql.DBStats
}

// SQLHealthChecker performs health checks on SQL databases.
type SQLHealthChecker struct {
	name   string
	config SQLHealthConfig
	db     SQLPinger
}

// NewSQLHealthChecker creates a new SQL health checker.
func NewSQLHealthChecker(name string, config SQLHealthConfig) *SQLHealthChecker {
	return &SQLHealthChecker{
		name:   name,
		config: config,
	}
}

// SetDB sets the database connection for testing with mock.
func (c *SQLHealthChecker) SetDB(db SQLPinger) {
	c.db = db
}

// SetDBConn sets a real database connection.
func (c *SQLHealthChecker) SetDBConn(db *sql.DB) {
	c.db = db
}

// Name returns the health checker name.
func (c *SQLHealthChecker) Name() string {
	return c.name
}

// Protocol returns the protocol type.
func (c *SQLHealthChecker) Protocol() ProtocolType {
	return ProtocolSQL
}

// Check performs a health check on the SQL database.
func (c *SQLHealthChecker) Check(ctx context.Context, service ServiceConfig) (HealthResult, error) {
	start := time.Now()
	result := HealthResult{
		Healthy:   true,
		Timestamp: start,
		Metadata:  make(map[string]interface{}),
	}

	// Check if we have a database connection
	if c.db == nil {
		result.Healthy = false
		result.Message = "no database connection configured"
		result.Duration = time.Since(start)
		return result, fmt.Errorf("no database connection")
	}

	// Check if any checks are enabled
	if !c.config.PingEnabled && !c.config.QueryLatencyEnabled && !c.config.ConnectionPoolEnabled {
		result.Message = "no checks enabled, assuming healthy"
		result.Duration = time.Since(start)
		return result, nil
	}

	var messages []string

	// Ping check
	if c.config.PingEnabled {
		pingStart := time.Now()
		if err := c.db.PingContext(ctx); err != nil {
			result.Healthy = false
			messages = append(messages, fmt.Sprintf("ping failed: %v", err))
		} else {
			pingLatency := time.Since(pingStart)
			result.Metadata["ping_latency_ms"] = pingLatency.Milliseconds()

			// Query latency check (using ping as proxy)
			if c.config.QueryLatencyEnabled && pingLatency > c.config.QueryLatencyThreshold {
				result.Healthy = false
				messages = append(messages, fmt.Sprintf("query latency %v exceeds threshold %v",
					pingLatency, c.config.QueryLatencyThreshold))
			}
		}
	}

	// Connection pool check
	if c.config.ConnectionPoolEnabled {
		stats := c.db.Stats()
		result.Metadata["open_connections"] = stats.OpenConnections
		result.Metadata["in_use_connections"] = stats.InUse
		result.Metadata["max_open_connections"] = stats.MaxOpenConnections

		maxConns := c.config.MaxConnections
		if stats.MaxOpenConnections > 0 {
			maxConns = stats.MaxOpenConnections
		}

		if maxConns > 0 {
			usagePct := (stats.InUse * 100) / maxConns

			// Check for pool exhaustion
			if stats.InUse >= maxConns {
				result.Healthy = false
				result.Metadata["pool_status"] = "exhausted"
				messages = append(messages, fmt.Sprintf("connection pool exhausted: %d/%d", stats.InUse, maxConns))
			} else if usagePct >= c.config.ConnectionPoolWarnPct {
				// Degraded but not unhealthy
				result.Metadata["pool_status"] = "degraded"
				result.Metadata["pool_usage_pct"] = usagePct
				messages = append(messages, fmt.Sprintf("connection pool at %d%% usage", usagePct))
			} else {
				result.Metadata["pool_status"] = "healthy"
				result.Metadata["pool_usage_pct"] = usagePct
			}
		}
	}

	result.Duration = time.Since(start)

	if len(messages) > 0 {
		result.Message = fmt.Sprintf("%v", messages)
	} else if result.Healthy {
		result.Message = "all checks passed"
	}

	return result, nil
}
