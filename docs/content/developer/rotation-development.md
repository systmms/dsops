---
title: "Rotation Development Guide"
description: "Build custom rotation strategies for automated secret lifecycle management"
lead: "Learn how to implement rotation strategies that automatically update secrets in services while maintaining security and availability. This guide covers the rotation interfaces, strategies, and best practices."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 20
---

## Overview

Secret rotation is critical for security - it limits exposure time and reduces the impact of compromised credentials. dsops provides a comprehensive rotation framework that supports multiple strategies and can be extended for any service.

## Core Concepts

### Secret Value Rotation vs Storage Rotation

dsops distinguishes between two types of rotation:

1. **Secret Value Rotation** (this guide): Updating the actual credential values used by services
2. **Storage Rotation**: Creating new versions in secret stores (handled by Provider interface)

Examples of secret value rotation:
- Changing a PostgreSQL user's password
- Generating new Stripe API keys
- Issuing new TLS certificates
- Refreshing OAuth tokens

### Rotation Strategies

dsops supports multiple rotation strategies:

- **Immediate**: Replace secret instantly (brief downtime acceptable)
- **Two-Key**: Maintain two valid secrets for zero-downtime rotation
- **Overlap**: Gradual transition with configurable overlap period
- **Gradual**: Percentage-based rollout for large deployments
- **Custom**: User-defined scripts for special cases

## The SecretValueRotator Interface

The core interface for rotation strategies is defined in `pkg/rotation/interface.go`:

```go
type SecretValueRotator interface {
    // Name returns the unique strategy identifier
    Name() string

    // SupportsSecret determines if this rotator can handle the given secret
    SupportsSecret(ctx context.Context, secret SecretInfo) bool

    // Rotate performs the complete secret rotation lifecycle
    Rotate(ctx context.Context, request RotationRequest) (*RotationResult, error)

    // Verify checks that new secret credentials are working correctly
    Verify(ctx context.Context, request VerificationRequest) error

    // Rollback reverts to the previous secret value if possible
    Rollback(ctx context.Context, request RollbackRequest) error

    // GetStatus returns the current rotation status for a secret
    GetStatus(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error)
}
```

### Complete API Documentation

