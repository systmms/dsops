package providers_test

import (
	"os"
	"testing"

	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
)

func TestAWSSecretsManagerProviderContract(t *testing.T) {
	// Skip if AWS testing is not enabled
	if _, exists := os.LookupEnv("DSOPS_TEST_AWS"); !exists {
		t.Skip("Skipping AWS Secrets Manager provider contract tests. Set DSOPS_TEST_AWS=1 to run.")
		return
	}

	contract := provider.ContractTest{
		CreateProvider: func(t *testing.T) provider.Provider {
			config := map[string]interface{}{
				"region": "us-east-1",
			}
			p, err := providers.NewAWSSecretsManagerProvider("test-aws", config)
			if err != nil {
				t.Fatalf("Failed to create AWS Secrets Manager provider: %v", err)
			}
			return p
		},
		SetupTestSecret: func(t *testing.T, p provider.Provider) (string, func()) {
			// For real AWS testing, you'd need to:
			// 1. Ensure AWS credentials are configured
			// 2. Create a test secret in AWS Secrets Manager
			// 3. Return the secret name
			// 4. Clean up the secret afterwards
			
			// For now, we'll skip this as it requires AWS credentials
			t.Skip("AWS Secrets Manager contract tests require AWS credentials")
			return "", func() {}
		},
		// AWS provider validation always succeeds if credentials are available
		SkipValidation: false,
	}

	provider.RunContractTests(t, contract)
}

// Unit tests for AWS-specific functionality
func TestAWSSecretsManagerParseKey(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		wantSecret   string
		wantJSONPath string
	}{
		{
			name:         "simple secret name",
			key:          "my-secret",
			wantSecret:   "my-secret",
			wantJSONPath: "",
		},
		{
			name:         "hierarchical secret name",
			key:          "prod/database/config",
			wantSecret:   "prod/database/config",
			wantJSONPath: "",
		},
		{
			name:         "secret with JSON extraction",
			key:          "my-secret#.password",
			wantSecret:   "my-secret",
			wantJSONPath: ".password",
		},
		{
			name:         "secret with nested JSON path",
			key:          "config#.database.connection_string",
			wantSecret:   "config",
			wantJSONPath: ".database.connection_string",
		},
		{
			name:         "ARN format",
			key:          "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret-AbCdEf",
			wantSecret:   "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret-AbCdEf",
			wantJSONPath: "",
		},
		{
			name:         "ARN with JSON extraction",
			key:          "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret-AbCdEf#.api_key",
			wantSecret:   "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret-AbCdEf",
			wantJSONPath: ".api_key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would test the internal parseKey method
			// We'd need to make it public or test indirectly
		})
	}
}

func TestAWSSecretsManagerJSONExtraction(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		path     string
		want     string
		wantErr  bool
	}{
		{
			name: "simple field",
			json: `{"password": "secret123"}`,
			path: ".password",
			want: "secret123",
		},
		{
			name: "nested field",
			json: `{"database": {"host": "db.example.com", "port": 5432}}`,
			path: ".database.host",
			want: "db.example.com",
		},
		{
			name: "array access",
			json: `{"keys": ["first", "second", "third"]}`,
			path: ".keys[1]",
			want: "second",
		},
		{
			name:    "missing field",
			json:    `{"foo": "bar"}`,
			path:    ".missing",
			wantErr: true,
		},
		{
			name: "number conversion",
			json: `{"port": 5432}`,
			path: ".port",
			want: "5432",
		},
		{
			name: "boolean conversion",
			json: `{"enabled": true}`,
			path: ".enabled",
			want: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would test the JSON extraction functionality
			// We'd need access to the extractJSONPath function
		})
	}
}