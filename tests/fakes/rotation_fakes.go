// Package fakes provides test doubles for dsops testing.
package fakes

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/systmms/dsops/internal/dsopsdata"
	"github.com/systmms/dsops/pkg/rotation"
)

// FakeSecretValueRotator provides a mock implementation of SecretValueRotator for testing.
type FakeSecretValueRotator struct {
	mu sync.Mutex

	// Configuration
	StrategyName     string
	SupportedTypes   []rotation.SecretType
	SupportsAllTypes bool

	// Mock behaviors
	RotateFunc   func(ctx context.Context, req rotation.RotationRequest) (*rotation.RotationResult, error)
	VerifyFunc   func(ctx context.Context, req rotation.VerificationRequest) error
	RollbackFunc func(ctx context.Context, req rotation.RollbackRequest) error
	StatusFunc   func(ctx context.Context, secret rotation.SecretInfo) (*rotation.RotationStatusInfo, error)

	// Recorded calls for verification
	RotateCalls   []rotation.RotationRequest
	VerifyCalls   []rotation.VerificationRequest
	RollbackCalls []rotation.RollbackRequest
	StatusCalls   []rotation.SecretInfo
}

// NewFakeSecretValueRotator creates a new fake rotator with default behaviors.
func NewFakeSecretValueRotator(name string) *FakeSecretValueRotator {
	return &FakeSecretValueRotator{
		StrategyName:   name,
		SupportedTypes: []rotation.SecretType{rotation.SecretTypePassword},
		RotateCalls:    make([]rotation.RotationRequest, 0),
		VerifyCalls:    make([]rotation.VerificationRequest, 0),
		RollbackCalls:  make([]rotation.RollbackRequest, 0),
		StatusCalls:    make([]rotation.SecretInfo, 0),
	}
}

// Name returns the strategy name.
func (f *FakeSecretValueRotator) Name() string {
	return f.StrategyName
}

// SupportsSecret checks if the rotator supports the given secret type.
func (f *FakeSecretValueRotator) SupportsSecret(_ context.Context, secret rotation.SecretInfo) bool {
	if f.SupportsAllTypes {
		return true
	}
	for _, t := range f.SupportedTypes {
		if t == secret.SecretType {
			return true
		}
	}
	return false
}

// Rotate performs the rotation operation.
func (f *FakeSecretValueRotator) Rotate(ctx context.Context, request rotation.RotationRequest) (*rotation.RotationResult, error) {
	f.mu.Lock()
	f.RotateCalls = append(f.RotateCalls, request)
	f.mu.Unlock()

	if f.RotateFunc != nil {
		return f.RotateFunc(ctx, request)
	}

	// Default successful rotation
	now := time.Now()
	return &rotation.RotationResult{
		Secret:       request.Secret,
		Status:       rotation.StatusCompleted,
		NewSecretRef: &rotation.SecretReference{Provider: request.Secret.Provider, Key: request.Secret.Key + "_new", Version: "v2"},
		OldSecretRef: &rotation.SecretReference{Provider: request.Secret.Provider, Key: request.Secret.Key, Version: "v1"},
		RotatedAt:    &now,
		AuditTrail: []rotation.AuditEntry{
			{Timestamp: now, Action: "rotation_completed", Component: f.StrategyName, Status: "success"},
		},
	}, nil
}

// Verify checks the new secret.
func (f *FakeSecretValueRotator) Verify(ctx context.Context, request rotation.VerificationRequest) error {
	f.mu.Lock()
	f.VerifyCalls = append(f.VerifyCalls, request)
	f.mu.Unlock()

	if f.VerifyFunc != nil {
		return f.VerifyFunc(ctx, request)
	}

	return nil // Default: verification passes
}

// Rollback reverts to the previous secret.
func (f *FakeSecretValueRotator) Rollback(ctx context.Context, request rotation.RollbackRequest) error {
	f.mu.Lock()
	f.RollbackCalls = append(f.RollbackCalls, request)
	f.mu.Unlock()

	if f.RollbackFunc != nil {
		return f.RollbackFunc(ctx, request)
	}

	return nil // Default: rollback succeeds
}

