package gradual

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/logging"
)

func TestPercentageStrategy_Name(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	strategy := NewPercentageStrategy(nil, PercentageConfig{}, logger)

	assert.Equal(t, "percentage", strategy.Name())
}

func TestPercentageStrategy_Plan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		instances        []Instance
		wavePercentages  []int
		wantWaveCount    int
		wantTotalRotated int
		wantErr          bool
		errMsg           string
	}{
		{
			name: "default waves (5%, 25%, 50%, 100%)",
			instances: []Instance{
				{ID: "instance-1"}, {ID: "instance-2"}, {ID: "instance-3"}, {ID: "instance-4"},
				{ID: "instance-5"}, {ID: "instance-6"}, {ID: "instance-7"}, {ID: "instance-8"},
				{ID: "instance-9"}, {ID: "instance-10"}, {ID: "instance-11"}, {ID: "instance-12"},
				{ID: "instance-13"}, {ID: "instance-14"}, {ID: "instance-15"}, {ID: "instance-16"},
				{ID: "instance-17"}, {ID: "instance-18"}, {ID: "instance-19"}, {ID: "instance-20"},
			},
			wavePercentages:  []int{5, 25, 50, 100},
			wantWaveCount:    4,
			wantTotalRotated: 20,
			wantErr:          false,
		},
		{
			name: "custom waves (10%, 40%, 100%)",
			instances: []Instance{
				{ID: "instance-1"}, {ID: "instance-2"}, {ID: "instance-3"}, {ID: "instance-4"},
				{ID: "instance-5"}, {ID: "instance-6"}, {ID: "instance-7"}, {ID: "instance-8"},
				{ID: "instance-9"}, {ID: "instance-10"},
			},
			wavePercentages:  []int{10, 40, 100},
			wantWaveCount:    3,
			wantTotalRotated: 10,
			wantErr:          false,
		},
		{
			name: "small instance count (5 instances)",
			instances: []Instance{
				{ID: "instance-1"}, {ID: "instance-2"}, {ID: "instance-3"},
				{ID: "instance-4"}, {ID: "instance-5"},
			},
			wavePercentages:  []int{20, 60, 100},
			wantWaveCount:    3,
			wantTotalRotated: 5,
			wantErr:          false,
		},
		{
			name: "single instance",
			instances: []Instance{
				{ID: "instance-1"},
			},
			wavePercentages:  []int{100},
			wantWaveCount:    1,
			wantTotalRotated: 1,
			wantErr:          false,
		},
		{
			name:      "no instances",
			instances: []Instance{},
			wantErr:   true,
			errMsg:    "no instances available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := logging.New(false, true)
			strategy := NewPercentageStrategy(nil, PercentageConfig{}, logger)

			service := ServiceConfig{
				Name:        "test-service",
				Environment: "prod",
				Instances:   tt.instances,
			}

			// Set custom config if provided
			if len(tt.wavePercentages) > 0 {
				config := PercentageConfig{
					Waves: make([]WaveConfig, len(tt.wavePercentages)),
				}
				for i, pct := range tt.wavePercentages {
					config.Waves[i] = WaveConfig{
						Percentage:               pct,
						HealthMonitoringDuration: 2 * time.Minute,
						WaitDuration:             30 * time.Second,
					}
				}
				strategy.config = config
			}

			waves, err := strategy.Plan(context.Background(), service)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			assert.Len(t, waves, tt.wantWaveCount)

			// Verify all instances are covered
			allInstances := make(map[string]bool)
			for _, wave := range waves {
				for _, instID := range wave.Instances {
					allInstances[instID] = true
				}
			}
			assert.Equal(t, tt.wantTotalRotated, len(allInstances), "all instances should be in waves")
		})
	}
}

