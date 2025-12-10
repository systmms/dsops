package protocol

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExecuteCreateWithSQLMock tests the executeCreate method with a mocked database.
func TestExecuteCreateWithSQLMock(t *testing.T) {
	tests := []struct {
		name          string
		operation     Operation
		config        AdapterConfig
		setupMock     func(mock sqlmock.Sqlmock)
		expectSuccess bool
		expectError   bool
		errorContains string
	}{
		{
			name: "successful_create_user",
			operation: Operation{
				Target: "user",
				Action: "create",
				Parameters: map[string]interface{}{
					"username": "newuser",
					"password": "secret123",
				},
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"create_user": "CREATE USER {{.username}} WITH PASSWORD '{{.password}}'",
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("CREATE USER newuser WITH PASSWORD").
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectSuccess: true,
		},
		{
			name: "create_with_transaction_begin_failure",
			operation: Operation{
				Target: "user",
				Action: "create",
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"create_user": "CREATE USER test",
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(fmt.Errorf("connection lost"))
			},
			expectError:   true,
			errorContains: "begin transaction",
		},
		{
			name: "create_with_exec_failure",
			operation: Operation{
				Target: "user",
				Action: "create",
				Parameters: map[string]interface{}{
					"username": "newuser",
				},
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"create_user": "CREATE USER {{.username}}",
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("CREATE USER newuser").
					WillReturnError(fmt.Errorf("user already exists"))
				mock.ExpectRollback()
			},
			expectSuccess: false,
			expectError:   true,
		},
		{
			name: "create_with_commit_failure",
			operation: Operation{
				Target: "user",
				Action: "create",
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"create_user": "CREATE USER testuser",
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("CREATE USER testuser").
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit().WillReturnError(fmt.Errorf("commit failed"))
			},
			expectSuccess: false,
			expectError:   true,
		},
		{
			name: "create_missing_template",
			operation: Operation{
				Target: "user",
				Action: "create",
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{},
				},
			},
			setupMock:     func(mock sqlmock.Sqlmock) {},
			expectError:   true,
			errorContains: "command template not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			// Setup mock expectations
			tt.setupMock(mock)

			// Create adapter
			adapter := NewSQLAdapter()

			// Execute
			ctx := context.Background()
			result, err := adapter.executeCreate(ctx, db, tt.operation, tt.config)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expectSuccess, result.Success)
			}

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestExecuteVerifyWithSQLMock tests the executeVerify method with a mocked database.
func TestExecuteVerifyWithSQLMock(t *testing.T) {
	tests := []struct {
		name          string
		operation     Operation
		config        AdapterConfig
		setupMock     func(mock sqlmock.Sqlmock)
		expectSuccess bool
		expectError   bool
	}{
		{
			name: "successful_verify",
			operation: Operation{
				Target: "connection",
				Action: "verify",
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"verify": "SELECT 1",
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"result"}).AddRow(1)
				mock.ExpectQuery("SELECT 1").WillReturnRows(rows)
			},
			expectSuccess: true,
		},
		{
			name: "verify_with_default_query",
			operation: Operation{
				Target: "connection",
				Action: "verify",
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"result"}).AddRow(1)
				mock.ExpectQuery("SELECT 1").WillReturnRows(rows)
			},
			expectSuccess: true,
		},
		{
			name: "verify_query_failure",
			operation: Operation{
				Target: "connection",
				Action: "verify",
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"verify": "SELECT 1",
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT 1").WillReturnError(sql.ErrNoRows)
			},
			expectSuccess: false,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			tt.setupMock(mock)

			adapter := NewSQLAdapter()
			ctx := context.Background()
			result, err := adapter.executeVerify(ctx, db, tt.operation, tt.config)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expectSuccess, result.Success)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestExecuteRotateWithSQLMock tests the executeRotate method with a mocked database.
