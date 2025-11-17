package protocol

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FakeNoSQLConnection implements NoSQLConnection for testing
type FakeNoSQLConnection struct {
	responses map[string]interface{}
	errors    map[string]error
	closed    bool
}

func NewFakeNoSQLConnection() *FakeNoSQLConnection {
	return &FakeNoSQLConnection{
		responses: make(map[string]interface{}),
		errors:    make(map[string]error),
	}
}

func (c *FakeNoSQLConnection) SetResponse(command string, response interface{}) {
	c.responses[command] = response
}

func (c *FakeNoSQLConnection) SetError(command string, err error) {
	c.errors[command] = err
}

func (c *FakeNoSQLConnection) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if err, ok := c.errors[command]; ok {
		return nil, err
	}
	if resp, ok := c.responses[command]; ok {
		return resp, nil
	}
	// Default response
	return map[string]interface{}{"status": "ok", "command": command}, nil
}

func (c *FakeNoSQLConnection) Close() error {
	c.closed = true
	return nil
}

// FakeNoSQLHandler implements NoSQLHandler for testing
type FakeNoSQLHandler struct {
	conn      NoSQLConnection
	connErr   error
	validErr  error
	connected bool
}

func (h *FakeNoSQLHandler) Connect(ctx context.Context, config AdapterConfig) (NoSQLConnection, error) {
	if h.connErr != nil {
		return nil, h.connErr
	}
	h.connected = true
	return h.conn, nil
}

func (h *FakeNoSQLHandler) ValidateConfig(config AdapterConfig) error {
	return h.validErr
}

// TestNoSQLAdapterBasics tests basic adapter properties
func TestNoSQLAdapterBasics(t *testing.T) {
	adapter := NewNoSQLAdapter()

	assert.Equal(t, "NoSQL Protocol Adapter", adapter.Name())
	assert.Equal(t, AdapterTypeNoSQL, adapter.Type())
}

// TestNoSQLAdapterCapabilities tests capability reporting
func TestNoSQLAdapterCapabilities(t *testing.T) {
	adapter := NewNoSQLAdapter()
	caps := adapter.Capabilities()

	expectedActions := []string{"create", "verify", "rotate", "revoke", "list"}
	assert.Equal(t, expectedActions, caps.SupportedActions)

	expectedRequired := []string{"type", "host", "port"}
	assert.Equal(t, expectedRequired, caps.RequiredConfig)

	assert.True(t, caps.Features["document_store"])
	assert.True(t, caps.Features["key_value"])
	assert.True(t, caps.Features["json_support"])
}

