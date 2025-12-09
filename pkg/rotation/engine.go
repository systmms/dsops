package rotation

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/systmms/dsops/internal/dsopsdata"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/internal/rotation/health"
	"github.com/systmms/dsops/internal/rotation/notifications"
	rotationstorage "github.com/systmms/dsops/internal/rotation/storage"
	"github.com/systmms/dsops/internal/validation"
)

// DefaultRotationEngine implements the RotationEngine interface
type DefaultRotationEngine struct {
	strategies        map[string]SecretValueRotator
	storage           RotationStorage
	persistentStorage rotationstorage.Storage
	repository        *dsopsdata.Repository
	notifier          *notifications.Manager
	metrics           *health.RotationMetrics
	logger            *logging.Logger
	mu                sync.RWMutex
}

// NewRotationEngine creates a new rotation engine with in-memory storage
func NewRotationEngine(logger *logging.Logger) *DefaultRotationEngine {
	// Initialize persistent storage
	storageDir := rotationstorage.DefaultStorageDir()
	persistentStorage := rotationstorage.NewFileStorage(storageDir)

	return &DefaultRotationEngine{
		strategies:        make(map[string]SecretValueRotator),
		storage:           NewMemoryRotationStorage(),
		persistentStorage: persistentStorage,
		repository:        nil,
		notifier:          nil,
		metrics:           health.NewRotationMetrics(),
		logger:            logger,
	}
}

// SetNotifier sets the notification manager for rotation events.
// The notification manager must be started before it will send notifications.
func (e *DefaultRotationEngine) SetNotifier(notifier *notifications.Manager) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.notifier = notifier
	e.logger.Debug("Notification manager configured for rotation engine")
}

// NewRotationEngineWithStorage creates a new rotation engine with custom storage
func NewRotationEngineWithStorage(storage RotationStorage, logger *logging.Logger) *DefaultRotationEngine {
	// Initialize persistent storage
	storageDir := rotationstorage.DefaultStorageDir()
	persistentStorage := rotationstorage.NewFileStorage(storageDir)

	return &DefaultRotationEngine{
		strategies:        make(map[string]SecretValueRotator),
		storage:           storage,
		persistentStorage: persistentStorage,
		repository:        nil,
		metrics:           health.NewRotationMetrics(),
		logger:            logger,
	}
}

// SetRepository sets the dsops-data repository for schema-aware rotation
func (e *DefaultRotationEngine) SetRepository(repository *dsopsdata.Repository) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.repository = repository
	
	// Propagate repository to schema-aware strategies
	for name, strategy := range e.strategies {
		if schemaAware, ok := strategy.(SchemaAwareRotator); ok {
			schemaAware.SetRepository(repository)
			e.logger.Debug("Updated schema repository for strategy: %s", name)
		}
	}
	
	e.logger.Debug("Schema repository updated with %d service types", len(repository.ServiceTypes))
}

// RegisterStrategy adds a rotation strategy
func (e *DefaultRotationEngine) RegisterStrategy(strategy SecretValueRotator) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	name := strategy.Name()
	if _, exists := e.strategies[name]; exists {
		return fmt.Errorf("strategy '%s' already registered", name)
	}

	e.strategies[name] = strategy
	
	// Set repository on schema-aware strategies
	if e.repository != nil {
		if schemaAware, ok := strategy.(SchemaAwareRotator); ok {
			schemaAware.SetRepository(e.repository)
			e.logger.Debug("Set schema repository for newly registered strategy: %s", name)
		}
	}
	
	e.logger.Debug("Registered rotation strategy: %s", name)
	return nil
}

// GetStrategy returns a strategy by name
func (e *DefaultRotationEngine) GetStrategy(name string) (SecretValueRotator, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	strategy, exists := e.strategies[name]
	if !exists {
		return nil, fmt.Errorf("strategy '%s' not found", name)
	}

	return strategy, nil
}

// ListStrategies returns all available strategies
func (e *DefaultRotationEngine) ListStrategies() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	strategies := make([]string, 0, len(e.strategies))
	for name := range e.strategies {
		strategies = append(strategies, name)
	}
	return strategies
}

