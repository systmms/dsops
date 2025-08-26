package service

import (
	"testing"
)

func TestParseServiceRef(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected ServiceRef
		wantErr  bool
	}{
		{
			name: "basic service reference",
			uri:  "svc://github/acme-org?kind=pat",
			expected: ServiceRef{
				Type:     "github",
				Instance: "acme-org",
				Kind:     "pat",
				Options:  map[string]string{},
			},
		},
		{
			name: "with principal",
			uri:  "svc://postgres/prod-db?kind=password&principal=app-user",
			expected: ServiceRef{
				Type:      "postgres",
				Instance:  "prod-db",
				Kind:      "password",
				Principal: "app-user",
				Options:   map[string]string{},
			},
		},
		{
			name: "with custom options",
			uri:  "svc://stripe/main-account?kind=api-key&environment=live&region=us",
			expected: ServiceRef{
				Type:     "stripe",
				Instance: "main-account",
				Kind:     "api-key",
				Options: map[string]string{
					"environment": "live",
					"region":      "us",
				},
			},
		},
		{
			name: "complex service instance path",
			uri:  "svc://postgres/prod/app-db?kind=db_password&principal=ci-bot",
			expected: ServiceRef{
				Type:      "postgres",
				Instance:  "prod/app-db",
				Kind:      "db_password",
				Principal: "ci-bot",
				Options:   map[string]string{},
			},
		},
		{
			name: "complete reference",
			uri:  "svc://aws/iam-user?kind=access_key&principal=deploy-bot&account=123456789&region=us-east-1",
			expected: ServiceRef{
				Type:      "aws",
				Instance:  "iam-user",
				Kind:      "access_key",
				Principal: "deploy-bot",
				Options: map[string]string{
					"account": "123456789",
					"region":  "us-east-1",
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
			uri:     "http://github.com/org",
			wantErr: true,
		},
		{
			name:    "missing instance",
			uri:     "svc://github?kind=pat",
			wantErr: true,
		},
		{
			name:    "missing kind",
			uri:     "svc://github/acme-org",
			wantErr: true,
		},
		{
			name:    "empty type",
			uri:     "svc:///acme-org?kind=pat",
			wantErr: true,
		},
		{
			name:    "empty instance",
			uri:     "svc://github/?kind=pat",
			wantErr: true,
		},
		{
			name:    "invalid query params",
			uri:     "svc://github/acme-org?kind=pat&invalid%query",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseServiceRef(tt.uri)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseServiceRef() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("ParseServiceRef() unexpected error: %v", err)
				return
			}

			if result.Type != tt.expected.Type {
				t.Errorf("Type = %q, want %q", result.Type, tt.expected.Type)
			}
			if result.Instance != tt.expected.Instance {
				t.Errorf("Instance = %q, want %q", result.Instance, tt.expected.Instance)
			}
			if result.Kind != tt.expected.Kind {
				t.Errorf("Kind = %q, want %q", result.Kind, tt.expected.Kind)
			}
			if result.Principal != tt.expected.Principal {
				t.Errorf("Principal = %q, want %q", result.Principal, tt.expected.Principal)
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

func TestServiceRefString(t *testing.T) {
	tests := []struct {
		name string
		ref  ServiceRef
		want string
	}{
		{
			name: "basic service reference",
			ref: ServiceRef{
				Type:     "github",
				Instance: "acme-org",
				Kind:     "pat",
			},
			want: "svc://github/acme-org?kind=pat",
		},
		{
			name: "with principal",
			ref: ServiceRef{
				Type:      "postgres",
				Instance:  "prod-db",
				Kind:      "password",
				Principal: "app-user",
			},
			want: "svc://postgres/prod-db?kind=password&principal=app-user",
		},
		{
			name: "complex instance path",
			ref: ServiceRef{
				Type:     "postgres",
				Instance: "prod/app-db",
				Kind:     "db_password",
			},
			want: "svc://postgres/prod/app-db?kind=db_password",
		},
		{
			name: "invalid ref - missing type",
			ref: ServiceRef{
				Instance: "acme-org",
				Kind:     "pat",
			},
			want: "",
		},
		{
			name: "invalid ref - missing instance",
			ref: ServiceRef{
				Type: "github",
				Kind: "pat",
			},
			want: "",
		},
		{
			name: "invalid ref - missing kind",
			ref: ServiceRef{
				Type:     "github",
				Instance: "acme-org",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ref.String()

			// For refs with options, we need to handle URL parameter order
			if len(tt.ref.Options) > 0 {
				// Parse both to compare structured data
				expectedRef, err := ParseServiceRef(tt.want)
				if err != nil {
					t.Fatalf("Failed to parse expected URI: %v", err)
				}
				resultRef, err := ParseServiceRef(result)
				if err != nil {
					t.Fatalf("Failed to parse result URI: %v", err)
				}

				if expectedRef.Type != resultRef.Type ||
					expectedRef.Instance != resultRef.Instance ||
					expectedRef.Kind != resultRef.Kind ||
					expectedRef.Principal != resultRef.Principal {
					t.Errorf("String() structured comparison failed")
				}
				return
			}

			if result != tt.want {
				t.Errorf("String() = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestServiceRefRoundTrip(t *testing.T) {
	originalURIs := []string{
		"svc://github/acme-org?kind=pat",
		"svc://postgres/prod-db?kind=password&principal=app-user",
		"svc://stripe/main-account?kind=api-key",
		"svc://postgres/prod/app-db?kind=db_password&principal=ci-bot",
	}

	for _, originalURI := range originalURIs {
		t.Run(originalURI, func(t *testing.T) {
			// Parse the URI
			ref, err := ParseServiceRef(originalURI)
			if err != nil {
				t.Fatalf("ParseServiceRef() failed: %v", err)
			}

			// Convert back to string
			resultURI := ref.String()

			// Parse the result to ensure it's equivalent
			resultRef, err := ParseServiceRef(resultURI)
			if err != nil {
				t.Fatalf("ParseServiceRef() on result failed: %v", err)
			}

			// Compare the structured data
			if ref.Type != resultRef.Type {
				t.Errorf("Type mismatch: %q vs %q", ref.Type, resultRef.Type)
			}
			if ref.Instance != resultRef.Instance {
				t.Errorf("Instance mismatch: %q vs %q", ref.Instance, resultRef.Instance)
			}
			if ref.Kind != resultRef.Kind {
				t.Errorf("Kind mismatch: %q vs %q", ref.Kind, resultRef.Kind)
			}
			if ref.Principal != resultRef.Principal {
				t.Errorf("Principal mismatch: %q vs %q", ref.Principal, resultRef.Principal)
			}
		})
	}
}

func TestServiceRefIsValid(t *testing.T) {
	tests := []struct {
		name string
		ref  ServiceRef
		want bool
	}{
		{
			name: "valid ref",
			ref: ServiceRef{
				Type:     "github",
				Instance: "acme-org",
				Kind:     "pat",
			},
			want: true,
		},
		{
			name: "missing type",
			ref: ServiceRef{
				Instance: "acme-org",
				Kind:     "pat",
			},
			want: false,
		},
		{
			name: "missing instance",
			ref: ServiceRef{
				Type: "github",
				Kind: "pat",
			},
			want: false,
		},
		{
			name: "missing kind",
			ref: ServiceRef{
				Type:     "github",
				Instance: "acme-org",
			},
			want: false,
		},
		{
			name: "empty ref",
			ref:  ServiceRef{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.IsValid(); got != tt.want {
				t.Errorf("ServiceRef.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}