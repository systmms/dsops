package rotation_test

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/systmms/dsops/internal/dsopsdata"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/pkg/rotation"
)

// Example demonstrates basic secret rotation using a simple rotator
func ExampleSecretValueRotator_basic() {
	// Create a mock rotator for demonstration
	rotator := &MockDatabaseRotator{
		name: "postgresql",
		db:   &MockDatabase{},
	}

	// Create rotation request
	secretInfo := rotation.SecretInfo{
		Key:      "database_password",
		Provider: "postgresql",
		ProviderRef: provider.Reference{
			Provider: "aws.secretsmanager",
			Key:      "prod/db/password",
		},
		SecretType: rotation.SecretTypePassword,
		Metadata: map[string]string{
			"database":    "production",
			"environment": "prod",
		},
	}

	request := rotation.RotationRequest{
		Secret:   secretInfo,
		Strategy: "postgresql",
		DryRun:   false,
		Force:    false,
	}

	ctx := context.Background()

	// Check if rotator supports this secret
	if !rotator.SupportsSecret(ctx, secretInfo) {
		log.Fatalf("Rotator does not support this secret type")
	}

	// Perform the rotation
	result, err := rotator.Rotate(ctx, request)
	if err != nil {
		log.Fatalf("Rotation failed: %v", err)
	}

	fmt.Printf("Rotation status: %s\n", result.Status)
	fmt.Printf("Rotated at: %s\n", result.RotatedAt.Format("2006-01-02 15:04"))
	fmt.Printf("Verification tests passed: %d\n", len(result.VerificationResults))
	fmt.Printf("Audit trail entries: %d\n", len(result.AuditTrail))

	// Output:
	// Rotation status: completed
	// Rotated at: 2025-11-15 12:00
	// Verification tests passed: 1
	// Audit trail entries: 4
}

// Example demonstrates rotation engine orchestrating multiple strategies
func ExampleRotationEngine() {
	// Create rotation engine
	engine := &MockRotationEngine{
		strategies: make(map[string]rotation.SecretValueRotator),
	}

	// Register multiple rotation strategies
	_ = engine.RegisterStrategy(&MockDatabaseRotator{name: "postgresql"})
	_ = engine.RegisterStrategy(&MockAPIKeyRotator{name: "stripe"})
	_ = engine.RegisterStrategy(&MockCertificateRotator{name: "tls-cert"})

	// List available strategies
	strategies := engine.ListStrategies()
	fmt.Printf("Available strategies: %v\n", strategies)

	// Create rotation requests for different secret types
	dbRequest := rotation.RotationRequest{
		Secret: rotation.SecretInfo{
			Key:        "db_password",
			Provider:   "postgresql",
			SecretType: rotation.SecretTypePassword,
		},
		Strategy: "postgresql",
	}

	apiRequest := rotation.RotationRequest{
		Secret: rotation.SecretInfo{
			Key:        "api_key",
			Provider:   "stripe",
			SecretType: rotation.SecretTypeAPIKey,
		},
		Strategy: "stripe",
	}

	ctx := context.Background()

	// Rotate database password
	dbResult, err := engine.Rotate(ctx, dbRequest)
	if err != nil {
		log.Printf("Database rotation failed: %v", err)
	} else {
		fmt.Printf("Database rotation: %s\n", dbResult.Status)
	}

	// Rotate API key
	apiResult, err := engine.Rotate(ctx, apiRequest)
	if err != nil {
		log.Printf("API rotation failed: %v", err)
	} else {
		fmt.Printf("API rotation: %s\n", apiResult.Status)
	}

	// Batch rotate multiple secrets
	requests := []rotation.RotationRequest{dbRequest, apiRequest}
	results, err := engine.BatchRotate(ctx, requests)
	if err != nil {
		log.Printf("Batch rotation failed: %v", err)
	} else {
		fmt.Printf("Batch rotation completed: %d results\n", len(results))
		for i, result := range results {
			fmt.Printf("  Request %d: %s\n", i+1, result.Status)
		}
	}

	// Output:
	// Available strategies: [postgresql stripe tls-cert]
	// Database rotation: completed
	// API rotation: completed
	// Batch rotation completed: 2 results
	//   Request 1: completed
	//   Request 2: completed
}

