package providers

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBitwardenStatusParsing tests BitwardenStatus JSON parsing for various status scenarios.
func TestBitwardenStatusParsing(t *testing.T) {
	tests := []struct {
		name           string
		jsonResponse   string
		expectedStatus string
		expectError    bool
	}{
		{
			name:           "unlocked vault status",
			jsonResponse:   `{"serverUrl":"https://vault.bitwarden.com","lastSync":"2024-01-15T10:30:00.000Z","userEmail":"user@example.com","userId":"12345","status":"unlocked"}`,
			expectedStatus: "unlocked",
		},
		{
			name:           "locked vault status",
			jsonResponse:   `{"serverUrl":"https://vault.bitwarden.com","lastSync":null,"userEmail":"user@example.com","userId":"12345","status":"locked"}`,
			expectedStatus: "locked",
		},
		{
			name:           "unauthenticated status",
			jsonResponse:   `{"serverUrl":null,"lastSync":null,"userEmail":null,"userId":null,"status":"unauthenticated"}`,
			expectedStatus: "unauthenticated",
		},
		{
			name:           "minimal response with just status",
			jsonResponse:   `{"status":"unlocked"}`,
			expectedStatus: "unlocked",
		},
		{
			name:           "status with extra fields",
			jsonResponse:   `{"status":"unlocked","customField":"ignored"}`,
			expectedStatus: "unlocked",
		},
		{
			name:         "invalid JSON",
			jsonResponse: `{invalid json}`,
			expectError:  true,
		},
		{
			name:         "empty JSON object",
			jsonResponse: `{}`,
			expectedStatus: "",
		},
		{
			name:         "null status",
			jsonResponse: `{"status":null}`,
			expectedStatus: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var status BitwardenStatus
			err := json.Unmarshal([]byte(tt.jsonResponse), &status)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedStatus, status.Status)
			}
		})
	}
}

