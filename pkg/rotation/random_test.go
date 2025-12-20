package rotation

import (
	"context"
	"testing"

	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/pkg/provider"
)

func TestRandomRotator_Name(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewRandomRotator(logger)

	if rotator.Name() != "random" {
		t.Errorf("Expected name 'random', got %s", rotator.Name())
	}
}

func TestRandomRotator_SupportsSecret(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewRandomRotator(logger)
	ctx := context.Background()

	// Random rotator should support all secret types
	secrets := []SecretInfo{
		{Key: "PASSWORD", SecretType: SecretTypePassword},
		{Key: "API_KEY", SecretType: SecretTypeAPIKey},
		{Key: "CERT", SecretType: SecretTypeCertificate},
		{Key: "UNKNOWN", SecretType: "unknown-type"},
	}

	for _, secret := range secrets {
		if !rotator.SupportsSecret(ctx, secret) {
			t.Errorf("Expected random rotator to support secret type %s", secret.SecretType)
		}
	}
}

func TestRandomRotator_Rotate_Success(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewRandomRotator(logger)
	ctx := context.Background()

	secret := SecretInfo{
		Key:        "TEST_SECRET",
		Provider:   "aws",
		SecretType: SecretTypePassword,
		ProviderRef: provider.Reference{
			Key: "original-key",
		},
	}

	request := RotationRequest{
		Secret: secret,
	}

	result, err := rotator.Rotate(ctx, request)
	if err != nil {
		t.Fatalf("Rotation failed: %v", err)
	}

	if result.Status != StatusCompleted {
		t.Errorf("Expected status %s, got %s", StatusCompleted, result.Status)
	}

	if result.RotatedAt == nil {
		t.Error("Expected RotatedAt to be set")
	}

	if result.NewSecretRef == nil {
		t.Error("Expected NewSecretRef to be set")
	}

	if result.NewSecretRef.Metadata["strategy"] != "random" {
		t.Error("Expected strategy metadata to be 'random'")
	}

	// Verify audit trail
	if len(result.AuditTrail) < 2 {
		t.Error("Expected at least 2 audit entries")
	}
}

func TestRandomRotator_Rotate_DryRun(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewRandomRotator(logger)
	ctx := context.Background()

	secret := SecretInfo{
		Key:      "TEST_SECRET",
		Provider: "aws",
	}

	request := RotationRequest{
		Secret: secret,
		DryRun: true,
	}

	result, err := rotator.Rotate(ctx, request)
	if err != nil {
		t.Fatalf("Dry run rotation failed: %v", err)
	}

	// Dry run should return pending status
	if result.Status != StatusPending {
		t.Errorf("Expected status %s for dry run, got %s", StatusPending, result.Status)
	}

	// No actual rotation should happen
	if result.RotatedAt != nil {
		t.Error("Expected RotatedAt to be nil for dry run")
	}

	if result.NewSecretRef != nil {
		t.Error("Expected NewSecretRef to be nil for dry run")
	}

	// Should have audit entry mentioning dry run
	hasDryRunEntry := false
	for _, entry := range result.AuditTrail {
		if entry.Action == "dry_run_simulation" {
			hasDryRunEntry = true
			break
		}
	}
	if !hasDryRunEntry {
		t.Error("Expected dry_run_simulation audit entry")
	}
}

func TestRandomRotator_Rotate_LiteralValue(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewRandomRotator(logger)
	ctx := context.Background()

	secret := SecretInfo{
		Key:      "TEST_SECRET",
		Provider: "aws",
		ProviderRef: provider.Reference{
			Key: "original-key",
		},
	}

	request := RotationRequest{
		Secret: secret,
		NewValue: &NewSecretValue{
			Type:  ValueTypeLiteral,
			Value: "my-specific-value",
		},
	}

	result, err := rotator.Rotate(ctx, request)
	if err != nil {
		t.Fatalf("Rotation with literal value failed: %v", err)
	}

	if result.Status != StatusCompleted {
		t.Errorf("Expected status %s, got %s", StatusCompleted, result.Status)
	}
}