// AutoSelectStrategy automatically selects the best rotation strategy for a secret
func (e *DefaultRotationEngine) AutoSelectStrategy(ctx context.Context, secret SecretInfo) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// First, try to use schema-driven strategy selection
	if e.repository != nil {
		if serviceType, exists := e.repository.GetServiceType(string(secret.SecretType)); exists {
			// Use default rotation strategy from service type
			if serviceType.Spec.Defaults.RotationStrategy != "" {
				strategy := serviceType.Spec.Defaults.RotationStrategy
				e.logger.Debug("Using schema default strategy '%s' for service type '%s'", strategy, secret.SecretType)
				
				// Verify the strategy is available and supports the secret
				if strategyImpl, err := e.GetStrategy(strategy); err == nil {
					if strategyImpl.SupportsSecret(ctx, secret) {
						return strategy, nil
					}
				}
			}
		}
	}

	// Fallback to checking all registered strategies
	for name, strategy := range e.strategies {
		if strategy.SupportsSecret(ctx, secret) {
			e.logger.Debug("Auto-selected strategy '%s' for secret %s", name, logging.Secret(secret.Key))
			return name, nil
		}
	}

	return "", fmt.Errorf("no suitable rotation strategy found for secret type '%s'", secret.SecretType)
}

// GetServiceInstanceMetadata retrieves metadata from dsops-data for enhanced rotation
func (e *DefaultRotationEngine) GetServiceInstanceMetadata(serviceType, instanceID string) map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.repository == nil {
		return nil
	}

	instance, exists := e.repository.GetServiceInstance(serviceType, instanceID)
	if !exists {
		return nil
	}

	// Combine service instance config with endpoint and auth info
	metadata := make(map[string]interface{})
	if instance.Spec.Config != nil {
		for k, v := range instance.Spec.Config {
			metadata[k] = v
		}
	}
	
	// Add service instance specific fields
	metadata["endpoint"] = instance.Spec.Endpoint
	metadata["auth"] = instance.Spec.Auth
	metadata["service_type"] = instance.Metadata.Type
	metadata["instance_id"] = instance.Metadata.ID

	return metadata
}