// GetStatus returns the rotation status.
func (f *FakeSecretValueRotator) GetStatus(ctx context.Context, secret rotation.SecretInfo) (*rotation.RotationStatusInfo, error) {
	f.mu.Lock()
	f.StatusCalls = append(f.StatusCalls, secret)
	f.mu.Unlock()

	if f.StatusFunc != nil {
		return f.StatusFunc(ctx, secret)
	}

	// Default status
	lastRotated := time.Now().Add(-24 * time.Hour)
	nextRotation := time.Now().Add(30 * 24 * time.Hour)
	return &rotation.RotationStatusInfo{
		Status:       rotation.StatusCompleted,
		LastRotated:  &lastRotated,
		NextRotation: &nextRotation,
		CanRotate:    true,
	}, nil
}

// Reset clears all recorded calls.
func (f *FakeSecretValueRotator) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RotateCalls = make([]rotation.RotationRequest, 0)
	f.VerifyCalls = make([]rotation.VerificationRequest, 0)
	f.RollbackCalls = make([]rotation.RollbackRequest, 0)
	f.StatusCalls = make([]rotation.SecretInfo, 0)
}

// FakeTwoSecretRotator provides a mock implementation of TwoSecretRotator.
type FakeTwoSecretRotator struct {
	FakeSecretValueRotator

	// Mock behaviors for two-secret operations
	CreateSecondaryFunc  func(ctx context.Context, req rotation.SecondarySecretRequest) (*rotation.SecretReference, error)
	PromoteSecondaryFunc func(ctx context.Context, req rotation.PromoteRequest) error
	DeprecatePrimaryFunc func(ctx context.Context, req rotation.DeprecateRequest) error

	// Recorded calls
	CreateSecondaryCalls  []rotation.SecondarySecretRequest
	PromoteSecondaryCalls []rotation.PromoteRequest
	DeprecatePrimaryCalls []rotation.DeprecateRequest
}

// NewFakeTwoSecretRotator creates a new fake two-secret rotator.
func NewFakeTwoSecretRotator(name string) *FakeTwoSecretRotator {
	fake := &FakeTwoSecretRotator{
		FakeSecretValueRotator: *NewFakeSecretValueRotator(name),
		CreateSecondaryCalls:   make([]rotation.SecondarySecretRequest, 0),
		PromoteSecondaryCalls:  make([]rotation.PromoteRequest, 0),
		DeprecatePrimaryCalls:  make([]rotation.DeprecateRequest, 0),
	}
	return fake
}

// CreateSecondarySecret creates a secondary secret.
func (f *FakeTwoSecretRotator) CreateSecondarySecret(ctx context.Context, request rotation.SecondarySecretRequest) (*rotation.SecretReference, error) {
	f.mu.Lock()
	f.CreateSecondaryCalls = append(f.CreateSecondaryCalls, request)
	f.mu.Unlock()

	if f.CreateSecondaryFunc != nil {
		return f.CreateSecondaryFunc(ctx, request)
	}

	// Default: create secondary reference
	return &rotation.SecretReference{
		Provider:   request.Secret.Provider,
		Key:        request.Secret.Key + "_secondary",
		Version:    "v2",
		Identifier: "secondary_" + request.Secret.Key,
	}, nil
}

// PromoteSecondarySecret promotes the secondary to primary.
func (f *FakeTwoSecretRotator) PromoteSecondarySecret(ctx context.Context, request rotation.PromoteRequest) error {
	f.mu.Lock()
	f.PromoteSecondaryCalls = append(f.PromoteSecondaryCalls, request)
	f.mu.Unlock()

	if f.PromoteSecondaryFunc != nil {
		return f.PromoteSecondaryFunc(ctx, request)
	}

	return nil // Default: promotion succeeds
}

// DeprecatePrimarySecret deprecates the old primary.
func (f *FakeTwoSecretRotator) DeprecatePrimarySecret(ctx context.Context, request rotation.DeprecateRequest) error {
	f.mu.Lock()
	f.DeprecatePrimaryCalls = append(f.DeprecatePrimaryCalls, request)
	f.mu.Unlock()

	if f.DeprecatePrimaryFunc != nil {
		return f.DeprecatePrimaryFunc(ctx, request)
	}

	return nil // Default: deprecation succeeds
}

// Reset clears all recorded calls including two-secret operations.
func (f *FakeTwoSecretRotator) Reset() {
	f.FakeSecretValueRotator.Reset()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.CreateSecondaryCalls = make([]rotation.SecondarySecretRequest, 0)
	f.PromoteSecondaryCalls = make([]rotation.PromoteRequest, 0)
	f.DeprecatePrimaryCalls = make([]rotation.DeprecateRequest, 0)
}

