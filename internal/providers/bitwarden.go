package providers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	pkgexec "github.com/systmms/dsops/pkg/exec"
	"github.com/systmms/dsops/pkg/provider"
)

// bwLookPath locates the bw CLI on disk. Overridden in tests to allow
// mock-executor-based unit tests to run without bw installed.
var bwLookPath = func() error {
	_, err := exec.LookPath("bw")
	return err
}

// BitwardenProvider implements the provider interface for Bitwarden
type BitwardenProvider struct {
	name     string
	profile  string // Optional profile name passed via --session
	sync     bool   // If true, runs `bw sync` once before the first Resolve/Describe
	headless bool   // If true, Validate will attempt API-key login + passwordenv unlock
	executor pkgexec.CommandExecutor

	mu       sync.Mutex // guards session
	session  string     // captured from `bw unlock --raw` when headless
	syncOnce sync.Once
	syncErr  error
	authOnce sync.Once // headless auth attempted at most once per provider lifetime
	authErr  error
}

// NewBitwardenProvider creates a new Bitwarden provider
func NewBitwardenProvider(name string, config map[string]interface{}) *BitwardenProvider {
	bw := &BitwardenProvider{
		name:     name,
		executor: pkgexec.DefaultExecutor(),
	}
	applyBitwardenConfig(bw, config)
	return bw
}

// NewBitwardenProviderWithExecutor creates a new Bitwarden provider with a custom executor.
// This is primarily for testing, allowing command execution to be mocked.
func NewBitwardenProviderWithExecutor(name string, config map[string]interface{}, executor pkgexec.CommandExecutor) *BitwardenProvider {
	bw := &BitwardenProvider{
		name:     name,
		executor: executor,
	}
	applyBitwardenConfig(bw, config)
	return bw
}

func applyBitwardenConfig(bw *BitwardenProvider, config map[string]interface{}) {
	if profile, ok := config["profile"].(string); ok {
		bw.profile = profile
	}
	if v, ok := config["sync"].(bool); ok {
		bw.sync = v
	}
	if v, ok := config["headless"].(bool); ok {
		bw.headless = v
	}
}

// sessionArg returns the value to pass after --session when invoking bw, or
// the empty string when no session is configured. Headless unlock takes
// precedence over the profile name.
func (bw *BitwardenProvider) sessionArg() string {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	if bw.session != "" {
		return bw.session
	}
	return bw.profile
}

func (bw *BitwardenProvider) appendSessionArg(args []string) []string {
	if s := bw.sessionArg(); s != "" {
		return append(args, "--session", s)
	}
	return args
}

// ensureSync runs `bw sync` once per provider lifetime when sync is enabled.
// Errors are recorded but do not block resolution; a failed sync simply means
// resolutions may see the previously-cached vault state.
func (bw *BitwardenProvider) ensureSync(ctx context.Context) {
	if !bw.sync {
		return
	}
	bw.syncOnce.Do(func() {
		args := bw.appendSessionArg([]string{"sync"})
		if _, _, err := bw.executor.Execute(ctx, "bw", args...); err != nil {
			bw.syncErr = err
		}
	})
}

// Name returns the provider name
func (bw *BitwardenProvider) Name() string {
	return bw.name
}

// Resolve retrieves a secret from Bitwarden
func (bw *BitwardenProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	if err := bw.ensureHeadlessAuth(ctx); err != nil {
		return provider.SecretValue{}, err
	}
	bw.ensureSync(ctx)

	// Parse the key format: item-id[.field], item-id.custom.<name>, or item-id.attachment.<filename>
	itemID, field := bw.parseKey(ref.Key)

	// Attachments are fetched via a separate bw command and returned base64-encoded.
	if filename, ok := strings.CutPrefix(field, "attachment:"); ok {
		return bw.resolveAttachment(ctx, itemID, filename)
	}

	// Get the item from Bitwarden
	item, err := bw.getItem(ctx, itemID)
	if err != nil {
		return provider.SecretValue{}, err
	}

	// Extract the requested field
	value, err := bw.extractField(item, field)
	if err != nil {
		return provider.SecretValue{}, fmt.Errorf("failed to extract field '%s': %w", field, err)
	}

	return provider.SecretValue{
		Value:     value,
		Version:   item.RevisionDate,
		UpdatedAt: parseTimestamp(item.RevisionDate),
		Metadata: map[string]string{
			"provider":     bw.name,
			"item_id":      item.ID,
			"item_name":    item.Name,
			"organization": item.OrganizationID,
			"folder":       item.FolderID,
		},
	}, nil
}

