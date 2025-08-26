package rotation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/systmms/dsops/internal/logging"
)

// RotationStorage provides persistent storage for rotation metadata and history
type RotationStorage interface {
	// StoreRotationResult saves a rotation result to persistent storage
	StoreRotationResult(ctx context.Context, result RotationResult) error

	// GetRotationHistory retrieves rotation history for a secret
	GetRotationHistory(ctx context.Context, secret SecretInfo, limit int) ([]RotationResult, error)

	// GetRotationStatus gets the current rotation status for a secret
	GetRotationStatus(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error)

	// UpdateRotationStatus updates the current rotation status for a secret
	UpdateRotationStatus(ctx context.Context, secret SecretInfo, status RotationStatusInfo) error

	// ListSecrets returns all secrets that have rotation metadata
	ListSecrets(ctx context.Context) ([]SecretInfo, error)

	// Close closes any resources used by the storage
	Close() error
}

// FileRotationStorage implements RotationStorage using local file system
type FileRotationStorage struct {
	dataDir string
	logger  *logging.Logger
	mu      sync.RWMutex
}

// NewFileRotationStorage creates a new file-based rotation storage
func NewFileRotationStorage(dataDir string, logger *logging.Logger) (*FileRotationStorage, error) {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create rotation data directory: %w", err)
	}

	// Create subdirectories
	for _, subdir := range []string{"history", "status"} {
		path := filepath.Join(dataDir, subdir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return nil, fmt.Errorf("failed to create %s directory: %w", subdir, err)
		}
	}

	return &FileRotationStorage{
		dataDir: dataDir,
		logger:  logger,
	}, nil
}

// StoredRotationResult extends RotationResult with storage metadata
type StoredRotationResult struct {
	RotationResult
	StoredAt time.Time `json:"stored_at"`
	ID       string    `json:"id"`
}

