package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	pkgexec "github.com/systmms/dsops/pkg/exec"
	"github.com/systmms/dsops/pkg/provider"
)

// bwsLookPath locates the bws CLI on disk. Overridden in tests to allow
// mock-executor-based unit tests to run without bws installed.
var bwsLookPath = func() error {
	_, err := exec.LookPath("bws")
	return err
}

// uuidRegex matches Bitwarden Secrets Manager secret/project IDs.
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

const defaultBwsAccessTokenEnv = "BWS_ACCESS_TOKEN"

// BitwardenSecretsManagerProvider wraps the `bws` CLI to retrieve secrets from
// Bitwarden Secrets Manager — a separate product from Bitwarden Password
// Manager (which is handled by BitwardenProvider).
type BitwardenSecretsManagerProvider struct {
	name           string
	accessTokenEnv string // env var name to read the access token from
	serverURL      string
	executor       pkgexec.CommandExecutor

	mu             sync.Mutex
	cachedToken    string                 // token under which the caches were populated; invalidates on change
	projects       []bwsProject           // cached project list
	projectsErr    error                  // cached project list error
	projectsCached bool                   // true once a list attempt has been made for cachedToken
	secrets        map[string][]bwsSecret // projectID -> secrets, cached per token
	secretsErr     map[string]error       // projectID -> last list error
}

// bwsSecret mirrors the JSON shape of `bws secret get|list`.
type bwsSecret struct {
	ID           string `json:"id"`
	Key          string `json:"key"`
	Value        string `json:"value"`
	Note         string `json:"note"`
	ProjectID    string `json:"projectId"`
	CreationDate string `json:"creationDate"`
	RevisionDate string `json:"revisionDate"`
}

// bwsProject mirrors the JSON shape of `bws project get|list`.
type bwsProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// NewBitwardenSecretsManagerProvider creates a provider with the default
// command executor.
func NewBitwardenSecretsManagerProvider(name string, config map[string]interface{}) *BitwardenSecretsManagerProvider {
	p := &BitwardenSecretsManagerProvider{
		name:           name,
		accessTokenEnv: defaultBwsAccessTokenEnv,
		executor:       pkgexec.DefaultExecutor(),
		secrets:        map[string][]bwsSecret{},
		secretsErr:     map[string]error{},
	}
	applyBwsConfig(p, config)
	return p
}

// NewBitwardenSecretsManagerProviderWithExecutor creates a provider with a
// custom executor — used in unit tests.
func NewBitwardenSecretsManagerProviderWithExecutor(name string, config map[string]interface{}, executor pkgexec.CommandExecutor) *BitwardenSecretsManagerProvider {
	p := &BitwardenSecretsManagerProvider{
		name:           name,
		accessTokenEnv: defaultBwsAccessTokenEnv,
		executor:       executor,
		secrets:        map[string][]bwsSecret{},
		secretsErr:     map[string]error{},
	}
	applyBwsConfig(p, config)
	return p
}

func applyBwsConfig(p *BitwardenSecretsManagerProvider, config map[string]interface{}) {
	if v, ok := config["access_token_env"].(string); ok && v != "" {
		p.accessTokenEnv = v
	}
	if v, ok := config["server_url"].(string); ok {
		p.serverURL = v
	}
}

// Name returns the provider's configured name.
func (p *BitwardenSecretsManagerProvider) Name() string {
	return p.name
}

// Capabilities reports what this provider supports.
func (p *BitwardenSecretsManagerProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: false,
		SupportsMetadata:   true,
		SupportsWatching:   false,
		SupportsBinary:     false,
		RequiresAuth:       true,
		AuthMethods:        []string{"access-token"},
	}
}

// Validate checks that the bws CLI is available and the access token env var
// is set. An auth probe (`bws project list`) confirms the token is accepted.
func (p *BitwardenSecretsManagerProvider) Validate(ctx context.Context) error {
	if err := bwsLookPath(); err != nil {
		return fmt.Errorf("bitwarden secrets manager CLI 'bws' not found in PATH. Install: https://bitwarden.com/help/secrets-manager-cli/")
	}

	token, err := p.accessToken()
	if err != nil {
		return err
	}

	args := p.baseArgs(token)
	args = append(args, "project", "list")
	if _, _, err := p.executor.Execute(ctx, "bws", args...); err != nil {
		return provider.AuthError{
			Provider: p.name,
			Message:  fmt.Sprintf("auth probe (bws project list) failed: %v", err),
		}
	}
	return nil
}