func TestRandomRotator_Rotate_CustomLength(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewRandomRotator(logger)
	ctx := context.Background()

	secret := SecretInfo{
		Key:      "TEST_SECRET",
		Provider: "aws",
		ProviderRef: provider.Reference{
			Key: "original-key",
		},
	}

	request := RotationRequest{
		Secret: secret,
		NewValue: &NewSecretValue{
			Type: ValueTypeGenerated,
			Config: map[string]interface{}{
				"length": 64,
			},
		},
	}

	result, err := rotator.Rotate(ctx, request)
	if err != nil {
		t.Fatalf("Rotation with custom length failed: %v", err)
	}

	if result.Status != StatusCompleted {
		t.Errorf("Expected status %s, got %s", StatusCompleted, result.Status)
	}

	// Verify length metadata
	if result.NewSecretRef != nil && result.NewSecretRef.Metadata["length"] != "64" {
		t.Errorf("Expected length metadata to be '64', got %s", result.NewSecretRef.Metadata["length"])
	}
}

func TestRandomRotator_Verify(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewRandomRotator(logger)
	ctx := context.Background()

	request := VerificationRequest{
		Secret: SecretInfo{
			Key:      "TEST_SECRET",
			Provider: "aws",
		},
	}

	// Verify should always succeed for random rotator
	err := rotator.Verify(ctx, request)
	if err != nil {
		t.Errorf("Verify failed unexpectedly: %v", err)
	}
}

func TestRandomRotator_Rollback(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewRandomRotator(logger)
	ctx := context.Background()

	request := RollbackRequest{
		Secret: SecretInfo{
			Key:      "TEST_SECRET",
			Provider: "aws",
		},
	}

	// Rollback should always succeed for random rotator
	err := rotator.Rollback(ctx, request)
	if err != nil {
		t.Errorf("Rollback failed unexpectedly: %v", err)
	}
}

func TestRandomRotator_GetStatus(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewRandomRotator(logger)
	ctx := context.Background()

	secret := SecretInfo{
		Key:      "TEST_SECRET",
		Provider: "aws",
	}

	status, err := rotator.GetStatus(ctx, secret)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status.Status != StatusPending {
		t.Errorf("Expected status %s, got %s", StatusPending, status.Status)
	}

	if !status.CanRotate {
		t.Error("Expected CanRotate to be true")
	}
}

func TestRandomRotator_GenerateRandomBytes(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewRandomRotator(logger)

	tests := []struct {
		length int
	}{
		{16},
		{32},
		{64},
		{128},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result, err := rotator.generateRandomBytes(tt.length)
			if err != nil {
				t.Fatalf("generateRandomBytes failed: %v", err)
			}

			if len(result) != tt.length {
				t.Errorf("Expected length %d, got %d", tt.length, len(result))
			}

			// Verify only alphanumeric characters
			charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
			for _, b := range result {
				found := false
				for _, c := range []byte(charset) {
					if b == c {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Invalid character in random output: %c", b)
				}
			}
		})
	}
}

func TestRandomRotator_GenerateRandomValue_DefaultLength(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewRandomRotator(logger)

	// No NewSecretValue provided should use default 32 chars
	result, err := rotator.generateRandomValue(nil)
	if err != nil {
		t.Fatalf("generateRandomValue failed: %v", err)
	}

	if len(result) != 32 {
		t.Errorf("Expected default length 32, got %d", len(result))
	}
}

func TestRandomRotator_GenerateRandomValue_RandomType(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewRandomRotator(logger)

	newValue := &NewSecretValue{
		Type: ValueTypeRandom,
		Config: map[string]interface{}{
			"length": 48,
		},
	}

	result, err := rotator.generateRandomValue(newValue)
	if err != nil {
		t.Fatalf("generateRandomValue failed: %v", err)
	}

	if len(result) != 48 {
		t.Errorf("Expected length 48, got %d", len(result))
	}
}
