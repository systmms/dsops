package testutil

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// DockerTestEnv manages Docker Compose lifecycle for integration tests
type DockerTestEnv struct {
	t             *testing.T
	composePath   string
	services      []string
	started       bool
	clients       map[string]interface{}
	cleanupFuncs  []func()
	projectName   string
	ports         map[string]map[int]int // service -> containerPort -> hostPort
}

// VaultTestClient wraps Vault HTTP API for testing
type VaultTestClient struct {
	address string
	token   string
	client  *http.Client
}

// LocalStackTestClient wraps AWS SDK for LocalStack testing
type LocalStackTestClient struct {
	secretsManager *secretsmanager.Client
	ssm            *ssm.Client
}

// PostgresTestClient wraps PostgreSQL database connection
type PostgresTestClient struct {
	db *sql.DB
}

// QueryRowResult wraps sql.Row with context cancellation
// This ensures the context is not cancelled until Scan() is called
type QueryRowResult struct {
	row    *sql.Row
	cancel context.CancelFunc
}

// Scan scans the row and cancels the context
func (r *QueryRowResult) Scan(dest ...interface{}) error {
	defer r.cancel()
	return r.row.Scan(dest...)
}

// QueryResult wraps sql.Rows with context cancellation
type QueryResult struct {
	*sql.Rows
	cancel context.CancelFunc
}

// Close closes the rows and cancels the context
func (r *QueryResult) Close() error {
	defer r.cancel()
	return r.Rows.Close()
}

// MongoTestClient wraps MongoDB connection
type MongoTestClient struct {
	// TODO: Add MongoDB client when needed
	connectionString string
}

// StartDockerEnv starts Docker Compose services for integration testing
func StartDockerEnv(t *testing.T, services []string) *DockerTestEnv {
	t.Helper()

	// Check Docker availability
	SkipIfDockerUnavailable(t)

	// Clear provider-specific environment variables that could interfere with tests
	// These variables are read by providers and would override config addresses
	clearProviderEnvVars(t)

	// Find docker-compose.yml path (relative to test file)
	composePath := findDockerComposePath(t)
	if composePath == "" {
		t.Fatal("docker-compose.yml not found in tests/integration/")
	}

	// Use test name as project name to avoid conflicts
	// UnixNano provides nanosecond precision to prevent collisions in parallel tests
	projectName := fmt.Sprintf("dsops-test-%d", time.Now().UnixNano())

	env := &DockerTestEnv{
		t:           t,
		composePath: composePath,
		services:    services,
		clients:     make(map[string]interface{}),
		projectName: projectName,
	}

	// Start Docker Compose services
	env.start()

	// Register cleanup
	t.Cleanup(func() {
		env.Stop()
	})

	// Wait for services to be healthy
	if err := env.WaitForHealthy(60 * time.Second); err != nil {
		t.Fatalf("Docker services failed to become healthy: %v", err)
	}

	// Discover dynamically assigned ports
	if err := env.discoverPorts(); err != nil {
		t.Fatalf("Failed to discover ports: %v", err)
	}

	return env
}

// clearProviderEnvVars clears environment variables that providers read
// which could override test configuration with stale values
func clearProviderEnvVars(t *testing.T) {
	t.Helper()

	// Provider environment variables that override config addresses
	envVars := []string{
		// Vault provider
		"VAULT_ADDR",
		"VAULT_TOKEN",
		// AWS providers
		"AWS_ENDPOINT_URL",
		"AWS_ENDPOINT_URL_SECRETSMANAGER",
		"AWS_ENDPOINT_URL_SSM",
		// MongoDB
		"MONGODB_URI",
		// PostgreSQL
		"PGHOST",
		"PGPORT",
	}

	for _, env := range envVars {
		if val := os.Getenv(env); val != "" {
			t.Logf("Clearing %s (was: %s) to prevent config override", env, val)
			_ = os.Unsetenv(env)
		}
	}
}

