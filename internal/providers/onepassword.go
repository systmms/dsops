package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/systmms/dsops/pkg/provider"
)

// OnePasswordProvider implements the provider.Provider interface for 1Password CLI
type OnePasswordProvider struct {
	Account string `yaml:"account,omitempty"`
}

// NewOnePasswordProvider creates a new 1Password provider instance
func NewOnePasswordProvider(config map[string]interface{}) (provider.Provider, error) {
	p := &OnePasswordProvider{}
	
	if account, ok := config["account"].(string); ok {
		p.Account = account
	}

	return p, nil
}

func (op *OnePasswordProvider) Name() string {
	return "onepassword"
}

func (op *OnePasswordProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: false, // 1Password doesn't have version semantics like AWS
		SupportsMetadata:   true,  // Can get item metadata
		RequiresAuth:       true,  // Requires 'op signin'
		AuthMethods:        []string{"CLI session", "service account"},
	}
}

func (op *OnePasswordProvider) Validate(ctx context.Context) error {
	// Check if 'op' CLI is available
	if _, err := exec.LookPath("op"); err != nil {
		return fmt.Errorf("1Password CLI not found in PATH: %w", err)
	}

	// Check if user is signed in
	args := []string{"account", "get"}
	if op.Account != "" {
		args = append(args, "--account", op.Account)
	}

	cmd := exec.CommandContext(ctx, "op", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("1Password CLI authentication required. Run: op signin")
	}

	return nil
}

func (op *OnePasswordProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	// Parse key - support both URI format and simple format
	itemRef, fieldName := op.parseKey(ref.Key)
	
	// Get the item data
	args := []string{"item", "get", itemRef, "--format", "json"}
	if op.Account != "" {
		args = append(args, "--account", op.Account)
	}

	cmd := exec.CommandContext(ctx, "op", args...)
	cmd.Env = os.Environ()

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "not found") {
				return provider.SecretValue{}, fmt.Errorf("item '%s' not found in 1Password", itemRef)
			}
			return provider.SecretValue{}, fmt.Errorf("1Password CLI error: %s", stderr)
		}
		return provider.SecretValue{}, fmt.Errorf("failed to execute 1Password CLI: %w", err)
	}

	// Parse JSON response
	var item OnePasswordItem
	if err := json.Unmarshal(output, &item); err != nil {
		return provider.SecretValue{}, fmt.Errorf("failed to parse 1Password response: %w", err)
	}

	// Extract the requested field
	value, err := op.extractField(&item, fieldName)
	if err != nil {
		return provider.SecretValue{}, fmt.Errorf("failed to extract field '%s': %w", fieldName, err)
	}

	return provider.SecretValue{
		Value: value,
	}, nil
}

func (op *OnePasswordProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	itemRef, _ := op.parseKey(ref.Key)
	
	// Get basic item info without field values
	args := []string{"item", "get", itemRef, "--format", "json"}
	if op.Account != "" {
		args = append(args, "--account", op.Account)
	}

	cmd := exec.CommandContext(ctx, "op", args...)
	cmd.Env = os.Environ()

	output, err := cmd.Output()
	if err != nil {
		return provider.Metadata{}, fmt.Errorf("failed to get item metadata: %w", err)
	}

	var item OnePasswordItem
	if err := json.Unmarshal(output, &item); err != nil {
		return provider.Metadata{}, fmt.Errorf("failed to parse item metadata: %w", err)
	}

	tags := make(map[string]string)
	for i, tag := range item.Tags {
		tags[fmt.Sprintf("tag_%d", i)] = tag
	}

	return provider.Metadata{
		Exists: true,
		Type:   item.Category,
		Tags:   tags,
	}, nil
}

// parseKey parses various 1Password key formats:
// - "op://vault/item/field" (URI format)
// - "item-name.field"
// - "item-name" (defaults to password field)
func (op *OnePasswordProvider) parseKey(key string) (string, string) {
	// Handle op:// URI format
	if strings.HasPrefix(key, "op://") {
		parts := strings.Split(strings.TrimPrefix(key, "op://"), "/")
		if len(parts) >= 3 {
			// op://vault/item/field
			itemRef := fmt.Sprintf("%s/%s", parts[0], parts[1])
			field := parts[2]
			return itemRef, field
		} else if len(parts) >= 2 {
			// op://vault/item (default to password)
			itemRef := fmt.Sprintf("%s/%s", parts[0], parts[1])
			return itemRef, "password"
		}
	}

	// Handle dot notation: "item-name.field"
	if strings.Contains(key, ".") {
		parts := strings.SplitN(key, ".", 2)
		return parts[0], parts[1]
	}

	// Default to password field
	return key, "password"
}

// extractField extracts a specific field value from a 1Password item
func (op *OnePasswordProvider) extractField(item *OnePasswordItem, fieldName string) (string, error) {
	// Check standard fields first
	for _, field := range item.Fields {
		if field.Label == fieldName || field.ID == fieldName {
			return field.Value, nil
		}
	}

	// Handle special field names
	switch strings.ToLower(fieldName) {
	case "password":
		// Look for password field
		for _, field := range item.Fields {
			if field.Type == "CONCEALED" || strings.ToLower(field.Label) == "password" {
				return field.Value, nil
			}
		}
	case "username":
		// Look for username field
		for _, field := range item.Fields {
			if field.Type == "TEXT" && (strings.ToLower(field.Label) == "username" || 
				strings.ToLower(field.Label) == "email") {
				return field.Value, nil
			}
		}
	case "url", "website":
		// Look for URL field
		if len(item.URLs) > 0 {
			return item.URLs[0].Href, nil
		}
	case "notes":
		// Return notes field
		return item.Notes, nil
	case "title", "name":
		// Return item title
		return item.Title, nil
	}

	return "", fmt.Errorf("field '%s' not found in item", fieldName)
}

// OnePasswordItem represents the structure returned by 1Password CLI
type OnePasswordItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Notes    string `json:"notes"`
	Tags     []string `json:"tags"`
	Vault    struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"vault"`
	Fields []OnePasswordField `json:"fields"`
	URLs   []OnePasswordURL   `json:"urls"`
}

type OnePasswordField struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Label string `json:"label"`
	Value string `json:"value"`
}

type OnePasswordURL struct {
	Label   string `json:"label"`
	Primary bool   `json:"primary"`
	Href    string `json:"href"`
}