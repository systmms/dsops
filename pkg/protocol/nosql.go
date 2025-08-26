package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"time"
)

// NoSQLAdapter implements the Adapter interface for NoSQL databases
type NoSQLAdapter struct {
	// Protocol handlers for different NoSQL types
	handlers map[string]NoSQLHandler
}

// NoSQLHandler defines the interface for specific NoSQL database handlers
type NoSQLHandler interface {
	Connect(ctx context.Context, config AdapterConfig) (NoSQLConnection, error)
	ValidateConfig(config AdapterConfig) error
}

// NoSQLConnection represents a connection to a NoSQL database
type NoSQLConnection interface {
	Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error)
	Close() error
}

// NewNoSQLAdapter creates a new NoSQL protocol adapter
func NewNoSQLAdapter() *NoSQLAdapter {
	return &NoSQLAdapter{
		handlers: map[string]NoSQLHandler{
			"mongodb": &MongoHandler{},
			"redis":   &RedisHandler{},
			// Additional handlers can be added here
		},
	}
}

// Name returns the adapter name
func (a *NoSQLAdapter) Name() string {
	return "NoSQL Protocol Adapter"
}

// Type returns the adapter type
func (a *NoSQLAdapter) Type() AdapterType {
	return AdapterTypeNoSQL
}

// Execute performs a NoSQL operation
func (a *NoSQLAdapter) Execute(ctx context.Context, operation Operation, config AdapterConfig) (*Result, error) {
	// Validate configuration
	if err := a.Validate(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Get the appropriate handler
	dbType := strings.ToLower(config.Connection["type"])
	handler, ok := a.handlers[dbType]
	if !ok {
		return nil, fmt.Errorf("unsupported NoSQL database type: %s", dbType)
	}
	
	// Connect to the database
	conn, err := handler.Connect(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()
	
	// Execute the operation
	switch operation.Action {
	case "create":
		return a.executeCreate(ctx, conn, operation, config)
	case "verify":
		return a.executeVerify(ctx, conn, operation, config)
	case "rotate":
		return a.executeRotate(ctx, conn, operation, config)
	case "revoke":
		return a.executeRevoke(ctx, conn, operation, config)
	case "list":
		return a.executeList(ctx, conn, operation, config)
	default:
		return nil, fmt.Errorf("unsupported action: %s", operation.Action)
	}
}

// Validate checks if the configuration is valid
func (a *NoSQLAdapter) Validate(config AdapterConfig) error {
	if config.Connection == nil {
		return fmt.Errorf("connection configuration is required")
	}
	
	dbType, ok := config.Connection["type"]
	if !ok || dbType == "" {
		return fmt.Errorf("database type is required")
	}
	
	// Get handler for specific validation
	handler, ok := a.handlers[strings.ToLower(dbType)]
	if !ok {
		return fmt.Errorf("unsupported NoSQL database type: %s", dbType)
	}
	
	return handler.ValidateConfig(config)
}

// Capabilities returns what this adapter can do
func (a *NoSQLAdapter) Capabilities() Capabilities {
	return Capabilities{
		SupportedActions: []string{"create", "verify", "rotate", "revoke", "list"},
		RequiredConfig:   []string{"type", "host", "port"},
		OptionalConfig:   []string{"database", "collection", "keyspace", "timeout", "ssl"},
		Features: map[string]bool{
			"document_store": true,
			"key_value":      true,
			"json_support":   true,
		},
	}
}

// executeCreate creates a new credential or user
func (a *NoSQLAdapter) executeCreate(ctx context.Context, conn NoSQLConnection, operation Operation, config AdapterConfig) (*Result, error) {
	// Get command template
	createTemplate, err := a.getCommandTemplate("create", operation, config)
	if err != nil {
		return nil, err
	}
	
	// Render command
	command, params, err := a.renderCommand(createTemplate, operation)
	if err != nil {
		return nil, fmt.Errorf("failed to render create command: %w", err)
	}
	
	// Execute command
	result, err := conn.Execute(ctx, command, params)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to execute create: %v", err),
		}, err
	}
	
	return &Result{
		Success: true,
		Data: map[string]interface{}{
			"action": "create",
			"target": operation.Target,
			"result": result,
		},
		Metadata: map[string]string{
			"database_type": config.Connection["type"],
		},
	}, nil
}