// Resolve retrieves a secret value by UUID or by project/key path.
func (p *BitwardenSecretsManagerProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	token, err := p.accessToken()
	if err != nil {
		return provider.SecretValue{}, err
	}

	secret, err := p.lookup(ctx, token, ref.Key)
	if err != nil {
		return provider.SecretValue{}, err
	}

	value := secret.Value
	if ref.Field == "note" {
		value = secret.Note
	}

	return provider.SecretValue{
		Value:     value,
		Version:   secret.RevisionDate,
		UpdatedAt: parseTimestamp(secret.RevisionDate),
		Metadata: map[string]string{
			"provider":   p.name,
			"secret_id":  secret.ID,
			"secret_key": secret.Key,
			"project_id": secret.ProjectID,
		},
	}, nil
}

// Describe returns metadata about a secret without exposing its value.
func (p *BitwardenSecretsManagerProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	token, err := p.accessToken()
	if err != nil {
		return provider.Metadata{}, err
	}

	secret, err := p.lookup(ctx, token, ref.Key)
	if err != nil {
		var notFound *provider.NotFoundError
		if errors.As(err, &notFound) {
			return provider.Metadata{Exists: false}, nil
		}
		return provider.Metadata{}, err
	}

	return provider.Metadata{
		Exists:    true,
		Version:   secret.RevisionDate,
		UpdatedAt: parseTimestamp(secret.RevisionDate),
		Size:      len(secret.Value),
		Type:      "secret",
		Tags: map[string]string{
			"provider":   p.name,
			"secret_key": secret.Key,
			"project_id": secret.ProjectID,
		},
	}, nil
}

func (p *BitwardenSecretsManagerProvider) lookup(ctx context.Context, token, key string) (*bwsSecret, error) {
	if uuidRegex.MatchString(key) {
		return p.getByUUID(ctx, token, key)
	}

	parts := strings.SplitN(key, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("bws secret key must be a UUID or '<projectName>/<secretKey>', got %q", key)
	}
	projectName, secretKey := parts[0], parts[1]
	return p.getByPath(ctx, token, projectName, secretKey)
}

func (p *BitwardenSecretsManagerProvider) getByUUID(ctx context.Context, token, uuid string) (*bwsSecret, error) {
	args := p.baseArgs(token)
	args = append(args, "secret", "get", uuid)

	stdout, stderr, err := p.executor.Execute(ctx, "bws", args...)
	if err != nil {
		if isBwsNotFound(err, stderr) {
			return nil, &provider.NotFoundError{Provider: p.name, Key: uuid}
		}
		return nil, fmt.Errorf("bws secret get %s: %w", uuid, err)
	}

	var secret bwsSecret
	if err := json.Unmarshal(stdout, &secret); err != nil {
		return nil, fmt.Errorf("failed to parse bws secret response: %w", err)
	}
	return &secret, nil
}

func (p *BitwardenSecretsManagerProvider) getByPath(ctx context.Context, token, projectName, secretKey string) (*bwsSecret, error) {
	projects, err := p.listProjects(ctx, token)
	if err != nil {
		return nil, err
	}

	var matchingIDs []string
	for _, pr := range projects {
		if pr.Name == projectName {
			matchingIDs = append(matchingIDs, pr.ID)
		}
	}
	if len(matchingIDs) == 0 {
		return nil, &provider.NotFoundError{Provider: p.name, Key: projectName + "/" + secretKey}
	}
	if len(matchingIDs) > 1 {
		return nil, fmt.Errorf("ambiguous project name %q matches %d projects: %s", projectName, len(matchingIDs), strings.Join(matchingIDs, ", "))
	}
	projectID := matchingIDs[0]

	secrets, err := p.listSecrets(ctx, token, projectID)
	if err != nil {
		return nil, err
	}

	var matches []bwsSecret
	for _, s := range secrets {
		if s.Key == secretKey {
			matches = append(matches, s)
		}
	}
	if len(matches) == 0 {
		return nil, &provider.NotFoundError{Provider: p.name, Key: projectName + "/" + secretKey}
	}
	if len(matches) > 1 {
		ids := make([]string, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, m.ID)
		}
		return nil, fmt.Errorf("ambiguous secret key %q in project %q matches %d secrets: %s", secretKey, projectName, len(matches), strings.Join(ids, ", "))
	}
	matched := matches[0]
	return &matched, nil
}

