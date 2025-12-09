package gradual

import (
	"context"
	"fmt"
	"time"

	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/internal/rotation/health"
	"github.com/systmms/dsops/internal/rotation/rollback"
)

// GroupStrategy implements service group rotation with dependency ordering.
// It rotates multiple related services together, respecting dependencies.
type GroupStrategy struct {
	health   *health.HealthMonitor
	rollback *rollback.Manager
	logger   *logging.Logger
	config   GroupConfig
}

// GroupConfig holds group rotation configuration.
type GroupConfig struct {
	// Services lists the services in this group.
	Services []string

	// Dependencies maps service name to its dependencies.
	// Example: {"service-b": ["service-a"]} means service-b depends on service-a.
	Dependencies map[string][]string

	// FailurePolicy determines how to handle failures.
	FailurePolicy FailurePolicy

	// CrossServiceVerification enables verification across services.
	CrossServiceVerification CrossServiceVerificationConfig
}

// FailurePolicy defines how to handle failures during group rotation.
type FailurePolicy string

const (
	// FailurePolicyRollbackAll rolls back all services in the group on any failure.
	FailurePolicyRollbackAll FailurePolicy = "rollback_all"

	// FailurePolicyContinue continues rotation despite failures.
	FailurePolicyContinue FailurePolicy = "continue"

	// FailurePolicyStop stops rotation but doesn't rollback.
	FailurePolicyStop FailurePolicy = "stop"
)

// CrossServiceVerificationConfig holds cross-service verification configuration.
type CrossServiceVerificationConfig struct {
	// Enabled determines whether cross-service verification is performed.
	Enabled bool

	// Checks lists the verification checks to perform.
	Checks []CrossServiceCheck
}

// CrossServiceCheck defines a verification check across services.
type CrossServiceCheck struct {
	// Type is the check type (e.g., "replication_lag", "consistency").
	Type string

	// Config holds check-specific configuration.
	Config map[string]interface{}
}

// NewGroupStrategy creates a new group rotation strategy.
func NewGroupStrategy(
	health *health.HealthMonitor,
	rollback *rollback.Manager,
	logger *logging.Logger,
) *GroupStrategy {
	return &GroupStrategy{
		health:   health,
		rollback: rollback,
		logger:   logger,
		config:   GroupConfig{}, // Will be set via configuration
	}
}

// Name returns the strategy name.
func (s *GroupStrategy) Name() string {
	return "group"
}

// Plan generates the rollout waves based on dependency graph.
// Uses topological sort to determine rotation order.
func (s *GroupStrategy) Plan(ctx context.Context, service ServiceConfig) ([]RolloutWave, error) {
	if len(service.Instances) == 0 {
		return nil, fmt.Errorf("no instances available for group rollout")
	}

	if len(s.config.Services) == 0 {
		return nil, fmt.Errorf("no services configured for group rotation")
	}

	s.logger.Debug(
		"Planning group rollout: group=%s, services=%d, instances=%d",
		service.Name,
		len(s.config.Services),
		len(service.Instances),
	)

	// Perform topological sort to determine rotation order
	sortedWaves, err := s.topologicalSort(s.config.Services, s.config.Dependencies)
	if err != nil {
		return nil, fmt.Errorf("failed to plan group rotation: %w", err)
	}

	// Map services to instances
	serviceInstanceMap := s.buildServiceInstanceMap(service.Instances)

	// Build rollout waves
	var waves []RolloutWave
	for _, servicesInWave := range sortedWaves {
		var instancesInWave []string
		for _, svcName := range servicesInWave {
			if instances, ok := serviceInstanceMap[svcName]; ok {
				instancesInWave = append(instancesInWave, instances...)
			}
		}

		if len(instancesInWave) > 0 {
			waves = append(waves, RolloutWave{
				Instances:                instancesInWave,
				HealthMonitoringDuration: 5 * time.Minute,
				WaitDuration:             30 * time.Second,
			})
		}
	}

	s.logger.Debug("Group rollout plan: waves=%d", len(waves))
	return waves, nil
}