// executeVerify verifies a credential
func (a *NoSQLAdapter) executeVerify(ctx context.Context, conn NoSQLConnection, operation Operation, config AdapterConfig) (*Result, error) {
	// Get command template
	verifyTemplate, err := a.getCommandTemplate("verify", operation, config)
	if err != nil {
		// Default verify - ping/info command
		verifyTemplate = `{"ping": 1}`
	}
	
	// Render command
	command, params, err := a.renderCommand(verifyTemplate, operation)
	if err != nil {
		return nil, fmt.Errorf("failed to render verify command: %w", err)
	}
	
	// Execute command
	_, err = conn.Execute(ctx, command, params)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("verification failed: %v", err),
		}, err
	}
	
	return &Result{
		Success: true,
		Data: map[string]interface{}{
			"action":   "verify",
			"target":   operation.Target,
			"verified": true,
		},
	}, nil
}

// executeRotate rotates a credential
func (a *NoSQLAdapter) executeRotate(ctx context.Context, conn NoSQLConnection, operation Operation, config AdapterConfig) (*Result, error) {
	// Get command template
	rotateTemplate, err := a.getCommandTemplate("rotate", operation, config)
	if err != nil {
		return nil, err
	}
	
	// Render command
	command, params, err := a.renderCommand(rotateTemplate, operation)
	if err != nil {
		return nil, fmt.Errorf("failed to render rotate command: %w", err)
	}
	
	// Execute command
	result, err := conn.Execute(ctx, command, params)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to execute rotate: %v", err),
		}, err
	}
	
	return &Result{
		Success: true,
		Data: map[string]interface{}{
			"action": "rotate",
			"target": operation.Target,
			"result": result,
		},
	}, nil
}

// executeRevoke revokes a credential
func (a *NoSQLAdapter) executeRevoke(ctx context.Context, conn NoSQLConnection, operation Operation, config AdapterConfig) (*Result, error) {
	// Get command template
	revokeTemplate, err := a.getCommandTemplate("revoke", operation, config)
	if err != nil {
		return nil, err
	}
	
	// Render command
	command, params, err := a.renderCommand(revokeTemplate, operation)
	if err != nil {
		return nil, fmt.Errorf("failed to render revoke command: %w", err)
	}
	
	// Execute command
	result, err := conn.Execute(ctx, command, params)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to execute revoke: %v", err),
		}, err
	}
	
	return &Result{
		Success: true,
		Data: map[string]interface{}{
			"action": "revoke",
			"target": operation.Target,
			"result": result,
		},
	}, nil
}

