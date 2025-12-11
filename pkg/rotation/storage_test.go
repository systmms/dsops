package rotation

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/systmms/dsops/internal/logging"
)

// MemoryRotationStorage tests

func TestMemoryStorage_StoreAndRetrieveHistory(t *testing.T) {
	storage := NewMemoryRotationStorage()
	ctx := context.Background()

	secret := SecretInfo{
		Key:      "TEST_SECRET",
		Provider: "aws",
	}

	rotatedAt := time.Now()
	result := RotationResult{
		Secret:    secret,
		Status:    StatusCompleted,
		RotatedAt: &rotatedAt,
	}

	// Store result
	err := storage.StoreRotationResult(ctx, result)
	if err != nil {
		t.Fatalf("Failed to store rotation result: %v", err)
	}

	// Retrieve history
	history, err := storage.GetRotationHistory(ctx, secret, 10)
	if err != nil {
		t.Fatalf("Failed to get rotation history: %v", err)
	}

	if len(history) != 1 {
		t.Errorf("Expected 1 history entry, got %d", len(history))
	}

	if history[0].Secret.Key != secret.Key {
		t.Errorf("Expected secret key %s, got %s", secret.Key, history[0].Secret.Key)
	}
}

func TestMemoryStorage_HistoryLimit(t *testing.T) {
	storage := NewMemoryRotationStorage()
	ctx := context.Background()

	secret := SecretInfo{
		Key:      "TEST_SECRET",
		Provider: "aws",
	}

	// Store 110 results (exceeds 100 limit)
	for i := 0; i < 110; i++ {
		rotatedAt := time.Now().Add(time.Duration(i) * time.Second)
		result := RotationResult{
			Secret:    secret,
			Status:    StatusCompleted,
			RotatedAt: &rotatedAt,
		}
		_ = storage.StoreRotationResult(ctx, result)
	}

	// Verify only last 100 are kept
	history, err := storage.GetRotationHistory(ctx, secret, 0) // 0 = no limit
	if err != nil {
		t.Fatalf("Failed to get rotation history: %v", err)
	}

	if len(history) != 100 {
		t.Errorf("Expected 100 history entries (capped), got %d", len(history))
	}
}

func TestMemoryStorage_HistorySorting(t *testing.T) {
	storage := NewMemoryRotationStorage()
	ctx := context.Background()

	secret := SecretInfo{
		Key:      "TEST_SECRET",
		Provider: "aws",
	}

	// Store results with different timestamps (oldest first)
	baseTime := time.Now()
	for i := 0; i < 3; i++ {
		rotatedAt := baseTime.Add(time.Duration(i) * time.Hour)
		result := RotationResult{
			Secret:    secret,
			Status:    StatusCompleted,
			RotatedAt: &rotatedAt,
		}
		_ = storage.StoreRotationResult(ctx, result)
	}

	// Retrieve history - should be newest first
	history, err := storage.GetRotationHistory(ctx, secret, 10)
	if err != nil {
		t.Fatalf("Failed to get rotation history: %v", err)
	}

	if len(history) != 3 {
		t.Fatalf("Expected 3 history entries, got %d", len(history))
	}

	// Verify newest first ordering
	for i := 0; i < len(history)-1; i++ {
		if history[i].RotatedAt.Before(*history[i+1].RotatedAt) {
			t.Errorf("History not sorted newest first at index %d", i)
		}
	}
}

func TestMemoryStorage_HistoryWithLimit(t *testing.T) {
	storage := NewMemoryRotationStorage()
	ctx := context.Background()

	secret := SecretInfo{
		Key:      "TEST_SECRET",
		Provider: "aws",
	}

	// Store 5 results
	for i := 0; i < 5; i++ {
		rotatedAt := time.Now().Add(time.Duration(i) * time.Second)
		result := RotationResult{
			Secret:    secret,
			Status:    StatusCompleted,
			RotatedAt: &rotatedAt,
		}
		_ = storage.StoreRotationResult(ctx, result)
	}

	// Retrieve with limit of 2
	history, err := storage.GetRotationHistory(ctx, secret, 2)
	if err != nil {
		t.Fatalf("Failed to get rotation history: %v", err)
	}

	if len(history) != 2 {
		t.Errorf("Expected 2 history entries with limit, got %d", len(history))
	}
}

