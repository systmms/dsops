package providers_test

import (
	"context"
	osExec "os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/testutil"
)

func TestBitwardenProviderWithMockExecutor_Resolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		key         string
		mockOutput  string
		wantValue   string
		wantErr     bool
		errContains string
	}{
		{
			name: "resolve password field",
			key:  "item-123",
			mockOutput: `{
				"id": "item-123",
				"name": "Test Login",
				"organizationId": "org-1",
				"folderId": "folder-1",
				"type": 1,
				"login": {
					"username": "user@example.com",
					"password": "secret-password",
					"totp": "JBSWY3DPEHPK3PXP",
					"uris": [{"uri": "https://example.com", "match": 0}]
				},
				"fields": [],
				"notes": "Some notes",
				"revisionDate": "2024-01-15T10:30:00Z"
			}`,
			wantValue: "secret-password",
		},
		{
			name: "resolve username field",
			key:  "item-456.username",
			mockOutput: `{
				"id": "item-456",
				"name": "Another Login",
				"organizationId": "",
				"folderId": "",
				"type": 1,
				"login": {
					"username": "admin@company.com",
					"password": "admin-pass",
					"totp": "",
					"uris": []
				},
				"fields": [],
				"notes": "",
				"revisionDate": "2024-02-20T15:00:00Z"
			}`,
			wantValue: "admin@company.com",
		},
		{
			name: "resolve custom field",
			key:  "item-789.api_key",
			mockOutput: `{
				"id": "item-789",
				"name": "API Item",
				"organizationId": "",
				"folderId": "",
				"type": 1,
				"login": {
					"username": "",
					"password": "",
					"totp": "",
					"uris": []
				},
				"fields": [
					{"name": "api_key", "value": "sk-live-123456789", "type": 0}
				],
				"notes": "",
				"revisionDate": "2024-03-10T12:00:00Z"
			}`,
			wantValue: "sk-live-123456789",
		},
		{
			name:        "item not found",
			key:         "nonexistent",
			mockOutput:  "Not found",
			wantErr:     true,
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockCommandExecutor()

			// Parse the item ID from the key
			itemID := tt.key
			if idx := len(tt.key); idx > 0 {
				if dotIdx := len(tt.key); dotIdx > 0 {
					for i, c := range tt.key {
						if c == '.' {
							itemID = tt.key[:i]
							break
						}
					}
				}
			}

			if tt.wantErr {
				mockExec.AddErrorResponse("bw get item "+itemID, tt.mockOutput, 1)
			} else {
				mockExec.AddJSONResponse("bw get item "+itemID, tt.mockOutput)
			}

			config := map[string]interface{}{}
			p := providers.NewBitwardenProviderWithExecutor("bitwarden", config, mockExec)
			ref := provider.Reference{Key: tt.key}

			secret, err := p.Resolve(context.Background(), ref)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantValue, secret.Value)
				assert.NotEmpty(t, secret.Metadata["item_id"])
			}

			mockExec.AssertCalled(t, "bw")
		})
	}
}

func TestBitwardenProviderWithMockExecutor_Describe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		itemID     string
		mockOutput string
		wantExists bool
		wantErr    bool
	}{
		{
			name:   "existing item",
			itemID: "item-123",
			mockOutput: `{
				"id": "item-123",
				"name": "Test Item",
				"organizationId": "org-1",
				"folderId": "folder-1",
				"type": 1,
				"login": {"username": "", "password": "", "totp": "", "uris": []},
				"fields": [],
				"notes": "",
				"revisionDate": "2024-01-15T10:30:00Z"
			}`,
			wantExists: true,
		},
		{
			name:       "nonexistent item",
			itemID:     "missing",
			mockOutput: "Not found",
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockCommandExecutor()

			if !tt.wantExists {
				mockExec.AddErrorResponse("bw get item "+tt.itemID, tt.mockOutput, 1)
			} else {
				mockExec.AddJSONResponse("bw get item "+tt.itemID, tt.mockOutput)
			}

			p := providers.NewBitwardenProviderWithExecutor("bitwarden", map[string]interface{}{}, mockExec)
			ref := provider.Reference{Key: tt.itemID}

			meta, err := p.Describe(context.Background(), ref)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantExists, meta.Exists)
			}
		})
	}
}