func TestExecuteRotateWithSQLMock(t *testing.T) {
	tests := []struct {
		name          string
		operation     Operation
		config        AdapterConfig
		setupMock     func(mock sqlmock.Sqlmock)
		expectSuccess bool
		expectError   bool
	}{
		{
			name: "successful_rotate",
			operation: Operation{
				Target: "credential",
				Action: "rotate",
				Parameters: map[string]interface{}{
					"username":     "dbuser",
					"new_password": "NewPass123!",
				},
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"rotate_credential": "ALTER USER {{.username}} SET PASSWORD '{{.new_password}}'",
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("ALTER USER dbuser SET PASSWORD").
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectSuccess: true,
		},
		{
			name: "rotate_with_rollback",
			operation: Operation{
				Target: "credential",
				Action: "rotate",
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"rotate_credential": "ALTER USER test SET PASSWORD 'new'",
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("ALTER USER test").
					WillReturnError(fmt.Errorf("user not found"))
				mock.ExpectRollback()
			},
			expectSuccess: false,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			tt.setupMock(mock)

			adapter := NewSQLAdapter()
			ctx := context.Background()
			result, err := adapter.executeRotate(ctx, db, tt.operation, tt.config)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expectSuccess, result.Success)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestExecuteRevokeWithSQLMock tests the executeRevoke method with a mocked database.
func TestExecuteRevokeWithSQLMock(t *testing.T) {
	tests := []struct {
		name          string
		operation     Operation
		config        AdapterConfig
		setupMock     func(mock sqlmock.Sqlmock)
		expectSuccess bool
		expectError   bool
	}{
		{
			name: "successful_revoke",
			operation: Operation{
				Target: "user",
				Action: "revoke",
				Parameters: map[string]interface{}{
					"username": "olduser",
				},
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"revoke_user": "DROP USER {{.username}}",
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("DROP USER olduser").
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectSuccess: true,
		},
		{
			name: "revoke_nonexistent_user",
			operation: Operation{
				Target: "user",
				Action: "revoke",
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"revoke_user": "DROP USER ghost",
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("DROP USER ghost").
					WillReturnError(fmt.Errorf("user does not exist"))
				mock.ExpectRollback()
			},
			expectSuccess: false,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			tt.setupMock(mock)

			adapter := NewSQLAdapter()
			ctx := context.Background()
			result, err := adapter.executeRevoke(ctx, db, tt.operation, tt.config)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expectSuccess, result.Success)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestExecuteListWithSQLMock tests the executeList method with a mocked database.
func TestExecuteListWithSQLMock(t *testing.T) {
	tests := []struct {
		name          string
		operation     Operation
		config        AdapterConfig
		setupMock     func(mock sqlmock.Sqlmock)
		expectSuccess bool
		expectCount   int
		expectError   bool
	}{
		{
			name: "list_users_multiple",
			operation: Operation{
				Target: "users",
				Action: "list",
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"list_users": "SELECT username, created_at FROM users",
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"username", "created_at"}).
					AddRow("user1", time.Now()).
					AddRow("user2", time.Now()).
					AddRow("user3", time.Now())
				mock.ExpectQuery("SELECT username, created_at FROM users").WillReturnRows(rows)
			},
			expectSuccess: true,
			expectCount:   3,
		},
		{
			name: "list_empty_results",
			operation: Operation{
				Target: "users",
				Action: "list",
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"list_users": "SELECT username FROM users WHERE 1=0",
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"username"})
				mock.ExpectQuery("SELECT username FROM users").WillReturnRows(rows)
			},
			expectSuccess: true,
			expectCount:   0,
		},
		{
			name: "list_query_failure",
			operation: Operation{
				Target: "users",
				Action: "list",
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"list_users": "SELECT * FROM nonexistent",
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT").
					WillReturnError(fmt.Errorf("table does not exist"))
			},
			expectError: true,
		},
		{
			name: "list_with_multiple_columns",
			operation: Operation{
				Target: "credentials",
				Action: "list",
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"list_credentials": "SELECT id, name, type, enabled FROM credentials",
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "type", "enabled"}).
					AddRow(1, "cred1", "api_key", true).
					AddRow(2, "cred2", "password", false)
				mock.ExpectQuery("SELECT id, name, type, enabled FROM credentials").WillReturnRows(rows)
			},
			expectSuccess: true,
			expectCount:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			tt.setupMock(mock)

			adapter := NewSQLAdapter()
			ctx := context.Background()
			result, err := adapter.executeList(ctx, db, tt.operation, tt.config)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expectSuccess, result.Success)
				if data, ok := result.Data["count"].(int); ok {
					assert.Equal(t, tt.expectCount, data)
				}
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestSQLAdapterValidate tests configuration validation.
func TestSQLAdapterValidate(t *testing.T) {
	adapter := NewSQLAdapter()

	tests := []struct {
		name          string
		config        AdapterConfig
		expectError   bool
		errorContains string
	}{
		{
			name: "valid_config",
			config: AdapterConfig{
				Connection: map[string]string{
					"type":     "postgresql",
					"host":     "localhost",
					"port":     "5432",
					"database": "testdb",
				},
				Auth: map[string]string{
					"username": "user",
				},
			},
			expectError: false,
		},
		{
			name:          "nil_connection",
			config:        AdapterConfig{},
			expectError:   true,
			errorContains: "connection configuration is required",
		},
		{
			name: "missing_type",
			config: AdapterConfig{
				Connection: map[string]string{
					"host":     "localhost",
					"port":     "5432",
					"database": "testdb",
				},
				Auth: map[string]string{
					"username": "user",
				},
			},
			expectError:   true,
			errorContains: "type",
		},
		{
			name: "missing_host",
			config: AdapterConfig{
				Connection: map[string]string{
					"type":     "postgresql",
					"port":     "5432",
					"database": "testdb",
				},
				Auth: map[string]string{
					"username": "user",
				},
			},
			expectError:   true,
			errorContains: "host",
		},
		{
			name: "unsupported_db_type",
			config: AdapterConfig{
				Connection: map[string]string{
					"type":     "oracle",
					"host":     "localhost",
					"port":     "1521",
					"database": "orcl",
				},
				Auth: map[string]string{
					"username": "user",
				},
			},
			expectError:   true,
			errorContains: "unsupported database type",
		},
		{
			name: "missing_username",
			config: AdapterConfig{
				Connection: map[string]string{
					"type":     "postgresql",
					"host":     "localhost",
					"port":     "5432",
					"database": "testdb",
				},
				Auth: map[string]string{},
			},
			expectError:   true,
			errorContains: "username is required",
		},
		{
			name: "nil_auth",
			config: AdapterConfig{
				Connection: map[string]string{
					"type":     "postgresql",
					"host":     "localhost",
					"port":     "5432",
					"database": "testdb",
				},
			},
			expectError:   true,
			errorContains: "username is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.Validate(tt.config)
			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestSQLAdapterCapabilities tests capability reporting.
func TestSQLAdapterCapabilities(t *testing.T) {
	adapter := NewSQLAdapter()
	caps := adapter.Capabilities()

	// Check supported actions
	expectedActions := []string{"create", "verify", "rotate", "revoke", "list"}
	assert.Equal(t, expectedActions, caps.SupportedActions)

	// Check required config
	expectedRequired := []string{"type", "host", "port", "database", "username"}
	assert.Equal(t, expectedRequired, caps.RequiredConfig)

	// Check features
	assert.True(t, caps.Features["transactions"])
	assert.True(t, caps.Features["ssl"])
	assert.True(t, caps.Features["connection_pooling"])
}

// TestSQLAdapterNameAndType tests basic adapter properties.
func TestSQLAdapterNameAndType(t *testing.T) {
	adapter := NewSQLAdapter()

	assert.Equal(t, "SQL Protocol Adapter", adapter.Name())
	assert.Equal(t, AdapterTypeSQL, adapter.Type())
}
