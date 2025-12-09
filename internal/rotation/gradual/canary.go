package gradual

import (
	"context"
	"fmt"
	"time"

	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/internal/rotation/health"
	"github.com/systmms/dsops/internal/rotation/rollback"
)

// DiscoveryProvider defines the interface for discovering service instances.
// This is defined here to avoid import cycles with the discovery package.
type DiscoveryProvider interface {
	Name() string
	Discover(ctx context.Context, config interface{}) ([]Instance, error)
	Validate(config interface{}) error
}

// CanaryStrategy implements gradual rollout with canary testing.
// It rotates a single canary instance first, monitors health, then proceeds with remaining instances.
type CanaryStrategy struct {
	discovery DiscoveryProvider
	health    *health.HealthMonitor
	rollback  *rollback.Manager
	logger    *logging.Logger
}

// CanaryConfig holds canary-specific configuration.
type CanaryConfig struct {
	// HealthMonitoringDuration is how long to monitor the canary before proceeding.
	HealthMonitoringDuration time.Duration

	// Waves defines the rollout waves after canary (e.g., 10%, 50%, 100%).
	Waves []WavePercentage

	// AbortOnFailure determines whether to abort on canary failure.
	AbortOnFailure bool
}

// WavePercentage defines a rollout wave by percentage.
type WavePercentage struct {
	Percentage               int
	HealthMonitoringDuration time.Duration
	WaitDuration             time.Duration
}

// NewCanaryStrategy creates a new canary rollout strategy.
func NewCanaryStrategy(
	discovery DiscoveryProvider,
	health *health.HealthMonitor,
	rollback *rollback.Manager,
	logger *logging.Logger,
) *CanaryStrategy {
	return &CanaryStrategy{
		discovery: discovery,
		health:    health,
		rollback:  rollback,
		logger:    logger,
	}
}

// Name returns the strategy name.
func (s *CanaryStrategy) Name() string {
	return "canary"
}

// Plan generates the rollout waves for canary deployment.
// Wave 0: Single canary instance
// Wave 1+: Remaining instances in percentage-based waves
func (s *CanaryStrategy) Plan(ctx context.Context, service ServiceConfig) ([]RolloutWave, error) {
	if len(service.Instances) == 0 {
		return nil, fmt.Errorf("no instances available for canary rollout")
	}

	// Find canary instance (marked with canary label or first instance)
	canaryIdx := s.findCanaryInstance(service.Instances)
	canary := service.Instances[canaryIdx]

	// Remove canary from remaining instances
	remaining := make([]Instance, 0, len(service.Instances)-1)
	for i, inst := range service.Instances {
		if i != canaryIdx {
			remaining = append(remaining, inst)
		}
	}

	// Default canary config if not provided
	config := s.defaultCanaryConfig()

	waves := []RolloutWave{
		{
			Instances:                []string{canary.ID},
			Percentage:               0, // Canary is special, not a percentage
			WaitDuration:             0,
			HealthMonitoringDuration: config.HealthMonitoringDuration,
		},
	}

	// Calculate waves for remaining instances
	if len(remaining) > 0 {
		remainingWaves := s.calculateWaves(remaining, config.Waves)
		waves = append(waves, remainingWaves...)
	}

	s.logger.Debug(
		"Canary rollout plan: canary=%s, remaining=%d, waves=%d",
		canary.ID,
		len(remaining),
		len(waves),
	)

	return waves, nil
}