func TestMemoryStorage_StatusUpdates(t *testing.T) {
	storage := NewMemoryRotationStorage()
	ctx := context.Background()

	secret := SecretInfo{
		Key:      "TEST_SECRET",
		Provider: "aws",
	}

	// Get default status (no updates yet)
	status, err := storage.GetRotationStatus(ctx, secret)
	if err != nil {
		t.Fatalf("Failed to get rotation status: %v", err)
	}

	if status.Status != StatusPending {
		t.Errorf("Expected default status %s, got %s", StatusPending, status.Status)
	}

	if !status.CanRotate {
		t.Error("Expected CanRotate to be true by default")
	}

	// Update status
	newStatus := RotationStatusInfo{
		Status:    StatusCompleted,
		CanRotate: false,
		Reason:    "Recently rotated",
	}

	err = storage.UpdateRotationStatus(ctx, secret, newStatus)
	if err != nil {
		t.Fatalf("Failed to update rotation status: %v", err)
	}

	// Verify updated status
	status, err = storage.GetRotationStatus(ctx, secret)
	if err != nil {
		t.Fatalf("Failed to get updated status: %v", err)
	}

	if status.Status != StatusCompleted {
		t.Errorf("Expected status %s, got %s", StatusCompleted, status.Status)
	}

	if status.CanRotate {
		t.Error("Expected CanRotate to be false after update")
	}
}

func TestMemoryStorage_ListSecrets(t *testing.T) {
	storage := NewMemoryRotationStorage()
	ctx := context.Background()

	// Store history for two secrets
	secret1 := SecretInfo{Key: "SECRET_1", Provider: "aws"}
	secret2 := SecretInfo{Key: "SECRET_2", Provider: "vault"}

	rotatedAt := time.Now()
	_ = storage.StoreRotationResult(ctx, RotationResult{Secret: secret1, Status: StatusCompleted, RotatedAt: &rotatedAt})
	_ = storage.StoreRotationResult(ctx, RotationResult{Secret: secret2, Status: StatusCompleted, RotatedAt: &rotatedAt})

	// Add status for a third secret (no history)
	secret3 := SecretInfo{Key: "SECRET_3", Provider: "1password"}
	_ = storage.UpdateRotationStatus(ctx, secret3, RotationStatusInfo{Status: StatusPending})

	// List all secrets
	secrets, err := storage.ListSecrets(ctx)
	if err != nil {
		t.Fatalf("Failed to list secrets: %v", err)
	}

	if len(secrets) != 3 {
		t.Errorf("Expected 3 secrets, got %d", len(secrets))
	}

	// Verify all secrets are present
	found := make(map[string]bool)
	for _, s := range secrets {
		found[s.Key] = true
	}

	for _, key := range []string{"SECRET_1", "SECRET_2", "SECRET_3"} {
		if !found[key] {
			t.Errorf("Expected secret %s not found in list", key)
		}
	}
}

func TestMemoryStorage_EmptyHistory(t *testing.T) {
	storage := NewMemoryRotationStorage()
	ctx := context.Background()

	secret := SecretInfo{Key: "NONEXISTENT", Provider: "aws"}

	history, err := storage.GetRotationHistory(ctx, secret, 10)
	if err != nil {
		t.Fatalf("Failed to get rotation history: %v", err)
	}

	if len(history) != 0 {
		t.Errorf("Expected empty history, got %d entries", len(history))
	}
}