// SkipIfDockerUnavailable skips the test if Docker is not available
func SkipIfDockerUnavailable(t *testing.T) {
	t.Helper()

	if !IsDockerAvailable() {
		t.Skip("Docker not available, skipping integration test")
	}
}

// IsDockerAvailable checks if Docker is available and running
func IsDockerAvailable() bool {
	// Check if docker command exists
	if _, err := exec.LookPath("docker"); err != nil {
		return false
	}

	// Check if Docker daemon is running
	cmd := exec.Command("docker", "ps")
	if err := cmd.Run(); err != nil {
		return false
	}

	// Check if docker-compose exists (try both v1 and v2 syntax)
	if _, err := exec.LookPath("docker-compose"); err != nil {
		// Try docker compose (v2 syntax)
		cmd := exec.Command("docker", "compose", "version")
		if err := cmd.Run(); err != nil {
			return false
		}
	}

	return true
}

// start starts Docker Compose services
func (e *DockerTestEnv) start() {
	e.t.Helper()

	composeDir := filepath.Dir(e.composePath)

	// Build docker-compose command
	args := []string{
		"compose",
		"-f", e.composePath,
		"-p", e.projectName,
		"up", "-d",
	}
	args = append(args, e.services...)

	cmd := exec.Command("docker", args...)
	cmd.Dir = composeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	e.t.Logf("Starting Docker services: %v", e.services)

	if err := cmd.Run(); err != nil {
		e.t.Fatalf("Failed to start Docker services: %v", err)
	}

	e.started = true
}

// Stop stops and removes Docker Compose services
func (e *DockerTestEnv) Stop() {
	if !e.started {
		return
	}

	// Run cleanup functions
	for _, fn := range e.cleanupFuncs {
		fn()
	}

	composeDir := filepath.Dir(e.composePath)

	// Stop and remove containers
	cmd := exec.Command("docker", "compose",
		"-f", e.composePath,
		"-p", e.projectName,
		"down", "-v")
	cmd.Dir = composeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		e.t.Logf("Warning: Failed to stop Docker services: %v", err)
	}

	e.started = false
}

// WaitForHealthy waits for all services to be healthy
func (e *DockerTestEnv) WaitForHealthy(timeout time.Duration) error {
	e.t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for services to be healthy")
		case <-ticker.C:
			if e.checkHealth() {
				e.t.Logf("All services are healthy")
				return nil
			}
		}
	}
}

// checkHealth checks if all requested services are healthy
func (e *DockerTestEnv) checkHealth() bool {
	for _, service := range e.services {
		// Docker Compose generates container names as: {project_name}-{service}-{replica}
		containerName := fmt.Sprintf("%s-%s-1", e.projectName, service)

		cmd := exec.Command("docker", "inspect",
			"--format", "{{.State.Health.Status}}",
			containerName)

		output, err := cmd.Output()
		if err != nil {
			// Container might not have health check
			// Check if it's at least running
			cmd = exec.Command("docker", "inspect",
				"--format", "{{.State.Status}}",
				containerName)
			output, err = cmd.Output()
			if err != nil {
				return false
			}
			status := strings.TrimSpace(string(output))
			if status != "running" {
				return false
			}
			continue
		}

		status := strings.TrimSpace(string(output))
		if status != "healthy" && status != "" {
			return false
		}
	}
	return true
}

