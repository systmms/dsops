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

func TestOnePasswordProviderWithMockExecutor_Resolve(t *testing.T) {
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
			key:  "test-login",
			mockOutput: `{
				"id": "item-123",
				"title": "Test Login",
				"category": "LOGIN",
				"notes": "",
				"tags": [],
				"vault": {"id": "vault-1", "name": "Personal"},
				"fields": [
					{"id": "username", "type": "TEXT", "label": "username", "value": "user@example.com"},
					{"id": "password", "type": "CONCEALED", "label": "password", "value": "secret-pass-123"}
				],
				"urls": []
			}`,
			wantValue: "secret-pass-123",
		},
		{
			name: "resolve username field",
			key:  "test-login.username",
			mockOutput: `{
				"id": "item-456",
				"title": "Another Login",
				"category": "LOGIN",
				"notes": "",
				"tags": [],
				"vault": {"id": "vault-1", "name": "Personal"},
				"fields": [
					{"id": "username", "type": "TEXT", "label": "username", "value": "admin@company.com"},
					{"id": "password", "type": "CONCEALED", "label": "password", "value": "admin-pass"}
				],
				"urls": []
			}`,
			wantValue: "admin@company.com",
		},
		{
			name: "resolve with op:// URI format",
			key:  "op://Personal/my-item/password",
			mockOutput: `{
				"id": "uri-item",
				"title": "URI Test",
				"category": "LOGIN",
				"notes": "",
				"tags": [],
				"vault": {"id": "vault-1", "name": "Personal"},
				"fields": [
					{"id": "password", "type": "CONCEALED", "label": "password", "value": "uri-password"}
				],
				"urls": []
			}`,
			wantValue: "uri-password",
		},
		{
			name: "resolve custom field by label",
			key:  "api-item.api_key",
			mockOutput: `{
				"id": "api-item",
				"title": "API Keys",
				"category": "LOGIN",
				"notes": "",
				"tags": [],
				"vault": {"id": "vault-1", "name": "Personal"},
				"fields": [
					{"id": "custom1", "type": "TEXT", "label": "api_key", "value": "sk-1234567890abcdef"}
				],
				"urls": []
			}`,
			wantValue: "sk-1234567890abcdef",
		},
		{
			name:        "item not found",
			key:         "nonexistent",
			mockOutput:  "Item not found",
			wantErr:     true,
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockCommandExecutor()

			if tt.wantErr {
				mockExec.AddErrorResponse("op item get", tt.mockOutput, 1)
			} else {
				mockExec.AddJSONResponse("op item get", tt.mockOutput)
			}

			p, err := providers.NewOnePasswordProviderWithExecutor(map[string]interface{}{}, mockExec)
			require.NoError(t, err)

			ref := provider.Reference{Key: tt.key}
			secret, err := p.Resolve(context.Background(), ref)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantValue, secret.Value)
			}

			mockExec.AssertCalled(t, "op")
		})
	}
}

func TestOnePasswordProviderWithMockExecutor_Describe(t *testing.T) {
	t.Parallel()

	mockExec := testutil.NewMockCommandExecutor()
	mockOutput := `{
		"id": "item-desc",
		"title": "Describe Test",
		"category": "LOGIN",
		"notes": "Test notes",
		"tags": ["personal", "important"],
		"vault": {"id": "vault-1", "name": "Personal"},
		"fields": [],
		"urls": []
	}`
	mockExec.AddJSONResponse("op item get", mockOutput)

	p, err := providers.NewOnePasswordProviderWithExecutor(map[string]interface{}{}, mockExec)
	require.NoError(t, err)

	ref := provider.Reference{Key: "item-desc"}
	meta, err := p.Describe(context.Background(), ref)

	require.NoError(t, err)
	assert.True(t, meta.Exists)
	assert.Equal(t, "LOGIN", meta.Type)
	assert.Equal(t, "personal", meta.Tags["tag_0"])
	assert.Equal(t, "important", meta.Tags["tag_1"])
}