// StoredRotationStatus represents stored rotation status
type StoredRotationStatus struct {
	SecretKey   string              `json:"secret_key"`
	Provider    string              `json:"provider"`
	Status      RotationStatusInfo  `json:"status"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

// StoreRotationResult saves a rotation result to persistent storage
func (f *FileRotationStorage) StoreRotationResult(ctx context.Context, result RotationResult) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Create a stored result with metadata
	stored := StoredRotationResult{
		RotationResult: result,
		StoredAt:       time.Now(),
		ID:             generateRotationID(result),
	}

	// Determine file path based on secret key
	fileName := fmt.Sprintf("%s_%d.json", 
		sanitizeFileName(result.Secret.Key), 
		stored.StoredAt.Unix())
	filePath := filepath.Join(f.dataDir, "history", fileName)

	// Write to file
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal rotation result: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write rotation result: %w", err)
	}

	f.logger.Debug("Stored rotation result for %s at %s", 
		logging.Secret(result.Secret.Key), filePath)

	return nil
}

// GetRotationHistory retrieves rotation history for a secret
func (f *FileRotationStorage) GetRotationHistory(ctx context.Context, secret SecretInfo, limit int) ([]RotationResult, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	historyDir := filepath.Join(f.dataDir, "history")
	files, err := os.ReadDir(historyDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read history directory: %w", err)
	}

	var results []StoredRotationResult
	prefix := sanitizeFileName(secret.Key) + "_"

	// Read all history files for this secret
	for _, file := range files {
		if file.IsDir() || !strings.HasPrefix(file.Name(), prefix) {
			continue
		}

		filePath := filepath.Join(historyDir, file.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			f.logger.Debug("Failed to read history file %s: %v", filePath, err)
			continue
		}

		var stored StoredRotationResult
		if err := json.Unmarshal(data, &stored); err != nil {
			f.logger.Debug("Failed to unmarshal history file %s: %v", filePath, err)
			continue
		}

		// Verify this is for the correct secret
		if stored.Secret.Key == secret.Key && stored.Secret.Provider == secret.Provider {
			results = append(results, stored)
		}
	}

	// Sort by rotation time (newest first)
	sort.Slice(results, func(i, j int) bool {
		if results[i].RotatedAt == nil {
			return false
		}
		if results[j].RotatedAt == nil {
			return true
		}
		return results[i].RotatedAt.After(*results[j].RotatedAt)
	})

	// Apply limit
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	// Convert back to RotationResult
	history := make([]RotationResult, len(results))
	for i, stored := range results {
		history[i] = stored.RotationResult
	}

	return history, nil
}

// GetRotationStatus gets the current rotation status for a secret
func (f *FileRotationStorage) GetRotationStatus(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	fileName := fmt.Sprintf("%s_%s.json", 
		sanitizeFileName(secret.Key), 
		sanitizeFileName(secret.Provider))
	filePath := filepath.Join(f.dataDir, "status", fileName)

	// Check if status file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// No status stored, return default
		return &RotationStatusInfo{
			Status:    StatusPending,
			CanRotate: true,
			Reason:    "No rotation history found",
		}, nil
	}

	// Read status file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read status file: %w", err)
	}

	var stored StoredRotationStatus
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("failed to unmarshal status file: %w", err)
	}

	return &stored.Status, nil
}

// UpdateRotationStatus updates the current rotation status for a secret
func (f *FileRotationStorage) UpdateRotationStatus(ctx context.Context, secret SecretInfo, status RotationStatusInfo) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	stored := StoredRotationStatus{
		SecretKey: secret.Key,
		Provider:  secret.Provider,
		Status:    status,
		UpdatedAt: time.Now(),
	}

	fileName := fmt.Sprintf("%s_%s.json", 
		sanitizeFileName(secret.Key), 
		sanitizeFileName(secret.Provider))
	filePath := filepath.Join(f.dataDir, "status", fileName)

	// Write to file
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write status file: %w", err)
	}

	f.logger.Debug("Updated rotation status for %s", logging.Secret(secret.Key))

	return nil
}

// ListSecrets returns all secrets that have rotation metadata
func (f *FileRotationStorage) ListSecrets(ctx context.Context) ([]SecretInfo, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var secrets []SecretInfo
	secretMap := make(map[string]SecretInfo) // deduplicate

	// Check status directory
	statusDir := filepath.Join(f.dataDir, "status")
	if files, err := os.ReadDir(statusDir); err == nil {
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}

			filePath := filepath.Join(statusDir, file.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			var stored StoredRotationStatus
			if err := json.Unmarshal(data, &stored); err != nil {
				continue
			}

			key := fmt.Sprintf("%s:%s", stored.Provider, stored.SecretKey)
			secretMap[key] = SecretInfo{
				Key:      stored.SecretKey,
				Provider: stored.Provider,
			}
		}
	}

	// Check history directory for additional secrets
	historyDir := filepath.Join(f.dataDir, "history")
	if files, err := os.ReadDir(historyDir); err == nil {
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}

			filePath := filepath.Join(historyDir, file.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			var stored StoredRotationResult
			if err := json.Unmarshal(data, &stored); err != nil {
				continue
			}

			key := fmt.Sprintf("%s:%s", stored.Secret.Provider, stored.Secret.Key)
			secretMap[key] = stored.Secret
		}
	}

	// Convert map to slice
	for _, secret := range secretMap {
		secrets = append(secrets, secret)
	}

	return secrets, nil
}

// Close closes any resources used by the storage
func (f *FileRotationStorage) Close() error {
	// File storage doesn't need explicit cleanup
	return nil
}

// Helper functions

func generateRotationID(result RotationResult) string {
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s_%s_%d", 
		result.Secret.Provider, 
		sanitizeFileName(result.Secret.Key), 
		timestamp)
}

func sanitizeFileName(name string) string {
	// Replace problematic characters for file names
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)
	return replacer.Replace(name)
}

// MemoryRotationStorage implements RotationStorage using in-memory storage (for testing)
type MemoryRotationStorage struct {
	history map[string][]RotationResult
	status  map[string]RotationStatusInfo
	mu      sync.RWMutex
}

// NewMemoryRotationStorage creates a new in-memory rotation storage
func NewMemoryRotationStorage() *MemoryRotationStorage {
	return &MemoryRotationStorage{
		history: make(map[string][]RotationResult),
		status:  make(map[string]RotationStatusInfo),
	}
}

// StoreRotationResult saves a rotation result to memory
func (m *MemoryRotationStorage) StoreRotationResult(ctx context.Context, result RotationResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s:%s", result.Secret.Provider, result.Secret.Key)
	m.history[key] = append(m.history[key], result)

	// Keep only last 100 entries per secret
	if len(m.history[key]) > 100 {
		m.history[key] = m.history[key][len(m.history[key])-100:]
	}

	return nil
}

// GetRotationHistory retrieves rotation history for a secret
func (m *MemoryRotationStorage) GetRotationHistory(ctx context.Context, secret SecretInfo, limit int) ([]RotationResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", secret.Provider, secret.Key)
	history := m.history[key]

	if history == nil {
		return []RotationResult{}, nil
	}

	// Sort by rotation time (newest first)
	sorted := make([]RotationResult, len(history))
	copy(sorted, history)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].RotatedAt == nil {
			return false
		}
		if sorted[j].RotatedAt == nil {
			return true
		}
		return sorted[i].RotatedAt.After(*sorted[j].RotatedAt)
	})

	// Apply limit
	if limit > 0 && len(sorted) > limit {
		sorted = sorted[:limit]
	}

	return sorted, nil
}

// GetRotationStatus gets the current rotation status for a secret
func (m *MemoryRotationStorage) GetRotationStatus(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", secret.Provider, secret.Key)
	if status, exists := m.status[key]; exists {
		return &status, nil
	}

	// Return default status
	return &RotationStatusInfo{
		Status:    StatusPending,
		CanRotate: true,
		Reason:    "No rotation history found",
	}, nil
}

// UpdateRotationStatus updates the current rotation status for a secret
func (m *MemoryRotationStorage) UpdateRotationStatus(ctx context.Context, secret SecretInfo, status RotationStatusInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s:%s", secret.Provider, secret.Key)
	m.status[key] = status
	return nil
}

// ListSecrets returns all secrets that have rotation metadata
func (m *MemoryRotationStorage) ListSecrets(ctx context.Context) ([]SecretInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	secretMap := make(map[string]SecretInfo)

	// Add secrets from history
	for key, history := range m.history {
		if len(history) > 0 {
			secretMap[key] = history[0].Secret
		}
	}

	// Add secrets from status (may not have history yet)
	for key := range m.status {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) == 2 {
			secretMap[key] = SecretInfo{
				Provider: parts[0],
				Key:      parts[1],
			}
		}
	}

	secrets := make([]SecretInfo, 0, len(secretMap))
	for _, secret := range secretMap {
		secrets = append(secrets, secret)
	}

	return secrets, nil
}

// Close closes any resources used by the storage
func (m *MemoryRotationStorage) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.history = make(map[string][]RotationResult)
	m.status = make(map[string]RotationStatusInfo)
	return nil
}