// discoverPorts discovers dynamically assigned host ports for all services
func (e *DockerTestEnv) discoverPorts() error {
	e.ports = make(map[string]map[int]int)

	// Map of services to their container ports
	servicePorts := map[string][]int{
		"vault":      {8200},
		"postgres":   {5432},
		"localstack": {4566},
		"mongodb":    {27017},
		"mailhog":    {1025, 8025},
	}

	composeDir := filepath.Dir(e.composePath)

	for _, service := range e.services {
		ports, ok := servicePorts[service]
		if !ok {
			continue
		}

		e.ports[service] = make(map[int]int)

		for _, containerPort := range ports {
			// Run: docker compose -p PROJECT port SERVICE CONTAINER_PORT
			cmd := exec.Command("docker", "compose",
				"-f", e.composePath,
				"-p", e.projectName,
				"port", service, fmt.Sprintf("%d", containerPort))
			cmd.Dir = composeDir

			output, err := cmd.Output()
			if err != nil {
				return fmt.Errorf("failed to get port for %s:%d: %w", service, containerPort, err)
			}

			// Parse output: "0.0.0.0:32768" -> extract 32768
			portStr := strings.TrimSpace(string(output))
			parts := strings.Split(portStr, ":")
			if len(parts) != 2 {
				return fmt.Errorf("unexpected port output format: %s", portStr)
			}

			hostPort := 0
			if _, err := fmt.Sscanf(parts[1], "%d", &hostPort); err != nil {
				return fmt.Errorf("failed to parse host port from %s: %w", portStr, err)
			}

			e.ports[service][containerPort] = hostPort
			e.t.Logf("Discovered port mapping: %s:%d -> localhost:%d", service, containerPort, hostPort)
		}
	}

	return nil
}

// GetPort returns the host port for a service's container port
// Returns the container port as fallback if not found (for backward compatibility)
func (e *DockerTestEnv) GetPort(service string, containerPort int) int {
	if servicePorts, ok := e.ports[service]; ok {
		if hostPort, ok := servicePorts[containerPort]; ok {
			return hostPort
		}
	}
	return containerPort // fallback for compatibility
}

// PostgresConnString returns the PostgreSQL connection string with dynamic port
func (e *DockerTestEnv) PostgresConnString() string {
	port := e.GetPort("postgres", 5432)
	return fmt.Sprintf("host=127.0.0.1 port=%d user=test password=test-password dbname=testdb sslmode=disable", port)
}

// VaultAddress returns the Vault address with dynamic port
func (e *DockerTestEnv) VaultAddress() string {
	port := e.GetPort("vault", 8200)
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

// LocalStackEndpoint returns the LocalStack endpoint with dynamic port
func (e *DockerTestEnv) LocalStackEndpoint() string {
	port := e.GetPort("localstack", 4566)
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

// MailhogSMTPAddr returns the MailHog SMTP address with dynamic port
func (e *DockerTestEnv) MailhogSMTPAddr() string {
	port := e.GetPort("mailhog", 1025)
	return fmt.Sprintf("127.0.0.1:%d", port)
}

// MailhogAPIAddr returns the MailHog HTTP API address with dynamic port
func (e *DockerTestEnv) MailhogAPIAddr() string {
	port := e.GetPort("mailhog", 8025)
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

// MongoDBConnString returns the MongoDB connection string with dynamic port
func (e *DockerTestEnv) MongoDBConnString() string {
	port := e.GetPort("mongodb", 27017)
	return fmt.Sprintf("mongodb://test:test-password@127.0.0.1:%d/", port)
}

// VaultClient returns a Vault test client
func (e *DockerTestEnv) VaultClient() *VaultTestClient {
	e.t.Helper()

	if client, ok := e.clients["vault"].(*VaultTestClient); ok {
		return client
	}

	client := &VaultTestClient{
		address: e.VaultAddress(),
		token:   "test-root-token",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	e.clients["vault"] = client
	return client
}

// PostgresClient returns a PostgreSQL test client
func (e *DockerTestEnv) PostgresClient() *PostgresTestClient {
	e.t.Helper()

	if client, ok := e.clients["postgres"].(*PostgresTestClient); ok {
		return client
	}

	connStr := e.PostgresConnString()
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		e.t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		e.t.Fatalf("Failed to ping PostgreSQL: %v", err)
	}

	// Configure connection pool for concurrent operations
	// These settings prevent protocol corruption during concurrent DDL operations:
	// - MaxOpenConns: Allow enough connections for concurrent tests
	// - MaxIdleConns: Reduce connection churn by keeping connections warm
	// - ConnMaxLifetime: Rotate connections to prevent stale state accumulation
	// - ConnMaxIdleTime: Close idle connections that might be corrupted
	db.SetMaxOpenConns(20)                     // Allow up to 20 concurrent connections
	db.SetMaxIdleConns(10)                     // Keep up to 10 idle connections
	db.SetConnMaxLifetime(5 * time.Minute)     // Rotate connections every 5 minutes
	db.SetConnMaxIdleTime(1 * time.Minute)     // Close idle connections after 1 minute

	client := &PostgresTestClient{db: db}
	e.clients["postgres"] = client

	// Register cleanup
	e.cleanupFuncs = append(e.cleanupFuncs, func() {
		_ = db.Close()
	})

	return client
}

// LocalStackClient returns a LocalStack test client
func (e *DockerTestEnv) LocalStackClient() *LocalStackTestClient {
	e.t.Helper()

	if client, ok := e.clients["localstack"].(*LocalStackTestClient); ok {
		return client
	}

	ctx := context.Background()

	// Configure AWS SDK for LocalStack
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test", "test", "",
		)),
	)
	if err != nil {
		e.t.Fatalf("Failed to load AWS config: %v", err)
	}

	// Override endpoint for LocalStack with dynamic port
	endpoint := e.LocalStackEndpoint()

	smClient := secretsmanager.NewFromConfig(cfg, func(o *secretsmanager.Options) {
		o.BaseEndpoint = &endpoint
	})

	ssmClient := ssm.NewFromConfig(cfg, func(o *ssm.Options) {
		o.BaseEndpoint = &endpoint
	})

	client := &LocalStackTestClient{
		secretsManager: smClient,
		ssm:            ssmClient,
	}

	e.clients["localstack"] = client
	return client
}