// resolveAttachment fetches an attachment by filename for the given item and
// returns it base64-encoded (per the SecretValue.Value contract for binary data).
func (bw *BitwardenProvider) resolveAttachment(ctx context.Context, itemID, filename string) (provider.SecretValue, error) {
	item, err := bw.getItem(ctx, itemID)
	if err != nil {
		return provider.SecretValue{}, err
	}

	data, err := bw.getAttachment(ctx, item.ID, filename)
	if err != nil {
		return provider.SecretValue{}, fmt.Errorf("failed to retrieve attachment '%s': %w", filename, err)
	}

	return provider.SecretValue{
		Value:     base64.StdEncoding.EncodeToString(data),
		Version:   item.RevisionDate,
		UpdatedAt: parseTimestamp(item.RevisionDate),
		Metadata: map[string]string{
			"provider":     bw.name,
			"item_id":      item.ID,
			"item_name":    item.Name,
			"organization": item.OrganizationID,
			"folder":       item.FolderID,
			"attachment":   filename,
			"content_type": "application/octet-stream",
		},
	}, nil
}

// Describe returns metadata about a Bitwarden item
func (bw *BitwardenProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	if err := bw.ensureHeadlessAuth(ctx); err != nil {
		return provider.Metadata{}, err
	}
	bw.ensureSync(ctx)

	itemID, _ := bw.parseKey(ref.Key)

	item, err := bw.getItem(ctx, itemID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return provider.Metadata{Exists: false}, nil
		}
		return provider.Metadata{}, err
	}

	return provider.Metadata{
		Exists:    true,
		Version:   item.RevisionDate,
		UpdatedAt: parseTimestamp(item.RevisionDate),
		Size:      len(fmt.Sprintf("%+v", item)), // Rough size estimate
		Type:      fmt.Sprintf("type-%d", item.Type),
		Tags: map[string]string{
			"provider":     bw.name,
			"item_name":    item.Name,
			"organization": item.OrganizationID,
			"folder":       item.FolderID,
		},
	}, nil
}

// Capabilities returns Bitwarden provider capabilities
func (bw *BitwardenProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: false, // Bitwarden doesn't have explicit versioning
		SupportsMetadata:   true,
		SupportsWatching:   false,
		SupportsBinary:     true, // Attachments are returned base64-encoded
		RequiresAuth:       true,
		AuthMethods:        []string{"cli-session", "api-key"},
	}
}

// Validate checks if Bitwarden CLI is available and authenticated.
//
// When the provider is configured with headless: true, Validate will also
// attempt to recover from unauthenticated/locked states using API-key login
// and passwordenv unlock — see headlessLogin and headlessUnlock.
func (bw *BitwardenProvider) Validate(ctx context.Context) error {
	if err := bwLookPath(); err != nil {
		return fmt.Errorf("bitwarden CLI 'bw' not found in PATH. Install from: https://bitwarden.com/help/cli/")
	}

	status, err := bw.recoverAuthIfHeadless(ctx)
	if err != nil {
		return err
	}

	switch status {
	case "unauthenticated":
		return provider.AuthError{
			Provider: bw.name,
			Message:  "not logged in. Run: bw login (or set headless: true with BW_CLIENTID / BW_CLIENTSECRET)",
		}
	case "locked":
		return provider.AuthError{
			Provider: bw.name,
			Message:  "vault is locked. Run: bw unlock (or set headless: true with BW_PASSWORD)",
		}
	case "unlocked":
		return nil
	default:
		return provider.AuthError{
			Provider: bw.name,
			Message:  fmt.Sprintf("unknown status: %s", status),
		}
	}
}

// recoverAuthIfHeadless checks bw status, optionally drives `bw login --apikey`
// and `bw unlock --passwordenv` when headless is enabled, and returns the final
// status string.
func (bw *BitwardenProvider) recoverAuthIfHeadless(ctx context.Context) (string, error) {
	status, err := bw.fetchStatus(ctx)
	if err != nil {
		return "", err
	}

	if status == "unauthenticated" && bw.headless {
		if err := bw.headlessLogin(ctx); err != nil {
			return "", err
		}
		status, err = bw.fetchStatus(ctx)
		if err != nil {
			return "", err
		}
	}

	if status == "locked" && bw.headless {
		if err := bw.headlessUnlock(ctx); err != nil {
			return "", err
		}
		status, err = bw.fetchStatus(ctx)
		if err != nil {
			return "", err
		}
	}

	return status, nil
}

