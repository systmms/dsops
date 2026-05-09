package providers_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/testutil"
)

// secretJSONByUUID is a helper for the canonical bws secret get response.
const secretJSONByUUID = `{
	"id": "11111111-1111-4111-8111-111111111111",
	"key": "DB_PASSWORD",
	"value": "s3cret",
	"note": "rotates monthly",
	"projectId": "22222222-2222-4222-8222-222222222222",
	"creationDate": "2024-01-01T00:00:00Z",
	"revisionDate": "2024-05-01T00:00:00Z"
}`

func TestBitwardenSMProvider_NameAndCapabilities(t *testing.T) {
	// t.Parallel() omitted: t.Setenv conflicts with parallel

	p := providers.NewBitwardenSecretsManagerProvider("bw-sm", map[string]interface{}{})
	assert.Equal(t, "bw-sm", p.Name())

	caps := p.Capabilities()
	assert.True(t, caps.RequiresAuth)
	assert.True(t, caps.SupportsMetadata)
	assert.False(t, caps.SupportsVersioning)
	assert.False(t, caps.SupportsBinary)
	assert.Contains(t, caps.AuthMethods, "access-token")
}

func TestBitwardenSMProvider_ResolveByUUID(t *testing.T) {
	// t.Parallel() omitted: t.Setenv conflicts with parallel
	t.Setenv("BWS_ACCESS_TOKEN", "fake-token")

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("bws", secretJSONByUUID)

	p := providers.NewBitwardenSecretsManagerProviderWithExecutor("bw-sm", map[string]interface{}{}, mockExec)
	ref := provider.Reference{Key: "11111111-1111-4111-8111-111111111111"}

	secret, err := p.Resolve(context.Background(), ref)
	require.NoError(t, err)
	assert.Equal(t, "s3cret", secret.Value)
	assert.Equal(t, "11111111-1111-4111-8111-111111111111", secret.Metadata["secret_id"])
	assert.Equal(t, "22222222-2222-4222-8222-222222222222", secret.Metadata["project_id"])
	assert.Equal(t, "DB_PASSWORD", secret.Metadata["secret_key"])
}

func TestBitwardenSMProvider_ResolveNoteFieldOverride(t *testing.T) {
	// t.Parallel() omitted: t.Setenv conflicts with parallel
	t.Setenv("BWS_ACCESS_TOKEN", "fake-token")

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("bws", secretJSONByUUID)

	p := providers.NewBitwardenSecretsManagerProviderWithExecutor("bw-sm", map[string]interface{}{}, mockExec)
	ref := provider.Reference{Key: "11111111-1111-4111-8111-111111111111", Field: "note"}

	secret, err := p.Resolve(context.Background(), ref)
	require.NoError(t, err)
	assert.Equal(t, "rotates monthly", secret.Value, "Field=note should swap value -> note")
}

func TestBitwardenSMProvider_ResolveByPath(t *testing.T) {
	// t.Parallel() omitted: t.Setenv conflicts with parallel
	t.Setenv("BWS_ACCESS_TOKEN", "fake-token")

	projectListJSON := `[
		{"id": "33333333-3333-4333-8333-333333333333", "name": "production"},
		{"id": "44444444-4444-4444-8444-444444444444", "name": "staging"}
	]`
	secretListJSON := `[
		{"id": "55555555-5555-4555-8555-555555555555", "key": "API_KEY", "value": "prod-key", "note": "", "projectId": "33333333-3333-4333-8333-333333333333", "creationDate": "2024-01-01T00:00:00Z", "revisionDate": "2024-04-01T00:00:00Z"},
		{"id": "66666666-6666-4666-8666-666666666666", "key": "DB_URL", "value": "postgres://...", "note": "", "projectId": "33333333-3333-4333-8333-333333333333", "creationDate": "2024-01-01T00:00:00Z", "revisionDate": "2024-04-02T00:00:00Z"}
	]`

	mockExec := testutil.NewMockCommandExecutor()
	// Default BWS_ACCESS_TOKEN env var: token is inherited via env, not flag.
	mockExec.AddJSONResponse("bws --output json project list", projectListJSON)
	mockExec.AddJSONResponse("bws --output json secret list 33333333-3333-4333-8333-333333333333", secretListJSON)

	p := providers.NewBitwardenSecretsManagerProviderWithExecutor("bw-sm", map[string]interface{}{}, mockExec)

	t.Run("path resolves to value", func(t *testing.T) {
		ref := provider.Reference{Key: "production/API_KEY"}
		s, err := p.Resolve(context.Background(), ref)
		require.NoError(t, err)
		assert.Equal(t, "prod-key", s.Value)
	})

	t.Run("second resolve in same project hits secret cache", func(t *testing.T) {
		ref := provider.Reference{Key: "production/DB_URL"}
		s, err := p.Resolve(context.Background(), ref)
		require.NoError(t, err)
		assert.Equal(t, "postgres://...", s.Value)
	})

	// Verify caches: project list runs once, secret list runs once per project.
	projectListCalls := 0
	secretListCalls := 0
	for _, c := range mockExec.GetCalls("bws") {
		if hasArgsPrefix(c.Args, "project", "list") {
			projectListCalls++
		}
		if hasArgsPrefix(c.Args, "secret", "list") {
			secretListCalls++
		}
	}
	assert.Equal(t, 1, projectListCalls, "project list should be cached")
	assert.Equal(t, 1, secretListCalls, "secret list per project should be cached")
}