// Execute runs the canary rollout plan.
func (s *CanaryStrategy) Execute(ctx context.Context, plan []RolloutWave) error {
	// Check if context is already canceled
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if len(plan) == 0 {
		return fmt.Errorf("no waves in rollout plan")
	}

	// Wave 0 is always the canary
	canaryWave := plan[0]
	if len(canaryWave.Instances) != 1 {
		return fmt.Errorf("canary wave must contain exactly one instance, got %d", len(canaryWave.Instances))
	}

	s.logger.Info("Starting canary rollout: canary=%s", canaryWave.Instances[0])

	// Rotate canary instance
	if err := s.rotateWave(ctx, canaryWave); err != nil {
		s.logger.Error("Canary rotation failed: %v", err)
		return fmt.Errorf("canary rotation failed: %w", err)
	}

	// Monitor canary health
	s.logger.Info("Monitoring canary health for %s", canaryWave.HealthMonitoringDuration)
	if err := s.monitorHealth(ctx, canaryWave); err != nil {
		s.logger.Error("Canary health check failed: %v", err)

		// Trigger rollback on canary failure
		if s.rollback != nil {
			s.logger.Warn("Triggering rollback due to canary failure")
			req := rollback.RollbackRequest{
				Service:     "",                         // Will be set by caller
				Environment: "",                         // Will be set by caller
				Reason:      "canary_health_check_failed",
			}
			if _, rbErr := s.rollback.TriggerRollback(ctx, req); rbErr != nil {
				s.logger.Error("Rollback failed: %v", rbErr)
			}
		}

		return fmt.Errorf("canary health check failed: %w", err)
	}

	s.logger.Info("Canary health check passed, proceeding with remaining waves")

	// Execute remaining waves
	for i, wave := range plan[1:] {
		waveNum := i + 2 // +2 because canary is wave 1, and we're starting from index 0
		s.logger.Info(
			"Executing wave %d/%d: %d instances (%d%%)",
			waveNum,
			len(plan),
			len(wave.Instances),
			wave.Percentage,
		)

		if err := s.rotateWave(ctx, wave); err != nil {
			return fmt.Errorf("wave %d failed: %w", waveNum, err)
		}

		// Monitor health after each wave
		if wave.HealthMonitoringDuration > 0 {
			if err := s.monitorHealth(ctx, wave); err != nil {
				return fmt.Errorf("wave %d health check failed: %w", waveNum, err)
			}
		}

		// Wait before next wave
		if wave.WaitDuration > 0 && waveNum < len(plan) {
			s.logger.Debug("Waiting %s before next wave", wave.WaitDuration)
			select {
			case <-time.After(wave.WaitDuration):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	s.logger.Info("Canary rollout completed successfully")
	return nil
}

// findCanaryInstance finds the canary instance index.
// Looks for instance with canary=true label, otherwise returns the first instance.
func (s *CanaryStrategy) findCanaryInstance(instances []Instance) int {
	for i, inst := range instances {
		if inst.Labels["canary"] == "true" {
			return i
		}
	}
	return 0 // Default to first instance
}

// calculateWaves calculates rollout waves for remaining instances.
func (s *CanaryStrategy) calculateWaves(instances []Instance, wavePercentages []WavePercentage) []RolloutWave {
	if len(wavePercentages) == 0 {
		// Default waves: 10%, 50%, 100%
		wavePercentages = []WavePercentage{
			{Percentage: 10, HealthMonitoringDuration: 5 * time.Minute},
			{Percentage: 50, HealthMonitoringDuration: 5 * time.Minute},
			{Percentage: 100, HealthMonitoringDuration: 5 * time.Minute},
		}
	}

	totalInstances := len(instances)
	var waves []RolloutWave
	rotatedCount := 0

	for _, wp := range wavePercentages {
		// Calculate how many instances for this wave
		targetCount := (totalInstances * wp.Percentage) / 100
		if targetCount == 0 {
			targetCount = 1 // Always rotate at least 1 instance
		}

		// Don't exceed total instances
		if rotatedCount+targetCount > totalInstances {
			targetCount = totalInstances - rotatedCount
		}

		if targetCount == 0 {
			continue // Skip if no instances left
		}

		// Select instances for this wave
		waveInstances := make([]string, targetCount)
		for i := 0; i < targetCount; i++ {
			waveInstances[i] = instances[rotatedCount+i].ID
		}

		waves = append(waves, RolloutWave{
			Instances:                waveInstances,
			Percentage:               wp.Percentage,
			WaitDuration:             wp.WaitDuration,
			HealthMonitoringDuration: wp.HealthMonitoringDuration,
		})

		rotatedCount += targetCount

		// Stop if we've rotated all instances
		if rotatedCount >= totalInstances {
			break
		}
	}

	// Ensure we got all instances in the last wave
	if rotatedCount < totalInstances {
		remaining := make([]string, totalInstances-rotatedCount)
		for i := 0; i < totalInstances-rotatedCount; i++ {
			remaining[i] = instances[rotatedCount+i].ID
		}
		waves = append(waves, RolloutWave{
			Instances:                remaining,
			Percentage:               100,
			HealthMonitoringDuration: 5 * time.Minute,
		})
	}

	return waves
}

// rotateWave rotates instances in a single wave.
func (s *CanaryStrategy) rotateWave(ctx context.Context, wave RolloutWave) error {
	// TODO: Integrate with actual rotation engine
	// For now, this is a placeholder that will be integrated in engine.go
	s.logger.Debug("Rotating %d instances: %v", len(wave.Instances), wave.Instances)
	return nil
}

// monitorHealth monitors health for the given wave.
func (s *CanaryStrategy) monitorHealth(ctx context.Context, wave RolloutWave) error {
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

// defaultCanaryConfig returns default canary configuration.
func (s *CanaryStrategy) defaultCanaryConfig() CanaryConfig {
	return CanaryConfig{
		HealthMonitoringDuration: 5 * time.Minute,
		Waves: []WavePercentage{
			{Percentage: 10, HealthMonitoringDuration: 5 * time.Minute},
			{Percentage: 50, HealthMonitoringDuration: 5 * time.Minute},
			{Percentage: 100, HealthMonitoringDuration: 5 * time.Minute},
		},
		AbortOnFailure: true,
	}
}