// TestNoSQLAdapterValidate tests configuration validation
func TestNoSQLAdapterValidate(t *testing.T) {
	adapter := NewNoSQLAdapter()

	tests := []struct {
		name          string
		config        AdapterConfig
		expectError   bool
		errorContains string
	}{
		{
			name: "valid_mongodb_config",
			config: AdapterConfig{
				Connection: map[string]string{
					"type": "mongodb",
					"host": "localhost",
					"port": "27017",
				},
			},
			expectError: false,
		},
		{
			name: "valid_redis_config",
			config: AdapterConfig{
				Connection: map[string]string{
					"type": "redis",
					"host": "localhost",
					"port": "6379",
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
					"host": "localhost",
					"port": "27017",
				},
			},
			expectError:   true,
			errorContains: "database type is required",
		},
		{
			name: "empty_type",
			config: AdapterConfig{
				Connection: map[string]string{
					"type": "",
					"host": "localhost",
					"port": "27017",
				},
			},
			expectError:   true,
			errorContains: "database type is required",
		},
		{
			name: "unsupported_type",
			config: AdapterConfig{
				Connection: map[string]string{
					"type": "cassandra",
					"host": "localhost",
					"port": "9042",
				},
			},
			expectError:   true,
			errorContains: "unsupported NoSQL database type",
		},
		{
			name: "mongodb_missing_host",
			config: AdapterConfig{
				Connection: map[string]string{
					"type": "mongodb",
					"port": "27017",
				},
			},
			expectError:   true,
			errorContains: "host",
		},
		{
			name: "redis_missing_port",
			config: AdapterConfig{
				Connection: map[string]string{
					"type": "redis",
					"host": "localhost",
				},
			},
			expectError:   true,
			errorContains: "port",
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

// TestNoSQLRenderCommand tests command template rendering
func TestNoSQLRenderCommand(t *testing.T) {
	adapter := NewNoSQLAdapter()

	tests := []struct {
		name           string
		template       string
		operation      Operation
		expectedCmd    string
		expectedParams map[string]interface{}
		expectError    bool
	}{
		{
			name:     "plain_text_template",
			template: "createUser {{.username}}",
			operation: Operation{
				Target: "user",
				Action: "create",
				Parameters: map[string]interface{}{
					"username": "newuser",
					"password": "secret",
				},
			},
			expectedCmd: "createUser newuser",
			expectedParams: map[string]interface{}{
				"username": "newuser",
				"password": "secret",
			},
		},
		{
			name:     "json_template_with_command",
			template: `{"command": "createUser", "user": "{{.username}}"}`,
			operation: Operation{
				Target: "user",
				Action: "create",
				Parameters: map[string]interface{}{
					"username": "admin",
				},
			},
			expectedCmd: "createUser",
			expectedParams: map[string]interface{}{
				"user": "admin",
			},
		},
		{
			name:     "json_template_without_command",
			template: `{"ping": 1}`,
			operation: Operation{
				Target: "connection",
				Action: "verify",
			},
			expectedCmd:    `{"ping": 1}`,
			expectedParams: nil,
		},
		{
			name:     "template_with_target",
			template: "db.{{.Target}}.find()",
			operation: Operation{
				Target: "users",
				Action: "list",
			},
			expectedCmd:    "db.users.find()",
			expectedParams: map[string]interface{}{},
		},
		{
			name:     "template_with_action",
			template: "db.{{.Action}}User()",
			operation: Operation{
				Target: "user",
				Action: "rotate",
			},
			expectedCmd:    "db.rotateUser()",
			expectedParams: map[string]interface{}{},
		},
		{
			name:     "complex_json_template",
			template: `{"command": "updateUser", "user": "{{.username}}", "password": "{{.newPassword}}", "roles": ["{{.role}}"]}`,
			operation: Operation{
				Target: "user",
				Action: "rotate",
				Parameters: map[string]interface{}{
					"username":    "dbuser",
					"newPassword": "NewPass123",
					"role":        "readWrite",
				},
			},
			expectedCmd: "updateUser",
			expectedParams: map[string]interface{}{
				"user":     "dbuser",
				"password": "NewPass123",
				"roles":    []interface{}{"readWrite"},
			},
		},
		{
			name:     "invalid_template_syntax",
			template: "{{.Invalid syntax",
			operation: Operation{
				Target: "test",
			},
			expectError: true,
		},
		{
			name:     "invalid_json_after_render",
			template: `{"invalid json: {{.Target}}`,
			operation: Operation{
				Target: "test",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, params, err := adapter.renderCommand(tt.template, tt.operation)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedCmd, cmd)
				if tt.expectedParams != nil {
					assert.Equal(t, tt.expectedParams, params)
				}
			}
		})
	}
}

// TestNoSQLGetCommandTemplate tests command template retrieval
func TestNoSQLGetCommandTemplate(t *testing.T) {
	adapter := NewNoSQLAdapter()

	tests := []struct {
		name          string
		action        string
		operation     Operation
		config        AdapterConfig
		expected      string
		expectError   bool
		errorContains string
	}{
		{
			name:   "action_with_target",
			action: "create",
			operation: Operation{
				Target: "user",
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"create_user": `{"createUser": "{{.username}}"}`,
					},
				},
			},
			expected: `{"createUser": "{{.username}}"}`,
		},
		{
			name:   "fallback_to_action",
			action: "verify",
			operation: Operation{
				Target: "connection",
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"verify": `{"ping": 1}`,
					},
				},
			},
			expected: `{"ping": 1}`,
		},
		{
			name:   "missing_commands_section",
			action: "list",
			operation: Operation{
				Target: "users",
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{},
			},
			expectError:   true,
			errorContains: "commands configuration not found",
		},
		{
			name:   "command_not_found",
			action: "delete",
			operation: Operation{
				Target: "user",
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"create_user": "createUser",
					},
				},
			},
			expectError:   true,
			errorContains: "command template not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := adapter.getCommandTemplate(tt.action, tt.operation, tt.config)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestNoSQLBuildTemplateData tests template data construction
