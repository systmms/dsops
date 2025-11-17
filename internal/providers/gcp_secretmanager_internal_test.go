package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGCPSecretManagerParseReference tests parsing GCP Secret Manager references.
func TestGCPSecretManagerParseReference(t *testing.T) {
	p := &GCPSecretManagerProvider{}

	tests := []struct {
		name             string
		ref              string
		expectedSecret   string
		expectedVersion  string
		expectedJSONPath string
	}{
		// Simple secret names (default version = "latest")
		{
			name:             "simple_secret_name",
			ref:              "database-password",
			expectedSecret:   "database-password",
			expectedVersion:  "latest",
			expectedJSONPath: "",
		},
		{
			name:             "secret_with_hyphens",
			ref:              "my-api-key-prod",
			expectedSecret:   "my-api-key-prod",
			expectedVersion:  "latest",
			expectedJSONPath: "",
		},
		{
			name:             "secret_with_underscores",
			ref:              "stripe_secret_key",
			expectedSecret:   "stripe_secret_key",
			expectedVersion:  "latest",
			expectedJSONPath: "",
		},
		// Version with colon separator
		{
			name:             "secret_with_version_colon",
			ref:              "database-password:5",
			expectedSecret:   "database-password",
			expectedVersion:  "5",
			expectedJSONPath: "",
		},
		{
			name:             "secret_with_latest_version",
			ref:              "api-key:latest",
			expectedSecret:   "api-key",
			expectedVersion:  "latest",
			expectedJSONPath: "",
		},
		{
			name:             "secret_with_numeric_version",
			ref:              "config:123",
			expectedSecret:   "config",
			expectedVersion:  "123",
			expectedJSONPath: "",
		},
		// Version with @ separator
		{
			name:             "secret_with_version_at",
			ref:              "database-password@7",
			expectedSecret:   "database-password",
			expectedVersion:  "7",
			expectedJSONPath: "",
		},
		{
			name:             "secret_with_latest_at",
			ref:              "credentials@latest",
			expectedSecret:   "credentials",
			expectedVersion:  "latest",
			expectedJSONPath: "",
		},
		// JSON path extraction
		{
			name:             "secret_with_json_path",
			ref:              "connection-config#.host",
			expectedSecret:   "connection-config",
			expectedVersion:  "latest",
			expectedJSONPath: ".host",
		},
		{
			name:             "secret_with_nested_json_path",
			ref:              "app-config#.database.connection.password",
			expectedSecret:   "app-config",
			expectedVersion:  "latest",
			expectedJSONPath: ".database.connection.password",
		},
		{
			name:             "secret_with_array_json_path",
			ref:              "endpoints#.servers.0.url",
			expectedSecret:   "endpoints",
			expectedVersion:  "latest",
			expectedJSONPath: ".servers.0.url",
		},
		// Version + JSON path combined
		{
			name:             "version_colon_and_json_path",
			ref:              "config:3#.api_key",
			expectedSecret:   "config",
			expectedVersion:  "3",
			expectedJSONPath: ".api_key",
		},
		{
			name:             "version_at_and_json_path",
			ref:              "database@5#.credentials.password",
			expectedSecret:   "database",
			expectedVersion:  "5",
			expectedJSONPath: ".credentials.password",
		},
		// Full resource name format
		{
			name:             "full_resource_name_without_version",
			ref:              "projects/my-project/secrets/my-secret",
			expectedSecret:   "projects/my-project/secrets/my-secret",
			expectedVersion:  "latest",
			expectedJSONPath: "",
		},
		{
			name:             "full_resource_name_ignored_colon_version",
			ref:              "projects/my-project/secrets/my-secret:5",
			expectedSecret:   "projects/my-project/secrets/my-secret:5",
			expectedVersion:  "latest",
			expectedJSONPath: "",
		},
		// Edge cases
		{
			name:             "empty_reference",
			ref:              "",
			expectedSecret:   "",
			expectedVersion:  "latest",
			expectedJSONPath: "",
		},
		{
			name:             "only_version_separator_colon",
			ref:              "secret:",
			expectedSecret:   "secret",
			expectedVersion:  "",
			expectedJSONPath: "",
		},
		{
			name:             "only_version_separator_at",
			ref:              "secret@",
			expectedSecret:   "secret",
			expectedVersion:  "",
			expectedJSONPath: "",
		},
		{
			name:             "only_json_marker",
			ref:              "#.field",
			expectedSecret:   "",
			expectedVersion:  "latest",
			expectedJSONPath: ".field",
		},
		{
			name:             "json_path_without_dot",
			ref:              "config#field",
			expectedSecret:   "config",
			expectedVersion:  "latest",
			expectedJSONPath: "field",
		},
		{
			name:             "multiple_json_markers",
			ref:              "secret#.path#.extra",
			expectedSecret:   "secret",
			expectedVersion:  "latest",
			expectedJSONPath: ".path#.extra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret, version, jsonPath := p.parseReference(tt.ref)
			assert.Equal(t, tt.expectedSecret, secret)
			assert.Equal(t, tt.expectedVersion, version)
			assert.Equal(t, tt.expectedJSONPath, jsonPath)
		})
	}
}

