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

// mockDiscoveryProvider is a mock implementation of DiscoveryProvider for testing.
type mockDiscoveryProvider struct{}

func (m *mockDiscoveryProvider) Name() string {
	return "mock"
}

func (m *mockDiscoveryProvider) Discover(ctx context.Context, config interface{}) ([]Instance, error) {
	return nil, fmt.Errorf("not implemented in mock")
}

func (m *mockDiscoveryProvider) Validate(config interface{}) error {
	return nil
}

func TestCanaryStrategy_Name(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	discoveryProvider := &mockDiscoveryProvider{}
	strategy := NewCanaryStrategy(discoveryProvider, nil, nil, logger)

	assert.Equal(t, "canary", strategy.Name())
}

func TestCanaryStrategy_Plan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		instances     []Instance
		wantWaveCount int
		wantCanary    string
		wantErr       bool
		errMsg        string
	}{
		{
			name: "single instance",
			instances: []Instance{
				{ID: "instance-1", Labels: map[string]string{}},
			},
			wantWaveCount: 1, // Only canary wave
			wantCanary:    "instance-1",
			wantErr:       false,
		},
		{
			name: "multiple instances with explicit canary",
			instances: []Instance{
				{ID: "instance-1", Labels: map[string]string{}},
				{ID: "instance-2", Labels: map[string]string{"canary": "true"}},
				{ID: "instance-3", Labels: map[string]string{}},
			},
			wantWaveCount: 3, // Canary + 2 waves (with only 2 remaining instances)
			wantCanary:    "instance-2",
			wantErr:       false,
		},
		{
			name: "multiple instances without explicit canary",
			instances: []Instance{
				{ID: "instance-1", Labels: map[string]string{}},
				{ID: "instance-2", Labels: map[string]string{}},
				{ID: "instance-3", Labels: map[string]string{}},
				{ID: "instance-4", Labels: map[string]string{}},
			},
			wantWaveCount: 4, // Canary + waves
			wantCanary:    "instance-1", // First instance is canary
			wantErr:       false,
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
			discoveryProvider := &mockDiscoveryProvider{}
			strategy := NewCanaryStrategy(discoveryProvider, nil, nil, logger)

			service := ServiceConfig{
				Name:        "test-service",
				Environment: "prod",
				Instances:   tt.instances,
			}

			waves, err := strategy.Plan(context.Background(), service)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			assert.Len(t, waves, tt.wantWaveCount)

			// Verify canary wave (first wave)
			if tt.wantWaveCount > 0 {
				canaryWave := waves[0]
				assert.Len(t, canaryWave.Instances, 1, "canary wave should have exactly 1 instance")
				assert.Equal(t, tt.wantCanary, canaryWave.Instances[0])
				assert.Equal(t, 0, canaryWave.Percentage, "canary wave percentage should be 0")
				assert.Greater(t, canaryWave.HealthMonitoringDuration, time.Duration(0))
			}

			// Verify all instances are covered
			if tt.wantWaveCount > 1 {
				allInstances := make(map[string]bool)
				for _, wave := range waves {
					for _, instID := range wave.Instances {
						allInstances[instID] = true
					}
				}
				assert.Len(t, allInstances, len(tt.instances), "all instances should be in waves")
			}
		})
	}
}

