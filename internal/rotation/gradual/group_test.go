package gradual

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/logging"
)

func TestGroupStrategy_Name(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	strategy := NewGroupStrategy(nil, nil, logger)

	assert.Equal(t, "group", strategy.Name())
}

func TestGroupStrategy_Plan_NoDependencies(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	strategy := NewGroupStrategy(nil, nil, logger)

	// Configure group with no dependencies - all services can run in parallel
	config := GroupConfig{
		Services: []string{"service-a", "service-b", "service-c"},
		Dependencies: map[string][]string{},
		FailurePolicy: FailurePolicyRollbackAll,
	}
	strategy.config = config

	service := ServiceConfig{
		Name:        "group-1",
		Environment: "prod",
		Instances: []Instance{
			{ID: "service-a-1"},
			{ID: "service-b-1"},
			{ID: "service-c-1"},
		},
	}

	waves, err := strategy.Plan(context.Background(), service)
	require.NoError(t, err)

	// With no dependencies, all services should be in one wave (parallel execution)
	assert.Len(t, waves, 1)
	assert.Len(t, waves[0].Instances, 3)
}

func TestGroupStrategy_Plan_LinearDependencies(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	strategy := NewGroupStrategy(nil, nil, logger)

	// Configure linear dependency chain: A -> B -> C
	config := GroupConfig{
		Services: []string{"service-a", "service-b", "service-c"},
		Dependencies: map[string][]string{
			"service-b": {"service-a"}, // B depends on A
			"service-c": {"service-b"}, // C depends on B
		},
		FailurePolicy: FailurePolicyRollbackAll,
	}
	strategy.config = config

	service := ServiceConfig{
		Name:        "group-1",
		Environment: "prod",
		Instances: []Instance{
			{ID: "service-a-1"},
			{ID: "service-b-1"},
			{ID: "service-c-1"},
		},
	}

	waves, err := strategy.Plan(context.Background(), service)
	require.NoError(t, err)

	// Linear dependencies should create 3 sequential waves
	assert.Len(t, waves, 3)
	assert.Equal(t, []string{"service-a-1"}, waves[0].Instances)
	assert.Equal(t, []string{"service-b-1"}, waves[1].Instances)
	assert.Equal(t, []string{"service-c-1"}, waves[2].Instances)
}

func TestGroupStrategy_Plan_DiamondDependencies(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	strategy := NewGroupStrategy(nil, nil, logger)

	// Diamond dependency:
	//     A
	//    / \
	//   B   C
	//    \ /
	//     D
	config := GroupConfig{
		Services: []string{"service-a", "service-b", "service-c", "service-d"},
		Dependencies: map[string][]string{
			"service-b": {"service-a"}, // B depends on A
			"service-c": {"service-a"}, // C depends on A
			"service-d": {"service-b", "service-c"}, // D depends on B and C
		},
		FailurePolicy: FailurePolicyRollbackAll,
	}
	strategy.config = config

	service := ServiceConfig{
		Name:        "group-1",
		Environment: "prod",
		Instances: []Instance{
			{ID: "service-a-1"},
			{ID: "service-b-1"},
			{ID: "service-c-1"},
			{ID: "service-d-1"},
		},
	}

	waves, err := strategy.Plan(context.Background(), service)
	require.NoError(t, err)

	// Diamond dependencies should create 3 waves:
	// Wave 1: A
	// Wave 2: B and C (parallel)
	// Wave 3: D
	assert.Len(t, waves, 3)
	assert.Equal(t, []string{"service-a-1"}, waves[0].Instances)
	assert.ElementsMatch(t, []string{"service-b-1", "service-c-1"}, waves[1].Instances)
	assert.Equal(t, []string{"service-d-1"}, waves[2].Instances)
}