// FakeSchemaAwareRotator provides a mock implementation that uses dsops-data schemas.
type FakeSchemaAwareRotator struct {
	FakeSecretValueRotator
	Repository *dsopsdata.Repository
}

// NewFakeSchemaAwareRotator creates a new schema-aware rotator.
func NewFakeSchemaAwareRotator(name string) *FakeSchemaAwareRotator {
	return &FakeSchemaAwareRotator{
		FakeSecretValueRotator: *NewFakeSecretValueRotator(name),
	}
}

// SetRepository sets the dsops-data repository.
func (f *FakeSchemaAwareRotator) SetRepository(repository *dsopsdata.Repository) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Repository = repository
}

// FakeRotationEngine provides a mock implementation of RotationEngine.
type FakeRotationEngine struct {
	mu sync.Mutex

	// Registered strategies
	Strategies map[string]rotation.SecretValueRotator

	// Mock behaviors
	RotateFunc          func(ctx context.Context, req rotation.RotationRequest) (*rotation.RotationResult, error)
	BatchRotateFunc     func(ctx context.Context, reqs []rotation.RotationRequest) ([]rotation.RotationResult, error)
	GetHistoryFunc      func(ctx context.Context, secret rotation.SecretInfo, limit int) ([]rotation.RotationResult, error)
	ScheduleRotationFunc func(ctx context.Context, req rotation.RotationRequest, when time.Time) error

	// Recorded calls
	RotateCalls        []rotation.RotationRequest
	BatchRotateCalls   [][]rotation.RotationRequest
	GetHistoryCalls    []historyCall
	ScheduleCalls      []scheduleCall
}

type historyCall struct {
	Secret rotation.SecretInfo
	Limit  int
}

type scheduleCall struct {
	Request rotation.RotationRequest
	When    time.Time
}

// NewFakeRotationEngine creates a new fake rotation engine.
func NewFakeRotationEngine() *FakeRotationEngine {
	return &FakeRotationEngine{
		Strategies:       make(map[string]rotation.SecretValueRotator),
		RotateCalls:      make([]rotation.RotationRequest, 0),
		BatchRotateCalls: make([][]rotation.RotationRequest, 0),
		GetHistoryCalls:  make([]historyCall, 0),
		ScheduleCalls:    make([]scheduleCall, 0),
	}
}

// RegisterStrategy registers a rotation strategy.
func (f *FakeRotationEngine) RegisterStrategy(strategy rotation.SecretValueRotator) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Strategies[strategy.Name()] = strategy
	return nil
}

// GetStrategy returns a registered strategy by name.
func (f *FakeRotationEngine) GetStrategy(name string) (rotation.SecretValueRotator, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if strategy, ok := f.Strategies[name]; ok {
		return strategy, nil
	}
	return nil, fmt.Errorf("strategy not found: %s", name)
}

// ListStrategies returns all registered strategy names.
func (f *FakeRotationEngine) ListStrategies() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	names := make([]string, 0, len(f.Strategies))
	for name := range f.Strategies {
		names = append(names, name)
	}
	return names
}

// Rotate performs a rotation operation.
func (f *FakeRotationEngine) Rotate(ctx context.Context, request rotation.RotationRequest) (*rotation.RotationResult, error) {
	f.mu.Lock()
	f.RotateCalls = append(f.RotateCalls, request)
	f.mu.Unlock()

	if f.RotateFunc != nil {
		return f.RotateFunc(ctx, request)
	}

	// Use registered strategy if available
	strategy, err := f.GetStrategy(request.Strategy)
	if err != nil {
		return nil, err
	}

	return strategy.Rotate(ctx, request)
}

// BatchRotate performs multiple rotation operations.
func (f *FakeRotationEngine) BatchRotate(ctx context.Context, requests []rotation.RotationRequest) ([]rotation.RotationResult, error) {
	f.mu.Lock()
	f.BatchRotateCalls = append(f.BatchRotateCalls, requests)
	f.mu.Unlock()

	if f.BatchRotateFunc != nil {
		return f.BatchRotateFunc(ctx, requests)
	}

	// Default: rotate each request individually
	results := make([]rotation.RotationResult, len(requests))
	for i, req := range requests {
		result, err := f.Rotate(ctx, req)
		if err != nil {
			results[i] = rotation.RotationResult{
				Secret: req.Secret,
				Status: rotation.StatusFailed,
				Error:  err.Error(),
			}
		} else {
			results[i] = *result
		}
	}
	return results, nil
}

