package rotation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/systmms/dsops/internal/dsopsdata"
	"github.com/systmms/dsops/internal/logging"
)

// MockRotator implements SecretValueRotator for testing
type MockRotator struct {
	name           string
	supportedTypes []string
	supportsSecret bool // Add field for testing
	rotateFunc     func(ctx context.Context, request RotationRequest) (*RotationResult, error)
	verifyFunc     func(ctx context.Context, request VerificationRequest) error
}

func (m *MockRotator) Name() string {
	return m.name
}

func (m *MockRotator) SupportsSecret(ctx context.Context, secret SecretInfo) bool {
	// If supportsSecret field is explicitly set (for testing), use that
	if !m.supportsSecret && len(m.supportedTypes) == 0 {
		return m.supportsSecret
	}
	
	if len(m.supportedTypes) == 0 {
		return true // Support all types if none specified
	}
	
	for _, supportedType := range m.supportedTypes {
		if string(secret.SecretType) == supportedType {
			return true
		}
	}
	return false
}

func (m *MockRotator) Rotate(ctx context.Context, request RotationRequest) (*RotationResult, error) {
	if m.rotateFunc != nil {
		return m.rotateFunc(ctx, request)
	}
	
	rotatedAt := time.Now()
	return &RotationResult{
		Secret:    request.Secret,
		Status:    StatusCompleted,
		RotatedAt: &rotatedAt,
		NewSecretRef: &SecretReference{
			Identifier: "new-secret-123",
			Version:    "v2",
		},
	}, nil
}

func (m *MockRotator) Verify(ctx context.Context, request VerificationRequest) error {
	if m.verifyFunc != nil {
		return m.verifyFunc(ctx, request)
	}
	return nil
}

func (m *MockRotator) Rollback(ctx context.Context, request RollbackRequest) error {
	return nil
}

func (m *MockRotator) GetStatus(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
	return &RotationStatusInfo{
		Status:    StatusPending,
		CanRotate: true,
	}, nil
}

func TestEngineRegistration(t *testing.T) {
	logger := logging.New(false, true)
	engine := NewRotationEngine(logger)
	
	// Test registering a strategy
	strategy := &MockRotator{name: "test-strategy", supportsSecret: true}
	err := engine.RegisterStrategy(strategy)
	if err != nil {
		t.Errorf("Failed to register strategy: %v", err)
	}
	
	// Test duplicate registration
	err = engine.RegisterStrategy(strategy)
	if err == nil {
		t.Error("Expected error when registering duplicate strategy")
	}
	
	// Test getting registered strategy
	retrieved, err := engine.GetStrategy("test-strategy")
	if err != nil {
		t.Errorf("Failed to get registered strategy: %v", err)
	}
	if retrieved.Name() != "test-strategy" {
		t.Errorf("Retrieved wrong strategy: %s", retrieved.Name())
	}
	
	// Test getting non-existent strategy
	_, err = engine.GetStrategy("non-existent")
	if err == nil {
		t.Error("Expected error when getting non-existent strategy")
	}
	
	// Test listing strategies
	strategies := engine.ListStrategies()
	if len(strategies) != 1 || strategies[0] != "test-strategy" {
		t.Errorf("Unexpected strategy list: %v", strategies)
	}
}