// Example demonstrates two-secret rotation for zero-downtime
func ExampleTwoSecretRotator() {
	// Create a rotator that supports two-secret strategy
	rotator := &MockTwoSecretRotator{
		MockDatabaseRotator: MockDatabaseRotator{
			name: "postgresql-zero-downtime",
			db:   &MockDatabase{},
		},
		secondarySecrets: make(map[string]string),
	}

	secretInfo := rotation.SecretInfo{
		Key:        "critical_db_password",
		Provider:   "postgresql",
		SecretType: rotation.SecretTypePassword,
		Metadata: map[string]string{
			"criticality": "high",
			"downtime":    "not-acceptable",
		},
	}

	ctx := context.Background()

	// Phase 1: Create secondary secret
	secondaryRequest := rotation.SecondarySecretRequest{
		Secret: secretInfo,
		NewValue: &rotation.NewSecretValue{
			Type: rotation.ValueTypeRandom,
		},
	}

	fmt.Println("Phase 1: Creating secondary secret...")
	secondaryRef, err := rotator.CreateSecondarySecret(ctx, secondaryRequest)
	if err != nil {
		log.Fatalf("Failed to create secondary secret: %v", err)
	}

	fmt.Printf("Secondary secret created: %s\n", secondaryRef.Identifier)

	// Phase 2: Promote secondary to primary (after verification)
	promoteRequest := rotation.PromoteRequest{
		Secret:       secretInfo,
		SecondaryRef: *secondaryRef,
		GracePeriod:  5 * time.Minute,
		VerifyFirst:  true,
	}

	fmt.Println("Phase 2: Promoting secondary to primary...")
	err = rotator.PromoteSecondarySecret(ctx, promoteRequest)
	if err != nil {
		log.Fatalf("Failed to promote secondary secret: %v", err)
	}

	fmt.Println("Secondary promoted to primary")

	// Phase 3: Deprecate old primary (after grace period)
	deprecateRequest := rotation.DeprecateRequest{
		Secret: secretInfo,
		OldRef: rotation.SecretReference{
			Provider:   "postgresql",
			Key:        "old_primary",
			Identifier: "old_db_user",
		},
		GracePeriod: 1 * time.Minute,
		HardDelete:  false, // Keep for rollback
	}

	fmt.Println("Phase 3: Deprecating old primary...")
	err = rotator.DeprecatePrimarySecret(ctx, deprecateRequest)
	if err != nil {
		log.Printf("Warning: Failed to deprecate old primary: %v", err)
	} else {
		fmt.Println("Old primary deprecated")
	}

	fmt.Println("Zero-downtime rotation completed!")

	// Output:
	// Phase 1: Creating secondary secret...
	// Secondary secret created: db_user_secondary
	// Phase 2: Promoting secondary to primary...
	// Secondary promoted to primary
	// Phase 3: Deprecating old primary...
	// Old primary deprecated
	// Zero-downtime rotation completed!
}

// Example demonstrates schema-aware rotation using dsops-data
func ExampleSchemaAwareRotator() {
	// Create a generic rotator that uses dsops-data definitions
	rotator := &MockSchemaAwareRotator{
		name: "generic-data-driven",
	}

	// Mock dsops-data repository
	postgresType := &dsopsdata.ServiceType{
		APIVersion: "v1",
		Kind:       "ServiceType",
	}
	postgresType.Metadata.Name = "postgresql"
	postgresType.Metadata.Description = "PostgreSQL Database"
	postgresType.Metadata.Category = "database"
	postgresType.Spec.CredentialKinds = []dsopsdata.CredentialKind{
		{
			Name:        "password",
			Description: "Database password",
			Capabilities: []string{"read", "write"},
		},
	}
	postgresType.Spec.Defaults.RotationStrategy = "two-key"

	repository := &dsopsdata.Repository{
		ServiceTypes: map[string]*dsopsdata.ServiceType{
			"postgresql": postgresType,
		},
	}

	// Set the repository for schema-aware operations
	rotator.SetRepository(repository)

	secretInfo := rotation.SecretInfo{
		Key:        "app_db_password",
		Provider:   "postgresql",
		SecretType: rotation.SecretTypePassword,
	}

	ctx := context.Background()

	// The rotator can now use service definitions from dsops-data
	if rotator.SupportsSecret(ctx, secretInfo) {
		fmt.Printf("Rotator supports %s using schema: %s\n", 
			secretInfo.Provider, "postgresql")

		request := rotation.RotationRequest{
			Secret:   secretInfo,
			Strategy: "generic-data-driven",
		}

		result, err := rotator.Rotate(ctx, request)
		if err != nil {
			log.Printf("Schema-aware rotation failed: %v", err)
		} else {
			fmt.Printf("Schema-aware rotation: %s\n", result.Status)
		}
	}

	// Output:
	// Rotator supports postgresql using schema: postgresql
	// Schema-aware rotation: completed
}

