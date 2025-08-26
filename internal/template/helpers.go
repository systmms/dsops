package template

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// marshalJSON marshals an interface to JSON with proper formatting
func (r *Renderer) marshalJSON(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// base64Encode encodes a string to base64
func (r *Renderer) base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// base64Decode decodes a base64 string
func (r *Renderer) base64Decode(s string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// indent indents each line of a string with the given prefix
func (r *Renderer) indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	result := make([]string, len(lines))
	
	for i, line := range lines {
		if line != "" || i < len(lines)-1 {
			result[i] = prefix + line
		} else {
			result[i] = line // Don't indent empty last line
		}
	}
	
	return strings.Join(result, "\n")
}

// sha256Hash returns the SHA256 hash of a string
func (r *Renderer) sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}