func TestBitwardenSMProvider_ResolveMissingToken(t *testing.T) {
	// t.Parallel() omitted: t.Setenv conflicts with parallel
	t.Setenv("BWS_ACCESS_TOKEN", "")

	mockExec := testutil.NewMockCommandExecutor()
	p := providers.NewBitwardenSecretsManagerProviderWithExecutor("bw-sm", map[string]interface{}{}, mockExec)
	_, err := p.Resolve(context.Background(), provider.Reference{Key: "11111111-1111-4111-8111-111111111111"})

	require.Error(t, err)
	var authErr provider.AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Contains(t, authErr.Message, "BWS_ACCESS_TOKEN")
}

func TestBitwardenSMProvider_ResolveCustomTokenEnv(t *testing.T) {
	// t.Parallel() omitted: t.Setenv conflicts with parallel
	t.Setenv("MY_CUSTOM_BWS", "custom-token")
	t.Setenv("BWS_ACCESS_TOKEN", "wrong-default")

	mockExec := testutil.NewMockCommandExecutor()
	// Custom env var: token IS passed via --access-token flag.
	mockExec.AddJSONResponse("bws --output json --access-token custom-token secret get", secretJSONByUUID)

	cfg := map[string]interface{}{"access_token_env": "MY_CUSTOM_BWS"}
	p := providers.NewBitwardenSecretsManagerProviderWithExecutor("bw-sm", cfg, mockExec)
	_, err := p.Resolve(context.Background(), provider.Reference{Key: "11111111-1111-4111-8111-111111111111"})
	require.NoError(t, err)
}

func TestBitwardenSMProvider_DefaultTokenEnvNotInArgv(t *testing.T) {
	// t.Parallel() omitted: t.Setenv conflicts with parallel
	t.Setenv("BWS_ACCESS_TOKEN", "fake-token")

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("bws", secretJSONByUUID)

	p := providers.NewBitwardenSecretsManagerProviderWithExecutor("bw-sm", map[string]interface{}{}, mockExec)
	_, err := p.Resolve(context.Background(), provider.Reference{Key: "11111111-1111-4111-8111-111111111111"})
	require.NoError(t, err)

	// Token must NOT appear in argv when default env var is used: bws inherits
	// BWS_ACCESS_TOKEN from the process environment.
	calls := mockExec.GetCalls("bws")
	require.NotEmpty(t, calls)
	for _, c := range calls {
		for _, arg := range c.Args {
			assert.NotEqual(t, "fake-token", arg, "access token must not appear in argv when default env var is used")
		}
	}
}