// GetRotationHistory returns rotation history.
func (f *FakeRotationEngine) GetRotationHistory(ctx context.Context, secret rotation.SecretInfo, limit int) ([]rotation.RotationResult, error) {
	f.mu.Lock()
	f.GetHistoryCalls = append(f.GetHistoryCalls, historyCall{Secret: secret, Limit: limit})
	f.mu.Unlock()

	if f.GetHistoryFunc != nil {
		return f.GetHistoryFunc(ctx, secret, limit)
	}

	// Default: return empty history
	return []rotation.RotationResult{}, nil
}

// ScheduleRotation schedules a future rotation.
func (f *FakeRotationEngine) ScheduleRotation(ctx context.Context, request rotation.RotationRequest, when time.Time) error {
	f.mu.Lock()
	f.ScheduleCalls = append(f.ScheduleCalls, scheduleCall{Request: request, When: when})
	f.mu.Unlock()

	if f.ScheduleRotationFunc != nil {
		return f.ScheduleRotationFunc(ctx, request, when)
	}

	return nil // Default: scheduling succeeds
}

// Reset clears all recorded calls and strategies.
func (f *FakeRotationEngine) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Strategies = make(map[string]rotation.SecretValueRotator)
	f.RotateCalls = make([]rotation.RotationRequest, 0)
	f.BatchRotateCalls = make([][]rotation.RotationRequest, 0)
	f.GetHistoryCalls = make([]historyCall, 0)
	f.ScheduleCalls = make([]scheduleCall, 0)
}

// FakeRotationStorage provides in-memory storage for rotation state.
type FakeRotationStorage struct {
	mu sync.Mutex

	// Storage maps
	RotationHistory map[string][]rotation.RotationResult // key -> results
	RotationStatus  map[string]*rotation.RotationStatusInfo

	// Counters
	SaveCount   int
	LoadCount   int
	DeleteCount int
}

// NewFakeRotationStorage creates a new fake rotation storage.
func NewFakeRotationStorage() *FakeRotationStorage {
	return &FakeRotationStorage{
		RotationHistory: make(map[string][]rotation.RotationResult),
		RotationStatus:  make(map[string]*rotation.RotationStatusInfo),
	}
}

// SaveResult stores a rotation result.
func (f *FakeRotationStorage) SaveResult(secret rotation.SecretInfo, result rotation.RotationResult) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := secret.Key
	f.RotationHistory[key] = append(f.RotationHistory[key], result)

	// Update status based on result
	f.RotationStatus[key] = &rotation.RotationStatusInfo{
		Status:       result.Status,
		LastRotated:  result.RotatedAt,
		NextRotation: result.ExpiresAt,
		CanRotate:    result.Status == rotation.StatusCompleted,
	}

	f.SaveCount++
	return nil
}

// GetHistory retrieves rotation history for a secret.
func (f *FakeRotationStorage) GetHistory(secret rotation.SecretInfo, limit int) ([]rotation.RotationResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.LoadCount++

	history, ok := f.RotationHistory[secret.Key]
	if !ok {
		return []rotation.RotationResult{}, nil
	}

	if limit <= 0 || limit >= len(history) {
		return history, nil
	}

	// Return last N entries
	return history[len(history)-limit:], nil
}

// GetStatus retrieves rotation status for a secret.
func (f *FakeRotationStorage) GetStatus(secret rotation.SecretInfo) (*rotation.RotationStatusInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.LoadCount++

	status, ok := f.RotationStatus[secret.Key]
	if !ok {
		return &rotation.RotationStatusInfo{
			Status:    rotation.StatusPending,
			CanRotate: true,
		}, nil
	}

	return status, nil
}

// DeleteHistory removes rotation history for a secret.
func (f *FakeRotationStorage) DeleteHistory(secret rotation.SecretInfo) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.RotationHistory, secret.Key)
	delete(f.RotationStatus, secret.Key)

	f.DeleteCount++
	return nil
}

// Reset clears all storage.
func (f *FakeRotationStorage) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.RotationHistory = make(map[string][]rotation.RotationResult)
	f.RotationStatus = make(map[string]*rotation.RotationStatusInfo)
	f.SaveCount = 0
	f.LoadCount = 0
	f.DeleteCount = 0
}