// Rotate performs rotation using the appropriate strategy
func (e *DefaultRotationEngine) Rotate(ctx context.Context, request RotationRequest) (*RotationResult, error) {
	strategy, err := e.GetStrategy(request.Strategy)
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy: %w", err)
	}

	// Check if the strategy supports this secret
	if !strategy.SupportsSecret(ctx, request.Secret) {
		return &RotationResult{
			Secret: request.Secret,
			Status: StatusFailed,
			Error:  fmt.Sprintf("strategy '%s' does not support secret type '%s'", request.Strategy, request.Secret.SecretType),
		}, nil
	}

	// Create audit trail
	auditTrail := []AuditEntry{
		{
			Timestamp: time.Now(),
			Action:    "rotation_started",
			Component: "rotation_engine",
			Status:    "info",
			Message:   fmt.Sprintf("Starting rotation with strategy '%s'", request.Strategy),
			Details: map[string]interface{}{
				"secret_key": logging.Secret(request.Secret.Key),
				"strategy":   request.Strategy,
				"dry_run":    request.DryRun,
				"force":      request.Force,
			},
		},
	}

	e.logger.Info("Starting rotation for secret %s using strategy %s",
		logging.Secret(request.Secret.Key), request.Strategy)

	// Record metrics for rotation start
	rotationStartTime := time.Now()
	environment := ""
	if request.Config != nil {
		if env, ok := request.Config["environment"].(string); ok {
			environment = env
		}
	}
	if e.metrics != nil {
		e.metrics.RecordRotationStarted(string(request.Secret.SecretType), environment, request.Strategy)
	}

	// Send "started" notification
	startResult := &RotationResult{
		Secret: request.Secret,
		Status: StatusPending,
	}
	e.sendNotification(request, startResult, notifications.EventTypeStarted)

	// Create validator if repository is available
	var validator *validation.CredentialValidator
	if e.repository != nil {
		validator = validation.NewCredentialValidator(e.repository, e.logger)
	}

	// Validate new value if provided (pre-rotation validation)
	if request.NewValue != nil && request.NewValue.Value != "" && validator != nil {
		// Extract credential kind from metadata
		credentialKind := "default"
		if ck, exists := request.Secret.Metadata["credential_kind"]; exists {
			credentialKind = ck
		}
		
		validationResult := validator.ValidateNewCredential(
			string(request.Secret.SecretType),
			credentialKind,
			request.NewValue.Value,
			"", // Current value would be fetched from provider
		)
		
		if !validationResult.Valid {
			auditEntry := AuditEntry{
				Timestamp: time.Now(),
				Action:    "validation_failed",
				Component: "credential_validator",
				Status:    "error",
				Message:   "Credential validation failed",
				Details: map[string]interface{}{
					"errors": validationResult.Errors,
				},
			}
			auditTrail = append(auditTrail, auditEntry)
			
			return &RotationResult{
				Secret:     request.Secret,
				Status:     StatusFailed,
				Error:      fmt.Sprintf("Validation failed: %v", validationResult.Errors),
				AuditTrail: auditTrail,
			}, fmt.Errorf("validation failed: %v", validationResult.Errors)
		}
		
		// Add TTL to result if available
		if validationResult.TTLSeconds > 0 {
			expiresAt := time.Now().Add(time.Duration(validationResult.TTLSeconds) * time.Second)
			e.logger.Debug("Credential will expire at %v (TTL: %v seconds)", expiresAt, validationResult.TTLSeconds)
		}
	}

	// Enhance request with service instance metadata if available
	enhancedRequest := request
	if e.repository != nil {
		// Extract service type and instance ID from secret metadata or config
		if serviceType := request.Secret.SecretType; serviceType != "" {
			// Look for instance ID in metadata
			if instanceID, exists := request.Secret.Metadata["instance_id"]; exists {
				metadata := e.GetServiceInstanceMetadata(string(serviceType), instanceID)
				if metadata != nil {
					// Merge service instance metadata into request config
					if enhancedRequest.Config == nil {
						enhancedRequest.Config = make(map[string]interface{})
					}
					for k, v := range metadata {
						// Don't override existing config
						if _, exists := enhancedRequest.Config[k]; !exists {
							enhancedRequest.Config[k] = v
						}
					}
					e.logger.Debug("Enhanced rotation request with service instance metadata for %s/%s", serviceType, instanceID)
				}
			}
		}
	}

	// Perform the rotation
	result, err := strategy.Rotate(ctx, enhancedRequest)
	if err != nil {
		auditEntry := AuditEntry{
			Timestamp: time.Now(),
			Action:    "rotation_failed",
			Component: "rotation_strategy",
			Status:    "error",
			Message:   "Rotation failed",
			Error:     err.Error(),
		}
		auditTrail = append(auditTrail, auditEntry)

		return &RotationResult{
			Secret:     request.Secret,
			Status:     StatusFailed,
			Error:      err.Error(),
			AuditTrail: auditTrail,
		}, err
	}

	// Add audit trail to result
	result.AuditTrail = append(auditTrail, result.AuditTrail...)

	// Post-rotation TTL calculation based on credential type
	if result.Status == StatusCompleted && e.repository != nil {
		// Extract credential kind
		credentialKind := "default"
		if ck, exists := request.Secret.Metadata["credential_kind"]; exists {
			credentialKind = ck
		}
		
		// Get TTL from credential type definition
		if svcType, exists := e.repository.GetServiceType(string(request.Secret.SecretType)); exists {
			for _, credKind := range svcType.Spec.CredentialKinds {
				if credKind.Name == credentialKind && credKind.Constraints.TTL != "" {
					// Parse TTL
					ttl, err := time.ParseDuration(credKind.Constraints.TTL)
					if err != nil {
						// Try parsing as days (e.g., "365d")
						if len(credKind.Constraints.TTL) > 1 && credKind.Constraints.TTL[len(credKind.Constraints.TTL)-1] == 'd' {
							days := credKind.Constraints.TTL[:len(credKind.Constraints.TTL)-1]
							var d int
							if _, err := fmt.Sscanf(days, "%d", &d); err == nil {
								ttl = time.Duration(d) * 24 * time.Hour
							}
						}
					}
					
					if ttl > 0 {
						expiresAt := time.Now().Add(ttl)
						result.ExpiresAt = &expiresAt
						
						auditEntry := AuditEntry{
							Timestamp: time.Now(),
							Action:    "ttl_set",
							Component: "rotation_engine",
							Status:    "info",
							Message:   fmt.Sprintf("Credential expires at %v (TTL: %v)", expiresAt.Format(time.RFC3339), ttl),
							Details: map[string]interface{}{
								"ttl":         credKind.Constraints.TTL,
								"ttl_seconds": int64(ttl.Seconds()),
								"expires_at":  expiresAt,
							},
						}
						result.AuditTrail = append(result.AuditTrail, auditEntry)
						
						e.logger.Info("Rotated credential will expire at %v (TTL: %v)", expiresAt, ttl)
					}
					break
				}
			}
		}
	}

	// Store result in in-memory storage
	if err := e.storage.StoreRotationResult(ctx, *result); err != nil {
		e.logger.Warn("Failed to store rotation result in memory: %v", err)
		// Don't fail the rotation just because storage failed
	}
	
	// Store in persistent storage
	if e.persistentStorage != nil {
		// Create history entry
		historyEntry := &rotationstorage.HistoryEntry{
			Timestamp:      time.Now(),
			ServiceName:    string(request.Secret.SecretType),
			CredentialType: request.Secret.Metadata["credential_kind"],
			Action:         "rotate",
			Status:         string(result.Status),
			Duration:       time.Since(auditTrail[0].Timestamp),
			Strategy:       request.Strategy,
			User:           os.Getenv("USER"),
			Metadata:       make(map[string]string),
		}
		
		// Add error if failed
		if result.Status == StatusFailed && result.Error != "" {
			historyEntry.Error = result.Error
		}
		
		// Add version info if available (from secret refs)
		if result.OldSecretRef != nil && result.OldSecretRef.Version != "" {
			historyEntry.OldVersion = result.OldSecretRef.Version
		}
		if result.NewSecretRef != nil && result.NewSecretRef.Version != "" {
			historyEntry.NewVersion = result.NewSecretRef.Version
		}
		
		// Save history
		if err := e.persistentStorage.SaveHistory(historyEntry); err != nil {
			e.logger.Warn("Failed to save rotation history: %v", err)
		}
		
		// Update status
		status := &rotationstorage.RotationStatus{
			ServiceName:   string(request.Secret.SecretType),
			Status:        string(result.Status),
			LastRotation:  time.Now(),
			LastResult:    string(result.Status),
			RotationCount: 1, // This should be incremented from existing status
		}
		
		// Get existing status to update counts
		if existing, err := e.persistentStorage.GetStatus(status.ServiceName); err == nil {
			status.RotationCount = existing.RotationCount + 1
			status.SuccessCount = existing.SuccessCount
			status.FailureCount = existing.FailureCount
			
			if result.Status == StatusCompleted {
				status.SuccessCount++
			} else if result.Status == StatusFailed {
				status.FailureCount++
			}
		} else {
			// First rotation
			if result.Status == StatusCompleted {
				status.SuccessCount = 1
			} else if result.Status == StatusFailed {
				status.FailureCount = 1
			}
		}
		
		if result.Status == StatusFailed {
			status.LastError = result.Error
		}
		
		// Set next rotation if TTL is available
		if result.ExpiresAt != nil {
			status.NextRotation = result.ExpiresAt
		}
		
		// Save status
		if err := e.persistentStorage.SaveStatus(status); err != nil {
			e.logger.Warn("Failed to save rotation status: %v", err)
		}
	}

	// Update status based on result
	statusInfo := RotationStatusInfo{
		Status:      result.Status,
		CanRotate:   result.Status == StatusCompleted || result.Status == StatusFailed,
		LastRotated: result.RotatedAt,
	}

	switch result.Status {
	case StatusFailed:
		statusInfo.Reason = result.Error
	case StatusCompleted:
		statusInfo.Reason = "Successfully rotated"
	}

	if err := e.storage.UpdateRotationStatus(ctx, request.Secret, statusInfo); err != nil {
		e.logger.Warn("Failed to update rotation status: %v", err)
	}

	e.logger.Info("Completed rotation for secret %s with status %s",
		logging.Secret(request.Secret.Key), result.Status)

	// Record metrics for rotation completion
	if e.metrics != nil {
		status := "success"
		if result.Status == StatusFailed {
			status = "failure"
		}
		durationSeconds := time.Since(rotationStartTime).Seconds()
		e.metrics.RecordRotationCompleted(string(request.Secret.SecretType), environment, status, durationSeconds)
	}

	// Send completion notification
	if result.Status == StatusCompleted {
		e.sendNotification(request, result, notifications.EventTypeCompleted)
	} else if result.Status == StatusFailed {
		e.sendNotification(request, result, notifications.EventTypeFailed)
	}

	return result, nil
}