func TestMemoryStorage_Close(t *testing.T) {
	storage := NewMemoryRotationStorage()
	ctx := context.Background()

	secret := SecretInfo{Key: "TEST_SECRET", Provider: "aws"}
	rotatedAt := time.Now()
	_ = storage.StoreRotationResult(ctx, RotationResult{Secret: secret, Status: StatusCompleted, RotatedAt: &rotatedAt})

	// Close should clear data
	err := storage.Close()
	if err != nil {
		t.Fatalf("Failed to close storage: %v", err)
	}

	// Verify data is cleared
	history, _ := storage.GetRotationHistory(ctx, secret, 10)
	if len(history) != 0 {
		t.Error("Expected history to be cleared after Close")
	}
}

// FileRotationStorage tests

func TestFileStorage_CreateDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	logger := logging.New(false, true)

	dataDir := filepath.Join(tmpDir, "rotation-data")
	storage, err := NewFileRotationStorage(dataDir, logger)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	// Verify directories were created
	historyDir := filepath.Join(dataDir, "history")
	statusDir := filepath.Join(dataDir, "status")

	if _, err := os.Stat(historyDir); os.IsNotExist(err) {
		t.Error("history directory was not created")
	}

	if _, err := os.Stat(statusDir); os.IsNotExist(err) {
		t.Error("status directory was not created")
	}
}

func TestFileStorage_StoreAndRetrieveHistory(t *testing.T) {
	tmpDir := t.TempDir()
	logger := logging.New(false, true)

	storage, err := NewFileRotationStorage(tmpDir, logger)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	secret := SecretInfo{
		Key:      "TEST_SECRET",
		Provider: "aws",
	}

	rotatedAt := time.Now()
	result := RotationResult{
		Secret:    secret,
		Status:    StatusCompleted,
		RotatedAt: &rotatedAt,
		NewSecretRef: &SecretReference{
			Identifier: "new-ref-123",
			Version:    "v2",
		},
	}

	// Store result
	err = storage.StoreRotationResult(ctx, result)
	if err != nil {
		t.Fatalf("Failed to store rotation result: %v", err)
	}

	// Verify file was created
	historyDir := filepath.Join(tmpDir, "history")
	files, _ := os.ReadDir(historyDir)
	if len(files) != 1 {
		t.Errorf("Expected 1 history file, got %d", len(files))
	}

	// Retrieve history
	history, err := storage.GetRotationHistory(ctx, secret, 10)
	if err != nil {
		t.Fatalf("Failed to get rotation history: %v", err)
	}

	if len(history) != 1 {
		t.Fatalf("Expected 1 history entry, got %d", len(history))
	}

	if history[0].Secret.Key != secret.Key {
		t.Errorf("Expected secret key %s, got %s", secret.Key, history[0].Secret.Key)
	}

	if history[0].NewSecretRef == nil || history[0].NewSecretRef.Identifier != "new-ref-123" {
		t.Error("NewSecretRef not preserved correctly")
	}
}

func TestFileStorage_StatusPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	logger := logging.New(false, true)

	storage, err := NewFileRotationStorage(tmpDir, logger)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}

	ctx := context.Background()
	secret := SecretInfo{
		Key:      "TEST_SECRET",
		Provider: "aws",
	}

	// Update status
	status := RotationStatusInfo{
		Status:    StatusCompleted,
		CanRotate: false,
		Reason:    "Recently rotated",
	}

	err = storage.UpdateRotationStatus(ctx, secret, status)
	if err != nil {
		t.Fatalf("Failed to update rotation status: %v", err)
	}

	// Close and recreate storage to test persistence
	_ = storage.Close()

	storage2, err := NewFileRotationStorage(tmpDir, logger)
	if err != nil {
		t.Fatalf("Failed to recreate file storage: %v", err)
	}
	defer func() { _ = storage2.Close() }()

	// Retrieve status from new storage instance
	retrieved, err := storage2.GetRotationStatus(ctx, secret)
	if err != nil {
		t.Fatalf("Failed to get rotation status: %v", err)
	}

	if retrieved.Status != StatusCompleted {
		t.Errorf("Expected status %s, got %s", StatusCompleted, retrieved.Status)
	}

	if retrieved.CanRotate {
		t.Error("Expected CanRotate to be false")
	}
}

