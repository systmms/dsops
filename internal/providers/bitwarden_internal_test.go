package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBitwardenParseKey tests the parseKey function for various input formats.
func TestBitwardenParseKey(t *testing.T) {
	bw := &BitwardenProvider{name: "test"}

	tests := []struct {
		name          string
		key           string
		expectedItem  string
		expectedField string
	}{
		{
			name:          "simple item ID returns password",
			key:           "abc123",
			expectedItem:  "abc123",
			expectedField: "password",
		},
		{
			name:          "item ID with password field",
			key:           "abc123.password",
			expectedItem:  "abc123",
			expectedField: "password",
		},
		{
			name:          "item ID with username field",
			key:           "abc123.username",
			expectedItem:  "abc123",
			expectedField: "username",
		},
		{
			name:          "item ID with totp field",
			key:           "abc123.totp",
			expectedItem:  "abc123",
			expectedField: "totp",
		},
		{
			name:          "item ID with notes field",
			key:           "abc123.notes",
			expectedItem:  "abc123",
			expectedField: "notes",
		},
		{
			name:          "item ID with custom field",
			key:           "abc123.api_key",
			expectedItem:  "abc123",
			expectedField: "api_key",
		},
		{
			name:          "item name with spaces encoded",
			key:           "my-login-item.password",
			expectedItem:  "my-login-item",
			expectedField: "password",
		},
		{
			name:          "GUID format item ID",
			key:           "550e8400-e29b-41d4-a716-446655440000.password",
			expectedItem:  "550e8400-e29b-41d4-a716-446655440000",
			expectedField: "password",
		},
		{
			name:          "item with uri field",
			key:           "item123.uri",
			expectedItem:  "item123",
			expectedField: "uri",
		},
		{
			name:          "item with indexed uri field",
			key:           "item123.uri0",
			expectedItem:  "item123",
			expectedField: "uri0",
		},
		{
			name:          "item with second uri",
			key:           "item123.uri1",
			expectedItem:  "item123",
			expectedField: "uri1",
		},
		{
			name:          "item name with hyphens",
			key:           "my-test-item",
			expectedItem:  "my-test-item",
			expectedField: "password",
		},
		{
			name:          "item name with underscores",
			key:           "my_test_item.username",
			expectedItem:  "my_test_item",
			expectedField: "username",
		},
		{
			name:          "empty key defaults to password",
			key:           "",
			expectedItem:  "",
			expectedField: "password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			itemID, field := bw.parseKey(tt.key)
			assert.Equal(t, tt.expectedItem, itemID)
			assert.Equal(t, tt.expectedField, field)
		})
	}
}