func TestEngineAutoSelectStrategy(t *testing.T) {
	logger := logging.New(false, true)
	engine := NewRotationEngine(logger)
	ctx := context.Background()
	
	// Register multiple strategies with different capabilities
	passwordStrategy := &MockRotator{
		name: "password-rotator",
		supportsSecret: true,
	}
	apiKeyStrategy := &MockRotator{
		name: "apikey-rotator",
		supportsSecret: false, // This strategy doesn't support the test secret
	}
	
	_ = engine.RegisterStrategy(passwordStrategy)
	_ = engine.RegisterStrategy(apiKeyStrategy)
	
	// Test auto-selection
	secret := SecretInfo{
		Key:        "TEST_PASSWORD",
		SecretType: SecretTypePassword,
		Provider:   "aws",
	}
	
	selected, err := engine.AutoSelectStrategy(ctx, secret)
	if err != nil {
		t.Errorf("Failed to auto-select strategy: %v", err)
	}
	if selected != "password-rotator" {
		t.Errorf("Expected password-rotator, got %s", selected)
	}
	
	// Test when no strategy supports the secret
	// Make both strategies not support this specific secret
	passwordStrategy.supportsSecret = false
	apiKeyStrategy.supportsSecret = false
	
	unsupportedSecret := SecretInfo{
		Key:        "UNSUPPORTED",
		SecretType: "unsupported-type",
		Provider:   "unknown",
	}
	
	_, err = engine.AutoSelectStrategy(ctx, unsupportedSecret)
	if err == nil {
		t.Error("Expected error when no strategy supports the secret")
	}
}

func TestEngineRotation(t *testing.T) {
	logger := logging.New(false, true)
	engine := NewRotationEngine(logger)
	ctx := context.Background()
	
	// Test successful rotation
	successStrategy := &MockRotator{
		name:           "success-strategy",
		supportsSecret: true,
	}
	_ = engine.RegisterStrategy(successStrategy)
	
	secret := SecretInfo{
		Key:        "TEST_SECRET",
		SecretType: SecretTypePassword,
		Provider:   "aws",
		Constraints: &RotationConstraints{
			MinRotationInterval: 24 * time.Hour,
		},
	}
	
	request := RotationRequest{
		Secret:   secret,
		Strategy: "success-strategy",
	}
	
	result, err := engine.Rotate(ctx, request)
	if err != nil {
		t.Errorf("Rotation failed: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Errorf("Expected completed status, got %s", result.Status)
	}
	if result.NewSecretRef == nil {
		t.Error("Expected new secret reference")
	}
	
	// Test rotation with unsupported secret
	unsupportedStrategy := &MockRotator{
		name:           "unsupported-strategy",
		supportsSecret: false,
	}
	_ = engine.RegisterStrategy(unsupportedStrategy)
	
	request.Strategy = "unsupported-strategy"
	result, err = engine.Rotate(ctx, request)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("Expected failed status, got %s", result.Status)
	}
	if !strings.Contains(result.Error, "does not support") {
		t.Errorf("Expected unsupported error message, got: %s", result.Error)
	}
	
	// Test rotation with strategy error
	errorStrategy := &MockRotator{
		name:           "error-strategy",
		supportsSecret: true,
		rotateFunc: func(ctx context.Context, req RotationRequest) (*RotationResult, error) {
			return nil, errors.New("rotation failed: network error")
		},
	}
	_ = engine.RegisterStrategy(errorStrategy)
	
	request.Strategy = "error-strategy"
	result, err = engine.Rotate(ctx, request)
	if err == nil {
		t.Error("Expected error from failed rotation")
	}
	if result.Status != StatusFailed {
		t.Errorf("Expected failed status, got %s", result.Status)
	}
}

func TestEngineBatchRotation(t *testing.T) {
	logger := logging.New(false, true)
	engine := NewRotationEngine(logger)
	ctx := context.Background()
	
	// Register a strategy
	strategy := &MockRotator{
		name:           "batch-strategy",
		supportsSecret: true,
	}
	_ = engine.RegisterStrategy(strategy)
	
	// Create multiple rotation requests
	requests := []RotationRequest{
		{
			Secret: SecretInfo{
				Key:        "SECRET_1",
				SecretType: SecretTypePassword,
			},
			Strategy: "batch-strategy",
		},
		{
			Secret: SecretInfo{
				Key:        "SECRET_2",
				SecretType: SecretTypePassword,
			},
			Strategy: "batch-strategy",
		},
		{
			Secret: SecretInfo{
				Key:        "SECRET_3",
				SecretType: SecretTypePassword,
			},
			Strategy: "batch-strategy",
		},
	}
	
	// Test batch rotation
	results, err := engine.BatchRotate(ctx, requests)
	if err != nil {
		t.Errorf("Batch rotation failed: %v", err)
	}
	
	if len(results) != len(requests) {
		t.Errorf("Expected %d results, got %d", len(requests), len(results))
	}
	
	// Verify all rotations completed
	for i, result := range results {
		if result.Status != StatusCompleted {
			t.Errorf("Rotation %d failed with status %s", i, result.Status)
		}
		if result.Secret.Key != requests[i].Secret.Key {
			t.Errorf("Result %d has wrong secret key", i)
		}
	}
}