// invalidateCacheIfTokenChanged clears the project/secret caches when a
// different access token is observed than the one used to populate them. This
// prevents long-lived processes from reusing one tenant's cache for another.
// Caller must hold p.mu.
func (p *BitwardenSecretsManagerProvider) invalidateCacheIfTokenChanged(token string) {
	if p.cachedToken == token {
		return
	}
	p.cachedToken = token
	p.projects = nil
	p.projectsErr = nil
	p.projectsCached = false
	p.secrets = map[string][]bwsSecret{}
	p.secretsErr = map[string]error{}
}

// listProjects returns the cached project list, fetching it on first call.
// The mutex is held across the CLI call so concurrent callers see a single
// fetch instead of issuing duplicate `bws project list` invocations.
func (p *BitwardenSecretsManagerProvider) listProjects(ctx context.Context, token string) ([]bwsProject, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.invalidateCacheIfTokenChanged(token)

	if p.projectsCached {
		return p.projects, p.projectsErr
	}

	args := p.baseArgs(token)
	args = append(args, "project", "list")
	stdout, _, err := p.executor.Execute(ctx, "bws", args...)
	p.projectsCached = true
	if err != nil {
		p.projectsErr = fmt.Errorf("bws project list: %w", err)
		return nil, p.projectsErr
	}
	var projects []bwsProject
	if err := json.Unmarshal(stdout, &projects); err != nil {
		p.projectsErr = fmt.Errorf("failed to parse bws project list: %w", err)
		return nil, p.projectsErr
	}
	p.projects = projects
	return projects, nil
}

// listSecrets returns the cached secret list for a project, fetching it on
// first call. The mutex is held across the CLI call to prevent duplicate
// fetches under concurrent resolution.
func (p *BitwardenSecretsManagerProvider) listSecrets(ctx context.Context, token, projectID string) ([]bwsSecret, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.invalidateCacheIfTokenChanged(token)

	if cached, ok := p.secrets[projectID]; ok {
		return cached, nil
	}
	if cachedErr, ok := p.secretsErr[projectID]; ok {
		return nil, cachedErr
	}

	// `bws secret list` takes the project filter as a POSITIONAL argument
	// (Option<Uuid>), not a --project-id flag. The flag form would error in
	// real bws invocations even though prefix-matching mocks would still pass.
	args := p.baseArgs(token)
	args = append(args, "secret", "list", projectID)
	stdout, _, err := p.executor.Execute(ctx, "bws", args...)
	if err != nil {
		p.secretsErr[projectID] = fmt.Errorf("bws secret list %s: %w", projectID, err)
		return nil, p.secretsErr[projectID]
	}
	var secrets []bwsSecret
	if err := json.Unmarshal(stdout, &secrets); err != nil {
		p.secretsErr[projectID] = fmt.Errorf("failed to parse bws secret list response: %w", err)
		return nil, p.secretsErr[projectID]
	}
	p.secrets[projectID] = secrets
	return secrets, nil
}

// baseArgs returns the global flags every bws invocation needs. Returns a
// fresh slice each call so callers can append safely.
//
// When the access token comes from the default BWS_ACCESS_TOKEN env var, the
// flag is omitted: bws inherits the variable from this process and reads it
// natively, keeping the secret out of /proc/PID/cmdline. When a custom env
// var name is configured, the token is passed via --access-token (visible in
// `ps`); this trade-off is documented.
func (p *BitwardenSecretsManagerProvider) baseArgs(token string) []string {
	args := []string{"--output", "json"}
	if p.serverURL != "" {
		args = append(args, "--server-url", p.serverURL)
	}
	if p.accessTokenEnv != defaultBwsAccessTokenEnv {
		args = append(args, "--access-token", token)
	}
	return args
}

func (p *BitwardenSecretsManagerProvider) accessToken() (string, error) {
	tok := os.Getenv(p.accessTokenEnv)
	if tok == "" {
		return "", provider.AuthError{
			Provider: p.name,
			Message:  fmt.Sprintf("missing access token: set %s", p.accessTokenEnv),
		}
	}
	return tok, nil
}

func isBwsNotFound(err error, stderr []byte) bool {
	if err == nil {
		return false
	}
	s := string(stderr) + " " + err.Error()
	low := strings.ToLower(s)
	return strings.Contains(low, "not found") || strings.Contains(low, "404")
}