// TestBitwardenItemJSONParsing tests parsing of Bitwarden item responses.
func TestBitwardenItemJSONParsing(t *testing.T) {
	tests := []struct {
		name         string
		jsonResponse string
		expectedItem *BitwardenItem
		expectError  bool
	}{
		{
			name: "complete login item",
			jsonResponse: `{
				"id": "550e8400-e29b-41d4-a716-446655440000",
				"name": "My Login",
				"type": 1,
				"login": {
					"username": "admin",
					"password": "secret123",
					"totp": "JBSWY3DPEHPK3PXP",
					"uris": [
						{"uri": "https://example.com"},
						{"uri": "https://api.example.com"}
					]
				},
				"notes": "Important notes here",
				"fields": [
					{"name": "api_key", "value": "key-123"},
					{"name": "client_secret", "value": "secret-456"}
				],
				"revisionDate": "2024-01-15T10:30:00.000Z"
			}`,
			expectedItem: &BitwardenItem{
				ID:   "550e8400-e29b-41d4-a716-446655440000",
				Name: "My Login",
				Login: &BitwardenLogin{
					Username: "admin",
					Password: "secret123",
					Totp:     "JBSWY3DPEHPK3PXP",
					Uris: []BitwardenUri{
						{URI: "https://example.com"},
						{URI: "https://api.example.com"},
					},
				},
				Notes: "Important notes here",
				Fields: []BitwardenField{
					{Name: "api_key", Value: "key-123"},
					{Name: "client_secret", Value: "secret-456"},
				},
				RevisionDate: "2024-01-15T10:30:00.000Z",
			},
		},
		{
			name: "item without login (secure note)",
			jsonResponse: `{
				"id": "secure-note-123",
				"name": "My Secure Note",
				"type": 2,
				"notes": "This is a secure note content"
			}`,
			expectedItem: &BitwardenItem{
				ID:    "secure-note-123",
				Name:  "My Secure Note",
				Notes: "This is a secure note content",
			},
		},
		{
			name: "item with empty fields",
			jsonResponse: `{
				"id": "item-456",
				"name": "Empty Item",
				"login": {
					"username": "",
					"password": "",
					"uris": []
				},
				"fields": []
			}`,
			expectedItem: &BitwardenItem{
				ID:   "item-456",
				Name: "Empty Item",
				Login: &BitwardenLogin{
					Username: "",
					Password: "",
					Uris:     []BitwardenUri{},
				},
				Fields: []BitwardenField{},
			},
		},
		{
			name: "item with unicode values",
			jsonResponse: `{
				"id": "unicode-item",
				"name": "ユーザー名",
				"login": {
					"username": "用户@example.com",
					"password": "пароль123"
				}
			}`,
			expectedItem: &BitwardenItem{
				ID:   "unicode-item",
				Name: "ユーザー名",
				Login: &BitwardenLogin{
					Username: "用户@example.com",
					Password: "пароль123",
				},
			},
		},
		{
			name: "item with special characters in password",
			jsonResponse: `{
				"id": "special-chars",
				"name": "Special Password",
				"login": {
					"password": "p@$$w0rd!#$%^&*(){}[]|\\:\";<>,.?/~` + "`" + `"
				}
			}`,
			expectedItem: &BitwardenItem{
				ID:   "special-chars",
				Name: "Special Password",
				Login: &BitwardenLogin{
					Password: "p@$$w0rd!#$%^&*(){}[]|\\:\";<>,.?/~`",
				},
			},
		},
		{
			name:         "invalid JSON response",
			jsonResponse: `{broken json`,
			expectError:  true,
		},
		{
			name:         "empty JSON object",
			jsonResponse: `{}`,
			expectedItem: &BitwardenItem{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var item BitwardenItem
			err := json.Unmarshal([]byte(tt.jsonResponse), &item)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedItem.ID, item.ID)
				assert.Equal(t, tt.expectedItem.Name, item.Name)
				assert.Equal(t, tt.expectedItem.Notes, item.Notes)

				if tt.expectedItem.Login != nil {
					require.NotNil(t, item.Login)
					assert.Equal(t, tt.expectedItem.Login.Username, item.Login.Username)
					assert.Equal(t, tt.expectedItem.Login.Password, item.Login.Password)
					assert.Equal(t, tt.expectedItem.Login.Totp, item.Login.Totp)
					assert.Equal(t, len(tt.expectedItem.Login.Uris), len(item.Login.Uris))
				}

				assert.Equal(t, len(tt.expectedItem.Fields), len(item.Fields))
			}
		})
	}
}

// TestBitwardenProviderStatusValidation tests the status validation logic.
// This tests the logic without requiring actual CLI execution.
func TestBitwardenProviderStatusValidation(t *testing.T) {
	tests := []struct {
		name          string
		status        BitwardenStatus
		expectError   bool
		errorContains string
	}{
		{
			name:        "unlocked status is valid",
			status:      BitwardenStatus{Status: "unlocked"},
			expectError: false,
		},
		{
			name:          "locked status requires unlock",
			status:        BitwardenStatus{Status: "locked"},
			expectError:   true,
			errorContains: "locked",
		},
		{
			name:          "unauthenticated requires login",
			status:        BitwardenStatus{Status: "unauthenticated"},
			expectError:   true,
			errorContains: "not logged in",
		},
		{
			name:          "unknown status",
			status:        BitwardenStatus{Status: "unknown-state"},
			expectError:   true,
			errorContains: "unknown status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBitwardenStatus(&tt.status, "test-provider")

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

// validateBitwardenStatus is a helper function that implements the validation logic
// extracted from the provider's Validate method for testing purposes.
func validateBitwardenStatus(status *BitwardenStatus, providerName string) error {
	switch status.Status {
	case "unlocked":
		return nil
	case "locked":
		return fmt.Errorf("vault is locked. Run: bw unlock")
	case "unauthenticated":
		return fmt.Errorf("not logged in. Run: bw login")
	default:
		return fmt.Errorf("unknown status: %s", status.Status)
	}
}
