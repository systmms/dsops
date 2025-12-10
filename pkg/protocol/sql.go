package protocol

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"text/template"
	"time"
	
	// Import common SQL drivers
	_ "github.com/lib/pq"          // PostgreSQL
	_ "github.com/go-sql-driver/mysql" // MySQL
)

// SQLAdapter implements the Adapter interface for SQL databases
type SQLAdapter struct {
	// Driver name mapping
	driverMap map[string]string
}

// NewSQLAdapter creates a new SQL protocol adapter
func NewSQLAdapter() *SQLAdapter {
	return &SQLAdapter{
		driverMap: map[string]string{
			"postgresql": "postgres",
			"postgres":   "postgres",
			"mysql":      "mysql",
			"mariadb":    "mysql",
			"sqlserver":  "sqlserver",
			"mssql":      "sqlserver",
		},
	}
}

// Name returns the adapter name
func (a *SQLAdapter) Name() string {
	return "SQL Protocol Adapter"
}

// Type returns the adapter type
func (a *SQLAdapter) Type() AdapterType {
	return AdapterTypeSQL
}

// Execute performs a SQL operation
func (a *SQLAdapter) Execute(ctx context.Context, operation Operation, config AdapterConfig) (*Result, error) {
	// Validate configuration
	if err := a.Validate(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Build connection string
	connStr, err := a.buildConnectionString(config)
	if err != nil {
		return nil, fmt.Errorf("failed to build connection string: %w", err)
	}
	
	// Get driver name
	dbType := config.Connection["type"]
	driver, ok := a.driverMap[strings.ToLower(dbType)]
	if !ok {
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
	
	// Open database connection
	db, err := sql.Open(driver, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	defer func() { _ = db.Close() }()
	
	// Set connection timeout
	timeout := 30 * time.Second
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}
	
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Test connection
	if err := db.PingContext(ctxWithTimeout); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	
	// Execute operation
	switch operation.Action {
	case "create":
		return a.executeCreate(ctxWithTimeout, db, operation, config)
	case "verify":
		return a.executeVerify(ctxWithTimeout, db, operation, config)
	case "rotate":
		return a.executeRotate(ctxWithTimeout, db, operation, config)
	case "revoke":
		return a.executeRevoke(ctxWithTimeout, db, operation, config)
	case "list":
		return a.executeList(ctxWithTimeout, db, operation, config)
	default:
		return nil, fmt.Errorf("unsupported action: %s", operation.Action)
	}
}

// Validate checks if the configuration is valid
func (a *SQLAdapter) Validate(config AdapterConfig) error {
	if config.Connection == nil {
		return fmt.Errorf("connection configuration is required")
	}
	
	// Check required fields
	required := []string{"type", "host", "port", "database"}
	for _, field := range required {
		if _, ok := config.Connection[field]; !ok {
			return fmt.Errorf("required connection field '%s' is missing", field)
		}
	}
	
	// Validate database type
	dbType := config.Connection["type"]
	if _, ok := a.driverMap[strings.ToLower(dbType)]; !ok {
		return fmt.Errorf("unsupported database type: %s", dbType)
	}
	
	// Check auth configuration
	if config.Auth == nil || config.Auth["username"] == "" {
		return fmt.Errorf("username is required in auth configuration")
	}
	
	return nil
}

// Capabilities returns what this adapter can do
func (a *SQLAdapter) Capabilities() Capabilities {
	return Capabilities{
		SupportedActions: []string{"create", "verify", "rotate", "revoke", "list"},
		RequiredConfig:   []string{"type", "host", "port", "database", "username"},
		OptionalConfig:   []string{"password", "sslmode", "timeout", "max_connections"},
		Features: map[string]bool{
			"transactions": true,
			"ssl":          true,
			"connection_pooling": true,
		},
	}
}

// buildConnectionString creates a database connection string
func (a *SQLAdapter) buildConnectionString(config AdapterConfig) (string, error) {
	dbType := strings.ToLower(config.Connection["type"])
	
	switch dbType {
	case "postgresql", "postgres":
		return a.buildPostgreSQLConnString(config), nil
		
	case "mysql", "mariadb":
		return a.buildMySQLConnString(config), nil
		
	case "sqlserver", "mssql":
		return a.buildSQLServerConnString(config), nil
		
	default:
		return "", fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// buildPostgreSQLConnString builds a PostgreSQL connection string
func (a *SQLAdapter) buildPostgreSQLConnString(config AdapterConfig) string {
	parts := []string{
		fmt.Sprintf("host=%s", config.Connection["host"]),
		fmt.Sprintf("port=%s", config.Connection["port"]),
		fmt.Sprintf("dbname=%s", config.Connection["database"]),
		fmt.Sprintf("user=%s", config.Auth["username"]),
	}
	
	if password, ok := config.Auth["password"]; ok && password != "" {
		parts = append(parts, fmt.Sprintf("password=%s", password))
	}
	
	if sslmode, ok := config.Connection["sslmode"]; ok {
		parts = append(parts, fmt.Sprintf("sslmode=%s", sslmode))
	} else {
		parts = append(parts, "sslmode=require")
	}
	
	return strings.Join(parts, " ")
}

// buildMySQLConnString builds a MySQL connection string
func (a *SQLAdapter) buildMySQLConnString(config AdapterConfig) string {
	password := ""
	if pass, ok := config.Auth["password"]; ok {
		password = pass
	}
	
	// MySQL DSN format: username:password@tcp(host:port)/database
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		config.Auth["username"],
		password,
		config.Connection["host"],
		config.Connection["port"],
		config.Connection["database"],
	)
}

// buildSQLServerConnString builds a SQL Server connection string
func (a *SQLAdapter) buildSQLServerConnString(config AdapterConfig) string {
	password := ""
	if pass, ok := config.Auth["password"]; ok {
		password = pass
	}
	
	// SQL Server connection string format
	return fmt.Sprintf("server=%s,%s;user id=%s;password=%s;database=%s",
		config.Connection["host"],
		config.Connection["port"],
		config.Auth["username"],
		password,
		config.Connection["database"],
	)
}

// executeCreate creates a new database user or credential
func (a *SQLAdapter) executeCreate(ctx context.Context, db *sql.DB, operation Operation, config AdapterConfig) (*Result, error) {
	// Get SQL command template from service config
	createTemplate, err := a.getCommandTemplate("create", operation, config)
	if err != nil {
		return nil, err
	}
	
	// Render SQL command
	createSQL, err := a.renderSQLTemplate(createTemplate, operation)
	if err != nil {
		return nil, fmt.Errorf("failed to render create SQL: %w", err)
	}
	
	// Execute in transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	
	// Execute create command
	_, err = tx.ExecContext(ctx, createSQL)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to execute create command: %v", err),
		}, err
	}
	
	// Commit transaction
	if err := tx.Commit(); err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to commit transaction: %v", err),
		}, err
	}
	
	return &Result{
		Success: true,
		Data: map[string]interface{}{
			"action": "create",
			"target": operation.Target,
		},
		Metadata: map[string]string{
			"database_type": config.Connection["type"],
		},
	}, nil
}