// Example demonstrates comprehensive verification and rollback
func ExampleSecretValueRotator_verification() {
	rotator := &MockDatabaseRotator{
		name: "postgresql-with-verification",
		db:   &MockDatabase{},
	}

	secretInfo := rotation.SecretInfo{
		Key:        "verified_password",
		Provider:   "postgresql",
		SecretType: rotation.SecretTypePassword,
		Constraints: &rotation.RotationConstraints{
			RequiredTests: []rotation.VerificationTest{
				{
					Name:     "connection_test",
					Type:     rotation.TestTypeConnection,
					Required: true,
					Timeout:  30 * time.Second,
				},
				{
					Name:     "permission_test",
					Type:     rotation.TestTypeQuery,
					Required: true,
					Config: map[string]interface{}{
						"query": "SELECT 1",
					},
				},
			},
		},
	}

	ctx := context.Background()

	// First, simulate a rotation that will fail verification
	fmt.Println("Attempting rotation with failing verification...")
	
	// Mock a verification request that will fail
	verifyRequest := rotation.VerificationRequest{
		Secret: secretInfo,
		NewSecretRef: rotation.SecretReference{
			Provider: "postgresql",
			Key:      "new_password",
		},
		Tests: secretInfo.Constraints.RequiredTests,
	}

	err := rotator.Verify(ctx, verifyRequest)
	if err != nil {
		fmt.Printf("Verification failed: %v\n", err)
		
		// Rollback to previous secret
		rollbackRequest := rotation.RollbackRequest{
			Secret: secretInfo,
			OldSecretRef: rotation.SecretReference{
				Provider: "postgresql",
				Key:      "old_password",
			},
			Reason: "verification_failure",
		}

		fmt.Println("Rolling back to previous secret...")
		if err := rotator.Rollback(ctx, rollbackRequest); err != nil {
			log.Printf("Rollback failed: %v", err)
		} else {
			fmt.Println("Rollback successful")
		}
	}

	// Now demonstrate successful rotation
	fmt.Println("\nAttempting rotation with passing verification...")
	
	request := rotation.RotationRequest{
		Secret:   secretInfo,
		Strategy: "postgresql-with-verification",
	}

	result, err := rotator.Rotate(ctx, request)
	if err != nil {
		log.Printf("Rotation failed: %v", err)
	} else {
		fmt.Printf("Rotation successful: %s\n", result.Status)
		
		// Show verification results
		for _, vResult := range result.VerificationResults {
			fmt.Printf("Test %s: %s (duration: %s)\n", 
				vResult.Test.Name, vResult.Status, vResult.Duration)
		}
	}

	// Output:
	// Attempting rotation with failing verification...
	// Verification failed: connection test failed: mock connection error
	// Rolling back to previous secret...
	// Rollback successful
	//
	// Attempting rotation with passing verification...
	// Rotation successful: completed
	// Test connection_test: passed (duration: 100ms)
	// Test permission_test: passed (duration: 50ms)
}

// Mock implementations for examples

type MockDatabaseRotator struct {
	name string
	db   *MockDatabase
}

func (r *MockDatabaseRotator) Name() string {
	return r.name
}

func (r *MockDatabaseRotator) SupportsSecret(ctx context.Context, secret rotation.SecretInfo) bool {
	return secret.SecretType == rotation.SecretTypePassword
}

func (r *MockDatabaseRotator) Rotate(ctx context.Context, request rotation.RotationRequest) (*rotation.RotationResult, error) {
	// Use fixed timestamp for consistent example output
	fixedTime := time.Date(2025, 11, 15, 12, 0, 0, 0, time.UTC)

	// Build verification results based on request constraints
	verificationResults := []rotation.VerificationResult{
		{
			Test: rotation.VerificationTest{
				Name: "connection_test",
				Type: rotation.TestTypeConnection,
			},
			Status:   rotation.TestStatusPassed,
			Duration: 100 * time.Millisecond,
			Message:  "Database connection successful",
		},
	}

	// If there are required tests in constraints, add them to results
	if request.Secret.Constraints != nil && len(request.Secret.Constraints.RequiredTests) > 1 {
		verificationResults = append(verificationResults, rotation.VerificationResult{
			Test: rotation.VerificationTest{
				Name: "permission_test",
				Type: rotation.TestTypeQuery,
			},
			Status:   rotation.TestStatusPassed,
			Duration: 50 * time.Millisecond,
			Message:  "Permission check successful",
		})
	}

	result := &rotation.RotationResult{
		Secret:              request.Secret,
		Status:              rotation.StatusCompleted,
		RotatedAt:           &fixedTime,
		VerificationResults: verificationResults,
		AuditTrail: []rotation.AuditEntry{
			{Timestamp: fixedTime, Action: "rotation_started", Status: "success"},
			{Timestamp: fixedTime, Action: "password_generated", Status: "success"},
			{Timestamp: fixedTime, Action: "database_updated", Status: "success"},
			{Timestamp: fixedTime, Action: "verification_completed", Status: "success"},
		},
	}

	return result, nil
}