// MongoClient returns a MongoDB test client
func (e *DockerTestEnv) MongoClient() *MongoTestClient {
	e.t.Helper()

	if client, ok := e.clients["mongodb"].(*MongoTestClient); ok {
		return client
	}

	// TODO: Implement MongoDB client when needed
	client := &MongoTestClient{
		connectionString: e.MongoDBConnString(),
	}

	e.clients["mongodb"] = client
	return client
}

// VaultConfig returns Vault configuration for provider testing
func (e *DockerTestEnv) VaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"address": e.VaultAddress(),
		"token":   "test-root-token",
	}
}

// PostgresConfig returns PostgreSQL configuration
func (e *DockerTestEnv) PostgresConfig() map[string]interface{} {
	return map[string]interface{}{
		"host":     "127.0.0.1",
		"port":     e.GetPort("postgres", 5432),
		"user":     "test",
		"password": "test-password",
		"database": "testdb",
		"sslmode":  "disable",
	}
}

// LocalStackConfig returns LocalStack configuration
// Includes dummy credentials that LocalStack accepts for testing
func (e *DockerTestEnv) LocalStackConfig() map[string]interface{} {
	return map[string]interface{}{
		"region":            "us-east-1",
		"endpoint":          e.LocalStackEndpoint(),
		"access_key_id":     "test",
		"secret_access_key": "test",
	}
}