// BatchRotate rotates multiple secrets
func (e *DefaultRotationEngine) BatchRotate(ctx context.Context, requests []RotationRequest) ([]RotationResult, error) {
	results := make([]RotationResult, len(requests))
	var wg sync.WaitGroup
	
	// Use a semaphore to limit concurrent rotations
	semaphore := make(chan struct{}, 5) // Max 5 concurrent rotations
	
	for i, request := range requests {
		wg.Add(1)
		go func(idx int, req RotationRequest) {
			defer wg.Done()
			semaphore <- struct{}{} // Acquire
			defer func() { <-semaphore }() // Release

			result, err := e.Rotate(ctx, req)
			if err != nil {
				results[idx] = RotationResult{
					Secret: req.Secret,
					Status: StatusFailed,
					Error:  err.Error(),
				}
			} else {
				results[idx] = *result
			}
		}(i, request)
	}

	wg.Wait()
	return results, nil
}

// GetRotationHistory returns rotation history for a secret
func (e *DefaultRotationEngine) GetRotationHistory(ctx context.Context, secret SecretInfo, limit int) ([]RotationResult, error) {
	return e.storage.GetRotationHistory(ctx, secret, limit)
}

// ScheduleRotation schedules a rotation for future execution
func (e *DefaultRotationEngine) ScheduleRotation(ctx context.Context, request RotationRequest, when time.Time) error {
	// TODO: Implement rotation scheduling
	// This would typically use a job queue or scheduler
	return fmt.Errorf("rotation scheduling not yet implemented")
}