func TestEngineAuditTrail(t *testing.T) {
	logger := logging.New(false, true)
	engine := NewRotationEngine(logger)
	ctx := context.Background()
	
	// Register strategy that adds audit entries
	strategy := &MockRotator{
		name:           "audit-strategy",
		supportsSecret: true,
		rotateFunc: func(ctx context.Context, req RotationRequest) (*RotationResult, error) {
			rotatedAt := time.Now()
			return &RotationResult{
				Secret:    req.Secret,
				Status:    StatusCompleted,
				RotatedAt: &rotatedAt,
				AuditTrail: []AuditEntry{
					{
						Timestamp: time.Now(),
						Action:    "credential_generated",
						Component: "audit-strategy",
						Status:    "info",
						Message:   "Generated new credential",
					},
				},
			}, nil
		},
	}
	_ = engine.RegisterStrategy(strategy)
	
	request := RotationRequest{
		Secret: SecretInfo{
			Key:        "AUDIT_TEST",
			SecretType: SecretTypePassword,
		},
		Strategy: "audit-strategy",
	}
	
	result, err := engine.Rotate(ctx, request)
	if err != nil {
		t.Errorf("Rotation failed: %v", err)
	}
	
	// Check audit trail
	if len(result.AuditTrail) < 2 {
		t.Error("Expected at least 2 audit entries")
	}
	
	// Should have rotation_started from engine
	hasStarted := false
	hasGenerated := false
	
	for _, entry := range result.AuditTrail {
		if entry.Action == "rotation_started" {
			hasStarted = true
			if entry.Component != "rotation_engine" {
				t.Errorf("Expected rotation_engine component, got %s", entry.Component)
			}
		}
		if entry.Action == "credential_generated" {
			hasGenerated = true
			if entry.Component != "audit-strategy" {
				t.Errorf("Expected audit-strategy component, got %s", entry.Component)
			}
		}
	}
	
	if !hasStarted {
		t.Error("Missing rotation_started audit entry")
	}
	if !hasGenerated {
		t.Error("Missing credential_generated audit entry")
	}
}

func TestEngineWithRepository(t *testing.T) {
	logger := logging.New(false, true)
	engine := NewRotationEngine(logger)
	
	// Create simple repository for testing
	repo := &dsopsdata.Repository{
		ServiceTypes: make(map[string]*dsopsdata.ServiceType),
	}
	
	// Create a test service type
	serviceType := &dsopsdata.ServiceType{}
	serviceType.Metadata.Name = "postgresql"
	serviceType.Spec.Defaults.RotationStrategy = "database-rotator"
	
	repo.ServiceTypes["postgresql"] = serviceType
	
	// Set repository on engine
	engine.SetRepository(repo)
	
	// Register matching strategy
	dbStrategy := &MockRotator{
		name:           "database-rotator",
		supportsSecret: true,
	}
	_ = engine.RegisterStrategy(dbStrategy)
	
	// Test auto-selection with schema
	secret := SecretInfo{
		Key:        "DB_PASSWORD",
		SecretType: "postgresql",
		Provider:   "aws",
	}
	
	ctx := context.Background()
	selected, err := engine.AutoSelectStrategy(ctx, secret)
	if err != nil {
		t.Errorf("Failed to auto-select strategy: %v", err)
	}
	if selected != "database-rotator" {
		t.Errorf("Expected database-rotator from schema, got %s", selected)
	}
}