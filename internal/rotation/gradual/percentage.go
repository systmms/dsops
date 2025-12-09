package gradual

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/internal/rotation/health"
)

// PercentageStrategy implements percentage-based gradual rollout.
// It rotates instances in waves based on percentage of total instances.
type PercentageStrategy struct {
	health   *health.HealthMonitor
	logger   *logging.Logger
	config   PercentageConfig
	stateDir string // Directory for persisting rollout state
}

// PercentageConfig holds percentage rollout configuration.
type PercentageConfig struct {
	// Waves defines the rollout waves by percentage.
	Waves []WaveConfig

	// PauseOnFailure determines whether to pause rollout on health check failure.
	// If true, manual approval is required to continue.
	PauseOnFailure bool

	// StateDir is the directory for persisting rollout progress.
	StateDir string
}

// WaveConfig defines a single rollout wave configuration.
type WaveConfig struct {
	// Percentage is the target percentage of instances (cumulative).
	Percentage int

	// HealthMonitoringDuration is how long to monitor health after this wave.
	HealthMonitoringDuration time.Duration

	// WaitDuration is the time to wait before the next wave.
	WaitDuration time.Duration
}

// RolloutProgress tracks the progress of a percentage rollout.
type RolloutProgress struct {
	// ServiceName is the service being rotated.
	ServiceName string

	// Environment is the environment.
	Environment string

	// CurrentWave is the index of the current wave (0-based).
	CurrentWave int

	// TotalWaves is the total number of waves.
	TotalWaves int

	// CompletedInstances lists instances that have been rotated.
	CompletedInstances []string

	// FailedInstances lists instances that failed rotation.
	FailedInstances []string

	// Status is the current rollout status.
	Status RolloutProgressStatus

	// StartTime is when the rollout started.
	StartTime time.Time

	// LastUpdateTime is when progress was last updated.
	LastUpdateTime time.Time

	// PausedReason is why the rollout is paused (if paused).
	PausedReason string
}

// RolloutProgressStatus represents the rollout status.
type RolloutProgressStatus string

const (
	// RolloutStatusInProgress indicates rollout is actively running.
	RolloutStatusInProgress RolloutProgressStatus = "in_progress"

	// RolloutStatusPaused indicates rollout is paused awaiting manual approval.
	RolloutStatusPaused RolloutProgressStatus = "paused"

	// RolloutStatusCompleted indicates rollout finished successfully.
	RolloutStatusCompleted RolloutProgressStatus = "completed"

	// RolloutStatusFailed indicates rollout failed and was aborted.
	RolloutStatusFailed RolloutProgressStatus = "failed"
)

// NewPercentageStrategy creates a new percentage rollout strategy.
func NewPercentageStrategy(
	health *health.HealthMonitor,
	config PercentageConfig,
	logger *logging.Logger,
) *PercentageStrategy {
	// Set default config if not provided
	if len(config.Waves) == 0 {
		config.Waves = defaultPercentageWaves()
	}

	// Set default state directory
	if config.StateDir == "" {
		config.StateDir = filepath.Join(os.TempDir(), "dsops-rollout-state")
	}

	return &PercentageStrategy{
		health:   health,
		logger:   logger,
		config:   config,
		stateDir: config.StateDir,
	}
}

// Name returns the strategy name.
func (s *PercentageStrategy) Name() string {
	return "percentage"
}

// Plan generates the rollout waves based on percentage configuration.
func (s *PercentageStrategy) Plan(ctx context.Context, service ServiceConfig) ([]RolloutWave, error) {
	if len(service.Instances) == 0 {
		return nil, fmt.Errorf("no instances available for percentage rollout")
	}

	s.logger.Debug(
		"Planning percentage rollout: service=%s, instances=%d, waves=%d",
		service.Name,
		len(service.Instances),
		len(s.config.Waves),
	)

	waves := s.calculateWaveInstances(service.Instances, s.config.Waves)

	s.logger.Debug(
		"Percentage rollout plan: service=%s, total_waves=%d",
		service.Name,
		len(waves),
	)

	return waves, nil
}