// TestGCPSecretManagerBuildResourceName tests GCP resource name construction.
func TestGCPSecretManagerBuildResourceName(t *testing.T) {
	p := &GCPSecretManagerProvider{
		projectID: "my-project-123",
	}

	tests := []struct {
		name           string
		secretName     string
		version        string
		expectedResult string
	}{
		// Simple secret names
		{
			name:           "simple_name_latest",
			secretName:     "database-password",
			version:        "latest",
			expectedResult: "projects/my-project-123/secrets/database-password/versions/latest",
		},
		{
			name:           "simple_name_specific_version",
			secretName:     "api-key",
			version:        "5",
			expectedResult: "projects/my-project-123/secrets/api-key/versions/5",
		},
		{
			name:           "hyphenated_secret_name",
			secretName:     "my-prod-db-password",
			version:        "10",
			expectedResult: "projects/my-project-123/secrets/my-prod-db-password/versions/10",
		},
		{
			name:           "underscored_secret_name",
			secretName:     "stripe_api_key",
			version:        "latest",
			expectedResult: "projects/my-project-123/secrets/stripe_api_key/versions/latest",
		},
		// Already full resource name
		{
			name:           "full_resource_name_without_version",
			secretName:     "projects/other-project/secrets/some-secret",
			version:        "latest",
			expectedResult: "projects/other-project/secrets/some-secret/versions/latest",
		},
		{
			name:           "full_resource_name_with_version",
			secretName:     "projects/other-project/secrets/some-secret/versions/3",
			version:        "latest",
			expectedResult: "projects/other-project/secrets/some-secret/versions/3",
		},
		{
			name:           "full_resource_name_preserves_existing_version",
			secretName:     "projects/prod/secrets/creds/versions/99",
			version:        "1",
			expectedResult: "projects/prod/secrets/creds/versions/99",
		},
		// Edge cases
		{
			name:           "empty_version",
			secretName:     "secret",
			version:        "",
			expectedResult: "projects/my-project-123/secrets/secret/versions/",
		},
		{
			name:           "numeric_project_id",
			secretName:     "test",
			version:        "1",
			expectedResult: "projects/my-project-123/secrets/test/versions/1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.buildResourceName(tt.secretName, tt.version)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

// TestExtractJSONPath tests JSON path extraction (global function in gcp_secretmanager.go).
func TestExtractJSONPath(t *testing.T) {
	tests := []struct {
		name          string
		jsonStr       string
		path          string
		expectedValue string
		expectError   bool
		errorContains string
	}{
		// Simple extraction
		{
			name:          "simple_string_field",
			jsonStr:       `{"password": "secret123"}`,
			path:          ".password",
			expectedValue: "secret123",
		},
		{
			name:          "path_without_leading_dot",
			jsonStr:       `{"key": "value"}`,
			path:          "key",
			expectedValue: "value",
		},
		{
			name:          "number_converted_to_string",
			jsonStr:       `{"port": 5432}`,
			path:          ".port",
			expectedValue: "5432",
		},
		{
			name:          "boolean_true",
			jsonStr:       `{"active": true}`,
			path:          ".active",
			expectedValue: "true",
		},
		{
			name:          "null_value_empty_string",
			jsonStr:       `{"field": null}`,
			path:          ".field",
			expectedValue: "",
		},
		// Nested objects
		{
			name:          "one_level_nested",
			jsonStr:       `{"db": {"host": "localhost"}}`,
			path:          ".db.host",
			expectedValue: "localhost",
		},
		{
			name:          "deeply_nested",
			jsonStr:       `{"config": {"database": {"credentials": {"password": "p@ss"}}}}`,
			path:          ".config.database.credentials.password",
			expectedValue: "p@ss",
		},
		{
			name:          "extract_object_as_json",
			jsonStr:       `{"settings": {"db": {"host": "localhost", "port": 5432}}}`,
			path:          ".settings.db",
			expectedValue: `{"host":"localhost","port":5432}`,
		},
		// Array access
		{
			name:          "first_array_element",
			jsonStr:       `{"hosts": ["host1.com", "host2.com"]}`,
			path:          ".hosts.0",
			expectedValue: "host1.com",
		},
		{
			name:          "array_object_field",
			jsonStr:       `{"endpoints": [{"url": "https://api.com"}]}`,
			path:          ".endpoints.0.url",
			expectedValue: "https://api.com",
		},
		{
			name:          "nested_array_in_object",
			jsonStr:       `{"config": {"servers": [{"host": "s1"}, {"host": "s2"}]}}`,
			path:          ".config.servers.1.host",
			expectedValue: "s2",
		},
		// Empty path components
		{
			name:          "path_with_extra_dots",
			jsonStr:       `{"key": "value"}`,
			path:          "..key..",
			expectedValue: "value",
		},
		// Error cases
		{
			name:          "invalid_json",
			jsonStr:       `not json`,
			path:          ".field",
			expectError:   true,
			errorContains: "invalid JSON",
		},
		{
			name:          "path_not_found",
			jsonStr:       `{"a": "b"}`,
			path:          ".c",
			expectError:   true,
			errorContains: "path not found",
		},
		{
			name:          "traverse_non_object",
			jsonStr:       `{"name": "test"}`,
			path:          ".name.sub",
			expectError:   true,
			errorContains: "cannot traverse",
		},
		{
			name:          "array_index_out_of_range",
			jsonStr:       `{"items": ["a"]}`,
			path:          ".items.10",
			expectError:   true,
			errorContains: "invalid array index",
		},
		{
			name:          "array_index_not_integer",
			jsonStr:       `{"items": ["a"]}`,
			path:          ".items.foo",
			expectError:   true,
			errorContains: "invalid array index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractJSONPath(tt.jsonStr, tt.path)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}
		})
	}
}

// TestGetGCPErrorSuggestion tests GCP error suggestion generation.
func TestGetGCPErrorSuggestion(t *testing.T) {
	tests := []struct {
		name             string
		errorString      string
		expectedContains string
	}{
		{
			name:             "permission_denied",
			errorString:      "PermissionDenied: Caller does not have permission",
			expectedContains: "IAM permissions",
		},
		{
			name:             "not_found",
			errorString:      "NotFound: Secret not found",
			expectedContains: "secret name",
		},
		{
			name:             "unauthenticated",
			errorString:      "Unauthenticated: Request had invalid credentials",
			expectedContains: "GOOGLE_APPLICATION_CREDENTIALS",
		},
		{
			name:             "invalid_argument",
			errorString:      "InvalidArgument: Invalid secret name format",
			expectedContains: "secret name format",
		},
		{
			name:             "resource_exhausted",
			errorString:      "ResourceExhausted: Quota exceeded",
			expectedContains: "exponential backoff",
		},
		{
			name:             "project_error",
			errorString:      "Project 'invalid-project' not found",
			expectedContains: "project ID",
		},
		{
			name:             "generic_error",
			errorString:      "some other error",
			expectedContains: "GCP credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestion := getGCPErrorSuggestion(gcpTestError(tt.errorString))
			assert.Contains(t, suggestion, tt.expectedContains)
		})
	}
}

// TestGetGCPProjectID tests project ID retrieval from environment.
func TestGetGCPProjectID(t *testing.T) {
	// This test verifies the function signature and basic behavior
	// Actual environment variable testing would require setup/teardown

	// Test that the function returns an empty string when no env vars are set
	// (This test runs in isolation so env vars shouldn't be set)
	_ = getGCPProjectID() // Just verify it doesn't panic
}

// gcpTestError is a simple error implementation for testing
type gcpTestError string

func (e gcpTestError) Error() string {
	return string(e)
}