For detailed API documentation:
- **[pkg/rotation GoDoc](https://pkg.go.dev/github.com/systmms/dsops/pkg/rotation)** - Full interface documentation
- **[Rotation examples](https://github.com/systmms/dsops/tree/main/pkg/rotation/examples_test.go)** - Working code examples

## Implementation Steps

### 1. Create Rotator Structure

Start by creating a struct that implements SecretValueRotator:

```go
package myrotator

import (
    "context"
    "fmt"
    "time"
    
    "github.com/systmms/dsops/pkg/rotation"
    "github.com/systmms/dsops/pkg/provider"
)

type PostgresRotator struct {
    // Database connection pool
    db *sql.DB
    
    // Provider for storing new passwords
    secretProvider provider.Provider
}

func NewPostgresRotator(connStr string, provider provider.Provider) (*PostgresRotator, error) {
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %w", err)
    }
    
    return &PostgresRotator{
        db:             db,
        secretProvider: provider,
    }, nil
}
```

### 2. Implement Name and SupportsSecret

```go
func (r *PostgresRotator) Name() string {
    return "postgresql"
}

func (r *PostgresRotator) SupportsSecret(ctx context.Context, secret rotation.SecretInfo) bool {
    // Check if this is a PostgreSQL password secret
    if secret.SecretType != rotation.SecretTypePassword {
        return false
    }
    
    // Check for required metadata
    dbType, ok := secret.Metadata["database_type"]
    if !ok || dbType != "postgresql" {
        return false
    }
    
    // Verify we have required configuration
    if secret.Metadata["database_name"] == "" || secret.Metadata["username"] == "" {
        return false
    }
    
    return true
}
```

### 3. Implement the Rotate Method

The Rotate method orchestrates the complete rotation lifecycle:

```go
func (r *PostgresRotator) Rotate(ctx context.Context, request rotation.RotationRequest) (*rotation.RotationResult, error) {
    startTime := time.Now()
    result := &rotation.RotationResult{
        Secret: request.Secret,
        Status: rotation.StatusRotating,
        AuditTrail: []rotation.AuditEntry{
            {
                Timestamp: startTime,
                Action:    "rotation_started",
                Component: "postgresql",
                Status:    "success",
            },
        },
    }
    
    // Handle dry run
    if request.DryRun {
        return r.planRotation(ctx, request)
    }
    
    // Step 1: Generate new password
    newPassword, err := r.generatePassword(request)
    if err != nil {
        result.Status = rotation.StatusFailed
        result.Error = fmt.Sprintf("password generation failed: %v", err)
        return result, err
    }
    
    result.AuditTrail = append(result.AuditTrail, rotation.AuditEntry{
        Timestamp: time.Now(),
        Action:    "password_generated",
        Component: "postgresql",
        Status:    "success",
        Details: map[string]interface{}{
            "length": len(newPassword),
        },
    })
    
    // Step 2: Update database user password
    username := request.Secret.Metadata["username"]
    if err := r.updateDatabasePassword(ctx, username, newPassword); err != nil {
        result.Status = rotation.StatusFailed
        result.Error = fmt.Sprintf("database update failed: %v", err)
        return result, err
    }
    
    result.AuditTrail = append(result.AuditTrail, rotation.AuditEntry{
        Timestamp: time.Now(),
        Action:    "database_password_updated",
        Component: "postgresql",
        Status:    "success",
        Details: map[string]interface{}{
            "username": username,
        },
    })
    
    // Step 3: Verify new password works
    verifyReq := rotation.VerificationRequest{
        Secret: request.Secret,
        NewSecretRef: rotation.SecretReference{
            Provider: request.Secret.Provider,
            Key:      request.Secret.Key,
        },
        Tests: []rotation.VerificationTest{
            {
                Name:     "connection_test",
                Type:     rotation.TestTypeConnection,
                Required: true,
                Timeout:  10 * time.Second,
            },
        },
    }
    
    if err := r.Verify(ctx, verifyReq); err != nil {
        // Attempt rollback
        rollbackErr := r.attemptRollback(ctx, request.Secret, request.Secret.ProviderRef)
        result.Status = rotation.StatusRolledBack
        result.Error = fmt.Sprintf("verification failed: %v (rollback: %v)", err, rollbackErr)
        return result, err
    }
    
    // Step 4: Update secret store with new password
    if err := r.updateSecretStore(ctx, request, newPassword); err != nil {
        result.Status = rotation.StatusFailed
        result.Error = fmt.Sprintf("secret store update failed: %v", err)
        return result, err
    }
    
    // Success!
    now := time.Now()
    result.Status = rotation.StatusCompleted
    result.RotatedAt = &now
    result.AuditTrail = append(result.AuditTrail, rotation.AuditEntry{
        Timestamp: now,
        Action:    "rotation_completed",
        Component: "postgresql",
        Status:    "success",
        Details: map[string]interface{}{
            "duration": time.Since(startTime).Seconds(),
        },
    })
    
    return result, nil
}
```

### 4. Implement Verification

```go
func (r *PostgresRotator) Verify(ctx context.Context, request rotation.VerificationRequest) error {
    for _, test := range request.Tests {
        switch test.Type {
        case rotation.TestTypeConnection:
            if err := r.verifyConnection(ctx, request.NewSecretRef, test); err != nil {
                return fmt.Errorf("connection test failed: %w", err)
            }
            
        case rotation.TestTypeQuery:
            if err := r.verifyQuery(ctx, request.NewSecretRef, test); err != nil {
                return fmt.Errorf("query test failed: %w", err)
            }
            
        default:
            return fmt.Errorf("unsupported test type: %s", test.Type)
        }
    }
    
    return nil
}

func (r *PostgresRotator) verifyConnection(ctx context.Context, ref rotation.SecretReference, test rotation.VerificationTest) error {
    // Get the new password from secret store
    newPassword, err := r.getSecretValue(ctx, ref)
    if err != nil {
        return err
    }
    
    // Create test connection
    testConnStr := fmt.Sprintf("postgres://%s:%s@%s/%s",
        ref.Metadata["username"],
        newPassword,
        ref.Metadata["host"],
        ref.Metadata["database"],
    )
    
    testDB, err := sql.Open("postgres", testConnStr)
    if err != nil {
        return err
    }
    defer testDB.Close()
    
    // Set timeout from test configuration
    ctx, cancel := context.WithTimeout(ctx, test.Timeout)
    defer cancel()
    
    // Verify connection
    if err := testDB.PingContext(ctx); err != nil {
        return fmt.Errorf("connection failed: %w", err)
    }
    
    return nil
}
```

### 5. Implement Rollback

```go
func (r *PostgresRotator) Rollback(ctx context.Context, request rotation.RollbackRequest) error {
    // Get the old password
    oldPassword, err := r.getSecretValue(ctx, request.OldSecretRef)
    if err != nil {
        return fmt.Errorf("cannot retrieve old password: %w", err)
    }
    
    // Restore the old password in database
    username := request.Secret.Metadata["username"]
    if err := r.updateDatabasePassword(ctx, username, oldPassword); err != nil {
        return fmt.Errorf("failed to restore old password: %w", err)
    }
    
    // Verify rollback succeeded
    if err := r.verifyConnection(ctx, request.OldSecretRef, rotation.VerificationTest{
        Name:    "rollback_verification",
        Type:    rotation.TestTypeConnection,
        Timeout: 10 * time.Second,
    }); err != nil {
        return fmt.Errorf("rollback verification failed: %w", err)
    }
    
    // Log rollback
    r.logRollback(request.Secret, request.Reason)
    
    return nil
}
```

## Zero-Downtime Rotation

### Implementing TwoSecretRotator

For services requiring zero downtime, implement the TwoSecretRotator interface:

```go
type PostgresTwoKeyRotator struct {
    PostgresRotator
}

func (r *PostgresTwoKeyRotator) CreateSecondarySecret(ctx context.Context, request rotation.SecondarySecretRequest) (*rotation.SecretReference, error) {
    // Generate new password
    newPassword, err := r.generatePassword(rotation.RotationRequest{
        Secret:   request.Secret,
        NewValue: request.NewValue,
        Config:   request.Config,
    })
    if err != nil {
        return nil, err
    }
    
    // Create new database user with same permissions
    primaryUser := request.Secret.Metadata["username"]
    secondaryUser := fmt.Sprintf("%s_rotating", primaryUser)
    
    // Create user with new password
    if err := r.createUserWithSamePermissions(ctx, primaryUser, secondaryUser, newPassword); err != nil {
        return nil, fmt.Errorf("failed to create secondary user: %w", err)
    }
    
    // Store secondary credentials
    secondaryRef := &rotation.SecretReference{
        Provider:   request.Secret.Provider,
        Key:        fmt.Sprintf("%s_secondary", request.Secret.Key),
        Identifier: secondaryUser,
        Metadata: map[string]string{
            "username": secondaryUser,
            "primary":  primaryUser,
        },
    }
    
    return secondaryRef, nil
}

func (r *PostgresTwoKeyRotator) PromoteSecondarySecret(ctx context.Context, request rotation.PromoteRequest) error {
    if request.VerifyFirst {
        // Verify secondary works before promotion
        if err := r.Verify(ctx, rotation.VerificationRequest{
            Secret:       request.Secret,
            NewSecretRef: request.SecondaryRef,
            Tests: []rotation.VerificationTest{
                {Name: "pre_promotion", Type: rotation.TestTypeConnection},
            },
        }); err != nil {
            return fmt.Errorf("pre-promotion verification failed: %w", err)
        }
    }
    
    // Update application configuration to use secondary
    // This is typically done through configuration management
    // or by updating connection strings in secret stores
    
    // Wait for grace period to ensure all connections switch
    time.Sleep(request.GracePeriod)
    
    return nil
}

func (r *PostgresTwoKeyRotator) DeprecatePrimarySecret(ctx context.Context, request rotation.DeprecateRequest) error {
    oldUsername := request.OldRef.Identifier
    
    if request.HardDelete {
        // Drop the old user
        if err := r.dropUser(ctx, oldUsername); err != nil {
            return fmt.Errorf("failed to drop old user: %w", err)
        }
    } else {
        // Disable the old user but keep for rollback
        if err := r.disableUser(ctx, oldUsername); err != nil {
            return fmt.Errorf("failed to disable old user: %w", err)
        }
    }
    
    return nil
}
```

## Schema-Aware Rotation

### Using dsops-data Definitions

Implement SchemaAwareRotator to use community service definitions:

```go
type GenericSchemaRotator struct {
    repository *dsopsdata.Repository
    protocols  map[string]protocol.Adapter
}

func (r *GenericSchemaRotator) SetRepository(repository *dsopsdata.Repository) {
    r.repository = repository
}

func (r *GenericSchemaRotator) Rotate(ctx context.Context, request rotation.RotationRequest) (*rotation.RotationResult, error) {
    // Get service definition from dsops-data
    serviceDef, err := r.repository.GetServiceType(request.Secret.Provider)
    if err != nil {
        return nil, fmt.Errorf("unknown service type: %w", err)
    }
    
    // Select appropriate protocol adapter
    protocolType := serviceDef.Protocol
    adapter, ok := r.protocols[protocolType]
    if !ok {
        return nil, fmt.Errorf("unsupported protocol: %s", protocolType)
    }
    
    // Use adapter to perform rotation based on service definition
    return adapter.Rotate(ctx, request, serviceDef)
}
```

## Testing Rotation Strategies

### Unit Tests

Write comprehensive tests for your rotator:

```go
func TestPostgresRotator_Rotate(t *testing.T) {
    // Setup mock database
    db, mock, err := sqlmock.New()
    require.NoError(t, err)
    defer db.Close()
    
    rotator := &PostgresRotator{
        db: db,
        secretProvider: &MockProvider{},
    }
    
    // Setup expectations
    mock.ExpectExec("ALTER USER").
        WithArgs("testuser", sqlmock.AnyArg()).
        WillReturnResult(sqlmock.NewResult(0, 1))
    
    mock.ExpectPing()
    
    // Test rotation
    request := rotation.RotationRequest{
        Secret: rotation.SecretInfo{
            Key:        "db-password",
            Provider:   "postgresql",
            SecretType: rotation.SecretTypePassword,
            Metadata: map[string]string{
                "database_type": "postgresql",
                "database_name": "testdb",
                "username":      "testuser",
            },
        },
    }
    
    result, err := rotator.Rotate(context.Background(), request)
    assert.NoError(t, err)
    assert.Equal(t, rotation.StatusCompleted, result.Status)
    assert.NotNil(t, result.RotatedAt)
    
    // Verify all expectations met
    assert.NoError(t, mock.ExpectationsWereMet())
}
```

### Integration Tests

Test with real services:

```go
func TestPostgresRotator_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Setup test database
    container := setupPostgresContainer(t)
    defer container.Terminate(context.Background())
    
    connStr := container.ConnectionString()
    rotator, err := NewPostgresRotator(connStr, realProvider)
    require.NoError(t, err)
    
    // Create test user
    setupTestUser(t, rotator.db)
    
    // Test actual rotation
    request := rotation.RotationRequest{
        Secret: rotation.SecretInfo{
            Key:        "test-db-password",
            Provider:   "postgresql",
            SecretType: rotation.SecretTypePassword,
            Metadata: map[string]string{
                "database_type": "postgresql",
                "database_name": "postgres",
                "username":      "testuser",
                "host":          container.Host(),
                "port":          container.Port(),
            },
        },
    }
    
    result, err := rotator.Rotate(context.Background(), request)
    assert.NoError(t, err)
    assert.Equal(t, rotation.StatusCompleted, result.Status)
    
    // Verify old password no longer works
    oldConn := fmt.Sprintf("postgres://testuser:oldpassword@%s:%s/postgres", 
        container.Host(), container.Port())
    _, err = sql.Open("postgres", oldConn)
    assert.Error(t, err)
}
```

## Security Best Practices

### 1. Secure Password Generation

```go
func (r *PostgresRotator) generatePassword(request rotation.RotationRequest) (string, error) {
    // Check constraints
    constraints := request.Secret.Constraints
    minLength := 16
    if constraints != nil && constraints.MinValueLength > 0 {
        minLength = constraints.MinValueLength
    }
    
    // Use crypto/rand for secure generation
    const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
    
    password := make([]byte, minLength)
    for i := range password {
        n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
        if err != nil {
            return "", err
        }
        password[i] = charset[n.Int64()]
    }
    
    return string(password), nil
}
```

### 2. Audit Logging

```go
func (r *PostgresRotator) auditLog(action string, details map[string]interface{}) {
    entry := rotation.AuditEntry{
        Timestamp: time.Now(),
        Action:    action,
        Component: "postgresql",
        Status:    "success",
        Details:   details,
    }
    
    // Never log actual secret values
    if password, ok := details["password"]; ok {
        details["password"] = logging.Secret(password)
    }
    
    r.logger.Info("rotation audit", 
        "action", entry.Action,
        "component", entry.Component,
        "details", entry.Details,
    )
}
```

### 3. Transaction Safety

```go
func (r *PostgresRotator) updateDatabasePassword(ctx context.Context, username, newPassword string) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // Use parameterized query to prevent SQL injection
    query := `ALTER USER $1 WITH PASSWORD $2`
    if _, err := tx.ExecContext(ctx, query, username, newPassword); err != nil {
        return fmt.Errorf("failed to update password: %w", err)
    }
    
    return tx.Commit()
}
```

## Advanced Topics

### Custom Verification Tests

Create sophisticated verification tests:

```go
func (r *PostgresRotator) verifyPermissions(ctx context.Context, ref rotation.SecretReference, test rotation.VerificationTest) error {
    // Connect with new credentials
    db, err := r.connectWithRef(ctx, ref)
    if err != nil {
        return err
    }
    defer db.Close()
    
    // Check required permissions
    requiredPerms := test.Config["permissions"].([]string)
    
    for _, perm := range requiredPerms {
        hasPermQuery := `
            SELECT has_table_privilege($1, $2, $3)
        `
        
        var hasPriv bool
        err := db.QueryRowContext(ctx, hasPermQuery, 
            ref.Metadata["username"],
            test.Config["table"],
            perm,
        ).Scan(&hasPriv)
        
        if err != nil || !hasPriv {
            return fmt.Errorf("missing permission %s on table %s", 
                perm, test.Config["table"])
        }
    }
    
    return nil
}
```

### Notification Integration

Send notifications during rotation:

```go
func (r *PostgresRotator) notifyRotation(result *rotation.RotationResult, recipients []string) error {
    for _, recipient := range recipients {
        switch {
        case strings.HasPrefix(recipient, "slack://"):
            if err := r.notifySlack(recipient, result); err != nil {
                return err
            }
            
        case strings.Contains(recipient, "@"):
            if err := r.notifyEmail(recipient, result); err != nil {
                return err
            }
        }
    }
    
    return nil
}
```

### Performance Optimization

Handle batch rotations efficiently:

```go
func (r *PostgresRotator) BatchRotate(ctx context.Context, requests []rotation.RotationRequest) ([]rotation.RotationResult, error) {
    results := make([]rotation.RotationResult, len(requests))
    
    // Use worker pool for parallel rotation
    workers := 5
    jobs := make(chan rotationJob, len(requests))
    resultsChan := make(chan rotationResult, len(requests))
    
    // Start workers
    var wg sync.WaitGroup
    for i := 0; i < workers; i++ {
        wg.Add(1)
        go r.rotationWorker(ctx, jobs, resultsChan, &wg)
    }
    
    // Queue jobs
    for i, req := range requests {
        jobs <- rotationJob{index: i, request: req}
    }
    close(jobs)
    
    // Collect results
    go func() {
        wg.Wait()
        close(resultsChan)
    }()
    
    for res := range resultsChan {
        results[res.index] = res.result
    }
    
    return results, nil
}
```

## Registration

Register your rotator with the rotation engine:

```go
func init() {
    engine := rotation.GetDefaultEngine()
    
    // Register PostgreSQL rotator
    postgresRotator := &PostgresRotator{}
    engine.RegisterStrategy(postgresRotator)
    
    // Register two-key variant
    twoKeyRotator := &PostgresTwoKeyRotator{
        PostgresRotator: *postgresRotator,
    }
    engine.RegisterStrategy(twoKeyRotator)
}
```

## Next Steps

1. Study existing rotators in `pkg/rotation/` for patterns
2. Review [dsops-data](https://github.com/systmms/dsops-data) service definitions
3. Read the [Security Guidelines](/developer/security/) for security requirements
4. Check the [Testing Guide](/developer/testing/) for testing strategies
5. Submit your rotator as a pull request

## Resources

- [Rotation Interface GoDoc](https://pkg.go.dev/github.com/systmms/dsops/pkg/rotation)
- [Example Rotators](https://github.com/systmms/dsops/tree/main/pkg/rotation/examples_test.go)
- [Protocol Adapters](https://pkg.go.dev/github.com/systmms/dsops/pkg/protocol)
- [dsops-data Repository](https://github.com/systmms/dsops-data)
- [Rotation Strategies Documentation](/rotation/strategies/)