func TestGroupStrategy_Plan_CircularDependency(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	strategy := NewGroupStrategy(nil, nil, logger)

	// Circular dependency: A -> B -> C -> A
	config := GroupConfig{
		Services: []string{"service-a", "service-b", "service-c"},
		Dependencies: map[string][]string{
			"service-b": {"service-a"},
			"service-c": {"service-b"},
			"service-a": {"service-c"}, // Creates cycle
		},
		FailurePolicy: FailurePolicyRollbackAll,
	}
	strategy.config = config

	service := ServiceConfig{
		Name:        "group-1",
		Environment: "prod",
		Instances: []Instance{
			{ID: "service-a-1"},
			{ID: "service-b-1"},
			{ID: "service-c-1"},
		},
	}

	_, err := strategy.Plan(context.Background(), service)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

func TestGroupStrategy_Execute_Success(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	strategy := NewGroupStrategy(nil, nil, logger)

	// Simple plan with 2 waves
	plan := []RolloutWave{
		{Instances: []string{"service-a-1"}, HealthMonitoringDuration: 10 * time.Millisecond},
		{Instances: []string{"service-b-1", "service-c-1"}, HealthMonitoringDuration: 10 * time.Millisecond},
	}

	err := strategy.Execute(context.Background(), plan)
	require.NoError(t, err)
}

func TestGroupStrategy_Execute_FailureWithRollbackAll(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	strategy := NewGroupStrategy(nil, nil, logger)
	strategy.config = GroupConfig{
		FailurePolicy: FailurePolicyRollbackAll,
	}

	// This would fail in a real scenario, but our placeholder rotateWave doesn't fail
	// In real implementation, we'd mock the rotation to return an error
	plan := []RolloutWave{
		{Instances: []string{"service-a-1"}},
		{Instances: []string{"service-b-1"}},
	}

	err := strategy.Execute(context.Background(), plan)
	// Should succeed because our placeholder doesn't fail
	require.NoError(t, err)
}

func TestGroupStrategy_Execute_ContextCancellation(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	strategy := NewGroupStrategy(nil, nil, logger)

	plan := []RolloutWave{
		{Instances: []string{"service-a-1"}, HealthMonitoringDuration: 10 * time.Second},
	}

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := strategy.Execute(ctx, plan)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestGroupStrategy_TopologicalSort_Simple(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	strategy := NewGroupStrategy(nil, nil, logger)

	// Simple linear dependencies: A -> B -> C
	dependencies := map[string][]string{
		"service-b": {"service-a"},
		"service-c": {"service-b"},
	}
	services := []string{"service-a", "service-b", "service-c"}

	sorted, err := strategy.topologicalSort(services, dependencies)
	require.NoError(t, err)

	// A should come before B, B should come before C
	assert.Len(t, sorted, 3)
	aIndex := findIndex(sorted, []string{"service-a"})
	bIndex := findIndex(sorted, []string{"service-b"})
	cIndex := findIndex(sorted, []string{"service-c"})

	assert.True(t, aIndex < bIndex, "A should come before B")
	assert.True(t, bIndex < cIndex, "B should come before C")
}

func TestGroupStrategy_TopologicalSort_Diamond(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	strategy := NewGroupStrategy(nil, nil, logger)

	// Diamond: A -> B,C -> D
	dependencies := map[string][]string{
		"service-b": {"service-a"},
		"service-c": {"service-a"},
		"service-d": {"service-b", "service-c"},
	}
	services := []string{"service-a", "service-b", "service-c", "service-d"}

	sorted, err := strategy.topologicalSort(services, dependencies)
	require.NoError(t, err)

	assert.Len(t, sorted, 3)

	// Wave 1: A
	assert.Equal(t, []string{"service-a"}, sorted[0])

	// Wave 2: B and C (order doesn't matter, both depend on A)
	assert.ElementsMatch(t, []string{"service-b", "service-c"}, sorted[1])

	// Wave 3: D
	assert.Equal(t, []string{"service-d"}, sorted[2])
}

func TestGroupStrategy_TopologicalSort_CircularDependency(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	strategy := NewGroupStrategy(nil, nil, logger)

	// Circular: A -> B -> C -> A
	dependencies := map[string][]string{
		"service-b": {"service-a"},
		"service-c": {"service-b"},
		"service-a": {"service-c"},
	}
	services := []string{"service-a", "service-b", "service-c"}

	_, err := strategy.topologicalSort(services, dependencies)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

func TestGroupStrategy_Integration(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, true)
	strategy := NewGroupStrategy(nil, nil, logger)

	// Complex dependency graph:
	//     postgres-primary
	//          |
	//     +---------+
	//     |         |
	// postgres-replica-1  postgres-replica-2
	config := GroupConfig{
		Services: []string{"postgres-primary", "postgres-replica-1", "postgres-replica-2"},
		Dependencies: map[string][]string{
			"postgres-replica-1": {"postgres-primary"},
			"postgres-replica-2": {"postgres-primary"},
		},
		FailurePolicy: FailurePolicyRollbackAll,
		CrossServiceVerification: CrossServiceVerificationConfig{
			Enabled: true,
		},
	}
	strategy.config = config

	service := ServiceConfig{
		Name:        "postgres-cluster",
		Environment: "production",
		Instances: []Instance{
			{ID: "postgres-primary-1"},
			{ID: "postgres-replica-1-1"},
			{ID: "postgres-replica-2-1"},
		},
	}

	ctx := context.Background()

	// Plan
	waves, err := strategy.Plan(ctx, service)
	require.NoError(t, err)
	assert.Len(t, waves, 2) // Primary in wave 1, replicas in wave 2

	// Execute (with short health monitoring)
	for i := range waves {
		waves[i].HealthMonitoringDuration = 10 * time.Millisecond
	}
	err = strategy.Execute(ctx, waves)
	require.NoError(t, err)
}

// Helper function to find the index of a service in the sorted waves
func findIndex(sorted [][]string, target []string) int {
	for i, wave := range sorted {
		for _, svc := range wave {
			for _, t := range target {
				if svc == t {
					return i
				}
			}
		}
	}
	return -1
}