// findDockerComposePath finds the docker-compose.yml file
func findDockerComposePath(t *testing.T) string {
	t.Helper()

	// Try relative paths from common test locations
	candidates := []string{
		"../../tests/integration/docker-compose.yml",
		"../integration/docker-compose.yml",
		"./tests/integration/docker-compose.yml",
		"tests/integration/docker-compose.yml",
	}

	// Also try from GOPATH or module root
	if wd, err := os.Getwd(); err == nil {
		// Walk up to find module root (contains go.mod)
		dir := wd
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				candidates = append(candidates, filepath.Join(dir, "tests/integration/docker-compose.yml"))
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	for _, path := range candidates {
		if absPath, err := filepath.Abs(path); err == nil {
			if _, err := os.Stat(absPath); err == nil {
				return absPath
			}
		}
	}

	return ""
}

// ============================================================================
// VaultTestClient Methods
// ============================================================================

// Write writes a secret to Vault (KV v2)
func (v *VaultTestClient) Write(path string, data map[string]interface{}) error {
	// KV v2 requires wrapping data in "data" field
	payload := map[string]interface{}{
		"data": data,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	url := fmt.Sprintf("%s/v1/%s", v.address, path)
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", v.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to write secret: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("vault write failed: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// Read reads a secret from Vault (KV v2)
func (v *VaultTestClient) Read(path string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/v1/%s", v.address, path)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", v.token)

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vault read failed: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Data struct {
			Data     map[string]interface{} `json:"data"`
			Metadata map[string]interface{} `json:"metadata"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data.Data, nil
}

// Delete deletes a secret from Vault
func (v *VaultTestClient) Delete(path string) error {
	url := fmt.Sprintf("%s/v1/%s", v.address, path)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", v.token)

	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("vault delete failed: status=%d", resp.StatusCode)
	}

	return nil
}

// ListSecrets lists secrets at a path
func (v *VaultTestClient) ListSecrets(path string) ([]string, error) {
	url := fmt.Sprintf("%s/v1/%s?list=true", v.address, path)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", v.token)

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vault list failed: status=%d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data.Keys, nil
}

// ============================================================================
// LocalStackTestClient Methods
// ============================================================================

// CreateSecret creates a secret in AWS Secrets Manager (LocalStack)
func (l *LocalStackTestClient) CreateSecret(name string, value map[string]interface{}) error {
	ctx := context.Background()

	secretBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal secret: %w", err)
	}

	secretString := string(secretBytes)

	_, err = l.secretsManager.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(name),
		SecretString: aws.String(secretString),
	})

	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}

	return nil
}

// PutParameter puts a parameter in SSM Parameter Store (LocalStack)
func (l *LocalStackTestClient) PutParameter(name, value string) error {
	ctx := context.Background()

	_, err := l.ssm.PutParameter(ctx, &ssm.PutParameterInput{
		Name:  aws.String(name),
		Value: aws.String(value),
		Type:  "String",
	})

	if err != nil {
		return fmt.Errorf("failed to put parameter: %w", err)
	}

	return nil
}

// GetSecretValue retrieves a secret from Secrets Manager
func (l *LocalStackTestClient) GetSecretValue(name string) (map[string]interface{}, error) {
	ctx := context.Background()

	result, err := l.secretsManager.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(name),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(*result.SecretString), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secret: %w", err)
	}

	return data, nil
}

// GetParameter retrieves a parameter from SSM Parameter Store
func (l *LocalStackTestClient) GetParameter(name string) (string, error) {
	ctx := context.Background()

	result, err := l.ssm.GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String(name),
	})

	if err != nil {
		return "", fmt.Errorf("failed to get parameter: %w", err)
	}

	return *result.Parameter.Value, nil
}

// ============================================================================
// PostgresTestClient Methods
// ============================================================================

// Exec executes a SQL statement
func (p *PostgresTestClient) Exec(query string, args ...interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := p.db.ExecContext(ctx, query, args...)
	return err
}

// Query executes a SQL query
func (p *PostgresTestClient) Query(query string, args ...interface{}) (*QueryResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		cancel()
		return nil, err
	}
	return &QueryResult{Rows: rows, cancel: cancel}, nil
}

// QueryRow executes a SQL query that returns a single row
func (p *PostgresTestClient) QueryRow(query string, args ...interface{}) *QueryRowResult {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	row := p.db.QueryRowContext(ctx, query, args...)
	return &QueryRowResult{row: row, cancel: cancel}
}

// CreateTestUser creates a test PostgreSQL user
func (p *PostgresTestClient) CreateTestUser(username, password string) error {
	query := fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", username, password)
	return p.Exec(query)
}

// DropTestUser drops a test PostgreSQL user
func (p *PostgresTestClient) DropTestUser(username string) error {
	query := fmt.Sprintf("DROP USER IF EXISTS %s", username)
	return p.Exec(query)
}

// UserExists checks if a user exists
func (p *PostgresTestClient) UserExists(username string) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)"
	err := p.QueryRow(query, username).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// Close closes the PostgreSQL connection
func (p *PostgresTestClient) Close() error {
	return p.db.Close()
}