// ensureHeadlessAuth performs the headless auth recovery flow at most once per
// provider lifetime when headless mode is enabled. Resolve and Describe call
// this so the headless flow runs even when the caller skips Validate.
//
// When headless is disabled, this is a no-op — the existing behavior of letting
// the underlying bw call surface "vault is locked" is preserved so users in
// interactive mode still receive the actionable error message.
func (bw *BitwardenProvider) ensureHeadlessAuth(ctx context.Context) error {
	if !bw.headless {
		return nil
	}
	bw.authOnce.Do(func() {
		if _, err := bw.recoverAuthIfHeadless(ctx); err != nil {
			bw.authErr = err
		}
	})
	return bw.authErr
}

// fetchStatus invokes `bw status` and returns the status string from the
// resulting JSON, or an error.
func (bw *BitwardenProvider) fetchStatus(ctx context.Context) (string, error) {
	args := bw.appendSessionArg([]string{"status"})
	output, _, err := bw.executor.Execute(ctx, "bw", args...)
	if err != nil {
		return "", fmt.Errorf("failed to check bitwarden status: %w", err)
	}
	var status BitwardenStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return "", fmt.Errorf("failed to parse bitwarden status: %w", err)
	}
	return status.Status, nil
}

// headlessLogin runs `bw login --apikey`. The bw CLI itself reads the client
// credentials from BW_CLIENTID and BW_CLIENTSECRET in the process environment.
func (bw *BitwardenProvider) headlessLogin(ctx context.Context) error {
	if os.Getenv("BW_CLIENTID") == "" || os.Getenv("BW_CLIENTSECRET") == "" {
		return provider.AuthError{
			Provider: bw.name,
			Message:  "headless login requires BW_CLIENTID and BW_CLIENTSECRET env vars",
		}
	}
	if _, _, err := bw.executor.Execute(ctx, "bw", "login", "--apikey"); err != nil {
		return provider.AuthError{
			Provider: bw.name,
			Message:  fmt.Sprintf("bw login --apikey failed: %v", err),
		}
	}
	return nil
}

// headlessUnlock runs `bw unlock --passwordenv BW_PASSWORD --raw` and stores
// the resulting session token for use in subsequent calls.
func (bw *BitwardenProvider) headlessUnlock(ctx context.Context) error {
	if os.Getenv("BW_PASSWORD") == "" {
		return provider.AuthError{
			Provider: bw.name,
			Message:  "headless unlock requires BW_PASSWORD env var",
		}
	}
	stdout, _, err := bw.executor.Execute(ctx, "bw", "unlock", "--passwordenv", "BW_PASSWORD", "--raw")
	if err != nil {
		return provider.AuthError{
			Provider: bw.name,
			Message:  fmt.Sprintf("bw unlock failed: %v", err),
		}
	}
	token := strings.TrimSpace(string(stdout))
	if token == "" {
		return provider.AuthError{
			Provider: bw.name,
			Message:  "bw unlock returned an empty session token",
		}
	}
	bw.mu.Lock()
	bw.session = token
	bw.mu.Unlock()
	return nil
}

// parseKey parses a Bitwarden key into item ID/name and field.
//
// Formats supported:
//   - "item"                       -> ("item", "")               // per-type default applied in extractField
//   - "item.field"                 -> ("item", "field")          // direct field on the item
//   - "item.custom.<name>"         -> ("item", "custom:<name>")  // explicit custom field; preserves dots in <name>
//   - "item.attachment.<filename>" -> ("item", "attachment:<filename>") // attachment retrieval; preserves filename dots
//
// Item names may not contain dots; this is a known limitation.
//
// When no field is specified the empty string is returned. extractField then
// applies a per-item-type default (Login→password, Card→number,
// Identity→email, SshKey→privateKey, Note→notes).
func (bw *BitwardenProvider) parseKey(key string) (itemID, field string) {
	parts := strings.SplitN(key, ".", 3)
	itemID = parts[0]

	if len(parts) == 1 {
		return itemID, ""
	}

	if len(parts) == 3 {
		switch parts[1] {
		case "custom":
			return itemID, "custom:" + parts[2]
		case "attachment":
			return itemID, "attachment:" + parts[2]
		}
	}

	return itemID, parts[1]
}

