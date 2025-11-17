package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildPostgreSQLConnString tests PostgreSQL connection string building.
func TestBuildPostgreSQLConnString(t *testing.T) {
	adapter := NewSQLAdapter()

	tests := []struct {
		name        string
		config      AdapterConfig
		expected    string
		shouldMatch []string
	}{
		{
			name: "basic_config",
			config: AdapterConfig{
				Connection: map[string]string{
					"type":     "postgresql",
					"host":     "localhost",
					"port":     "5432",
					"database": "mydb",
				},
				Auth: map[string]string{
					"username": "user",
				},
			},
			shouldMatch: []string{
				"host=localhost",
				"port=5432",
				"dbname=mydb",
				"user=user",
				"sslmode=require",
			},
		},
		{
			name: "with_password",
			config: AdapterConfig{
				Connection: map[string]string{
					"host":     "db.example.com",
					"port":     "5432",
					"database": "production",
				},
				Auth: map[string]string{
					"username": "admin",
					"password": "secret123",
				},
			},
			shouldMatch: []string{
				"host=db.example.com",
				"port=5432",
				"dbname=production",
				"user=admin",
				"password=secret123",
				"sslmode=require",
			},
		},
		{
			name: "custom_sslmode",
			config: AdapterConfig{
				Connection: map[string]string{
					"host":     "localhost",
					"port":     "5432",
					"database": "test",
					"sslmode":  "disable",
				},
				Auth: map[string]string{
					"username": "test",
				},
			},
			shouldMatch: []string{
				"host=localhost",
				"sslmode=disable",
			},
		},
		{
			name: "special_chars_in_password",
			config: AdapterConfig{
				Connection: map[string]string{
					"host":     "localhost",
					"port":     "5432",
					"database": "db",
				},
				Auth: map[string]string{
					"username": "user",
					"password": "p@ss=word!#$",
				},
			},
			shouldMatch: []string{
				"password=p@ss=word!#$",
			},
		},
		{
			name: "empty_password",
			config: AdapterConfig{
				Connection: map[string]string{
					"host":     "localhost",
					"port":     "5432",
					"database": "db",
				},
				Auth: map[string]string{
					"username": "user",
					"password": "",
				},
			},
			shouldMatch: []string{
				"user=user",
				"sslmode=require",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.buildPostgreSQLConnString(tt.config)
			for _, match := range tt.shouldMatch {
				assert.Contains(t, result, match)
			}
		})
	}
}

// TestBuildMySQLConnString tests MySQL connection string building.
func TestBuildMySQLConnString(t *testing.T) {
	adapter := NewSQLAdapter()

	tests := []struct {
		name        string
		config      AdapterConfig
		expected    string
		shouldMatch []string
	}{
		{
			name: "basic_config",
			config: AdapterConfig{
				Connection: map[string]string{
					"host":     "localhost",
					"port":     "3306",
					"database": "mydb",
				},
				Auth: map[string]string{
					"username": "user",
					"password": "pass",
				},
			},
			expected: "user:pass@tcp(localhost:3306)/mydb?parseTime=true",
		},
		{
			name: "empty_password",
			config: AdapterConfig{
				Connection: map[string]string{
					"host":     "localhost",
					"port":     "3306",
					"database": "test",
				},
				Auth: map[string]string{
					"username": "root",
				},
			},
			expected: "root:@tcp(localhost:3306)/test?parseTime=true",
		},
		{
			name: "complex_password",
			config: AdapterConfig{
				Connection: map[string]string{
					"host":     "db.mysql.com",
					"port":     "3307",
					"database": "production",
				},
				Auth: map[string]string{
					"username": "admin",
					"password": "P@ss:word/123",
				},
			},
			expected: "admin:P@ss:word/123@tcp(db.mysql.com:3307)/production?parseTime=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.buildMySQLConnString(tt.config)
			if tt.expected != "" {
				assert.Equal(t, tt.expected, result)
			}
			for _, match := range tt.shouldMatch {
				assert.Contains(t, result, match)
			}
		})
	}
}