// GetRotationStatus returns the current rotation status for a secret
func (e *DefaultRotationEngine) GetRotationStatus(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
	return e.storage.GetRotationStatus(ctx, secret)
}

// ListSecrets returns all secrets that have rotation metadata
func (e *DefaultRotationEngine) ListSecrets(ctx context.Context) ([]SecretInfo, error) {
	return e.storage.ListSecrets(ctx)
}

// Close closes the rotation engine and any associated resources
func (e *DefaultRotationEngine) Close() error {
	return e.storage.Close()
}

// Helper function for creating audit entries
func createAuditEntry(action, component, status, message string, details map[string]interface{}) AuditEntry {
	return AuditEntry{
		Timestamp: time.Now(),
		Action:    action,
		Component: component,
		Status:    status,
		Message:   message,
		Details:   details,
	}
}

// sendNotification sends a rotation event notification if a notifier is configured.
func (e *DefaultRotationEngine) sendNotification(request RotationRequest, result *RotationResult, eventType notifications.EventType) {
	e.mu.RLock()
	notifier := e.notifier
	e.mu.RUnlock()

	if notifier == nil {
		return
	}

	// Map rotation status to notification status
	var status notifications.RotationStatus
	switch result.Status {
	case StatusCompleted:
		status = notifications.StatusSuccess
	case StatusFailed:
		status = notifications.StatusFailure
	default:
		status = notifications.StatusFailure
	}

	// Get environment from config or metadata
	environment := ""
	if request.Config != nil {
		if env, ok := request.Config["environment"].(string); ok {
			environment = env
		}
	}

	// Calculate duration
	var duration time.Duration
	if result.RotatedAt != nil {
		duration = time.Since(*result.RotatedAt)
	}

	// Get version info
	var previousVersion, newVersion string
	if result.OldSecretRef != nil {
		previousVersion = result.OldSecretRef.Version
	}
	if result.NewSecretRef != nil {
		newVersion = result.NewSecretRef.Version
	}

	// Create the notification event
	event := notifications.RotationEvent{
		Type:            eventType,
		Service:         string(request.Secret.SecretType),
		Environment:     environment,
		Strategy:        request.Strategy,
		Status:          status,
		Duration:        duration,
		Timestamp:       time.Now(),
		PreviousVersion: previousVersion,
		NewVersion:      newVersion,
		InitiatedBy:     os.Getenv("USER"),
		Metadata: map[string]string{
			"secret_key": request.Secret.Key,
		},
	}

	// Add error if present
	if result.Error != "" {
		event.Error = fmt.Errorf("%s", result.Error)
	}

	notifier.Send(event)
}