// defaultFieldForType returns the field name extractField uses when the user
// addresses an item without specifying a field (e.g. "my-card" with no
// trailing ".field").
func defaultFieldForType(t BitwardenItemType) string {
	switch t {
	case TypeLogin:
		return "password"
	case TypeNote:
		return "notes"
	case TypeCard:
		return "number"
	case TypeIdentity:
		return "email"
	case TypeSshKey:
		return "privateKey"
	default:
		return "password"
	}
}

// getItem retrieves an item from Bitwarden by ID or name
func (bw *BitwardenProvider) getItem(ctx context.Context, itemID string) (*BitwardenItem, error) {
	args := bw.appendSessionArg([]string{"get", "item", itemID})

	stdout, stderr, err := bw.executor.Execute(ctx, "bw", args...)
	if err != nil {
		stderrStr := string(stderr)
		errStr := err.Error()
		if strings.Contains(stderrStr, "Not found") || strings.Contains(stderrStr, "not found") ||
			strings.Contains(errStr, "Not found") || strings.Contains(errStr, "not found") {
			return nil, &provider.NotFoundError{
				Provider: bw.name,
				Key:      itemID,
			}
		}
		return nil, fmt.Errorf("failed to get bitwarden item '%s': %w", itemID, err)
	}

	var item BitwardenItem
	if err := json.Unmarshal(stdout, &item); err != nil {
		return nil, fmt.Errorf("failed to parse bitwarden item: %w", err)
	}

	return &item, nil
}

// extractField extracts a specific field from a Bitwarden item.
//
// Field handling:
//   - "" (empty)            -> per-item-type default (see defaultFieldForType)
//   - "custom:<name>"       -> custom field by name (explicit form)
//   - "name"                -> the item display name
//   - type-specific names   -> dispatched to extractLoginField / extractCardField / extractIdentityField / extractSshKeyField
//   - bare names            -> fall back to custom-field-by-name lookup for backward compatibility
//   - "uri", "uri0"...      -> indexed URIs from a Login item
//
// Attachment retrieval is handled in Resolve, not here, since it requires a
// separate bw command and returns binary data.
func (bw *BitwardenProvider) extractField(item *BitwardenItem, field string) (string, error) {
	if field == "" {
		field = defaultFieldForType(item.Type)
	}

	// Catch incomplete user-friendly forms like "item.custom" / "item.attachment"
	// that lack the third dot-separated part, before they hit the catch-all
	// "field not found" path.
	if field == "custom" {
		return "", fmt.Errorf("incomplete key: custom field name is missing (expected 'item.custom.<field-name>')")
	}
	if field == "attachment" {
		return "", fmt.Errorf("incomplete key: attachment filename is missing (expected 'item.attachment.<filename>')")
	}

	if name, ok := strings.CutPrefix(field, "custom:"); ok {
		for _, customField := range item.Fields {
			if customField.Name == name {
				return customField.Value, nil
			}
		}
		return "", fmt.Errorf("custom field '%s' not found", name)
	}

	if field == "name" {
		return item.Name, nil
	}

	if field == "notes" {
		if item.Notes != "" {
			return item.Notes, nil
		}
		return "", fmt.Errorf("no notes field found")
	}

	switch item.Type {
	case TypeCard:
		if v, err, handled := bw.extractCardField(item, field); handled {
			return v, err
		}
	case TypeIdentity:
		if v, err, handled := bw.extractIdentityField(item, field); handled {
			return v, err
		}
	case TypeSshKey:
		if v, err, handled := bw.extractSshKeyField(item, field); handled {
			return v, err
		}
	}

	// Login fields work for any item that has a Login object (which is rare
	// outside TypeLogin) and are also the default branch for TypeLogin.
	if v, err, handled := bw.extractLoginField(item, field); handled {
		return v, err
	}

	// Back-compat: bare field name matches a custom field
	for _, customField := range item.Fields {
		if customField.Name == field {
			return customField.Value, nil
		}
	}

	return "", fmt.Errorf("field '%s' not found", field)
}

// extractLoginField returns the requested Login field, or (handled=false) if
// the field name is not a Login concept.
func (bw *BitwardenProvider) extractLoginField(item *BitwardenItem, field string) (string, error, bool) {
	switch field {
	case "password":
		if item.Login != nil && item.Login.Password != "" {
			return item.Login.Password, nil, true
		}
		return "", fmt.Errorf("no password field found"), true
	case "username":
		if item.Login != nil && item.Login.Username != "" {
			return item.Login.Username, nil, true
		}
		return "", fmt.Errorf("no username field found"), true
	case "totp":
		if item.Login != nil && item.Login.Totp != "" {
			return item.Login.Totp, nil, true
		}
		return "", fmt.Errorf("no TOTP field found"), true
	}
	if strings.HasPrefix(field, "uri") && item.Login != nil {
		v, err := bw.extractUriField(item, field)
		return v, err, true
	}
	return "", nil, false
}