// TestBuildSQLServerConnString tests SQL Server connection string building.
func TestBuildSQLServerConnString(t *testing.T) {
	adapter := NewSQLAdapter()

	tests := []struct {
		name     string
		config   AdapterConfig
		expected string
	}{
		{
			name: "basic_config",
			config: AdapterConfig{
				Connection: map[string]string{
					"host":     "localhost",
					"port":     "1433",
					"database": "mydb",
				},
				Auth: map[string]string{
					"username": "sa",
					"password": "password123",
				},
			},
			expected: "server=localhost,1433;user id=sa;password=password123;database=mydb",
		},
		{
			name: "empty_password",
			config: AdapterConfig{
				Connection: map[string]string{
					"host":     "sqlserver.local",
					"port":     "1433",
					"database": "test",
				},
				Auth: map[string]string{
					"username": "user",
				},
			},
			expected: "server=sqlserver.local,1433;user id=user;password=;database=test",
		},
		{
			name: "complex_database_name",
			config: AdapterConfig{
				Connection: map[string]string{
					"host":     "db.example.com",
					"port":     "1434",
					"database": "My-Database_v2",
				},
				Auth: map[string]string{
					"username": "admin",
					"password": "SecurePass!@#",
				},
			},
			expected: "server=db.example.com,1434;user id=admin;password=SecurePass!@#;database=My-Database_v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.buildSQLServerConnString(tt.config)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestBuildConnectionString tests the connection string router.
func TestBuildConnectionString(t *testing.T) {
	adapter := NewSQLAdapter()

	tests := []struct {
		name          string
		config        AdapterConfig
		expectError   bool
		errorContains string
		shouldContain string
	}{
		{
			name: "postgresql",
			config: AdapterConfig{
				Connection: map[string]string{
					"type":     "postgresql",
					"host":     "localhost",
					"port":     "5432",
					"database": "db",
				},
				Auth: map[string]string{
					"username": "user",
				},
			},
			shouldContain: "host=localhost",
		},
		{
			name: "postgres_alias",
			config: AdapterConfig{
				Connection: map[string]string{
					"type":     "postgres",
					"host":     "localhost",
					"port":     "5432",
					"database": "db",
				},
				Auth: map[string]string{
					"username": "user",
				},
			},
			shouldContain: "host=localhost",
		},
		{
			name: "mysql",
			config: AdapterConfig{
				Connection: map[string]string{
					"type":     "mysql",
					"host":     "localhost",
					"port":     "3306",
					"database": "db",
				},
				Auth: map[string]string{
					"username": "user",
					"password": "pass",
				},
			},
			shouldContain: "@tcp(",
		},
		{
			name: "mariadb_alias",
			config: AdapterConfig{
				Connection: map[string]string{
					"type":     "mariadb",
					"host":     "localhost",
					"port":     "3306",
					"database": "db",
				},
				Auth: map[string]string{
					"username": "user",
					"password": "pass",
				},
			},
			shouldContain: "@tcp(",
		},
		{
			name: "sqlserver",
			config: AdapterConfig{
				Connection: map[string]string{
					"type":     "sqlserver",
					"host":     "localhost",
					"port":     "1433",
					"database": "db",
				},
				Auth: map[string]string{
					"username": "sa",
					"password": "pass",
				},
			},
			shouldContain: "server=",
		},
		{
			name: "mssql_alias",
			config: AdapterConfig{
				Connection: map[string]string{
					"type":     "mssql",
					"host":     "localhost",
					"port":     "1433",
					"database": "db",
				},
				Auth: map[string]string{
					"username": "sa",
					"password": "pass",
				},
			},
			shouldContain: "server=",
		},
		{
			name: "unsupported_type",
			config: AdapterConfig{
				Connection: map[string]string{
					"type":     "oracle",
					"host":     "localhost",
					"port":     "1521",
					"database": "orcl",
				},
				Auth: map[string]string{
					"username": "user",
					"password": "pass",
				},
			},
			expectError:   true,
			errorContains: "unsupported database type",
		},
		{
			name: "case_insensitive_type",
			config: AdapterConfig{
				Connection: map[string]string{
					"type":     "POSTGRESQL",
					"host":     "localhost",
					"port":     "5432",
					"database": "db",
				},
				Auth: map[string]string{
					"username": "user",
				},
			},
			shouldContain: "host=localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := adapter.buildConnectionString(tt.config)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				if tt.shouldContain != "" {
					assert.Contains(t, result, tt.shouldContain)
				}
			}
		})
	}
}