func (r *MockDatabaseRotator) Verify(ctx context.Context, request rotation.VerificationRequest) error {
	// Simulate verification failure for demonstration
	if request.NewSecretRef.Key == "new_password" {
		return fmt.Errorf("connection test failed: mock connection error")
	}
	return nil
}

func (r *MockDatabaseRotator) Rollback(ctx context.Context, request rotation.RollbackRequest) error {
	// Simulate successful rollback
	return nil
}

func (r *MockDatabaseRotator) GetStatus(ctx context.Context, secret rotation.SecretInfo) (*rotation.RotationStatusInfo, error) {
	fixedTime := time.Date(2025, 11, 15, 12, 0, 0, 0, time.UTC)
	return &rotation.RotationStatusInfo{
		Status:      rotation.StatusCompleted,
		LastRotated: &fixedTime,
		CanRotate:   true,
	}, nil
}

type MockAPIKeyRotator struct {
	name string
}

func (r *MockAPIKeyRotator) Name() string {
	return r.name
}

func (r *MockAPIKeyRotator) SupportsSecret(ctx context.Context, secret rotation.SecretInfo) bool {
	return secret.SecretType == rotation.SecretTypeAPIKey
}

func (r *MockAPIKeyRotator) Rotate(ctx context.Context, request rotation.RotationRequest) (*rotation.RotationResult, error) {
	fixedTime := time.Date(2025, 11, 15, 12, 0, 0, 0, time.UTC)
	return &rotation.RotationResult{
		Secret:    request.Secret,
		Status:    rotation.StatusCompleted,
		RotatedAt: &fixedTime,
	}, nil
}

func (r *MockAPIKeyRotator) Verify(ctx context.Context, request rotation.VerificationRequest) error {
	return nil
}

func (r *MockAPIKeyRotator) Rollback(ctx context.Context, request rotation.RollbackRequest) error {
	return nil
}

func (r *MockAPIKeyRotator) GetStatus(ctx context.Context, secret rotation.SecretInfo) (*rotation.RotationStatusInfo, error) {
	fixedTime := time.Date(2025, 11, 15, 12, 0, 0, 0, time.UTC)
	return &rotation.RotationStatusInfo{
		Status:    rotation.StatusCompleted,
		CanRotate: true,
		LastRotated: &fixedTime,
	}, nil
}

type MockCertificateRotator struct {
	name string
}

func (r *MockCertificateRotator) Name() string {
	return r.name
}

func (r *MockCertificateRotator) SupportsSecret(ctx context.Context, secret rotation.SecretInfo) bool {
	return secret.SecretType == rotation.SecretTypeCertificate
}

func (r *MockCertificateRotator) Rotate(ctx context.Context, request rotation.RotationRequest) (*rotation.RotationResult, error) {
	fixedTime := time.Date(2025, 11, 15, 12, 0, 0, 0, time.UTC)
	return &rotation.RotationResult{
		Secret:    request.Secret,
		Status:    rotation.StatusCompleted,
		RotatedAt: &fixedTime,
	}, nil
}

func (r *MockCertificateRotator) Verify(ctx context.Context, request rotation.VerificationRequest) error {
	return nil
}

func (r *MockCertificateRotator) Rollback(ctx context.Context, request rotation.RollbackRequest) error {
	return nil
}

func (r *MockCertificateRotator) GetStatus(ctx context.Context, secret rotation.SecretInfo) (*rotation.RotationStatusInfo, error) {
	fixedTime := time.Date(2025, 11, 15, 12, 0, 0, 0, time.UTC)
	return &rotation.RotationStatusInfo{
		Status:    rotation.StatusCompleted,
		CanRotate: true,
		LastRotated: &fixedTime,
	}, nil
}

type MockTwoSecretRotator struct {
	MockDatabaseRotator
	secondarySecrets map[string]string
}