// extractCardField returns the requested Card field. The third return value
// indicates whether the field is a Card concept; callers should fall through
// to other lookups (e.g. custom fields) when handled=false.
func (bw *BitwardenProvider) extractCardField(item *BitwardenItem, field string) (string, error, bool) {
	if item.Card == nil {
		// Field names below are Card-specific; if we see one, surface a clearer error.
		switch field {
		case "number", "code", "cvv", "cardholderName", "brand", "expMonth", "expYear":
			return "", fmt.Errorf("no card data on item"), true
		}
		return "", nil, false
	}
	switch field {
	case "number":
		return item.Card.Number, nil, true
	case "code", "cvv":
		return item.Card.Code, nil, true
	case "cardholderName":
		return item.Card.CardholderName, nil, true
	case "brand":
		return item.Card.Brand, nil, true
	case "expMonth":
		return item.Card.ExpMonth, nil, true
	case "expYear":
		return item.Card.ExpYear, nil, true
	}
	return "", nil, false
}

// extractIdentityField returns the requested Identity field.
func (bw *BitwardenProvider) extractIdentityField(item *BitwardenItem, field string) (string, error, bool) {
	if item.Identity == nil {
		return "", nil, false
	}
	switch field {
	case "title":
		return item.Identity.Title, nil, true
	case "firstName":
		return item.Identity.FirstName, nil, true
	case "middleName":
		return item.Identity.MiddleName, nil, true
	case "lastName":
		return item.Identity.LastName, nil, true
	case "address1":
		return item.Identity.Address1, nil, true
	case "address2":
		return item.Identity.Address2, nil, true
	case "address3":
		return item.Identity.Address3, nil, true
	case "city":
		return item.Identity.City, nil, true
	case "state":
		return item.Identity.State, nil, true
	case "postalCode":
		return item.Identity.PostalCode, nil, true
	case "country":
		return item.Identity.Country, nil, true
	case "company":
		return item.Identity.Company, nil, true
	case "email":
		return item.Identity.Email, nil, true
	case "phone":
		return item.Identity.Phone, nil, true
	case "ssn":
		return item.Identity.SSN, nil, true
	case "username":
		return item.Identity.Username, nil, true
	case "passportNumber":
		return item.Identity.PassportNumber, nil, true
	case "licenseNumber":
		return item.Identity.LicenseNumber, nil, true
	}
	return "", nil, false
}

// extractSshKeyField returns the requested SshKey field.
func (bw *BitwardenProvider) extractSshKeyField(item *BitwardenItem, field string) (string, error, bool) {
	if item.SshKey == nil {
		switch field {
		case "privateKey", "publicKey", "keyFingerprint":
			return "", fmt.Errorf("no ssh key data on item"), true
		}
		return "", nil, false
	}
	switch field {
	case "privateKey":
		return item.SshKey.PrivateKey, nil, true
	case "publicKey":
		return item.SshKey.PublicKey, nil, true
	case "keyFingerprint":
		return item.SshKey.KeyFingerprint, nil, true
	}
	return "", nil, false
}

// getAttachment retrieves the raw bytes of a named attachment on a Bitwarden item.
// Uses `bw get attachment <filename> --itemid <id> --raw`; the --raw flag is
// what makes bw write the attachment bytes to stdout instead of saving to a file.
func (bw *BitwardenProvider) getAttachment(ctx context.Context, itemID, filename string) ([]byte, error) {
	args := bw.appendSessionArg([]string{"get", "attachment", filename, "--itemid", itemID, "--raw"})

	stdout, stderr, err := bw.executor.Execute(ctx, "bw", args...)
	if err != nil {
		stderrStr := string(stderr)
		errStr := err.Error()
		if strings.Contains(stderrStr, "Not found") || strings.Contains(stderrStr, "not found") ||
			strings.Contains(errStr, "Not found") || strings.Contains(errStr, "not found") {
			return nil, &provider.NotFoundError{
				Provider: bw.name,
				Key:      itemID + " attachment:" + filename,
			}
		}
		return nil, fmt.Errorf("failed to get bitwarden attachment '%s' from item '%s': %w", filename, itemID, err)
	}

	return stdout, nil
}