func TestNoSQLBuildTemplateData(t *testing.T) {
	adapter := NewNoSQLAdapter()

	tests := []struct {
		name      string
		operation Operation
		checkKeys []string
		checkVals map[string]interface{}
	}{
		{
			name: "basic_operation",
			operation: Operation{
				Target: "users",
				Action: "list",
			},
			checkKeys: []string{"Target", "Action"},
			checkVals: map[string]interface{}{
				"Target": "users",
				"Action": "list",
			},
		},
		{
			name: "with_parameters",
			operation: Operation{
				Target: "user",
				Action: "create",
				Parameters: map[string]interface{}{
					"username": "newuser",
					"password": "secret",
				},
			},
			checkKeys: []string{"Target", "Action", "username", "password"},
			checkVals: map[string]interface{}{
				"username": "newuser",
				"password": "secret",
			},
		},
		{
			name: "with_metadata",
			operation: Operation{
				Target: "audit",
				Action: "verify",
				Metadata: map[string]string{
					"source": "rotation",
				},
			},
			checkKeys: []string{"Target", "Action", "Metadata"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := adapter.buildTemplateData(tt.operation)

			for _, key := range tt.checkKeys {
				assert.Contains(t, data, key)
			}
			for k, v := range tt.checkVals {
				assert.Equal(t, v, data[k])
			}
		})
	}
}

// TestMongoHandlerValidation tests MongoDB handler validation
func TestMongoHandlerValidation(t *testing.T) {
	handler := &MongoHandler{}

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
					"host": "localhost",
					"port": "27017",
				},
			},
			expectError: false,
		},
		{
			name: "missing_host",
			config: AdapterConfig{
				Connection: map[string]string{
					"port": "27017",
				},
			},
			expectError:   true,
			errorContains: "host",
		},
		{
			name: "missing_port",
			config: AdapterConfig{
				Connection: map[string]string{
					"host": "localhost",
				},
			},
			expectError:   true,
			errorContains: "port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.ValidateConfig(tt.config)
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

// TestRedisHandlerValidation tests Redis handler validation
func TestRedisHandlerValidation(t *testing.T) {
	handler := &RedisHandler{}

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
					"host": "localhost",
					"port": "6379",
				},
			},
			expectError: false,
		},
		{
			name: "missing_host",
			config: AdapterConfig{
				Connection: map[string]string{
					"port": "6379",
				},
			},
			expectError:   true,
			errorContains: "host",
		},
		{
			name: "missing_port",
			config: AdapterConfig{
				Connection: map[string]string{
					"host": "localhost",
				},
			},
			expectError:   true,
			errorContains: "port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.ValidateConfig(tt.config)
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

// TestMockNoSQLConnection tests the mock connection behavior
func TestMockNoSQLConnection(t *testing.T) {
	conn := &MockNoSQLConnection{dbType: "mongodb"}

	t.Run("execute_returns_result", func(t *testing.T) {
		ctx := context.Background()
		result, err := conn.Execute(ctx, "testCmd", nil)
		require.NoError(t, err)
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "success", resultMap["status"])
		assert.Equal(t, "testCmd", resultMap["command"])
		assert.Equal(t, "mongodb", resultMap["type"])
	})

	t.Run("execute_respects_context_cancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		// Mock connection has 100ms delay, should timeout
		_, err := conn.Execute(ctx, "slowCmd", nil)
		assert.Error(t, err)
	})

	t.Run("close_connection", func(t *testing.T) {
		err := conn.Close()
		assert.NoError(t, err)
	})
}