// executeVerify verifies a database credential
func (a *SQLAdapter) executeVerify(ctx context.Context, db *sql.DB, operation Operation, config AdapterConfig) (*Result, error) {
	// Get verify command template
	verifyTemplate, err := a.getCommandTemplate("verify", operation, config)
	if err != nil {
		// Default verify command
		verifyTemplate = "SELECT 1"
	}
	
	// Render SQL command
	verifySQL, err := a.renderSQLTemplate(verifyTemplate, operation)
	if err != nil {
		return nil, fmt.Errorf("failed to render verify SQL: %w", err)
	}
	
	// Execute verify command
	var result interface{}
	err = db.QueryRowContext(ctx, verifySQL).Scan(&result)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("verification failed: %v", err),
		}, err
	}
	
	return &Result{
		Success: true,
		Data: map[string]interface{}{
			"action": "verify",
			"target": operation.Target,
			"verified": true,
		},
	}, nil
}

// executeRotate rotates a database credential
func (a *SQLAdapter) executeRotate(ctx context.Context, db *sql.DB, operation Operation, config AdapterConfig) (*Result, error) {
	// Get rotate command template
	rotateTemplate, err := a.getCommandTemplate("rotate", operation, config)
	if err != nil {
		return nil, err
	}
	
	// Render SQL command
	rotateSQL, err := a.renderSQLTemplate(rotateTemplate, operation)
	if err != nil {
		return nil, fmt.Errorf("failed to render rotate SQL: %w", err)
	}
	
	// Execute in transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	
	// Execute rotate command
	_, err = tx.ExecContext(ctx, rotateSQL)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to execute rotate command: %v", err),
		}, err
	}
	
	// Commit transaction
	if err := tx.Commit(); err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to commit transaction: %v", err),
		}, err
	}
	
	return &Result{
		Success: true,
		Data: map[string]interface{}{
			"action": "rotate",
			"target": operation.Target,
		},
	}, nil
}