func TestPercentageStrategy_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		plan    []RolloutWave
		wantErr bool
		errMsg  string
	}{
		{
			name: "single wave",
			plan: []RolloutWave{
				{
					Instances:                []string{"instance-1"},
					Percentage:               100,
					HealthMonitoringDuration: 100 * time.Millisecond,
				},
			},
			wantErr: false,
		},
		{
			name: "multiple waves",
			plan: []RolloutWave{
				{Instances: []string{"instance-1"}, Percentage: 5, HealthMonitoringDuration: 50 * time.Millisecond, WaitDuration: 10 * time.Millisecond},
				{Instances: []string{"instance-2", "instance-3", "instance-4", "instance-5"}, Percentage: 25, HealthMonitoringDuration: 50 * time.Millisecond, WaitDuration: 10 * time.Millisecond},
				{Instances: []string{"instance-6", "instance-7", "instance-8", "instance-9", "instance-10"}, Percentage: 50, HealthMonitoringDuration: 50 * time.Millisecond, WaitDuration: 10 * time.Millisecond},
				{Instances: []string{"instance-11", "instance-12", "instance-13", "instance-14", "instance-15", "instance-16", "instance-17", "instance-18", "instance-19", "instance-20"}, Percentage: 100, HealthMonitoringDuration: 50 * time.Millisecond},
			},
			wantErr: false,
		},
		{
			name:    "empty plan",
			plan:    []RolloutWave{},
			wantErr: true,
			errMsg:  "no waves in rollout plan",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := logging.New(false, true)
			strategy := NewPercentageStrategy(nil, PercentageConfig{}, logger)

			err := strategy.Execute(context.Background(), tt.plan)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPercentageStrategy_Execute_ContextCancellation(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	strategy := NewPercentageStrategy(nil, PercentageConfig{}, logger)

	// Create plan with long health monitoring
	plan := []RolloutWave{
		{
			Instances:                []string{"instance-1"},
			Percentage:               100,
			HealthMonitoringDuration: 10 * time.Second, // Long duration
		},
	}

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := strategy.Execute(ctx, plan)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestPercentageStrategy_Execute_PauseOnFailure(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	strategy := NewPercentageStrategy(nil, PercentageConfig{}, logger)

	// Configure pause on failure
	strategy.config = PercentageConfig{
		PauseOnFailure: true,
		Waves: []WaveConfig{
			{Percentage: 50, HealthMonitoringDuration: 100 * time.Millisecond},
			{Percentage: 100, HealthMonitoringDuration: 100 * time.Millisecond},
		},
	}

	plan := []RolloutWave{
		{Instances: []string{"instance-1", "instance-2"}, Percentage: 50, HealthMonitoringDuration: 100 * time.Millisecond},
		{Instances: []string{"instance-3", "instance-4"}, Percentage: 100, HealthMonitoringDuration: 100 * time.Millisecond},
	}

	// Execute should succeed without health failures
	err := strategy.Execute(context.Background(), plan)
	require.NoError(t, err)
}

func TestPercentageStrategy_CalculateWaveInstances(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		totalCount    int
		percentages   []int
		wantCounts    []int
		wantAllCovered bool
	}{
		{
			name:          "20 instances, default percentages (5%, 25%, 50%, 100%)",
			totalCount:    20,
			percentages:   []int{5, 25, 50, 100},
			wantCounts:    []int{1, 4, 5, 10}, // 5%=1, 25%=5 total (add 4), 50%=10 total (add 5), 100%=20 total (add 10)
			wantAllCovered: true,
		},
		{
			name:          "10 instances, custom percentages (10%, 40%, 100%)",
			totalCount:    10,
			percentages:   []int{10, 40, 100},
			wantCounts:    []int{1, 3, 6}, // 10%=1, 40%=4 total (add 3), 100%=10 total (add 6)
			wantAllCovered: true,
		},
		{
			name:          "5 instances, aggressive percentages (20%, 60%, 100%)",
			totalCount:    5,
			percentages:   []int{20, 60, 100},
			wantCounts:    []int{1, 2, 2}, // 20%=1, 60%=3 total (add 2), 100%=5 total (add 2)
			wantAllCovered: true,
		},
		{
			name:          "100 instances, fine-grained percentages (1%, 5%, 25%, 50%, 100%)",
			totalCount:    100,
			percentages:   []int{1, 5, 25, 50, 100},
			wantCounts:    []int{1, 4, 20, 25, 50}, // 1%=1, 5%=5 total (add 4), 25%=25 total (add 20), 50%=50 total (add 25), 100%=100 total (add 50)
			wantAllCovered: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := logging.New(false, true)
			strategy := NewPercentageStrategy(nil, PercentageConfig{}, logger)

			// Create instances
			instances := make([]Instance, tt.totalCount)
			for i := 0; i < tt.totalCount; i++ {
				instances[i] = Instance{
					ID:     fmt.Sprintf("instance-%d", i+1),
					Labels: map[string]string{},
				}
			}

			// Create wave configs
			waveConfigs := make([]WaveConfig, len(tt.percentages))
			for i, pct := range tt.percentages {
				waveConfigs[i] = WaveConfig{
					Percentage:               pct,
					HealthMonitoringDuration: 2 * time.Minute,
				}
			}

			waves := strategy.calculateWaveInstances(instances, waveConfigs)

			// Verify wave counts
			require.Len(t, waves, len(tt.wantCounts), "unexpected number of waves")
			for i, wave := range waves {
				assert.Equal(t, tt.wantCounts[i], len(wave.Instances), "wave %d has unexpected instance count", i)
			}

			// Verify all instances are covered if expected
			if tt.wantAllCovered {
				allInstances := make(map[string]bool)
				for _, wave := range waves {
					for _, instID := range wave.Instances {
						allInstances[instID] = true
					}
				}
				assert.Equal(t, tt.totalCount, len(allInstances), "all instances should be covered")
			}
		})
	}
}

func TestPercentageStrategy_Integration(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	strategy := NewPercentageStrategy(nil, PercentageConfig{}, logger)

	// Configure with custom waves
	strategy.config = PercentageConfig{
		Waves: []WaveConfig{
			{Percentage: 10, HealthMonitoringDuration: 10 * time.Millisecond, WaitDuration: 5 * time.Millisecond},
			{Percentage: 50, HealthMonitoringDuration: 10 * time.Millisecond, WaitDuration: 5 * time.Millisecond},
			{Percentage: 100, HealthMonitoringDuration: 10 * time.Millisecond},
		},
		PauseOnFailure: false,
	}

	// Create service with instances
	service := ServiceConfig{
		Name:        "postgres-prod",
		Environment: "production",
		Instances: []Instance{
			{ID: "instance-1"}, {ID: "instance-2"}, {ID: "instance-3"}, {ID: "instance-4"},
			{ID: "instance-5"}, {ID: "instance-6"}, {ID: "instance-7"}, {ID: "instance-8"},
			{ID: "instance-9"}, {ID: "instance-10"},
		},
	}

	ctx := context.Background()

	// Plan
	waves, err := strategy.Plan(ctx, service)
	require.NoError(t, err)
	assert.Equal(t, 3, len(waves), "should have 3 waves")

	// Execute
	err = strategy.Execute(ctx, waves)
	require.NoError(t, err)
}

func TestPercentageStrategy_ProgressPersistence(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	strategy := NewPercentageStrategy(nil, PercentageConfig{}, logger)

	service := ServiceConfig{
		Name:        "test-service",
		Environment: "prod",
		Instances: []Instance{
			{ID: "instance-1"}, {ID: "instance-2"}, {ID: "instance-3"},
		},
	}

	// Plan rollout
	waves, err := strategy.Plan(context.Background(), service)
	require.NoError(t, err)

	// Save progress
	progress := RolloutProgress{
		ServiceName: service.Name,
		Environment: service.Environment,
		CurrentWave: 1,
		TotalWaves:  len(waves),
		CompletedInstances: []string{"instance-1"},
		StartTime:   time.Now(),
		Status:      RolloutStatusInProgress,
	}

	err = strategy.SaveProgress(progress)
	require.NoError(t, err)

	// Load progress
	loaded, err := strategy.LoadProgress(service.Name, service.Environment)
	require.NoError(t, err)
	assert.Equal(t, progress.CurrentWave, loaded.CurrentWave)
	assert.Equal(t, progress.TotalWaves, loaded.TotalWaves)
	assert.Equal(t, progress.CompletedInstances, loaded.CompletedInstances)
	assert.Equal(t, progress.Status, loaded.Status)
}