func (r *MockTwoSecretRotator) CreateSecondarySecret(ctx context.Context, request rotation.SecondarySecretRequest) (*rotation.SecretReference, error) {
	identifier := "db_user_secondary"
	r.secondarySecrets[request.Secret.Key] = identifier
	
	return &rotation.SecretReference{
		Provider:   request.Secret.Provider,
		Key:        request.Secret.Key + "_secondary",
		Identifier: identifier,
	}, nil
}

func (r *MockTwoSecretRotator) PromoteSecondarySecret(ctx context.Context, request rotation.PromoteRequest) error {
	// Simulate promotion logic
	return nil
}

func (r *MockTwoSecretRotator) DeprecatePrimarySecret(ctx context.Context, request rotation.DeprecateRequest) error {
	// Simulate deprecation logic
	return nil
}

type MockSchemaAwareRotator struct {
	name       string
	repository *dsopsdata.Repository
}

func (r *MockSchemaAwareRotator) Name() string {
	return r.name
}

func (r *MockSchemaAwareRotator) SetRepository(repository *dsopsdata.Repository) {
	r.repository = repository
}

func (r *MockSchemaAwareRotator) SupportsSecret(ctx context.Context, secret rotation.SecretInfo) bool {
	if r.repository == nil {
		return false
	}
	_, exists := r.repository.ServiceTypes[secret.Provider]
	return exists
}

func (r *MockSchemaAwareRotator) Rotate(ctx context.Context, request rotation.RotationRequest) (*rotation.RotationResult, error) {
	fixedTime := time.Date(2025, 11, 15, 12, 0, 0, 0, time.UTC)
	return &rotation.RotationResult{
		Secret:    request.Secret,
		Status:    rotation.StatusCompleted,
		RotatedAt: &fixedTime,
	}, nil
}

func (r *MockSchemaAwareRotator) Verify(ctx context.Context, request rotation.VerificationRequest) error {
	return nil
}

func (r *MockSchemaAwareRotator) Rollback(ctx context.Context, request rotation.RollbackRequest) error {
	return nil
}

func (r *MockSchemaAwareRotator) GetStatus(ctx context.Context, secret rotation.SecretInfo) (*rotation.RotationStatusInfo, error) {
	fixedTime := time.Date(2025, 11, 15, 12, 0, 0, 0, time.UTC)
	return &rotation.RotationStatusInfo{
		Status:    rotation.StatusCompleted,
		CanRotate: true,
		LastRotated: &fixedTime,
	}, nil
}

type MockRotationEngine struct {
	strategies map[string]rotation.SecretValueRotator
}

func (e *MockRotationEngine) RegisterStrategy(strategy rotation.SecretValueRotator) error {
	e.strategies[strategy.Name()] = strategy
	return nil
}

func (e *MockRotationEngine) GetStrategy(name string) (rotation.SecretValueRotator, error) {
	strategy, exists := e.strategies[name]
	if !exists {
		return nil, fmt.Errorf("strategy not found: %s", name)
	}
	return strategy, nil
}

func (e *MockRotationEngine) ListStrategies() []string {
	var names []string
	for name := range e.strategies {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (e *MockRotationEngine) Rotate(ctx context.Context, request rotation.RotationRequest) (*rotation.RotationResult, error) {
	strategy, err := e.GetStrategy(request.Strategy)
	if err != nil {
		return nil, err
	}
	return strategy.Rotate(ctx, request)
}

func (e *MockRotationEngine) BatchRotate(ctx context.Context, requests []rotation.RotationRequest) ([]rotation.RotationResult, error) {
	results := make([]rotation.RotationResult, len(requests))
	for i, request := range requests {
		result, err := e.Rotate(ctx, request)
		if err != nil {
			result = &rotation.RotationResult{
				Secret: request.Secret,
				Status: rotation.StatusFailed,
				Error:  err.Error(),
			}
		}
		results[i] = *result
	}
	return results, nil
}

func (e *MockRotationEngine) GetRotationHistory(ctx context.Context, secret rotation.SecretInfo, limit int) ([]rotation.RotationResult, error) {
	// Mock implementation - return empty history
	return []rotation.RotationResult{}, nil
}

func (e *MockRotationEngine) ScheduleRotation(ctx context.Context, request rotation.RotationRequest, when time.Time) error {
	// Mock implementation - just return success
	return nil
}

type MockDatabase struct {
	// users field is commented out as unused, but kept for potential future use
	// users map[string]string
}

// Additional mock implementations would go here...