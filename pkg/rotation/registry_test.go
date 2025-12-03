package rotation

import (
	"sort"
	"testing"

	"github.com/systmms/dsops/internal/logging"
)

func TestNewStrategyRegistry(t *testing.T) {
	logger := logging.New(false, true)
	registry := NewStrategyRegistry(logger)

	if registry == nil {
		t.Fatal("Expected non-nil registry")
	}

	// Verify built-in strategies are registered
	builtinStrategies := []string{"random", "webhook", "script"}
	for _, name := range builtinStrategies {
		if !registry.HasStrategy(name) {
			t.Errorf("Built-in strategy %s not registered", name)
		}
	}
}

func TestRegistry_CreateStrategy(t *testing.T) {
	logger := logging.New(false, true)
	registry := NewStrategyRegistry(logger)

	tests := []struct {
		name     string
		expected string
	}{
		{"random", "random"},
		{"webhook", "webhook"},
		{"script", "script"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, err := registry.CreateStrategy(tt.name)
			if err != nil {
				t.Fatalf("Failed to create strategy %s: %v", tt.name, err)
			}

			if strategy.Name() != tt.expected {
				t.Errorf("Expected strategy name %s, got %s", tt.expected, strategy.Name())
			}
		})
	}
}

func TestRegistry_CreateStrategy_Unknown(t *testing.T) {
	logger := logging.New(false, true)
	registry := NewStrategyRegistry(logger)

	_, err := registry.CreateStrategy("nonexistent")
	if err == nil {
		t.Error("Expected error when creating unknown strategy")
	}
}

func TestRegistry_ListStrategies(t *testing.T) {
	logger := logging.New(false, true)
	registry := NewStrategyRegistry(logger)

	strategies := registry.ListStrategies()

	if len(strategies) < 3 {
		t.Errorf("Expected at least 3 strategies, got %d", len(strategies))
	}

	// Sort for consistent comparison
	sort.Strings(strategies)

	expected := []string{"random", "script", "webhook"}
	sort.Strings(expected)

	for _, exp := range expected {
		found := false
		for _, s := range strategies {
			if s == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected strategy %s not in list: %v", exp, strategies)
		}
	}
}

func TestRegistry_HasStrategy(t *testing.T) {
	logger := logging.New(false, true)
	registry := NewStrategyRegistry(logger)

	tests := []struct {
		name     string
		expected bool
	}{
		{"random", true},
		{"webhook", true},
		{"script", true},
		{"nonexistent", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := registry.HasStrategy(tt.name)
			if result != tt.expected {
				t.Errorf("HasStrategy(%s) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestRegistry_RegisterCustomStrategy(t *testing.T) {
	logger := logging.New(false, true)
	registry := NewStrategyRegistry(logger)

	// Register custom strategy
	customFactory := func(l *logging.Logger) SecretValueRotator {
		return NewRandomRotator(l) // Use random as a stand-in
	}

	err := registry.RegisterCustomStrategy("custom", customFactory)
	if err != nil {
		t.Fatalf("Failed to register custom strategy: %v", err)
	}

	// Verify it's now available
	if !registry.HasStrategy("custom") {
		t.Error("Custom strategy not found after registration")
	}

	// Create instance
	strategy, err := registry.CreateStrategy("custom")
	if err != nil {
		t.Fatalf("Failed to create custom strategy: %v", err)
	}

	if strategy == nil {
		t.Error("Created strategy is nil")
	}
}

func TestRegistry_RegisterCustomStrategy_Duplicate(t *testing.T) {
	logger := logging.New(false, true)
	registry := NewStrategyRegistry(logger)

	customFactory := func(l *logging.Logger) SecretValueRotator {
		return NewRandomRotator(l)
	}

	// Register first time
	err := registry.RegisterCustomStrategy("custom", customFactory)
	if err != nil {
		t.Fatalf("Failed to register custom strategy: %v", err)
	}

	// Try to register again
	err = registry.RegisterCustomStrategy("custom", customFactory)
	if err == nil {
		t.Error("Expected error when registering duplicate strategy")
	}
}

func TestRegistry_RegisterCustomStrategy_OverrideBuiltin(t *testing.T) {
	logger := logging.New(false, true)
	registry := NewStrategyRegistry(logger)

	customFactory := func(l *logging.Logger) SecretValueRotator {
		return NewRandomRotator(l)
	}

	// Try to override built-in strategy
	err := registry.RegisterCustomStrategy("random", customFactory)
	if err == nil {
		t.Error("Expected error when trying to override built-in strategy")
	}
}
