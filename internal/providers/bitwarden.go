package providers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	pkgexec "github.com/systmms/dsops/pkg/exec"
	"github.com/systmms/dsops/pkg/provider"
)

// BitwardenProvider implements the provider interface for Bitwarden
type BitwardenProvider struct {
	name     string
	profile  string // Optional profile name
	executor pkgexec.CommandExecutor
}

// NewBitwardenProvider creates a new Bitwarden provider
func NewBitwardenProvider(name string, config map[string]interface{}) *BitwardenProvider {
	bw := &BitwardenProvider{
		name:     name,
		executor: pkgexec.DefaultExecutor(),
	}

	// Extract profile from config
	if profile, ok := config["profile"].(string); ok {
		bw.profile = profile
	}

	return bw
}

// NewBitwardenProviderWithExecutor creates a new Bitwarden provider with a custom executor.
// This is primarily for testing, allowing command execution to be mocked.
func NewBitwardenProviderWithExecutor(name string, config map[string]interface{}, executor pkgexec.CommandExecutor) *BitwardenProvider {
	bw := &BitwardenProvider{
		name:     name,
		executor: executor,
	}

	// Extract profile from config
	if profile, ok := config["profile"].(string); ok {
		bw.profile = profile
	}

	return bw
}

// Name returns the provider name
func (bw *BitwardenProvider) Name() string {
	return bw.name
}

// Resolve retrieves a secret from Bitwarden
func (bw *BitwardenProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
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

	data, err := bw.getAttachment(ctx, itemID, filename)
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

// Validate checks if Bitwarden CLI is available and authenticated
func (bw *BitwardenProvider) Validate(ctx context.Context) error {
	// Check if bw CLI is available
	if _, err := exec.LookPath("bw"); err != nil {
		return fmt.Errorf("bitwarden CLI 'bw' not found in PATH. Install from: https://bitwarden.com/help/cli/")
	}

	// Check authentication status
	args := []string{"status"}
	if bw.profile != "" {
		args = append(args, "--session", bw.profile)
	}

	output, _, err := bw.executor.Execute(ctx, "bw", args...)
	if err != nil {
		return fmt.Errorf("failed to check bitwarden status: %w", err)
	}

	var status BitwardenStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return fmt.Errorf("failed to parse bitwarden status: %w", err)
	}

	switch status.Status {
	case "unauthenticated":
		return provider.AuthError{
			Provider: bw.name,
			Message:  "not logged in. Run: bw login",
		}
	case "locked":
		return provider.AuthError{
			Provider: bw.name,
			Message:  "vault is locked. Run: bw unlock",
		}
	case "unlocked":
		return nil
	default:
		return provider.AuthError{
			Provider: bw.name,
			Message:  fmt.Sprintf("unknown status: %s", status.Status),
		}
	}
}

// parseKey parses a Bitwarden key into item ID/name and field.
//
// Formats supported:
//   - "item"                       -> ("item", "password")
//   - "item.field"                 -> ("item", "field")           // direct field on the item
//   - "item.custom.<name>"         -> ("item", "custom:<name>")   // explicit custom field; allows dots in <name>
//   - "item.attachment.<filename>" -> ("item", "attachment:<filename>") // attachment retrieval; preserves filename dots
//
// Item names may not contain dots; this is a known limitation also present
// before this change.
func (bw *BitwardenProvider) parseKey(key string) (itemID, field string) {
	parts := strings.SplitN(key, ".", 3)
	itemID = parts[0]

	if len(parts) == 1 {
		return itemID, "password"
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

// getItem retrieves an item from Bitwarden by ID or name
func (bw *BitwardenProvider) getItem(ctx context.Context, itemID string) (*BitwardenItem, error) {
	args := []string{"get", "item", itemID}
	if bw.profile != "" {
		args = append(args, "--session", bw.profile)
	}

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
//   - well-known names (password, username, totp, notes, name) hit the typed branches
//   - "custom:<name>" looks up a custom field by name (explicit form)
//   - bare names fall back to custom-field-by-name lookup for backward compatibility
//   - "uri", "uri0", "uri1"... select indexed URIs from a Login item
//
// Attachment retrieval is handled in Resolve, not here, since it requires a
// separate bw command and returns binary data.
func (bw *BitwardenProvider) extractField(item *BitwardenItem, field string) (string, error) {
	if name, ok := strings.CutPrefix(field, "custom:"); ok {
		for _, customField := range item.Fields {
			if customField.Name == name {
				return customField.Value, nil
			}
		}
		return "", fmt.Errorf("custom field '%s' not found", name)
	}

	switch field {
	case "password":
		if item.Login != nil && item.Login.Password != "" {
			return item.Login.Password, nil
		}
		return "", fmt.Errorf("no password field found")

	case "username":
		if item.Login != nil && item.Login.Username != "" {
			return item.Login.Username, nil
		}
		return "", fmt.Errorf("no username field found")

	case "totp":
		if item.Login != nil && item.Login.Totp != "" {
			return item.Login.Totp, nil
		}
		return "", fmt.Errorf("no TOTP field found")

	case "notes":
		if item.Notes != "" {
			return item.Notes, nil
		}
		return "", fmt.Errorf("no notes field found")

	case "name":
		return item.Name, nil

	default:
		// Check custom fields (back-compat: bare field name without "custom:" prefix)
		for _, customField := range item.Fields {
			if customField.Name == field {
				return customField.Value, nil
			}
		}

		// Check if it's a URI field reference
		if strings.HasPrefix(field, "uri") && item.Login != nil {
			return bw.extractUriField(item, field)
		}

		return "", fmt.Errorf("field '%s' not found", field)
	}
}

// getAttachment retrieves the raw bytes of a named attachment on a Bitwarden item.
// Uses `bw get attachment <filename> --itemid <id> --raw`; the --raw flag is
// what makes bw write the attachment bytes to stdout instead of saving to a file.
func (bw *BitwardenProvider) getAttachment(ctx context.Context, itemID, filename string) ([]byte, error) {
	args := []string{"get", "attachment", filename, "--itemid", itemID, "--raw"}
	if bw.profile != "" {
		args = append(args, "--session", bw.profile)
	}

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
)

// BitwardenItem represents a Bitwarden vault item
type BitwardenItem struct {
	ID             string            `json:"id"`
	OrganizationID string            `json:"organizationId"`
	FolderID       string            `json:"folderId"`
	Type           BitwardenItemType `json:"type"`
	Name           string            `json:"name"`
	Notes          string            `json:"notes"`
	Favorite       bool              `json:"favorite"`
	Fields         []BitwardenField  `json:"fields"`
	Login          *BitwardenLogin   `json:"login"`
	CollectionIds  []string          `json:"collectionIds"`
	RevisionDate   string            `json:"revisionDate"`
	CreationDate   string            `json:"creationDate"`
	DeletedDate    string            `json:"deletedDate"`
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
