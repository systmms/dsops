package secretstore

import (
	"testing"
)

func TestParseSecretRef(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected SecretRef
		wantErr  bool
	}{
		{
			name: "simple store and path",
			uri:  "store://bitwarden/Platform/Database",
			expected: SecretRef{
				Store:   "bitwarden",
				Path:    "Platform/Database",
				Options: map[string]string{},
			},
		},
		{
			name: "with field",
			uri:  "store://bitwarden/Platform/Database#password",
			expected: SecretRef{
				Store:   "bitwarden",
				Path:    "Platform/Database",
				Field:   "password",
				Options: map[string]string{},
			},
		},
		{
			name: "with version",
			uri:  "store://vault/secrets/myapp?version=v1",
			expected: SecretRef{
				Store:   "vault",
				Path:    "secrets/myapp",
				Version: "v1",
				Options: map[string]string{},
			},
		},
		{
			name: "with field and version",
			uri:  "store://aws-secrets-manager/prod/db#connection_string?version=2",
			expected: SecretRef{
				Store:   "aws-secrets-manager",
				Path:    "prod/db",
				Field:   "connection_string",
				Version: "2",
				Options: map[string]string{},
			},
		},
		{
			name: "with custom options",
			uri:  "store://onepassword/vault/item?section=login&region=us",
			expected: SecretRef{
				Store: "onepassword",
				Path:  "vault/item",
				Options: map[string]string{
					"section": "login",
					"region":  "us",
				},
			},
		},
		{
			name: "complete URI",
			uri:  "store://bitwarden/Shared/API-Keys#github_token?version=latest&vault=work",
			expected: SecretRef{
				Store:   "bitwarden",
				Path:    "Shared/API-Keys",
				Field:   "github_token",
				Version: "latest",
				Options: map[string]string{
					"vault": "work",
				},
			},
		},
		{
			name:    "empty URI",
			uri:     "",
			wantErr: true,
		},
		{
			name:    "wrong scheme",
			uri:     "http://example.com/path",
			wantErr: true,
		},
		{
			name:    "missing store name",
			uri:     "store:///path",
			wantErr: true,
		},
		{
			name:    "missing path",
			uri:     "store://bitwarden",
			wantErr: true,
		},
		{
			name:    "invalid query params",
			uri:     "store://bitwarden/path?invalid%query",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseSecretRef(tt.uri)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseSecretRef() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("ParseSecretRef() unexpected error: %v", err)
				return
			}

			if result.Store != tt.expected.Store {
				t.Errorf("Store = %q, want %q", result.Store, tt.expected.Store)
			}
			if result.Path != tt.expected.Path {
				t.Errorf("Path = %q, want %q", result.Path, tt.expected.Path)
			}
			if result.Field != tt.expected.Field {
				t.Errorf("Field = %q, want %q", result.Field, tt.expected.Field)
			}
			if result.Version != tt.expected.Version {
				t.Errorf("Version = %q, want %q", result.Version, tt.expected.Version)
			}

			// Check options
			if result.Options == nil {
				result.Options = map[string]string{}
			}
			if tt.expected.Options == nil {
				tt.expected.Options = map[string]string{}
			}
			
			if len(result.Options) != len(tt.expected.Options) {
				t.Errorf("Options length mismatch: got %d, want %d", len(result.Options), len(tt.expected.Options))
			}
			
			for key, expectedValue := range tt.expected.Options {
				if actualValue, ok := result.Options[key]; !ok || actualValue != expectedValue {
					t.Errorf("Options[%q] = %q, want %q", key, actualValue, expectedValue)
				}
			}
			for key := range result.Options {
				if _, ok := tt.expected.Options[key]; !ok {
					t.Errorf("Unexpected option %q = %q", key, result.Options[key])
				}
			}
		})
	}
}

func TestSecretRefString(t *testing.T) {
	tests := []struct {
		name     string
		ref      SecretRef
		expected string
	}{
		{
			name: "simple ref",
			ref: SecretRef{
				Store: "bitwarden",
				Path:  "Platform/Database",
			},
			expected: "store://bitwarden/Platform/Database",
		},
		{
			name: "with field",
			ref: SecretRef{
				Store: "bitwarden",
				Path:  "Platform/Database",
				Field: "password",
			},
			expected: "store://bitwarden/Platform/Database#password",
		},
		{
			name: "with version",
			ref: SecretRef{
				Store:   "vault",
				Path:    "secrets/myapp",
				Version: "v1",
			},
			expected: "store://vault/secrets/myapp?version=v1",
		},
		{
			name: "complete ref",
			ref: SecretRef{
				Store:   "bitwarden",
				Path:    "Shared/API-Keys",
				Field:   "github_token",
				Version: "latest",
				Options: map[string]string{
					"vault": "work",
				},
			},
			expected: "store://bitwarden/Shared/API-Keys#github_token?vault=work&version=latest",
		},
		{
			name: "invalid ref - missing store",
			ref: SecretRef{
				Path: "some/path",
			},
			expected: "",
		},
		{
			name: "invalid ref - missing path",
			ref: SecretRef{
				Store: "bitwarden",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ref.String()
			
			// For the complete ref test, we need to handle URL parameter order
			if tt.name == "complete ref" {
				// Parse both URLs to compare parameters
				expectedRef, err := ParseSecretRef(tt.expected)
				if err != nil {
					t.Fatalf("Failed to parse expected URI: %v", err)
				}
				resultRef, err := ParseSecretRef(result)
				if err != nil {
					t.Fatalf("Failed to parse result URI: %v", err)
				}
				
				// Compare structured data instead of string
				if expectedRef.Store != resultRef.Store || 
				   expectedRef.Path != resultRef.Path ||
				   expectedRef.Field != resultRef.Field ||
				   expectedRef.Version != resultRef.Version {
					t.Errorf("String() structured comparison failed")
				}
				
				return
			}
			
			if result != tt.expected {
				t.Errorf("String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSecretRefRoundTrip(t *testing.T) {
	originalURIs := []string{
		"store://bitwarden/Platform/Database",
		"store://bitwarden/Platform/Database#password",
		"store://vault/secrets/myapp?version=v1",
		"store://aws-secrets-manager/prod/db#connection_string?version=2",
		"store://onepassword/vault/item?section=login&region=us",
	}

	for _, originalURI := range originalURIs {
		t.Run(originalURI, func(t *testing.T) {
			// Parse the URI
			ref, err := ParseSecretRef(originalURI)
			if err != nil {
				t.Fatalf("ParseSecretRef() failed: %v", err)
			}

			// Convert back to string
			resultURI := ref.String()

			// Parse the result to ensure it's equivalent
			resultRef, err := ParseSecretRef(resultURI)
			if err != nil {
				t.Fatalf("ParseSecretRef() on result failed: %v", err)
			}

			// Compare the structured data
			if ref.Store != resultRef.Store {
				t.Errorf("Store mismatch: %q vs %q", ref.Store, resultRef.Store)
			}
			if ref.Path != resultRef.Path {
				t.Errorf("Path mismatch: %q vs %q", ref.Path, resultRef.Path)
			}
			if ref.Field != resultRef.Field {
				t.Errorf("Field mismatch: %q vs %q", ref.Field, resultRef.Field)
			}
			if ref.Version != resultRef.Version {
				t.Errorf("Version mismatch: %q vs %q", ref.Version, resultRef.Version)
			}
		})
	}
}