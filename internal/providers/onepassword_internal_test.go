package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOnePasswordParseKey tests the parseKey function for various 1Password key formats.
func TestOnePasswordParseKey(t *testing.T) {
	op := &OnePasswordProvider{}

	tests := []struct {
		name          string
		key           string
		expectedItem  string
		expectedField string
	}{
		// op:// URI format
		{
			name:          "op:// format with vault/item/field",
			key:           "op://vault/item/password",
			expectedItem:  "vault/item",
			expectedField: "password",
		},
		{
			name:          "op:// format with vault/item defaults to password",
			key:           "op://vault/item",
			expectedItem:  "vault/item",
			expectedField: "password",
		},
		{
			name:          "op:// format with complex vault name",
			key:           "op://my-vault-123/login-item/api_key",
			expectedItem:  "my-vault-123/login-item",
			expectedField: "api_key",
		},
		{
			name:          "op:// format with username field",
			key:           "op://Personal/GitHub/username",
			expectedItem:  "Personal/GitHub",
			expectedField: "username",
		},
		{
			name:          "op:// format with notes field",
			key:           "op://Work/Credentials/notes",
			expectedItem:  "Work/Credentials",
			expectedField: "notes",
		},
		// Dot notation
		{
			name:          "dot notation with password field",
			key:           "item-name.password",
			expectedItem:  "item-name",
			expectedField: "password",
		},
		{
			name:          "dot notation with username field",
			key:           "my-login.username",
			expectedItem:  "my-login",
			expectedField: "username",
		},
		{
			name:          "dot notation with custom field",
			key:           "api-service.api_key",
			expectedItem:  "api-service",
			expectedField: "api_key",
		},
		{
			name:          "dot notation with spaces in field name",
			key:           "service.one time password",
			expectedItem:  "service",
			expectedField: "one time password",
		},
		// Simple item name (defaults to password)
		{
			name:          "simple item name defaults to password",
			key:           "my-login-item",
			expectedItem:  "my-login-item",
			expectedField: "password",
		},
		{
			name:          "item name with hyphens",
			key:           "prod-database-creds",
			expectedItem:  "prod-database-creds",
			expectedField: "password",
		},
		{
			name:          "item name with underscores",
			key:           "my_item_name",
			expectedItem:  "my_item_name",
			expectedField: "password",
		},
		{
			name:          "empty key",
			key:           "",
			expectedItem:  "",
			expectedField: "password",
		},
		// Edge cases
		{
			name:          "key with multiple dots uses first as separator",
			key:           "item.field.extra",
			expectedItem:  "item",
			expectedField: "field.extra",
		},
		{
			name:          "key ending with dot",
			key:           "item.",
			expectedItem:  "item",
			expectedField: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, field := op.parseKey(tt.key)
			assert.Equal(t, tt.expectedItem, item)
			assert.Equal(t, tt.expectedField, field)
		})
	}
}

