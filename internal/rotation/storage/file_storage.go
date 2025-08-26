package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// FileStorage implements Storage using the filesystem
type FileStorage struct {
	baseDir string
	mu      sync.RWMutex
}

// NewFileStorage creates a new file-based storage
func NewFileStorage(baseDir string) *FileStorage {
	return &FileStorage{
		baseDir: baseDir,
	}
}

// DefaultStorageDir returns the default storage directory
func DefaultStorageDir() string {
	// Check for test environment variable first
	if testDir := os.Getenv("DSOPS_ROTATION_DIR"); testDir != "" {
		return testDir
	}
	
	// Try to use XDG_DATA_HOME first
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, "dsops", "rotation")
	}
	
	// Fall back to ~/.local/share
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "dsops", "rotation")
	}
	
	// Last resort: use temp directory
	return filepath.Join(os.TempDir(), "dsops", "rotation")
}

// SaveStatus saves the current rotation status for a service
func (fs *FileStorage) SaveStatus(status *RotationStatus) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	// Ensure directory exists
	statusDir := filepath.Join(fs.baseDir, "status")
	if err := os.MkdirAll(statusDir, 0700); err != nil {
		return fmt.Errorf("failed to create status directory: %w", err)
	}
	
	// Write status file
	filename := filepath.Join(statusDir, fmt.Sprintf("%s.json", sanitizeFilename(status.ServiceName)))
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}
	
	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write status file: %w", err)
	}
	
	return nil
}

// GetStatus retrieves the current rotation status for a service
func (fs *FileStorage) GetStatus(serviceName string) (*RotationStatus, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	
	filename := filepath.Join(fs.baseDir, "status", fmt.Sprintf("%s.json", sanitizeFilename(serviceName)))
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no status found for service %s", serviceName)
		}
		return nil, fmt.Errorf("failed to read status file: %w", err)
	}
	
	var status RotationStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal status: %w", err)
	}
	
	return &status, nil
}

// SaveHistory saves a rotation history entry
func (fs *FileStorage) SaveHistory(entry *HistoryEntry) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	// Ensure directory exists
	historyDir := filepath.Join(fs.baseDir, "history", sanitizeFilename(entry.ServiceName))
	if err := os.MkdirAll(historyDir, 0700); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}
	
	// Generate unique ID if not set
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("%d-%s", entry.Timestamp.UnixNano(), entry.ServiceName)
	}
	
	// Write history entry
	filename := filepath.Join(historyDir, fmt.Sprintf("%s.json", entry.Timestamp.Format("20060102-150405")))
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history entry: %w", err)
	}
	
	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}
	
	return nil
}

// GetHistory retrieves rotation history for a service
func (fs *FileStorage) GetHistory(serviceName string, limit int) ([]HistoryEntry, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	
	historyDir := filepath.Join(fs.baseDir, "history", sanitizeFilename(serviceName))
	
	// Check if directory exists
	if _, err := os.Stat(historyDir); os.IsNotExist(err) {
		return []HistoryEntry{}, nil
	}
	
	// Read all history files
	files, err := os.ReadDir(historyDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read history directory: %w", err)
	}
	
	var entries []HistoryEntry
	
	// Sort files by name (newest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() > files[j].Name()
	})
	
	// Read files up to limit
	count := 0
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			filename := filepath.Join(historyDir, file.Name())
			data, err := os.ReadFile(filename)
			if err != nil {
				continue // Skip files that can't be read
			}
			
			var entry HistoryEntry
			if err := json.Unmarshal(data, &entry); err != nil {
				continue // Skip invalid JSON files
			}
			
			entries = append(entries, entry)
			count++
			
			if limit > 0 && count >= limit {
				break
			}
		}
	}
	
	return entries, nil
}

// GetAllHistory retrieves rotation history for all services
func (fs *FileStorage) GetAllHistory(limit int) ([]HistoryEntry, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	
	historyDir := filepath.Join(fs.baseDir, "history")
	
	// Check if directory exists
	if _, err := os.Stat(historyDir); os.IsNotExist(err) {
		return []HistoryEntry{}, nil
	}
	
	// Read all service directories
	serviceDirs, err := os.ReadDir(historyDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read history directory: %w", err)
	}
	
	var allEntries []HistoryEntry
	
	// Read history from each service directory
	for _, serviceDir := range serviceDirs {
		if serviceDir.IsDir() {
			serviceName := serviceDir.Name()
			entries, err := fs.GetHistory(serviceName, -1) // Get all entries for this service
			if err != nil {
				continue // Skip services with errors
			}
			allEntries = append(allEntries, entries...)
		}
	}
	
	// Sort all entries by timestamp (newest first)
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Timestamp.After(allEntries[j].Timestamp)
	})
	
	// Apply limit
	if limit > 0 && len(allEntries) > limit {
		allEntries = allEntries[:limit]
	}
	
	return allEntries, nil
}

// CleanupOldEntries removes history entries older than the specified duration
func (fs *FileStorage) CleanupOldEntries(olderThan time.Duration) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	historyDir := filepath.Join(fs.baseDir, "history")
	cutoffTime := time.Now().Add(-olderThan)
	
	// Walk through all history directories
	err := filepath.Walk(historyDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		
		// Only process JSON files
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			// Parse timestamp from filename
			filename := filepath.Base(path)
			// Expected format: 20060102-150405.json
			if len(filename) >= 15 {
				timestampStr := filename[:15]
				if timestamp, err := time.Parse("20060102-150405", timestampStr); err == nil {
					if timestamp.Before(cutoffTime) {
						// Delete old file
						if err := os.Remove(path); err != nil {
							// Log error but continue
							fmt.Fprintf(os.Stderr, "Failed to remove old history file %s: %v\n", path, err)
						}
					}
				}
			}
		}
		
		return nil
	})
	
	return err
}

// sanitizeFilename replaces characters that might be problematic in filenames
func sanitizeFilename(name string) string {
	// Replace potentially problematic characters
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
		" ", "_",
	)
	return replacer.Replace(name)
}