// TestBitwardenExtractField tests field extraction from Bitwarden items.
func TestBitwardenExtractField(t *testing.T) {
	bw := &BitwardenProvider{name: "test"}

	// Create a comprehensive test item
	testItem := &BitwardenItem{
		ID:   "test-id-123",
		Name: "Test Item",
		Login: &BitwardenLogin{
			Username: "testuser",
			Password: "secretpass123",
			Totp:     "JBSWY3DPEHPK3PXP",
			Uris: []BitwardenUri{
				{URI: "https://example.com"},
				{URI: "https://api.example.com"},
				{URI: "https://admin.example.com"},
			},
		},
		Notes: "These are test notes",
		Fields: []BitwardenField{
			{Name: "api_key", Value: "sk-live-123456"},
			{Name: "client_id", Value: "client-abc"},
			{Name: "custom_field", Value: "custom-value"},
		},
	}

	tests := []struct {
		name          string
		item          *BitwardenItem
		field         string
		expectedValue string
		expectError   bool
		errorContains string
	}{
		{
			name:          "extract password",
			item:          testItem,
			field:         "password",
			expectedValue: "secretpass123",
		},
		{
			name:          "extract username",
			item:          testItem,
			field:         "username",
			expectedValue: "testuser",
		},
		{
			name:          "extract totp",
			item:          testItem,
			field:         "totp",
			expectedValue: "JBSWY3DPEHPK3PXP",
		},
		{
			name:          "extract notes",
			item:          testItem,
			field:         "notes",
			expectedValue: "These are test notes",
		},
		{
			name:          "extract name",
			item:          testItem,
			field:         "name",
			expectedValue: "Test Item",
		},
		{
			name:          "extract custom field api_key",
			item:          testItem,
			field:         "api_key",
			expectedValue: "sk-live-123456",
		},
		{
			name:          "extract custom field client_id",
			item:          testItem,
			field:         "client_id",
			expectedValue: "client-abc",
		},
		{
			name:          "extract custom field custom_field",
			item:          testItem,
			field:         "custom_field",
			expectedValue: "custom-value",
		},
		{
			name:          "extract uri (defaults to uri0)",
			item:          testItem,
			field:         "uri",
			expectedValue: "https://example.com",
		},
		{
			name:          "extract uri0 explicitly",
			item:          testItem,
			field:         "uri0",
			expectedValue: "https://example.com",
		},
		{
			name:          "extract uri1",
			item:          testItem,
			field:         "uri1",
			expectedValue: "https://api.example.com",
		},
		{
			name:          "extract uri2",
			item:          testItem,
			field:         "uri2",
			expectedValue: "https://admin.example.com",
		},
		{
			name:          "missing password field",
			item:          &BitwardenItem{Name: "no-login"},
			field:         "password",
			expectError:   true,
			errorContains: "no password field",
		},
		{
			name: "empty password field",
			item: &BitwardenItem{
				Name:  "empty-pass",
				Login: &BitwardenLogin{Password: ""},
			},
			field:         "password",
			expectError:   true,
			errorContains: "no password field",
		},
		{
			name:          "missing username field",
			item:          &BitwardenItem{Name: "no-login"},
			field:         "username",
			expectError:   true,
			errorContains: "no username field",
		},
		{
			name: "empty username field",
			item: &BitwardenItem{
				Name:  "empty-user",
				Login: &BitwardenLogin{Username: ""},
			},
			field:         "username",
			expectError:   true,
			errorContains: "no username field",
		},
		{
			name:          "missing totp field",
			item:          &BitwardenItem{Name: "no-totp", Login: &BitwardenLogin{}},
			field:         "totp",
			expectError:   true,
			errorContains: "no TOTP field",
		},
		{
			name:          "missing notes field",
			item:          &BitwardenItem{Name: "no-notes", Notes: ""},
			field:         "notes",
			expectError:   true,
			errorContains: "no notes field",
		},
		{
			name:          "non-existent custom field",
			item:          testItem,
			field:         "nonexistent",
			expectError:   true,
			errorContains: "field 'nonexistent' not found",
		},
		{
			name: "uri on item without login",
			item: &BitwardenItem{Name: "no-login"},
			field:         "uri0",
			expectError:   true,
			errorContains: "field 'uri0' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := bw.extractField(tt.item, tt.field)

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

// TestBitwardenExtractUriField tests URI field extraction with indexing.
func TestBitwardenExtractUriField(t *testing.T) {
	bw := &BitwardenProvider{name: "test"}

	tests := []struct {
		name          string
		item          *BitwardenItem
		field         string
		expectedValue string
		expectError   bool
		errorContains string
	}{
		{
			name: "extract first uri with 'uri'",
			item: &BitwardenItem{
				Login: &BitwardenLogin{
					Uris: []BitwardenUri{{URI: "https://first.com"}},
				},
			},
			field:         "uri",
			expectedValue: "https://first.com",
		},
		{
			name: "extract uri0",
			item: &BitwardenItem{
				Login: &BitwardenLogin{
					Uris: []BitwardenUri{
						{URI: "https://first.com"},
						{URI: "https://second.com"},
					},
				},
			},
			field:         "uri0",
			expectedValue: "https://first.com",
		},
		{
			name: "extract uri1",
			item: &BitwardenItem{
				Login: &BitwardenLogin{
					Uris: []BitwardenUri{
						{URI: "https://first.com"},
						{URI: "https://second.com"},
					},
				},
			},
			field:         "uri1",
			expectedValue: "https://second.com",
		},
		{
			name: "extract uri with high index",
			item: &BitwardenItem{
				Login: &BitwardenLogin{
					Uris: []BitwardenUri{
						{URI: "https://0.com"},
						{URI: "https://1.com"},
						{URI: "https://2.com"},
						{URI: "https://3.com"},
						{URI: "https://4.com"},
					},
				},
			},
			field:         "uri4",
			expectedValue: "https://4.com",
		},
		{
			name: "index out of range",
			item: &BitwardenItem{
				Login: &BitwardenLogin{
					Uris: []BitwardenUri{{URI: "https://only.com"}},
				},
			},
			field:         "uri1",
			expectError:   true,
			errorContains: "URI index 1 not found",
		},
		{
			name: "high index out of range",
			item: &BitwardenItem{
				Login: &BitwardenLogin{
					Uris: []BitwardenUri{{URI: "https://only.com"}},
				},
			},
			field:         "uri99",
			expectError:   true,
			errorContains: "URI index 99 not found",
		},
		{
			name:          "no login object",
			item:          &BitwardenItem{Name: "no-login"},
			field:         "uri0",
			expectError:   true,
			errorContains: "no URI fields found",
		},
		{
			name: "empty uris array",
			item: &BitwardenItem{
				Login: &BitwardenLogin{Uris: []BitwardenUri{}},
			},
			field:         "uri0",
			expectError:   true,
			errorContains: "no URI fields found",
		},
		{
			name: "invalid uri index format (non-numeric)",
			item: &BitwardenItem{
				Login: &BitwardenLogin{
					Uris: []BitwardenUri{{URI: "https://first.com"}},
				},
			},
			field:         "uriabc",
			expectedValue: "https://first.com", // Falls back to index 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := bw.extractUriField(tt.item, tt.field)

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

// TestBitwardenParseTimestamp tests timestamp parsing.
func TestBitwardenParseTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		timestamp string
		expectNow bool
	}{
		{
			name:      "valid RFC3339 timestamp",
			timestamp: "2024-01-15T10:30:00Z",
			expectNow: false,
		},
		{
			name:      "empty timestamp returns zero time",
			timestamp: "",
			expectNow: false,
		},
		{
			name:      "invalid timestamp format",
			timestamp: "invalid-date",
			expectNow: true, // Falls back to Now()
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTimestamp(tt.timestamp)
			if tt.timestamp == "" {
				assert.True(t, result.IsZero())
			} else if !tt.expectNow {
				assert.False(t, result.IsZero())
			}
		})
	}
}