func TestOnePasswordProviderWithMockExecutor_Validate(t *testing.T) {
	t.Parallel()

	// Skip if op CLI is not installed
	if _, err := osExec.LookPath("op"); err != nil {
		t.Skip("Skipping Validate tests - op CLI not installed")
	}

	tests := []struct {
		name       string
		mockOutput string
		mockErr    bool
		wantErr    bool
	}{
		{
			name: "authenticated",
			mockOutput: `{
				"id": "ABCD123",
				"name": "Personal",
				"domain": "my.1password.com"
			}`,
			wantErr: false,
		},
		{
			name:    "not authenticated",
			mockErr: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockCommandExecutor()

			if tt.mockErr {
				mockExec.AddErrorResponse("op account get", "You are not currently signed in", 1)
			} else {
				mockExec.AddJSONResponse("op account get", tt.mockOutput)
			}

			p, err := providers.NewOnePasswordProviderWithExecutor(map[string]interface{}{}, mockExec)
			require.NoError(t, err)

			err = p.Validate(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "signin")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestOnePasswordProviderWithMockExecutor_WithAccount(t *testing.T) {
	t.Parallel()

	mockExec := testutil.NewMockCommandExecutor()
	mockOutput := `{
		"id": "item-acc",
		"title": "Account Test",
		"category": "LOGIN",
		"notes": "",
		"tags": [],
		"vault": {"id": "vault-1", "name": "Work"},
		"fields": [
			{"id": "password", "type": "CONCEALED", "label": "password", "value": "account-pass"}
		],
		"urls": []
	}`

	// With account, args include --account
	mockExec.AddJSONResponse("op item get", mockOutput)

	config := map[string]interface{}{
		"account": "work.1password.com",
	}
	p, err := providers.NewOnePasswordProviderWithExecutor(config, mockExec)
	require.NoError(t, err)

	ref := provider.Reference{Key: "test-item"}
	secret, err := p.Resolve(context.Background(), ref)

	require.NoError(t, err)
	assert.Equal(t, "account-pass", secret.Value)

	// Verify the command included --account flag
	calls := mockExec.GetCalls("op")
	require.NotEmpty(t, calls)
	if len(calls) > 0 {
		args := calls[0].Args
		hasAccount := false
		for _, arg := range args {
			if arg == "--account" {
				hasAccount = true
				break
			}
		}
		assert.True(t, hasAccount, "Expected --account flag in command args")
	}
}

func TestOnePasswordProviderWithMockExecutor_FieldExtraction(t *testing.T) {
	t.Parallel()

	baseItem := `{
		"id": "field-test",
		"title": "Field Test Item",
		"category": "LOGIN",
		"notes": "These are important notes",
		"tags": ["test"],
		"vault": {"id": "vault-1", "name": "Personal"},
		"fields": [
			{"id": "username", "type": "TEXT", "label": "username", "value": "testuser"},
			{"id": "password", "type": "CONCEALED", "label": "password", "value": "testpass"},
			{"id": "custom", "type": "TEXT", "label": "custom_field", "value": "custom_value"}
		],
		"urls": [
			{"label": "website", "primary": true, "href": "https://example.com"}
		]
	}`

	tests := []struct {
		name      string
		key       string
		wantValue string
		wantErr   bool
	}{
		{"password", "field-test.password", "testpass", false},
		{"username", "field-test.username", "testuser", false},
		{"notes", "field-test.notes", "These are important notes", false},
		{"title", "field-test.title", "Field Test Item", false},
		{"name", "field-test.name", "Field Test Item", false},
		{"url", "field-test.url", "https://example.com", false},
		{"website", "field-test.website", "https://example.com", false},
		{"custom field", "field-test.custom_field", "custom_value", false},
		{"nonexistent", "field-test.nonexistent", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockCommandExecutor()
			mockExec.AddJSONResponse("op item get", baseItem)

			p, err := providers.NewOnePasswordProviderWithExecutor(map[string]interface{}{}, mockExec)
			require.NoError(t, err)

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

func TestOnePasswordProviderWithMockExecutor_KeyParsing(t *testing.T) {
	t.Parallel()

	baseItem := `{
		"id": "parse-test",
		"title": "Parse Test",
		"category": "LOGIN",
		"notes": "",
		"tags": [],
		"vault": {"id": "vault-1", "name": "Personal"},
		"fields": [
			{"id": "password", "type": "CONCEALED", "label": "password", "value": "parsed-pass"}
		],
		"urls": []
	}`

	tests := []struct {
		name string
		key  string
	}{
		{"simple item", "parse-test"},
		{"item with field", "parse-test.password"},
		{"op:// URI", "op://Personal/parse-test/password"},
		{"op:// vault/item", "op://Personal/parse-test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockCommandExecutor()
			mockExec.AddJSONResponse("op item get", baseItem)

			p, err := providers.NewOnePasswordProviderWithExecutor(map[string]interface{}{}, mockExec)
			require.NoError(t, err)

			ref := provider.Reference{Key: tt.key}
			secret, err := p.Resolve(context.Background(), ref)

			require.NoError(t, err)
			assert.Equal(t, "parsed-pass", secret.Value)
		})
	}
}

func TestOnePasswordProviderConstructors(t *testing.T) {
	t.Parallel()

	t.Run("default constructor", func(t *testing.T) {
		t.Parallel()
		p, err := providers.NewOnePasswordProvider(map[string]interface{}{})
		require.NoError(t, err)
		assert.NotNil(t, p)
		assert.Equal(t, "onepassword", p.Name())
	})

	t.Run("with executor constructor", func(t *testing.T) {
		t.Parallel()
		mockExec := testutil.NewMockCommandExecutor()
		p, err := providers.NewOnePasswordProviderWithExecutor(map[string]interface{}{}, mockExec)
		require.NoError(t, err)
		assert.NotNil(t, p)
		assert.Equal(t, "onepassword", p.Name())
	})

	t.Run("with account config", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"account": "team.1password.com",
		}
		p, err := providers.NewOnePasswordProvider(config)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})

	t.Run("capabilities", func(t *testing.T) {
		t.Parallel()
		p, err := providers.NewOnePasswordProvider(map[string]interface{}{})
		require.NoError(t, err)
		caps := p.Capabilities()
		assert.True(t, caps.RequiresAuth)
		assert.True(t, caps.SupportsMetadata)
		assert.Contains(t, caps.AuthMethods, "CLI session")
	})
}