// TestNoSQLExecuteOperationsWithFake tests execute operations with fake connection
func TestNoSQLExecuteOperationsWithFake(t *testing.T) {
	adapter := NewNoSQLAdapter()

	t.Run("executeCreate_success", func(t *testing.T) {
		conn := NewFakeNoSQLConnection()
		conn.SetResponse("createUser newuser", map[string]interface{}{"created": true})

		operation := Operation{
			Target: "user",
			Action: "create",
			Parameters: map[string]interface{}{
				"username": "newuser",
			},
		}
		config := AdapterConfig{
			ServiceConfig: map[string]interface{}{
				"commands": map[string]interface{}{
					"create_user": "createUser {{.username}}",
				},
			},
			Connection: map[string]string{
				"type": "mongodb",
			},
		}

		result, err := adapter.executeCreate(context.Background(), conn, operation, config)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, "create", result.Data["action"])
	})

	t.Run("executeVerify_success", func(t *testing.T) {
		conn := NewFakeNoSQLConnection()

		operation := Operation{
			Target: "connection",
			Action: "verify",
		}
		config := AdapterConfig{
			ServiceConfig: map[string]interface{}{
				"commands": map[string]interface{}{
					"verify": `{"ping": 1}`,
				},
			},
		}

		result, err := adapter.executeVerify(context.Background(), conn, operation, config)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.True(t, result.Data["verified"].(bool))
	})

	t.Run("executeRotate_success", func(t *testing.T) {
		conn := NewFakeNoSQLConnection()

		operation := Operation{
			Target: "credential",
			Action: "rotate",
			Parameters: map[string]interface{}{
				"newPassword": "NewPass123",
			},
		}
		config := AdapterConfig{
			ServiceConfig: map[string]interface{}{
				"commands": map[string]interface{}{
					"rotate_credential": "updatePassword {{.newPassword}}",
				},
			},
		}

		result, err := adapter.executeRotate(context.Background(), conn, operation, config)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, "rotate", result.Data["action"])
	})

	t.Run("executeRevoke_success", func(t *testing.T) {
		conn := NewFakeNoSQLConnection()

		operation := Operation{
			Target: "user",
			Action: "revoke",
		}
		config := AdapterConfig{
			ServiceConfig: map[string]interface{}{
				"commands": map[string]interface{}{
					"revoke_user": "dropUser",
				},
			},
		}

		result, err := adapter.executeRevoke(context.Background(), conn, operation, config)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, "revoke", result.Data["action"])
	})

	t.Run("executeList_returns_items", func(t *testing.T) {
		conn := NewFakeNoSQLConnection()
		conn.SetResponse("listUsers", []interface{}{
			map[string]interface{}{"name": "user1"},
			map[string]interface{}{"name": "user2"},
		})

		operation := Operation{
			Target: "users",
			Action: "list",
		}
		config := AdapterConfig{
			ServiceConfig: map[string]interface{}{
				"commands": map[string]interface{}{
					"list_users": "listUsers",
				},
			},
		}

		result, err := adapter.executeList(context.Background(), conn, operation, config)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, 2, result.Data["count"])
	})

	t.Run("executeCreate_failure", func(t *testing.T) {
		conn := NewFakeNoSQLConnection()
		conn.SetError("createUser existing", fmt.Errorf("user already exists"))

		operation := Operation{
			Target: "user",
			Action: "create",
			Parameters: map[string]interface{}{
				"username": "existing",
			},
		}
		config := AdapterConfig{
			ServiceConfig: map[string]interface{}{
				"commands": map[string]interface{}{
					"create_user": "createUser {{.username}}",
				},
			},
		}

		result, err := adapter.executeCreate(context.Background(), conn, operation, config)
		require.Error(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "user already exists")
	})
}

// TestNoSQLExecuteListResultTypes tests list result type conversions
func TestNoSQLExecuteListResultTypes(t *testing.T) {
	adapter := NewNoSQLAdapter()

	tests := []struct {
		name          string
		response      interface{}
		expectedCount int
	}{
		{
			name: "slice_of_interfaces",
			response: []interface{}{
				map[string]interface{}{"id": 1},
				map[string]interface{}{"id": 2},
			},
			expectedCount: 2,
		},
		{
			name: "slice_of_maps",
			response: []map[string]interface{}{
				{"id": 1, "name": "user1"},
				{"id": 2, "name": "user2"},
				{"id": 3, "name": "user3"},
			},
			expectedCount: 3,
		},
		{
			name:          "single_result",
			response:      map[string]interface{}{"total": 100},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := NewFakeNoSQLConnection()
			conn.SetResponse("listCmd", tt.response)

			operation := Operation{
				Target: "items",
				Action: "list",
			}
			config := AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"list_items": "listCmd",
					},
				},
			}

			result, err := adapter.executeList(context.Background(), conn, operation, config)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, result.Data["count"])
		})
	}
}