// executeRevoke revokes a database credential
func (a *SQLAdapter) executeRevoke(ctx context.Context, db *sql.DB, operation Operation, config AdapterConfig) (*Result, error) {
	// Get revoke command template
	revokeTemplate, err := a.getCommandTemplate("revoke", operation, config)
	if err != nil {
		return nil, err
	}
	
	// Render SQL command
	revokeSQL, err := a.renderSQLTemplate(revokeTemplate, operation)
	if err != nil {
		return nil, fmt.Errorf("failed to render revoke SQL: %w", err)
	}
	
	// Execute in transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	
	// Execute revoke command
	_, err = tx.ExecContext(ctx, revokeSQL)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to execute revoke command: %v", err),
		}, err
	}
	
	// Commit transaction
	if err := tx.Commit(); err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to commit transaction: %v", err),
		}, err
	}
	
	return &Result{
		Success: true,
		Data: map[string]interface{}{
			"action": "revoke",
			"target": operation.Target,
		},
	}, nil
}

// executeList lists database users or credentials
func (a *SQLAdapter) executeList(ctx context.Context, db *sql.DB, operation Operation, config AdapterConfig) (*Result, error) {
	// Get list command template
	listTemplate, err := a.getCommandTemplate("list", operation, config)
	if err != nil {
		return nil, err
	}
	
	// Render SQL command
	listSQL, err := a.renderSQLTemplate(listTemplate, operation)
	if err != nil {
		return nil, fmt.Errorf("failed to render list SQL: %w", err)
	}
	
	// Execute query
	rows, err := db.QueryContext(ctx, listSQL)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to execute list query: %v", err),
		}, err
	}
	defer func() { _ = rows.Close() }()
	
	// Collect results
	var items []map[string]interface{}
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	
	for rows.Next() {
		// Create a slice of interface{} to represent each column
		values := make([]interface{}, len(columns))
		valuePointers := make([]interface{}, len(columns))
		for i := range values {
			valuePointers[i] = &values[i]
		}
		
		// Scan the row
		if err := rows.Scan(valuePointers...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		
		// Create map for this row
		item := make(map[string]interface{})
		for i, col := range columns {
			item[col] = values[i]
		}
		items = append(items, item)
	}
	
	return &Result{
		Success: true,
		Data: map[string]interface{}{
			"action": "list",
			"target": operation.Target,
			"items":  items,
			"count":  len(items),
		},
	}, nil
}

// getCommandTemplate retrieves the SQL command template for an operation
func (a *SQLAdapter) getCommandTemplate(action string, operation Operation, config AdapterConfig) (string, error) {
	// Look for commands in service config
	commands, ok := config.ServiceConfig["commands"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("commands configuration not found")
	}
	
	// Find command for this action and target
	key := fmt.Sprintf("%s_%s", action, operation.Target)
	if cmd, ok := commands[key].(string); ok {
		return cmd, nil
	}
	
	// Fall back to action-only key
	if cmd, ok := commands[action].(string); ok {
		return cmd, nil
	}
	
	return "", fmt.Errorf("command template not found for action %s", action)
}

// renderSQLTemplate renders a SQL template with operation data
func (a *SQLAdapter) renderSQLTemplate(templateStr string, operation Operation) (string, error) {
	tmpl, err := template.New("sql").Parse(templateStr)
	if err != nil {
		return "", err
	}
	
	var buf strings.Builder
	data := map[string]interface{}{
		"Target":     operation.Target,
		"Action":     operation.Action,
		"Parameters": operation.Parameters,
		"Metadata":   operation.Metadata,
	}
	
	// Add common parameters
	if operation.Parameters != nil {
		for k, v := range operation.Parameters {
			data[k] = v
		}
	}
	
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	
	return buf.String(), nil
}