// extractUriField extracts URI-related fields
func (bw *BitwardenProvider) extractUriField(item *BitwardenItem, field string) (string, error) {
	if item.Login == nil || len(item.Login.Uris) == 0 {
		return "", fmt.Errorf("no URI fields found")
	}

	// Parse field like "uri0", "uri1", or just "uri" (defaults to uri0)
	index := 0
	if len(field) > 3 {
		indexStr := field[3:]
		if i, err := strconv.Atoi(indexStr); err == nil {
			index = i
		}
	}

	if index >= len(item.Login.Uris) {
		return "", fmt.Errorf("URI index %d not found", index)
	}

	return item.Login.Uris[index].URI, nil
}

// parseTimestamp converts Bitwarden timestamp to time.Time
func parseTimestamp(timestamp string) time.Time {
	if timestamp == "" {
		return time.Time{}
	}

	// Bitwarden uses ISO 8601 format
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		// Fallback to current time if parsing fails
		return time.Now()
	}
	return t
}

// Bitwarden data structures

// BitwardenStatus represents the status response from 'bw status'
type BitwardenStatus struct {
	Status    string `json:"status"`
	LastSync  string `json:"lastSync"`
	UserEmail string `json:"userEmail"`
	UserID    string `json:"userId"`
	Template  string `json:"template"`
}

// BitwardenItemType represents the type of Bitwarden item
type BitwardenItemType int

const (
	TypeLogin    BitwardenItemType = 1
	TypeNote     BitwardenItemType = 2
	TypeCard     BitwardenItemType = 3
	TypeIdentity BitwardenItemType = 4
	TypeSshKey   BitwardenItemType = 5
)

// BitwardenItem represents a Bitwarden vault item
type BitwardenItem struct {
	ID             string             `json:"id"`
	OrganizationID string             `json:"organizationId"`
	FolderID       string             `json:"folderId"`
	Type           BitwardenItemType  `json:"type"`
	Name           string             `json:"name"`
	Notes          string             `json:"notes"`
	Favorite       bool               `json:"favorite"`
	Fields         []BitwardenField   `json:"fields"`
	Login          *BitwardenLogin    `json:"login"`
	Card           *BitwardenCard     `json:"card"`
	Identity       *BitwardenIdentity `json:"identity"`
	SshKey         *BitwardenSshKey   `json:"sshKey"`
	CollectionIds  []string           `json:"collectionIds"`
	RevisionDate   string             `json:"revisionDate"`
	CreationDate   string             `json:"creationDate"`
	DeletedDate    string             `json:"deletedDate"`
}

// BitwardenLogin represents login-specific data
type BitwardenLogin struct {
	Username string         `json:"username"`
	Password string         `json:"password"`
	Totp     string         `json:"totp"`
	Uris     []BitwardenUri `json:"uris"`
}

// BitwardenUri represents a URI associated with a login item
type BitwardenUri struct {
	Match int    `json:"match"`
	URI   string `json:"uri"`
}

// BitwardenField represents a custom field in a Bitwarden item
type BitwardenField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  int    `json:"type"`
}

// BitwardenCard represents card-specific data. The Code field is the CVV.
type BitwardenCard struct {
	CardholderName string `json:"cardholderName"`
	Brand          string `json:"brand"`
	Number         string `json:"number"`
	ExpMonth       string `json:"expMonth"`
	ExpYear        string `json:"expYear"`
	Code           string `json:"code"`
}

// BitwardenIdentity represents identity-specific data
type BitwardenIdentity struct {
	Title          string `json:"title"`
	FirstName      string `json:"firstName"`
	MiddleName     string `json:"middleName"`
	LastName       string `json:"lastName"`
	Address1       string `json:"address1"`
	Address2       string `json:"address2"`
	Address3       string `json:"address3"`
	City           string `json:"city"`
	State          string `json:"state"`
	PostalCode     string `json:"postalCode"`
	Country        string `json:"country"`
	Company        string `json:"company"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	SSN            string `json:"ssn"`
	Username       string `json:"username"`
	PassportNumber string `json:"passportNumber"`
	LicenseNumber  string `json:"licenseNumber"`
}

// BitwardenSshKey represents an SSH key item. Field names mirror the bw CLI JSON.
type BitwardenSshKey struct {
	PrivateKey     string `json:"privateKey"`
	PublicKey      string `json:"publicKey"`
	KeyFingerprint string `json:"keyFingerprint"`
}