// executeList lists credentials or users
func (a *NoSQLAdapter) executeList(ctx context.Context, conn NoSQLConnection, operation Operation, config AdapterConfig) (*Result, error) {
	// Get command template
	listTemplate, err := a.getCommandTemplate("list", operation, config)
	if err != nil {
		return nil, err
	}
	
	// Render command
	command, params, err := a.renderCommand(listTemplate, operation)
	if err != nil {
		return nil, fmt.Errorf("failed to render list command: %w", err)
	}
	
	// Execute command
	result, err := conn.Execute(ctx, command, params)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to execute list: %v", err),
		}, err
	}
	
	// Convert result to list if needed
	var items []interface{}
	switch v := result.(type) {
	case []interface{}:
		items = v
	case []map[string]interface{}:
		for _, item := range v {
			items = append(items, item)
		}
	default:
		items = []interface{}{result}
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

// getCommandTemplate retrieves the command template for an operation
func (a *NoSQLAdapter) getCommandTemplate(action string, operation Operation, config AdapterConfig) (string, error) {
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

// renderCommand renders a command template with operation data
func (a *NoSQLAdapter) renderCommand(templateStr string, operation Operation) (string, map[string]interface{}, error) {
	// First, try to parse as JSON template
	if strings.TrimSpace(templateStr)[0] == '{' {
		// It's likely a JSON template
		tmpl, err := template.New("command").Parse(templateStr)
		if err != nil {
			return "", nil, err
		}
		
		var buf strings.Builder
		data := a.buildTemplateData(operation)
		
		if err := tmpl.Execute(&buf, data); err != nil {
			return "", nil, err
		}
		
		// Parse the rendered JSON to extract parameters
		var jsonData map[string]interface{}
		if err := json.Unmarshal([]byte(buf.String()), &jsonData); err != nil {
			return "", nil, fmt.Errorf("invalid JSON in rendered command: %w", err)
		}
		
		// Extract command and parameters
		if cmd, ok := jsonData["command"].(string); ok {
			delete(jsonData, "command")
			return cmd, jsonData, nil
		}
		
		// If no explicit command field, use the entire JSON as the command
		return buf.String(), nil, nil
	}
	
	// Plain text command template
	tmpl, err := template.New("command").Parse(templateStr)
	if err != nil {
		return "", nil, err
	}
	
	var buf strings.Builder
	data := a.buildTemplateData(operation)
	
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", nil, err
	}
	
	// Extract parameters from operation
	params := make(map[string]interface{})
	if operation.Parameters != nil {
		for k, v := range operation.Parameters {
			params[k] = v
		}
	}
	
	return buf.String(), params, nil
}

// buildTemplateData builds the data for template rendering
func (a *NoSQLAdapter) buildTemplateData(operation Operation) map[string]interface{} {
	data := map[string]interface{}{
		"Target":     operation.Target,
		"Action":     operation.Action,
		"Parameters": operation.Parameters,
		"Metadata":   operation.Metadata,
	}
	
	// Add all parameters as top-level fields for easier access
	if operation.Parameters != nil {
		for k, v := range operation.Parameters {
			data[k] = v
		}
	}
	
	return data
}

// MongoHandler handles MongoDB connections
type MongoHandler struct{}

func (h *MongoHandler) Connect(ctx context.Context, config AdapterConfig) (NoSQLConnection, error) {
	// This is a placeholder - in real implementation, would use MongoDB driver
	return &MockNoSQLConnection{dbType: "mongodb"}, nil
}

func (h *MongoHandler) ValidateConfig(config AdapterConfig) error {
	// Check MongoDB-specific requirements
	required := []string{"host", "port"}
	for _, field := range required {
		if _, ok := config.Connection[field]; !ok {
			return fmt.Errorf("required field '%s' is missing", field)
		}
	}
	return nil
}

// RedisHandler handles Redis connections
type RedisHandler struct{}

func (h *RedisHandler) Connect(ctx context.Context, config AdapterConfig) (NoSQLConnection, error) {
	// This is a placeholder - in real implementation, would use Redis driver
	return &MockNoSQLConnection{dbType: "redis"}, nil
}

func (h *RedisHandler) ValidateConfig(config AdapterConfig) error {
	// Check Redis-specific requirements
	required := []string{"host", "port"}
	for _, field := range required {
		if _, ok := config.Connection[field]; !ok {
			return fmt.Errorf("required field '%s' is missing", field)
		}
	}
	return nil
}

// MockNoSQLConnection is a placeholder implementation
type MockNoSQLConnection struct {
	dbType string
}

func (c *MockNoSQLConnection) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	// This is a mock implementation
	// In real implementation, would execute actual database commands
	result := map[string]interface{}{
		"status":  "success",
		"command": command,
		"type":    c.dbType,
	}
	
	// Add timeout check
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(100 * time.Millisecond):
		// Simulate some processing time
	}
	
	return result, nil
}

func (c *MockNoSQLConnection) Close() error {
	return nil
}