func TestCanaryStrategy_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		plan    []RolloutWave
		wantErr bool
		errMsg  string
	}{
		{
			name: "single wave (canary only)",
			plan: []RolloutWave{
				{
					Instances:                []string{"canary-1"},
					Percentage:               0,
					HealthMonitoringDuration: 100 * time.Millisecond,
				},
			},
			wantErr: false,
		},
		{
			name: "multiple waves",
			plan: []RolloutWave{
				{Instances: []string{"canary-1"}, Percentage: 0, HealthMonitoringDuration: 100 * time.Millisecond},
				{Instances: []string{"instance-2", "instance-3"}, Percentage: 50, HealthMonitoringDuration: 100 * time.Millisecond},
				{Instances: []string{"instance-4", "instance-5"}, Percentage: 100, HealthMonitoringDuration: 100 * time.Millisecond},
			},
			wantErr: false,
		},
		{
			name:    "empty plan",
			plan:    []RolloutWave{},
			wantErr: true,
			errMsg:  "no waves in rollout plan",
		},
		{
			name: "canary wave with multiple instances (invalid)",
			plan: []RolloutWave{
				{
					Instances:  []string{"canary-1", "canary-2"},
					Percentage: 0,
				},
			},
			wantErr: true,
			errMsg:  "canary wave must contain exactly one instance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := logging.New(false, true)
			discoveryProvider := &mockDiscoveryProvider{}
			strategy := NewCanaryStrategy(discoveryProvider, nil, nil, logger)

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

func TestCanaryStrategy_Execute_ContextCancellation(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	discoveryProvider := &mockDiscoveryProvider{}
	strategy := NewCanaryStrategy(discoveryProvider, nil, nil, logger)

	// Create plan with long health monitoring
	plan := []RolloutWave{
		{
			Instances:                []string{"canary-1"},
			Percentage:               0,
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

func TestCanaryStrategy_FindCanaryInstance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		instances []Instance
		wantIdx   int
	}{
		{
			name: "explicit canary label",
			instances: []Instance{
				{ID: "instance-1", Labels: map[string]string{}},
				{ID: "instance-2", Labels: map[string]string{"canary": "true"}},
				{ID: "instance-3", Labels: map[string]string{}},
			},
			wantIdx: 1,
		},
		{
			name: "multiple canary labels (returns first)",
			instances: []Instance{
				{ID: "instance-1", Labels: map[string]string{}},
				{ID: "instance-2", Labels: map[string]string{"canary": "true"}},
				{ID: "instance-3", Labels: map[string]string{"canary": "true"}},
			},
			wantIdx: 1,
		},
		{
			name: "no canary label (defaults to first)",
			instances: []Instance{
				{ID: "instance-1", Labels: map[string]string{}},
				{ID: "instance-2", Labels: map[string]string{}},
			},
			wantIdx: 0,
		},
		{
			name: "single instance",
			instances: []Instance{
				{ID: "instance-1", Labels: map[string]string{}},
			},
			wantIdx: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := logging.New(false, true)
			discoveryProvider := &mockDiscoveryProvider{}
			strategy := NewCanaryStrategy(discoveryProvider, nil, nil, logger)

			idx := strategy.findCanaryInstance(tt.instances)
			assert.Equal(t, tt.wantIdx, idx)
		})
	}
}

func TestCanaryStrategy_CalculateWaves(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		instanceCount    int
		wavePercentages  []WavePercentage
		wantWaveCount    int
		wantTotalRotated int
	}{
		{
			name:          "default waves (10%, 50%, 100%)",
			instanceCount: 10,
			wavePercentages: []WavePercentage{
				{Percentage: 10},
				{Percentage: 50},
				{Percentage: 100},
			},
			wantWaveCount:    3,
			wantTotalRotated: 10,
		},
		{
			name:          "small instance count with default waves",
			instanceCount: 3,
			wavePercentages: []WavePercentage{
				{Percentage: 10},
				{Percentage: 50},
				{Percentage: 100},
			},
			wantWaveCount:    3,
			wantTotalRotated: 3,
		},
		{
			name:             "no wave percentages (uses defaults)",
			instanceCount:    5,
			wavePercentages:  nil,
			wantWaveCount:    3,
			wantTotalRotated: 5,
		},
		{
			name:          "custom waves",
			instanceCount: 20,
			wavePercentages: []WavePercentage{
				{Percentage: 25},
				{Percentage: 75},
				{Percentage: 100},
			},
			wantWaveCount:    2, // 25% = 5, 75% = 15, total 20 (all covered)
			wantTotalRotated: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := logging.New(false, true)
			discoveryProvider := &mockDiscoveryProvider{}
			strategy := NewCanaryStrategy(discoveryProvider, nil, nil, logger)

			// Create instances
			instances := make([]Instance, tt.instanceCount)
			for i := 0; i < tt.instanceCount; i++ {
				instances[i] = Instance{
					ID:     fmt.Sprintf("instance-%d", i+1),
					Labels: map[string]string{},
				}
			}

			waves := strategy.calculateWaves(instances, tt.wavePercentages)

			assert.Len(t, waves, tt.wantWaveCount)

			// Count total instances rotated
			totalRotated := 0
			for _, wave := range waves {
				totalRotated += len(wave.Instances)
			}
			assert.Equal(t, tt.wantTotalRotated, totalRotated, "all instances should be rotated")
		})
	}
}

func TestCanaryStrategy_Integration(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	discoveryProvider := &mockDiscoveryProvider{}
	strategy := NewCanaryStrategy(discoveryProvider, nil, nil, logger)

	// Create service with instances
	service := ServiceConfig{
		Name:        "postgres-prod",
		Environment: "production",
		Instances: []Instance{
			{ID: "instance-1", Labels: map[string]string{}},
			{ID: "instance-2", Labels: map[string]string{"canary": "true"}},
			{ID: "instance-3", Labels: map[string]string{}},
			{ID: "instance-4", Labels: map[string]string{}},
			{ID: "instance-5", Labels: map[string]string{}},
		},
	}

	ctx := context.Background()

	// Plan
	waves, err := strategy.Plan(ctx, service)
	require.NoError(t, err)
	assert.Greater(t, len(waves), 1, "should have canary + additional waves")

	// Verify canary is instance-2
	assert.Equal(t, "instance-2", waves[0].Instances[0])

	// Execute (with short health monitoring)
	for i := range waves {
		waves[i].HealthMonitoringDuration = 10 * time.Millisecond
	}
	err = strategy.Execute(ctx, waves)
	require.NoError(t, err)
}
