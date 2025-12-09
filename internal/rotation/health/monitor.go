package health

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RollbackTrigger defines the interface for triggering rollbacks from health failures.
type RollbackTrigger interface {
	TriggerHealthCheckRollback(ctx context.Context, service, environment, reason string) error
}

// MonitorConfig holds configuration for the health monitor.
type MonitorConfig struct {
	// Interval is how often health checks are performed.
	// Default: 30 seconds
	Interval time.Duration

	// Period is how long to monitor after rotation.
	// Default: 10 minutes
	Period time.Duration

	// FailureThreshold is the number of consecutive failures before triggering rollback.
	// Default: 3
	FailureThreshold int
}

// DefaultMonitorConfig returns the default monitor configuration.
func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		Interval:         30 * time.Second,
		Period:           10 * time.Minute,
		FailureThreshold: 3,
	}
}

// monitorState holds the state for a single service being monitored.
type monitorState struct {
	service          ServiceConfig
	environment      string
	consecutiveFails int
	lastStatus       HealthStatus
	lastCheck        time.Time
	startTime        time.Time
	cancel           context.CancelFunc
	done             chan struct{}
}

// HealthMonitor coordinates health checks after rotation.
type HealthMonitor struct {
	config          MonitorConfig
	checkers        []HealthChecker
	rollbackTrigger RollbackTrigger
	monitors        map[string]*monitorState
	mu              sync.RWMutex
}

// NewHealthMonitor creates a new health monitor with the given configuration.
func NewHealthMonitor(config MonitorConfig) *HealthMonitor {
	return &HealthMonitor{
		config:   config,
		checkers: make([]HealthChecker, 0),
		monitors: make(map[string]*monitorState),
	}
}

// RegisterChecker adds a health checker to the monitor.
func (m *HealthMonitor) RegisterChecker(checker HealthChecker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkers = append(m.checkers, checker)
}

// SetRollbackTrigger sets the rollback trigger for health failures.
func (m *HealthMonitor) SetRollbackTrigger(trigger RollbackTrigger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rollbackTrigger = trigger
}

// monitorKey generates a unique key for a service/environment combination.
func monitorKey(service, environment string) string {
	return fmt.Sprintf("%s/%s", service, environment)
}

// StartMonitoring begins health monitoring for a service after rotation.
func (m *HealthMonitor) StartMonitoring(ctx context.Context, service ServiceConfig, environment string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := monitorKey(service.Name, environment)

	// Check if already monitoring
	if _, exists := m.monitors[key]; exists {
		return fmt.Errorf("already monitoring %s", key)
	}

	// Check if we have any checkers
	if len(m.checkers) == 0 {
		return fmt.Errorf("no health checkers registered")
	}

	// Create cancelable context
	monitorCtx, cancel := context.WithCancel(ctx)

	state := &monitorState{
		service:          service,
		environment:      environment,
		consecutiveFails: 0,
		lastStatus:       StatusUnknown,
		startTime:        time.Now(),
		cancel:           cancel,
		done:             make(chan struct{}),
	}

	m.monitors[key] = state

	// Start background monitoring goroutine
	go m.runMonitor(monitorCtx, state, key)

	return nil
}

// StopMonitoring stops health monitoring for a service.
func (m *HealthMonitor) StopMonitoring(service, environment string) {
	m.mu.Lock()
	key := monitorKey(service, environment)
	state, exists := m.monitors[key]
	m.mu.Unlock()

	if !exists {
		return
	}

	// Cancel the monitoring goroutine
	state.cancel()

	// Wait for goroutine to finish
	<-state.done

	// Remove from monitors
	m.mu.Lock()
	delete(m.monitors, key)
	m.mu.Unlock()
}

// IsMonitoring returns true if the service is currently being monitored.
func (m *HealthMonitor) IsMonitoring(service, environment string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := monitorKey(service, environment)
	_, exists := m.monitors[key]
	return exists
}