func TestBitwardenSMProvider_ResolveNotFound(t *testing.T) {
	// t.Parallel() omitted: t.Setenv conflicts with parallel
	t.Setenv("BWS_ACCESS_TOKEN", "fake-token")

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddErrorResponse("bws --output json secret get", "404: Resource not found", 1)

	p := providers.NewBitwardenSecretsManagerProviderWithExecutor("bw-sm", map[string]interface{}{}, mockExec)
	_, err := p.Resolve(context.Background(), provider.Reference{Key: "11111111-1111-4111-8111-111111111111"})

	require.Error(t, err)
	var notFound *provider.NotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestBitwardenSMProvider_ResolveAmbiguousProjectName(t *testing.T) {
	// t.Parallel() omitted: t.Setenv conflicts with parallel
	t.Setenv("BWS_ACCESS_TOKEN", "fake-token")

	projectListJSON := `[
		{"id": "aaaa1111-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "name": "shared"},
		{"id": "bbbb2222-bbbb-4bbb-8bbb-bbbbbbbbbbbb", "name": "shared"}
	]`

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("bws --output json project list", projectListJSON)

	p := providers.NewBitwardenSecretsManagerProviderWithExecutor("bw-sm", map[string]interface{}{}, mockExec)
	_, err := p.Resolve(context.Background(), provider.Reference{Key: "shared/SOMETHING"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous project name")
}

func TestBitwardenSMProvider_ResolveMalformedKey(t *testing.T) {
	// t.Parallel() omitted: t.Setenv conflicts with parallel
	t.Setenv("BWS_ACCESS_TOKEN", "fake-token")

	mockExec := testutil.NewMockCommandExecutor()
	p := providers.NewBitwardenSecretsManagerProviderWithExecutor("bw-sm", map[string]interface{}{}, mockExec)
	_, err := p.Resolve(context.Background(), provider.Reference{Key: "not-a-uuid-not-a-path"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a UUID or '<projectName>/<secretKey>'")
}

func TestBitwardenSMProvider_DescribeNotFoundReturnsExistsFalse(t *testing.T) {
	// t.Parallel() omitted: t.Setenv conflicts with parallel
	t.Setenv("BWS_ACCESS_TOKEN", "fake-token")

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddErrorResponse("bws --output json secret get", "404: Resource not found", 1)

	p := providers.NewBitwardenSecretsManagerProviderWithExecutor("bw-sm", map[string]interface{}{}, mockExec)
	meta, err := p.Describe(context.Background(), provider.Reference{Key: "11111111-1111-4111-8111-111111111111"})

	require.NoError(t, err, "Describe must convert NotFound into Exists=false rather than erroring")
	assert.False(t, meta.Exists)
}

func TestBitwardenSMProvider_DescribeFound(t *testing.T) {
	// t.Parallel() omitted: t.Setenv conflicts with parallel
	t.Setenv("BWS_ACCESS_TOKEN", "fake-token")

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("bws", secretJSONByUUID)

	p := providers.NewBitwardenSecretsManagerProviderWithExecutor("bw-sm", map[string]interface{}{}, mockExec)
	meta, err := p.Describe(context.Background(), provider.Reference{Key: "11111111-1111-4111-8111-111111111111"})

	require.NoError(t, err)
	assert.True(t, meta.Exists)
	assert.Equal(t, "secret", meta.Type)
	assert.Equal(t, "DB_PASSWORD", meta.Tags["secret_key"])
}

func TestBitwardenSMProvider_ValidateMissingToken(t *testing.T) {
	// t.Parallel() omitted: t.Setenv conflicts with parallel
	t.Setenv("BWS_ACCESS_TOKEN", "")

	restore := providers.SetBwsLookPathForTesting(func() error { return nil })
	defer restore()

	mockExec := testutil.NewMockCommandExecutor()
	p := providers.NewBitwardenSecretsManagerProviderWithExecutor("bw-sm", map[string]interface{}{}, mockExec)
	err := p.Validate(context.Background())

	require.Error(t, err)
	var authErr provider.AuthError
	require.ErrorAs(t, err, &authErr)
}

func TestBitwardenSMProvider_ValidateOK(t *testing.T) {
	// t.Parallel() omitted: t.Setenv conflicts with parallel
	t.Setenv("BWS_ACCESS_TOKEN", "fake-token")

	restore := providers.SetBwsLookPathForTesting(func() error { return nil })
	defer restore()

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("bws --output json project list", `[]`)

	p := providers.NewBitwardenSecretsManagerProviderWithExecutor("bw-sm", map[string]interface{}{}, mockExec)
	err := p.Validate(context.Background())
	require.NoError(t, err)
}

func TestBitwardenSMProvider_ServerURLForwarded(t *testing.T) {
	// t.Parallel() omitted: t.Setenv conflicts with parallel
	t.Setenv("BWS_ACCESS_TOKEN", "fake-token")

	mockExec := testutil.NewMockCommandExecutor()
	mockExec.AddJSONResponse("bws", secretJSONByUUID)

	cfg := map[string]interface{}{
		"server_url": "https://self-hosted.example.com",
	}
	p := providers.NewBitwardenSecretsManagerProviderWithExecutor("bw-sm", cfg, mockExec)
	_, err := p.Resolve(context.Background(), provider.Reference{Key: "11111111-1111-4111-8111-111111111111"})
	require.NoError(t, err)

	calls := mockExec.GetCalls("bws")
	require.NotEmpty(t, calls)
	args := calls[0].Args

	assert.Contains(t, args, "--server-url")
	assert.Contains(t, args, "https://self-hosted.example.com")
}

func TestBitwardenSMProvider_CacheInvalidatesOnTokenChange(t *testing.T) {
	// t.Parallel() omitted: t.Setenv conflicts with parallel
	projectListA := `[{"id": "aaaa1111-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "name": "tenant-a"}]`
	projectListB := `[{"id": "bbbb2222-bbbb-4bbb-8bbb-bbbbbbbbbbbb", "name": "tenant-b"}]`
	secretListA := `[{"id": "11111111-1111-4111-8111-111111111111", "key": "K", "value": "value-a", "note": "", "projectId": "aaaa1111-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "creationDate": "2024-01-01T00:00:00Z", "revisionDate": "2024-01-01T00:00:00Z"}]`
	secretListB := `[{"id": "22222222-2222-4222-8222-222222222222", "key": "K", "value": "value-b", "note": "", "projectId": "bbbb2222-bbbb-4bbb-8bbb-bbbbbbbbbbbb", "creationDate": "2024-01-01T00:00:00Z", "revisionDate": "2024-01-01T00:00:00Z"}]`

	cfg := map[string]interface{}{"access_token_env": "MY_BWS"}
	mockExec := testutil.NewMockCommandExecutor()
	// With custom env var, --access-token IS in argv, so we can match by token.
	mockExec.AddJSONResponse("bws --output json --access-token tenant-a-token project list", projectListA)
	mockExec.AddJSONResponse("bws --output json --access-token tenant-a-token secret list aaaa1111-aaaa-4aaa-8aaa-aaaaaaaaaaaa", secretListA)
	mockExec.AddJSONResponse("bws --output json --access-token tenant-b-token project list", projectListB)
	mockExec.AddJSONResponse("bws --output json --access-token tenant-b-token secret list bbbb2222-bbbb-4bbb-8bbb-bbbbbbbbbbbb", secretListB)

	p := providers.NewBitwardenSecretsManagerProviderWithExecutor("bw-sm", cfg, mockExec)

	t.Setenv("MY_BWS", "tenant-a-token")
	s1, err := p.Resolve(context.Background(), provider.Reference{Key: "tenant-a/K"})
	require.NoError(t, err)
	assert.Equal(t, "value-a", s1.Value)

	t.Setenv("MY_BWS", "tenant-b-token")
	s2, err := p.Resolve(context.Background(), provider.Reference{Key: "tenant-b/K"})
	require.NoError(t, err)
	assert.Equal(t, "value-b", s2.Value, "second tenant must NOT see first tenant's cached projects/secrets")
}

// TestBitwardenSMProvider_FailedListNotCached verifies that a transient
// `bws project list` failure does not poison the provider for its lifetime —
// subsequent calls must retry the CLI rather than return the cached error.
func TestBitwardenSMProvider_FailedListNotCached(t *testing.T) {
	// t.Parallel() omitted: t.Setenv conflicts with parallel
	t.Setenv("BWS_ACCESS_TOKEN", "fake-token")

	projectListJSON := `[{"id": "33333333-3333-4333-8333-333333333333", "name": "production"}]`
	secretListJSON := `[{"id": "55555555-5555-4555-8555-555555555555", "key": "API_KEY", "value": "v", "note": "", "projectId": "33333333-3333-4333-8333-333333333333", "creationDate": "2024-01-01T00:00:00Z", "revisionDate": "2024-01-01T00:00:00Z"}]`

	mockExec := testutil.NewMockCommandExecutor()
	// Default response: error. Specific responses below take precedence on
	// exact-match, but for this test we want the FIRST call to fail and the
	// SECOND to succeed. Since the mock doesn't support sequence-aware
	// responses, we exercise the property by calling Resolve once with no
	// project-list response (returns no-match → empty stdout → JSON parse
	// fails), verifying the error, then registering the success response and
	// calling Resolve again.

	p := providers.NewBitwardenSecretsManagerProviderWithExecutor("bw-sm", map[string]interface{}{}, mockExec)

	// First call: no project-list mock registered → empty stdout from default
	// non-strict mock → JSON parse error. Should NOT cache the failure.
	_, err := p.Resolve(context.Background(), provider.Reference{Key: "production/API_KEY"})
	require.Error(t, err, "first call should fail without project-list mock")

	// Now register working mocks and retry. If the failure had been cached,
	// the cached error would be returned without consulting the mock.
	mockExec.AddJSONResponse("bws --output json project list", projectListJSON)
	mockExec.AddJSONResponse("bws --output json secret list 33333333-3333-4333-8333-333333333333", secretListJSON)

	s, err := p.Resolve(context.Background(), provider.Reference{Key: "production/API_KEY"})
	require.NoError(t, err, "second call should succeed (failure must not be cached)")
	assert.Equal(t, "v", s.Value)
}

// hasArgsPrefix returns true if args contains the given sub-sequence in order
// (anywhere in the args slice). Used to match "subcommand arg1 arg2" patterns
// while ignoring global flags.
func hasArgsPrefix(args []string, want ...string) bool {
	if len(want) == 0 || len(args) < len(want) {
		return false
	}
	for i := 0; i+len(want) <= len(args); i++ {
		match := true
		for j := range want {
			if args[i+j] != want[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