// Execute runs the percentage rollout plan.
func (s *PercentageStrategy) Execute(ctx context.Context, plan []RolloutWave) error {
	// Check if context is already canceled
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if len(plan) == 0 {
		return fmt.Errorf("no waves in rollout plan")
	}

	s.logger.Info("Starting percentage rollout: waves=%d", len(plan))

	// Execute each wave sequentially
	for i, wave := range plan {
		waveNum := i + 1
		s.logger.Info(
			"Executing wave %d/%d: %d instances (%d%%)",
			waveNum,
			len(plan),
			len(wave.Instances),
			wave.Percentage,
		)

		// Rotate instances in this wave
		if err := s.rotateWave(ctx, wave); err != nil {
			s.logger.Error("Wave %d rotation failed: %v", waveNum, err)

			if s.config.PauseOnFailure {
				s.logger.Warn("Rollout paused due to failure (pause_on_failure=true)")
				// In a real implementation, this would pause and wait for manual approval
				// For now, we just return the error
				return fmt.Errorf("wave %d failed (rollout paused): %w", waveNum, err)
			}

			return fmt.Errorf("wave %d failed: %w", waveNum, err)
		}

		// Monitor health after each wave
		if wave.HealthMonitoringDuration > 0 {
			s.logger.Debug("Monitoring health for wave %d: duration=%s", waveNum, wave.HealthMonitoringDuration)
			if err := s.monitorHealth(ctx, wave); err != nil {
				s.logger.Error("Wave %d health check failed: %v", waveNum, err)

				if s.config.PauseOnFailure {
					s.logger.Warn("Rollout paused due to health check failure")
					return fmt.Errorf("wave %d health check failed (rollout paused): %w", waveNum, err)
				}

				return fmt.Errorf("wave %d health check failed: %w", waveNum, err)
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

	s.logger.Info("Percentage rollout completed successfully")
	return nil
}

// calculateWaveInstances calculates which instances go in each wave based on percentages.
func (s *PercentageStrategy) calculateWaveInstances(instances []Instance, waveConfigs []WaveConfig) []RolloutWave {
	totalInstances := len(instances)
	var waves []RolloutWave
	rotatedCount := 0

	for _, wc := range waveConfigs {
		// Calculate how many instances should be rotated by this percentage
		targetCount := (totalInstances * wc.Percentage) / 100
		if targetCount == 0 {
			targetCount = 1 // Always rotate at least 1 instance per wave
		}

		// Don't exceed total instances
		if targetCount > totalInstances {
			targetCount = totalInstances
		}

		// Calculate how many instances to add in this wave
		waveCount := targetCount - rotatedCount
		if waveCount <= 0 {
			continue // Skip if no new instances to rotate
		}

		// Don't exceed remaining instances
		if rotatedCount+waveCount > totalInstances {
			waveCount = totalInstances - rotatedCount
		}

		if waveCount == 0 {
			continue
		}

		// Select instances for this wave
		waveInstances := make([]string, waveCount)
		for i := 0; i < waveCount; i++ {
			waveInstances[i] = instances[rotatedCount+i].ID
		}

		waves = append(waves, RolloutWave{
			Instances:                waveInstances,
			Percentage:               wc.Percentage,
			WaitDuration:             wc.WaitDuration,
			HealthMonitoringDuration: wc.HealthMonitoringDuration,
		})

		rotatedCount += waveCount

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
			HealthMonitoringDuration: 2 * time.Minute,
		})
	}

	return waves
}

// rotateWave rotates instances in a single wave.
func (s *PercentageStrategy) rotateWave(ctx context.Context, wave RolloutWave) error {
	// TODO: Integrate with actual rotation engine
	// For now, this is a placeholder that will be integrated in engine.go
	s.logger.Debug("Rotating %d instances: %v", len(wave.Instances), wave.Instances)
	return nil
}

// monitorHealth monitors health for the given wave.
func (s *PercentageStrategy) monitorHealth(ctx context.Context, wave RolloutWave) error {
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

// SaveProgress persists rollout progress to disk for resumption.
func (s *PercentageStrategy) SaveProgress(progress RolloutProgress) error {
	// Ensure state directory exists
	if err := os.MkdirAll(s.stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Update last update time
	progress.LastUpdateTime = time.Now()

	// Serialize progress to JSON
	data, err := json.MarshalIndent(progress, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal progress: %w", err)
	}

	// Write to file
	filename := filepath.Join(s.stateDir, fmt.Sprintf("%s-%s.json", progress.ServiceName, progress.Environment))
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write progress file: %w", err)
	}

	s.logger.Debug("Saved rollout progress: %s", filename)
	return nil
}

// LoadProgress loads rollout progress from disk.
func (s *PercentageStrategy) LoadProgress(serviceName, environment string) (RolloutProgress, error) {
	filename := filepath.Join(s.stateDir, fmt.Sprintf("%s-%s.json", serviceName, environment))

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return RolloutProgress{}, fmt.Errorf("no saved progress found for %s/%s", serviceName, environment)
		}
		return RolloutProgress{}, fmt.Errorf("failed to read progress file: %w", err)
	}

	var progress RolloutProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		return RolloutProgress{}, fmt.Errorf("failed to unmarshal progress: %w", err)
	}

	s.logger.Debug("Loaded rollout progress: %s", filename)
	return progress, nil
}

// ClearProgress removes saved progress for a service/environment.
func (s *PercentageStrategy) ClearProgress(serviceName, environment string) error {
	filename := filepath.Join(s.stateDir, fmt.Sprintf("%s-%s.json", serviceName, environment))
	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove progress file: %w", err)
	}
	s.logger.Debug("Cleared rollout progress: %s", filename)
	return nil
}

// defaultPercentageWaves returns default percentage wave configuration.
func defaultPercentageWaves() []WaveConfig {
	return []WaveConfig{
		{Percentage: 5, HealthMonitoringDuration: 2 * time.Minute, WaitDuration: 30 * time.Second},
		{Percentage: 25, HealthMonitoringDuration: 5 * time.Minute, WaitDuration: 1 * time.Minute},
		{Percentage: 50, HealthMonitoringDuration: 5 * time.Minute, WaitDuration: 2 * time.Minute},
		{Percentage: 100, HealthMonitoringDuration: 10 * time.Minute},
	}
}