func TestBitwardenProviderWithMockExecutor_Validate(t *testing.T) {
	t.Parallel()

	// Skip if bw CLI is not installed
	if _, err := osExec.LookPath("bw"); err != nil {
		t.Skip("Skipping Validate tests - bw CLI not installed")
	}

	tests := []struct {
		name        string
		mockOutput  string
		wantErr     bool
		errContains string
	}{
		{
			name: "unlocked vault",
			mockOutput: `{
				"status": "unlocked",
				"lastSync": "2024-01-15T10:30:00Z",
				"userEmail": "user@example.com",
				"userId": "user-123",
				"template": null
			}`,
			wantErr: false,
		},
		{
			name: "locked vault",
			mockOutput: `{
				"status": "locked",
				"lastSync": "2024-01-15T10:30:00Z",
				"userEmail": "user@example.com",
				"userId": "user-123",
				"template": null
			}`,
			wantErr:     true,
			errContains: "locked",
		},
		{
			name: "unauthenticated",
			mockOutput: `{
				"status": "unauthenticated",
				"lastSync": null,
				"userEmail": null,
				"userId": null,
				"template": null
			}`,
			wantErr:     true,
			errContains: "not logged in",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockCommandExecutor()
			mockExec.AddJSONResponse("bw status", tt.mockOutput)

			p := providers.NewBitwardenProviderWithExecutor("bitwarden", map[string]interface{}{}, mockExec)

			err := p.Validate(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestBitwardenProviderWithMockExecutor_WithProfile(t *testing.T) {
	t.Parallel()

	mockExec := testutil.NewMockCommandExecutor()
	mockOutput := `{
		"id": "item-profile",
		"name": "Profile Item",
		"organizationId": "",
		"folderId": "",
		"type": 1,
		"login": {"username": "u", "password": "profile-pass", "totp": "", "uris": []},
		"fields": [],
		"notes": "",
		"revisionDate": "2024-01-01T00:00:00Z"
	}`

	// With profile, args include --session
	mockExec.AddJSONResponse("bw get item profile-item --session", mockOutput)

	config := map[string]interface{}{
		"profile": "test-session-key",
	}
	p := providers.NewBitwardenProviderWithExecutor("bitwarden", config, mockExec)

	ref := provider.Reference{Key: "profile-item"}
	secret, err := p.Resolve(context.Background(), ref)

	require.NoError(t, err)
	assert.Equal(t, "profile-pass", secret.Value)

	// Verify the command included session flag
	calls := mockExec.GetCalls("bw")
	require.NotEmpty(t, calls)
	if len(calls) > 0 {
		args := calls[0].Args
		hasSession := false
		for _, arg := range args {
			if arg == "--session" {
				hasSession = true
				break
			}
		}
		assert.True(t, hasSession, "Expected --session flag in command args")
	}
}

func TestBitwardenProviderWithMockExecutor_FieldExtraction(t *testing.T) {
	t.Parallel()

	baseItem := `{
		"id": "item-fields",
		"name": "Field Test Item",
		"organizationId": "",
		"folderId": "",
		"type": 1,
		"login": {
			"username": "testuser",
			"password": "testpass",
			"totp": "TOTPKEY123",
			"uris": [
				{"uri": "https://example.com", "match": 0},
				{"uri": "https://backup.example.com", "match": 0}
			]
		},
		"fields": [
			{"name": "custom_field", "value": "custom_value", "type": 0}
		],
		"notes": "Test notes here",
		"revisionDate": "2024-01-01T00:00:00Z"
	}`

	tests := []struct {
		name      string
		key       string
		wantValue string
		wantErr   bool
	}{
		{"password", "item-fields.password", "testpass", false},
		{"username", "item-fields.username", "testuser", false},
		{"totp", "item-fields.totp", "TOTPKEY123", false},
		{"notes", "item-fields.notes", "Test notes here", false},
		{"name", "item-fields.name", "Field Test Item", false},
		{"custom_field", "item-fields.custom_field", "custom_value", false},
		{"uri0", "item-fields.uri0", "https://example.com", false},
		{"uri1", "item-fields.uri1", "https://backup.example.com", false},
		{"nonexistent field", "item-fields.nonexistent", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockCommandExecutor()
			mockExec.AddJSONResponse("bw get item item-fields", baseItem)

			p := providers.NewBitwardenProviderWithExecutor("bitwarden", map[string]interface{}{}, mockExec)
			ref := provider.Reference{Key: tt.key}

			secret, err := p.Resolve(context.Background(), ref)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantValue, secret.Value)
			}
		})
	}
}

func TestBitwardenProviderConstructors(t *testing.T) {
	t.Parallel()

	t.Run("default constructor", func(t *testing.T) {
		t.Parallel()
		p := providers.NewBitwardenProvider("bw-test", map[string]interface{}{})
		assert.NotNil(t, p)
		assert.Equal(t, "bw-test", p.Name())
	})

	t.Run("with executor constructor", func(t *testing.T) {
		t.Parallel()
		mockExec := testutil.NewMockCommandExecutor()
		p := providers.NewBitwardenProviderWithExecutor("bw-mock", map[string]interface{}{}, mockExec)
		assert.NotNil(t, p)
		assert.Equal(t, "bw-mock", p.Name())
	})

	t.Run("with profile config", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"profile": "my-session",
		}
		p := providers.NewBitwardenProvider("bw-profile", config)
		assert.NotNil(t, p)
	})

	t.Run("capabilities", func(t *testing.T) {
		t.Parallel()
		p := providers.NewBitwardenProvider("bw", map[string]interface{}{})
		caps := p.Capabilities()
		assert.True(t, caps.RequiresAuth)
		assert.True(t, caps.SupportsMetadata)
		assert.Contains(t, caps.AuthMethods, "cli-session")
	})
}