// TestOnePasswordExtractField tests field extraction from 1Password items.
func TestOnePasswordExtractField(t *testing.T) {
	op := &OnePasswordProvider{}

	// Create a comprehensive test item
	testItem := &OnePasswordItem{
		ID:       "item-123",
		Title:    "Test Login",
		Category: "LOGIN",
		Notes:    "These are the notes for this item",
		Tags:     []string{"work", "important"},
		Vault: struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}{
			ID:   "vault-456",
			Name: "Personal",
		},
		Fields: []OnePasswordField{
			{ID: "username", Type: "TEXT", Label: "username", Value: "testuser@example.com"},
			{ID: "password", Type: "CONCEALED", Label: "password", Value: "super-secret-123"},
			{ID: "api_key", Type: "TEXT", Label: "api_key", Value: "sk-live-abcd1234"},
			{ID: "client_id", Type: "TEXT", Label: "Client ID", Value: "client-xyz"},
			{ID: "totp", Type: "OTP", Label: "one-time password", Value: "123456"},
		},
		URLs: []OnePasswordURL{
			{Label: "website", Primary: true, Href: "https://example.com"},
			{Label: "api", Primary: false, Href: "https://api.example.com"},
		},
	}

	tests := []struct {
		name          string
		item          *OnePasswordItem
		fieldName     string
		expectedValue string
		expectError   bool
		errorContains string
	}{
		// Direct field lookup by label
		{
			name:          "extract username by label",
			item:          testItem,
			fieldName:     "username",
			expectedValue: "testuser@example.com",
		},
		{
			name:          "extract password by label",
			item:          testItem,
			fieldName:     "password",
			expectedValue: "super-secret-123",
		},
		{
			name:          "extract api_key by label",
			item:          testItem,
			fieldName:     "api_key",
			expectedValue: "sk-live-abcd1234",
		},
		{
			name:          "extract by ID",
			item:          testItem,
			fieldName:     "client_id",
			expectedValue: "client-xyz",
		},
		{
			name:          "extract by display label",
			item:          testItem,
			fieldName:     "Client ID",
			expectedValue: "client-xyz",
		},
		// Special field names
		{
			name:          "extract url field",
			item:          testItem,
			fieldName:     "url",
			expectedValue: "https://example.com",
		},
		{
			name:          "extract website field",
			item:          testItem,
			fieldName:     "website",
			expectedValue: "https://example.com",
		},
		{
			name:          "extract notes field",
			item:          testItem,
			fieldName:     "notes",
			expectedValue: "These are the notes for this item",
		},
		{
			name:          "extract title field",
			item:          testItem,
			fieldName:     "title",
			expectedValue: "Test Login",
		},
		{
			name:          "extract name field",
			item:          testItem,
			fieldName:     "name",
			expectedValue: "Test Login",
		},
		// Case insensitivity for special fields
		{
			name:          "PASSWORD uppercase",
			item:          testItem,
			fieldName:     "PASSWORD",
			expectedValue: "super-secret-123",
		},
		{
			name:          "USERNAME uppercase",
			item:          testItem,
			fieldName:     "USERNAME",
			expectedValue: "testuser@example.com",
		},
		{
			name:          "NOTES uppercase",
			item:          testItem,
			fieldName:     "NOTES",
			expectedValue: "These are the notes for this item",
		},
		// Error cases
		{
			name:          "non-existent field",
			item:          testItem,
			fieldName:     "nonexistent",
			expectError:   true,
			errorContains: "field 'nonexistent' not found",
		},
		{
			name: "no password field in item",
			item: &OnePasswordItem{
				Title:  "No Password",
				Fields: []OnePasswordField{},
			},
			fieldName:     "password",
			expectError:   true,
			errorContains: "field 'password' not found",
		},
		{
			name: "no username field in item",
			item: &OnePasswordItem{
				Title:  "No Username",
				Fields: []OnePasswordField{},
			},
			fieldName:     "username",
			expectError:   true,
			errorContains: "field 'username' not found",
		},
		{
			name: "no URL in item",
			item: &OnePasswordItem{
				Title: "No URL",
				URLs:  []OnePasswordURL{},
			},
			fieldName:     "url",
			expectError:   true,
			errorContains: "field 'url' not found",
		},
		{
			name: "empty notes returns empty string",
			item: &OnePasswordItem{
				Title: "Empty Notes",
				Notes: "",
			},
			fieldName:     "notes",
			expectedValue: "",
		},
		// Email as username fallback
		{
			name: "email field used as username",
			item: &OnePasswordItem{
				Fields: []OnePasswordField{
					{ID: "email", Type: "TEXT", Label: "email", Value: "user@domain.com"},
				},
			},
			fieldName:     "username",
			expectedValue: "user@domain.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := op.extractField(tt.item, tt.fieldName)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedValue, value)
			}
		})
	}
}