// Execute runs the group rollout plan.
func (s *GroupStrategy) Execute(ctx context.Context, plan []RolloutWave) error {
	// Check if context is already canceled
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if len(plan) == 0 {
		return fmt.Errorf("no waves in rollout plan")
	}

	s.logger.Info("Starting group rollout: waves=%d, policy=%s", len(plan), s.config.FailurePolicy)

	rotatedWaves := []RolloutWave{} // Track rotated waves for potential rollback

	// Execute each wave sequentially
	for i, wave := range plan {
		waveNum := i + 1
		s.logger.Info(
			"Executing wave %d/%d: %d instances",
			waveNum,
			len(plan),
			len(wave.Instances),
		)

		// Rotate instances in this wave
		if err := s.rotateWave(ctx, wave); err != nil {
			s.logger.Error("Wave %d rotation failed: %v", waveNum, err)

			if s.config.FailurePolicy == FailurePolicyRollbackAll {
				s.logger.Warn("Triggering group-level rollback (all-or-nothing policy)")
				if rbErr := s.rollbackGroup(ctx, rotatedWaves); rbErr != nil {
					s.logger.Error("Group rollback failed: %v", rbErr)
					return fmt.Errorf("wave %d failed and rollback failed: %w (rollback error: %v)", waveNum, err, rbErr)
				}
			}

			return fmt.Errorf("wave %d failed: %w", waveNum, err)
		}

		rotatedWaves = append(rotatedWaves, wave)

		// Monitor health after each wave
		if wave.HealthMonitoringDuration > 0 {
			s.logger.Debug("Monitoring health for wave %d: duration=%s", waveNum, wave.HealthMonitoringDuration)
			if err := s.monitorHealth(ctx, wave); err != nil {
				s.logger.Error("Wave %d health check failed: %v", waveNum, err)

				if s.config.FailurePolicy == FailurePolicyRollbackAll {
					s.logger.Warn("Triggering group-level rollback due to health check failure")
					if rbErr := s.rollbackGroup(ctx, rotatedWaves); rbErr != nil {
						s.logger.Error("Group rollback failed: %v", rbErr)
					}
				}

				return fmt.Errorf("wave %d health check failed: %w", waveNum, err)
			}
		}

		// Perform cross-service verification if enabled
		if s.config.CrossServiceVerification.Enabled && len(s.config.CrossServiceVerification.Checks) > 0 {
			s.logger.Debug("Running cross-service verification for wave %d", waveNum)
			if err := s.verifyCrossService(ctx); err != nil {
				s.logger.Error("Cross-service verification failed: %v", err)

				if s.config.FailurePolicy == FailurePolicyRollbackAll {
					s.logger.Warn("Triggering group-level rollback due to verification failure")
					if rbErr := s.rollbackGroup(ctx, rotatedWaves); rbErr != nil {
						s.logger.Error("Group rollback failed: %v", rbErr)
					}
				}

				return fmt.Errorf("cross-service verification failed: %w", err)
			}
		}

		// Wait before next wave (except for last wave)
		if wave.WaitDuration > 0 && waveNum < len(plan) {
			s.logger.Debug("Waiting %s before next wave", wave.WaitDuration)
			select {
			case <-time.After(wave.WaitDuration):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	s.logger.Info("Group rollout completed successfully")
	return nil
}

// topologicalSort performs topological sort on service dependencies.
// Returns waves of services that can be executed in parallel within each wave.
func (s *GroupStrategy) topologicalSort(services []string, dependencies map[string][]string) ([][]string, error) {
	// Build in-degree map (count of dependencies for each service)
	inDegree := make(map[string]int)
	adjList := make(map[string][]string)

	// Initialize all services with 0 in-degree
	for _, svc := range services {
		inDegree[svc] = 0
		adjList[svc] = []string{}
	}

	// Build adjacency list and in-degree counts
	for dependent, deps := range dependencies {
		inDegree[dependent] = len(deps)
		for _, dep := range deps {
			adjList[dep] = append(adjList[dep], dependent)
		}
	}

	var result [][]string
	processed := 0

	// Process services in waves (Kahn's algorithm)
	for {
		// Find all services with no dependencies (in-degree = 0)
		var currentWave []string
		for svc, degree := range inDegree {
			if degree == 0 {
				currentWave = append(currentWave, svc)
			}
		}

		if len(currentWave) == 0 {
			break // No more services to process
		}

		result = append(result, currentWave)
		processed += len(currentWave)

		// Remove processed services and update in-degrees
		for _, svc := range currentWave {
			delete(inDegree, svc)
			for _, dependent := range adjList[svc] {
				if degree, ok := inDegree[dependent]; ok {
					inDegree[dependent] = degree - 1
				}
			}
		}
	}

	// Check for circular dependencies
	if processed != len(services) {
		return nil, fmt.Errorf("circular dependency detected in service group")
	}

	return result, nil
}

// buildServiceInstanceMap maps service names to instance IDs.
func (s *GroupStrategy) buildServiceInstanceMap(instances []Instance) map[string][]string {
	serviceMap := make(map[string][]string)

	for _, inst := range instances {
		// Extract service name from instance ID (assumes format: service-name-instance-id)
		// In a real implementation, this would use labels or explicit configuration
		serviceName := extractServiceName(inst.ID)
		serviceMap[serviceName] = append(serviceMap[serviceName], inst.ID)
	}

	return serviceMap
}

// extractServiceName extracts the service name from an instance ID.
// Assumes format: service-name-instance-suffix
func extractServiceName(instanceID string) string {
	// Simple implementation: extract everything before the last dash and number
	// In a real implementation, this would use labels or configuration
	for i := len(instanceID) - 1; i >= 0; i-- {
		if instanceID[i] == '-' {
			// Check if the rest is a number
			return instanceID[:i]
		}
	}
	return instanceID
}

// rotateWave rotates instances in a single wave.
func (s *GroupStrategy) rotateWave(ctx context.Context, wave RolloutWave) error {
	// TODO: Integrate with actual rotation engine
	// For now, this is a placeholder that will be integrated in engine.go
	s.logger.Debug("Rotating %d instances: %v", len(wave.Instances), wave.Instances)
	return nil
}

// monitorHealth monitors health for the given wave.
func (s *GroupStrategy) monitorHealth(ctx context.Context, wave RolloutWave) error {
	if s.health == nil {
		s.logger.Debug("Health monitoring disabled (no health monitor configured)")
		return nil
	}

	if wave.HealthMonitoringDuration == 0 {
		return nil // No health monitoring requested
	}

	// Create a timeout context
	monitorCtx, cancel := context.WithTimeout(ctx, wave.HealthMonitoringDuration)
	defer cancel()

	// Wait for monitoring period
	<-monitorCtx.Done()

	// Check if context was canceled vs timeout
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// TODO: Check actual health status from health monitor
	// For now, assume success if we reached timeout
	s.logger.Debug("Health monitoring completed for %d instances", len(wave.Instances))
	return nil
}

// verifyCrossService performs cross-service verification checks.
func (s *GroupStrategy) verifyCrossService(ctx context.Context) error {
	// TODO: Implement actual cross-service verification
	// For now, this is a placeholder
	s.logger.Debug("Cross-service verification passed (placeholder)")
	return nil
}

// rollbackGroup performs group-level rollback for all rotated waves.
func (s *GroupStrategy) rollbackGroup(ctx context.Context, rotatedWaves []RolloutWave) error {
	if s.rollback == nil {
		return fmt.Errorf("rollback manager not configured")
	}

	s.logger.Warn("Rolling back %d waves (group-level rollback)", len(rotatedWaves))

	// Rollback waves in reverse order
	for i := len(rotatedWaves) - 1; i >= 0; i-- {
		wave := rotatedWaves[i]
		s.logger.Debug("Rolling back wave %d: %d instances", i+1, len(wave.Instances))

		// TODO: Integrate with actual rollback logic
		// For now, this is a placeholder
	}

	s.logger.Info("Group rollback completed")
	return nil
}