func TestFileStorage_MissingStatusReturnsDefault(t *testing.T) {
	tmpDir := t.TempDir()
	logger := logging.New(false, true)

	storage, err := NewFileRotationStorage(tmpDir, logger)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	secret := SecretInfo{Key: "NONEXISTENT", Provider: "aws"}

	status, err := storage.GetRotationStatus(ctx, secret)
	if err != nil {
		t.Fatalf("Failed to get rotation status: %v", err)
	}

	if status.Status != StatusPending {
		t.Errorf("Expected default status %s, got %s", StatusPending, status.Status)
	}

	if !status.CanRotate {
		t.Error("Expected CanRotate to be true by default")
	}
}

func TestFileStorage_ListSecrets(t *testing.T) {
	tmpDir := t.TempDir()
	logger := logging.New(false, true)

	storage, err := NewFileRotationStorage(tmpDir, logger)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()

	// Store history for one secret
	secret1 := SecretInfo{Key: "SECRET_1", Provider: "aws"}
	rotatedAt := time.Now()
	_ = storage.StoreRotationResult(ctx, RotationResult{Secret: secret1, Status: StatusCompleted, RotatedAt: &rotatedAt})

	// Store status for another secret
	secret2 := SecretInfo{Key: "SECRET_2", Provider: "vault"}
	_ = storage.UpdateRotationStatus(ctx, secret2, RotationStatusInfo{Status: StatusPending})

	// List all secrets
	secrets, err := storage.ListSecrets(ctx)
	if err != nil {
		t.Fatalf("Failed to list secrets: %v", err)
	}

	if len(secrets) != 2 {
		t.Errorf("Expected 2 secrets, got %d", len(secrets))
	}
}

func TestFileStorage_HistoryFiltersBySecret(t *testing.T) {
	tmpDir := t.TempDir()
	logger := logging.New(false, true)

	storage, err := NewFileRotationStorage(tmpDir, logger)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()

	// Store history for two different secrets
	secret1 := SecretInfo{Key: "SECRET_1", Provider: "aws"}
	secret2 := SecretInfo{Key: "SECRET_2", Provider: "aws"}

	rotatedAt := time.Now()
	_ = storage.StoreRotationResult(ctx, RotationResult{Secret: secret1, Status: StatusCompleted, RotatedAt: &rotatedAt})
	_ = storage.StoreRotationResult(ctx, RotationResult{Secret: secret2, Status: StatusCompleted, RotatedAt: &rotatedAt})

	// Retrieve history for secret1 only
	history, err := storage.GetRotationHistory(ctx, secret1, 10)
	if err != nil {
		t.Fatalf("Failed to get rotation history: %v", err)
	}

	if len(history) != 1 {
		t.Errorf("Expected 1 history entry for secret1, got %d", len(history))
	}

	if history[0].Secret.Key != "SECRET_1" {
		t.Errorf("Expected SECRET_1, got %s", history[0].Secret.Key)
	}
}

// Helper function tests

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"path/to/secret", "path_to_secret"},
		{"secret:name", "secret_name"},
		{"secret*name", "secret_name"},
		{"secret?name", "secret_name"},
		{"secret<name>", "secret_name_"},
		{"secret|name", "secret_name"},
		{"secret name", "secret_name"},
		{"secret\\name", "secret_name"},
		{"secret\"name", "secret_name"},
		{"a/b:c*d?e<f>g|h i\\j\"k", "a_b_c_d_e_f_g_h_i_j_k"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeFileName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeFileName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateRotationID(t *testing.T) {
	result := RotationResult{
		Secret: SecretInfo{
			Key:      "test/secret",
			Provider: "aws",
		},
	}

	id := generateRotationID(result)

	// ID should contain provider and sanitized key
	if id == "" {
		t.Error("Expected non-empty rotation ID")
	}

	// ID format: {provider}_{sanitized_key}_{timestamp}
	if len(id) < 10 {
		t.Errorf("Rotation ID seems too short: %s", id)
	}
}