// GetStatus returns the current health status for a service.
func (m *HealthMonitor) GetStatus(service, environment string) HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := monitorKey(service, environment)
	state, exists := m.monitors[key]
	if !exists {
		return StatusUnknown
	}

	return state.lastStatus
}

// runMonitor runs the monitoring loop for a service.
func (m *HealthMonitor) runMonitor(ctx context.Context, state *monitorState, key string) {
	defer close(state.done)

	ticker := time.NewTicker(m.config.Interval)
	defer ticker.Stop()

	// Create a timer for the monitoring period
	periodTimer := time.NewTimer(m.config.Period)
	defer periodTimer.Stop()

	// Run initial check immediately
	m.runHealthChecks(ctx, state, key)

	for {
		select {
		case <-ctx.Done():
			return

		case <-periodTimer.C:
			// Monitoring period complete
			m.cleanupMonitor(key)
			return

		case <-ticker.C:
			m.runHealthChecks(ctx, state, key)
		}
	}
}

// runHealthChecks runs all registered health checkers.
func (m *HealthMonitor) runHealthChecks(ctx context.Context, state *monitorState, key string) {
	m.mu.RLock()
	checkers := m.checkers
	m.mu.RUnlock()

	allHealthy := true
	var failureMessages []string

	for _, checker := range checkers {
		result, err := checker.Check(ctx, state.service)
		if err != nil || !result.Healthy {
			allHealthy = false
			if err != nil {
				failureMessages = append(failureMessages, fmt.Sprintf("%s: %v", checker.Name(), err))
			} else {
				failureMessages = append(failureMessages, fmt.Sprintf("%s: %s", checker.Name(), result.Message))
			}
		}
	}

	m.mu.Lock()
	state.lastCheck = time.Now()

	if allHealthy {
		state.consecutiveFails = 0
		state.lastStatus = StatusHealthy
	} else {
		state.consecutiveFails++
		state.lastStatus = StatusUnhealthy
	}

	failCount := state.consecutiveFails
	m.mu.Unlock()

	// Check if we've exceeded the failure threshold
	if failCount >= m.config.FailureThreshold {
		m.triggerRollback(ctx, state, failureMessages)
	}
}

// triggerRollback triggers a rollback due to health check failures.
func (m *HealthMonitor) triggerRollback(ctx context.Context, state *monitorState, failureMessages []string) {
	m.mu.RLock()
	trigger := m.rollbackTrigger
	m.mu.RUnlock()

	if trigger == nil {
		return
	}

	reason := fmt.Sprintf("health check failed %d consecutive times", m.config.FailureThreshold)
	if len(failureMessages) > 0 {
		reason = fmt.Sprintf("%s: %v", reason, failureMessages)
	}

	// Trigger rollback (non-blocking)
	go func() {
		_ = trigger.TriggerHealthCheckRollback(ctx, state.service.Name, state.environment, reason)
	}()

	// Stop monitoring after triggering rollback
	m.mu.Lock()
	key := monitorKey(state.service.Name, state.environment)
	delete(m.monitors, key)
	m.mu.Unlock()

	state.cancel()
}

// cleanupMonitor removes a monitor after its period expires.
func (m *HealthMonitor) cleanupMonitor(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.monitors, key)
}

// GetAllStatuses returns the health status for all monitored services.
func (m *HealthMonitor) GetAllStatuses() map[string]HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make(map[string]HealthStatus)
	for key, state := range m.monitors {
		statuses[key] = state.lastStatus
	}
	return statuses
}

// Close stops all monitoring and releases resources.
func (m *HealthMonitor) Close() {
	m.mu.Lock()
	keys := make([]string, 0, len(m.monitors))
	for key := range m.monitors {
		keys = append(keys, key)
	}
	m.mu.Unlock()

	for _, key := range keys {
		// Parse service and environment from key
		var service, env string
		fmt.Sscanf(key, "%s/%s", &service, &env)
		m.StopMonitoring(service, env)
	}
}