// TestRenderSQLTemplate tests SQL template rendering.
func TestRenderSQLTemplate(t *testing.T) {
	adapter := NewSQLAdapter()

	tests := []struct {
		name        string
		template    string
		operation   Operation
		expected    string
		expectError bool
	}{
		{
			name:     "simple_template",
			template: "SELECT * FROM {{.Target}}",
			operation: Operation{
				Target: "users",
				Action: "list",
			},
			expected: "SELECT * FROM users",
		},
		{
			name:     "template_with_parameters",
			template: "CREATE USER {{.username}} WITH PASSWORD '{{.password}}'",
			operation: Operation{
				Target: "user",
				Action: "create",
				Parameters: map[string]interface{}{
					"username": "newuser",
					"password": "secret123",
				},
			},
			expected: "CREATE USER newuser WITH PASSWORD 'secret123'",
		},
		{
			name:     "template_with_action",
			template: "AUDIT {{.Action}} ON {{.Target}}",
			operation: Operation{
				Target: "credentials",
				Action: "rotate",
			},
			expected: "AUDIT rotate ON credentials",
		},
		{
			name:     "complex_template",
			template: "ALTER USER {{.username}} SET PASSWORD '{{.new_password}}' WHERE id = {{.user_id}}",
			operation: Operation{
				Target: "user",
				Action: "rotate",
				Parameters: map[string]interface{}{
					"username":     "admin",
					"new_password": "NewP@ss123!",
					"user_id":      42,
				},
			},
			expected: "ALTER USER admin SET PASSWORD 'NewP@ss123!' WHERE id = 42",
		},
		{
			name:     "empty_parameters",
			template: "SELECT {{.Action}} FROM {{.Target}}",
			operation: Operation{
				Target:     "table",
				Action:     "count",
				Parameters: nil,
			},
			expected: "SELECT count FROM table",
		},
		{
			name:     "template_with_metadata",
			template: "-- Operation: {{.Metadata.purpose}}\nSELECT * FROM {{.Target}}",
			operation: Operation{
				Target: "audit_log",
				Action: "list",
				Metadata: map[string]string{
					"purpose": "compliance",
				},
			},
			expected: "-- Operation: compliance\nSELECT * FROM audit_log",
		},
		{
			name:     "invalid_template_syntax",
			template: "SELECT {{.Invalid syntax",
			operation: Operation{
				Target: "test",
			},
			expectError: true,
		},
		{
			name:     "missing_template_variable",
			template: "SELECT * FROM {{.NonExistent}}",
			operation: Operation{
				Target: "test",
			},
			// Go templates don't error on missing keys, they output "<no value>"
			expected: "SELECT * FROM <no value>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := adapter.renderSQLTemplate(tt.template, tt.operation)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestGetCommandTemplate tests command template retrieval.
func TestGetCommandTemplate(t *testing.T) {
	adapter := NewSQLAdapter()

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
						"create_user": "CREATE USER {{.username}}",
					},
				},
			},
			expected: "CREATE USER {{.username}}",
		},
		{
			name:   "fallback_to_action_only",
			action: "verify",
			operation: Operation{
				Target: "connection",
			},
			config: AdapterConfig{
				ServiceConfig: map[string]interface{}{
					"commands": map[string]interface{}{
						"verify": "SELECT 1",
					},
				},
			},
			expected: "SELECT 1",
		},
		{
			name:   "missing_commands_section",
			action: "rotate",
			operation: Operation{
				Target: "credential",
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
						"create_user": "CREATE USER",
					},
				},
			},
			expectError:   true,
			errorContains: "command template not found",
		},
		{
			name:   "nil_service_config",
			action: "list",
			operation: Operation{
				Target: "users",
			},
			config:        AdapterConfig{},
			expectError:   true,
			errorContains: "commands configuration not found",
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

// TestSQLAdapterDriverMap tests driver name mapping.
func TestSQLAdapterDriverMap(t *testing.T) {
	adapter := NewSQLAdapter()

	tests := []struct {
		input    string
		expected string
		exists   bool
	}{
		{"postgresql", "postgres", true},
		{"postgres", "postgres", true},
		{"mysql", "mysql", true},
		{"mariadb", "mysql", true},
		{"sqlserver", "sqlserver", true},
		{"mssql", "sqlserver", true},
		{"oracle", "", false},
		{"sqlite", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			driver, ok := adapter.driverMap[tt.input]
			assert.Equal(t, tt.exists, ok)
			if tt.exists {
				assert.Equal(t, tt.expected, driver)
			}
		})
	}
}
