package providers_test

import (
	"context"
	"encoding/base64"
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

func TestBitwardenProviderWithMockExecutor_CustomFieldExplicitPrefix(t *testing.T) {
	t.Parallel()

	itemJSON := `{
		"id": "item-cust",
		"name": "Custom Field Item",
		"organizationId": "",
		"folderId": "",
		"type": 1,
		"login": {"username": "u", "password": "p", "totp": "", "uris": []},
		"fields": [
			{"name": "aws.region", "value": "us-east-1", "type": 0},
			{"name": "shared", "value": "explicit-wins", "type": 0}
		],
		"notes": "",
		"revisionDate": "2024-01-01T00:00:00Z"
	}`

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("bw get item item-cust", itemJSON)

	p := providers.NewBitwardenProviderWithExecutor("bitwarden", map[string]interface{}{}, mockExec)

	t.Run("custom field name with dots is preserved", func(t *testing.T) {
		ref := provider.Reference{Key: "item-cust.custom.aws.region"}
		secret, err := p.Resolve(context.Background(), ref)
		require.NoError(t, err)
		assert.Equal(t, "us-east-1", secret.Value)
	})

	t.Run("explicit custom prefix returns custom field", func(t *testing.T) {
		ref := provider.Reference{Key: "item-cust.custom.shared"}
		secret, err := p.Resolve(context.Background(), ref)
		require.NoError(t, err)
		assert.Equal(t, "explicit-wins", secret.Value)
	})

	t.Run("flat custom field still works for back-compat", func(t *testing.T) {
		ref := provider.Reference{Key: "item-cust.shared"}
		secret, err := p.Resolve(context.Background(), ref)
		require.NoError(t, err)
		assert.Equal(t, "explicit-wins", secret.Value)
	})
}

func TestBitwardenProviderWithMockExecutor_Attachment(t *testing.T) {
	t.Parallel()

	// Note: id and Name differ, simulating a name-based key lookup. The
	// attachment fetch must use the canonical id, not the user-supplied name,
	// because `bw get attachment --itemid` requires the exact UUID.
	itemJSON := `{
		"id": "item-att-uuid",
		"name": "Item With Attachment",
		"organizationId": "org-1",
		"folderId": "folder-1",
		"type": 2,
		"login": null,
		"fields": [],
		"notes": "",
		"revisionDate": "2024-05-01T12:00:00Z"
	}`

	rawBytes := []byte("-----BEGIN CERTIFICATE-----\nMIIBkTCB+w==\n-----END CERTIFICATE-----\n")

	mockExec := testutil.NewMockCommandExecutor()
	// User addresses the item by name; bw get item resolves it to the canonical id.
	mockExec.AddJSONResponse("bw get item Item With Attachment", itemJSON)
	// The attachment fetch MUST use the canonical id, not the original name.
	mockExec.AddResponse("bw get attachment fullchain.pem --itemid item-att-uuid --raw", testutil.MockResponse{
		Stdout: rawBytes,
	})

	p := providers.NewBitwardenProviderWithExecutor("bitwarden", map[string]interface{}{}, mockExec)

	t.Run("attachment is base64-encoded with metadata", func(t *testing.T) {
		ref := provider.Reference{Key: "Item With Attachment.attachment.fullchain.pem"}
		secret, err := p.Resolve(context.Background(), ref)
		require.NoError(t, err)
		assert.Equal(t, base64.StdEncoding.EncodeToString(rawBytes), secret.Value)
		assert.Equal(t, "fullchain.pem", secret.Metadata["attachment"])
		assert.Equal(t, "application/octet-stream", secret.Metadata["content_type"])
		assert.Equal(t, "item-att-uuid", secret.Metadata["item_id"])
	})

	t.Run("missing attachment returns NotFoundError", func(t *testing.T) {
		mockExec2 := testutil.NewMockCommandExecutor()
		mockExec2.AddJSONResponse("bw get item Item With Attachment", itemJSON)
		mockExec2.AddErrorResponse("bw get attachment ghost.pem --itemid item-att-uuid --raw", "Attachment `ghost.pem` was not found.", 1)

		p2 := providers.NewBitwardenProviderWithExecutor("bitwarden", map[string]interface{}{}, mockExec2)
		_, err := p2.Resolve(context.Background(), provider.Reference{Key: "Item With Attachment.attachment.ghost.pem"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestBitwardenProviderWithMockExecutor_CardItem(t *testing.T) {
	t.Parallel()

	itemJSON := `{
		"id": "item-card",
		"name": "My Card",
		"organizationId": "",
		"folderId": "",
		"type": 3,
		"login": null,
		"card": {
			"cardholderName": "Ada Lovelace",
			"brand": "Visa",
			"number": "4111111111111111",
			"expMonth": "12",
			"expYear": "2030",
			"code": "123"
		},
		"fields": [],
		"notes": "",
		"revisionDate": "2024-04-01T00:00:00Z"
	}`

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("bw get item item-card", itemJSON)
	p := providers.NewBitwardenProviderWithExecutor("bitwarden", map[string]interface{}{}, mockExec)

	cases := []struct{ key, want string }{
		{"item-card", "4111111111111111"}, // default for Card is number
		{"item-card.number", "4111111111111111"},
		{"item-card.code", "123"},
		{"item-card.cvv", "123"}, // alias
		{"item-card.cardholderName", "Ada Lovelace"},
		{"item-card.brand", "Visa"},
		{"item-card.expMonth", "12"},
		{"item-card.expYear", "2030"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.key, func(t *testing.T) {
			s, err := p.Resolve(context.Background(), provider.Reference{Key: tc.key})
			require.NoError(t, err)
			assert.Equal(t, tc.want, s.Value)
		})
	}
}

func TestBitwardenProviderWithMockExecutor_IdentityItem(t *testing.T) {
	t.Parallel()

	itemJSON := `{
		"id": "item-id",
		"name": "Personal Identity",
		"organizationId": "",
		"folderId": "",
		"type": 4,
		"login": null,
		"identity": {
			"title": "Dr",
			"firstName": "Grace",
			"lastName": "Hopper",
			"email": "grace@example.com",
			"phone": "+1-555-0100",
			"city": "Arlington",
			"country": "US"
		},
		"fields": [],
		"notes": "",
		"revisionDate": "2024-04-02T00:00:00Z"
	}`

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("bw get item item-id", itemJSON)
	p := providers.NewBitwardenProviderWithExecutor("bitwarden", map[string]interface{}{}, mockExec)

	cases := []struct{ key, want string }{
		{"item-id", "grace@example.com"}, // default for Identity is email
		{"item-id.email", "grace@example.com"},
		{"item-id.firstName", "Grace"},
		{"item-id.lastName", "Hopper"},
		{"item-id.title", "Dr"},
		{"item-id.phone", "+1-555-0100"},
		{"item-id.city", "Arlington"},
		{"item-id.country", "US"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.key, func(t *testing.T) {
			s, err := p.Resolve(context.Background(), provider.Reference{Key: tc.key})
			require.NoError(t, err)
			assert.Equal(t, tc.want, s.Value)
		})
	}
}

func TestBitwardenProviderWithMockExecutor_SshKeyItem(t *testing.T) {
	t.Parallel()

	itemJSON := `{
		"id": "item-ssh",
		"name": "Deploy Key",
		"organizationId": "",
		"folderId": "",
		"type": 5,
		"login": null,
		"sshKey": {
			"privateKey": "-----BEGIN OPENSSH PRIVATE KEY-----\nXXXXXXXX\n-----END OPENSSH PRIVATE KEY-----",
			"publicKey": "ssh-ed25519 AAAA... user@host",
			"keyFingerprint": "SHA256:abc123def456"
		},
		"fields": [],
		"notes": "",
		"revisionDate": "2024-04-03T00:00:00Z"
	}`

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("bw get item item-ssh", itemJSON)
	p := providers.NewBitwardenProviderWithExecutor("bitwarden", map[string]interface{}{}, mockExec)

	cases := []struct {
		key      string
		contains string
	}{
		{"item-ssh", "BEGIN OPENSSH PRIVATE KEY"}, // default for SshKey is privateKey
		{"item-ssh.privateKey", "BEGIN OPENSSH PRIVATE KEY"},
		{"item-ssh.publicKey", "ssh-ed25519"},
		{"item-ssh.keyFingerprint", "SHA256:abc123"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.key, func(t *testing.T) {
			s, err := p.Resolve(context.Background(), provider.Reference{Key: tc.key})
			require.NoError(t, err)
			assert.Contains(t, s.Value, tc.contains)
		})
	}
}

func TestBitwardenProviderWithMockExecutor_NoteItem(t *testing.T) {
	t.Parallel()

	itemJSON := `{
		"id": "item-note",
		"name": "Runbook",
		"organizationId": "",
		"folderId": "",
		"type": 2,
		"login": null,
		"fields": [],
		"notes": "step 1: don't panic\nstep 2: read the runbook",
		"revisionDate": "2024-04-04T00:00:00Z"
	}`

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("bw get item item-note", itemJSON)
	p := providers.NewBitwardenProviderWithExecutor("bitwarden", map[string]interface{}{}, mockExec)

	t.Run("default field is notes for Note items", func(t *testing.T) {
		s, err := p.Resolve(context.Background(), provider.Reference{Key: "item-note"})
		require.NoError(t, err)
		assert.Contains(t, s.Value, "don't panic")
	})
}

func TestBitwardenProviderWithMockExecutor_SyncOnce(t *testing.T) {
	t.Parallel()

	itemJSON := `{
		"id": "item-sync",
		"name": "Sync Item",
		"organizationId": "",
		"folderId": "",
		"type": 1,
		"login": {"username": "u", "password": "syncpass", "totp": "", "uris": []},
		"fields": [],
		"notes": "",
		"revisionDate": "2024-01-01T00:00:00Z"
	}`

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("bw sync", `{"object": "message", "title": "Sync"}`)
	mockExec.AddJSONResponse("bw get item item-sync", itemJSON)

	cfg := map[string]interface{}{"sync": true}
	p := providers.NewBitwardenProviderWithExecutor("bitwarden", cfg, mockExec)

	for i := 0; i < 3; i++ {
		_, err := p.Resolve(context.Background(), provider.Reference{Key: "item-sync.password"})
		require.NoError(t, err)
	}

	syncCalls := 0
	for _, c := range mockExec.GetCalls("bw") {
		if len(c.Args) > 0 && c.Args[0] == "sync" {
			syncCalls++
		}
	}
	assert.Equal(t, 1, syncCalls, "bw sync should run exactly once across multiple Resolve calls")
}

func TestBitwardenProviderWithMockExecutor_SyncDisabled(t *testing.T) {
	t.Parallel()

	itemJSON := `{
		"id": "x",
		"name": "x",
		"type": 1,
		"login": {"username": "", "password": "p", "totp": "", "uris": []},
		"fields": [],
		"notes": "",
		"revisionDate": "2024-01-01T00:00:00Z"
	}`

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("bw get item x", itemJSON)

	p := providers.NewBitwardenProviderWithExecutor("bitwarden", map[string]interface{}{}, mockExec)
	_, err := p.Resolve(context.Background(), provider.Reference{Key: "x"})
	require.NoError(t, err)

	for _, c := range mockExec.GetCalls("bw") {
		require.NotEmpty(t, c.Args)
		assert.NotEqual(t, "sync", c.Args[0], "bw sync should not run when sync config is false")
	}
}

func TestBitwardenProviderWithMockExecutor_HeadlessLogin(t *testing.T) {
	restore := providers.SetBwLookPathForTesting(func() error { return nil })
	defer restore()

	t.Setenv("BW_CLIENTID", "fake-client-id")
	t.Setenv("BW_CLIENTSECRET", "fake-client-secret")

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddResponse("bw status", testutil.MockResponse{
		Stdout: []byte(`{"status":"unauthenticated"}`),
	})
	mockExec.AddResponse("bw login --apikey", testutil.MockResponse{
		Stdout: []byte(`You are logged in!`),
	})

	cfg := map[string]interface{}{"headless": true}
	p := providers.NewBitwardenProviderWithExecutor("bitwarden", cfg, mockExec)

	// Status returns unauthenticated for both calls in this test, so the
	// final state is still unauthenticated and Validate must report that.
	err := p.Validate(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")

	// But bw login --apikey must have been attempted exactly once.
	loginCalls := 0
	for _, c := range mockExec.GetCalls("bw") {
		if len(c.Args) >= 2 && c.Args[0] == "login" && c.Args[1] == "--apikey" {
			loginCalls++
		}
	}
	assert.Equal(t, 1, loginCalls, "bw login --apikey should be invoked once in headless mode")
}

func TestBitwardenProviderWithMockExecutor_HeadlessLoginMissingEnv(t *testing.T) {
	restore := providers.SetBwLookPathForTesting(func() error { return nil })
	defer restore()

	t.Setenv("BW_CLIENTID", "")
	t.Setenv("BW_CLIENTSECRET", "")

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddResponse("bw status", testutil.MockResponse{
		Stdout: []byte(`{"status":"unauthenticated"}`),
	})

	cfg := map[string]interface{}{"headless": true}
	p := providers.NewBitwardenProviderWithExecutor("bitwarden", cfg, mockExec)

	err := p.Validate(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BW_CLIENTID")
}

func TestBitwardenProviderWithMockExecutor_HeadlessUnlock(t *testing.T) {
	restore := providers.SetBwLookPathForTesting(func() error { return nil })
	defer restore()

	t.Setenv("BW_PASSWORD", "secret-master-password")

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddResponse("bw status", testutil.MockResponse{
		Stdout: []byte(`{"status":"locked"}`),
	})
	mockExec.AddResponse("bw unlock --passwordenv BW_PASSWORD --raw", testutil.MockResponse{
		Stdout: []byte("captured-session-token-abc123\n"),
	})

	cfg := map[string]interface{}{"headless": true}
	p := providers.NewBitwardenProviderWithExecutor("bitwarden", cfg, mockExec)

	// Validate will: see locked, call unlock, re-check status (still locked
	// in our mock since the same response is replayed), and then surface the
	// locked AuthError. The contract under test is that bw unlock was
	// attempted with the expected args.
	_ = p.Validate(context.Background())

	unlockCalls := 0
	for _, c := range mockExec.GetCalls("bw") {
		if len(c.Args) >= 4 && c.Args[0] == "unlock" && c.Args[1] == "--passwordenv" && c.Args[2] == "BW_PASSWORD" && c.Args[3] == "--raw" {
			unlockCalls++
		}
	}
	assert.Equal(t, 1, unlockCalls, "bw unlock --passwordenv BW_PASSWORD --raw should be invoked once")
}

// TestBitwardenProviderEmptyDefaultFields locks in the contract that empty
// secret-bearing default fields surface as errors rather than silently
// returning a blank string into a deploy environment.
func TestBitwardenProviderEmptyDefaultFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		itemJSON string
		key      string
		errMatch string
	}{
		{
			name: "empty card number (default field)",
			itemJSON: `{
				"id": "card-1", "name": "Empty Card", "type": 3,
				"login": null,
				"card": {"number": "", "code": "", "cardholderName": "", "brand": "", "expMonth": "", "expYear": ""},
				"fields": [], "notes": "", "revisionDate": "2024-01-01T00:00:00Z"
			}`,
			key:      "card-1",
			errMatch: "no card number found",
		},
		{
			name: "empty identity email (default field)",
			itemJSON: `{
				"id": "id-1", "name": "Empty Identity", "type": 4,
				"login": null,
				"identity": {"email": "", "firstName": "", "lastName": ""},
				"fields": [], "notes": "", "revisionDate": "2024-01-01T00:00:00Z"
			}`,
			key:      "id-1",
			errMatch: "no email field found",
		},
		{
			name: "empty ssh private key (default field)",
			itemJSON: `{
				"id": "ssh-1", "name": "Empty SSH", "type": 5,
				"login": null,
				"sshKey": {"privateKey": "", "publicKey": "", "keyFingerprint": ""},
				"fields": [], "notes": "", "revisionDate": "2024-01-01T00:00:00Z"
			}`,
			key:      "ssh-1",
			errMatch: "no SSH private key found",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockExec := testutil.NewMockCommandExecutor()
			mockExec.AddJSONResponse("bw get item "+tt.key, tt.itemJSON)
			p := providers.NewBitwardenProviderWithExecutor("bitwarden", map[string]interface{}{}, mockExec)
			_, err := p.Resolve(context.Background(), provider.Reference{Key: tt.key})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMatch)
		})
	}
}

// TestBitwardenProviderHeadlessAuthRetriesOnFailure verifies the headless
// auth flow does NOT poison the provider permanently after a transient
// failure: a subsequent Resolve call must re-attempt the recovery flow.
func TestBitwardenProviderHeadlessAuthRetriesOnFailure(t *testing.T) {
	restore := providers.SetBwLookPathForTesting(func() error { return nil })
	defer restore()

	t.Setenv("BW_CLIENTID", "fake-client-id")
	t.Setenv("BW_CLIENTSECRET", "fake-client-secret")

	mockExec := testutil.NewMockCommandExecutor()
	// Status returns unauthenticated permanently (mock doesn't transition).
	// `bw login --apikey` succeeds.
	// We're not asserting Validate succeeds — only that the recovery flow
	// is attempted MORE THAN ONCE across calls when the prior attempt
	// returned an error to the caller.
	mockExec.AddResponse("bw status", testutil.MockResponse{Stdout: []byte(`{"status":"unauthenticated"}`)})
	mockExec.AddResponse("bw login --apikey", testutil.MockResponse{Stdout: []byte(`logged in`)})

	cfg := map[string]interface{}{"headless": true}
	p := providers.NewBitwardenProviderWithExecutor("bitwarden", cfg, mockExec)

	_ = p.Validate(context.Background())
	_ = p.Validate(context.Background())

	loginCalls := 0
	for _, c := range mockExec.GetCalls("bw") {
		if len(c.Args) >= 2 && c.Args[0] == "login" && c.Args[1] == "--apikey" {
			loginCalls++
		}
	}
	assert.GreaterOrEqual(t, loginCalls, 2,
		"headless auth should retry on each call until it succeeds (was %d)", loginCalls)
}

// TestBitwardenProviderCustomFieldNamedCustom verifies the back-compat
// fallback: a custom field literally named "custom" must resolve via the
// flat key form `item.custom` instead of triggering the incomplete-key error.
func TestBitwardenProviderCustomFieldNamedCustom(t *testing.T) {
	t.Parallel()

	itemJSON := `{
		"id": "edge-1", "name": "Edge Item",
		"organizationId": "", "folderId": "",
		"type": 1,
		"login": {"username": "u", "password": "p", "totp": "", "uris": []},
		"fields": [
			{"name": "custom", "value": "literal-custom-value", "type": 0}
		],
		"notes": "", "revisionDate": "2024-01-01T00:00:00Z"
	}`

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("bw get item edge-1", itemJSON)
	p := providers.NewBitwardenProviderWithExecutor("bitwarden", map[string]interface{}{}, mockExec)

	t.Run("flat key resolves field literally named custom", func(t *testing.T) {
		s, err := p.Resolve(context.Background(), provider.Reference{Key: "edge-1.custom"})
		require.NoError(t, err)
		assert.Equal(t, "literal-custom-value", s.Value)
	})

	t.Run("incomplete-key error when no such field exists", func(t *testing.T) {
		emptyJSON := `{"id":"e2","name":"E2","type":1,"login":{"password":"p","username":"u","totp":"","uris":[]},"fields":[],"notes":"","revisionDate":"2024-01-01T00:00:00Z"}`
		mockExec2 := testutil.NewMockCommandExecutor()
		mockExec2.AddJSONResponse("bw get item e2", emptyJSON)
		p2 := providers.NewBitwardenProviderWithExecutor("bitwarden", map[string]interface{}{}, mockExec2)
		_, err := p2.Resolve(context.Background(), provider.Reference{Key: "e2.custom"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "incomplete key")
	})
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
