package resolve

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// extractJSONPath extracts a value from JSON using a simple path syntax
// Supports paths like ".field", ".nested.field", ".array[0]"
func extractJSONPath(jsonStr, path string) (string, error) {
	if !strings.HasPrefix(path, ".") {
		return "", fmt.Errorf("JSON path must start with '.'")
	}

	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	// Remove the leading dot
	path = strings.TrimPrefix(path, ".")
	
	// Split path into components
	parts := strings.Split(path, ".")
	
	current := data
	for _, part := range parts {
		if part == "" {
			continue
		}

		// Handle array indexing like "field[0]"
		if strings.Contains(part, "[") && strings.Contains(part, "]") {
			// For now, we'll implement basic array access
			// This is a simplified implementation - a full JSON path would be more complex
			return "", fmt.Errorf("array indexing not yet implemented in JSON path")
		}

		// Navigate to the next level
		switch v := current.(type) {
		case map[string]interface{}:
			if val, exists := v[part]; exists {
				current = val
			} else {
				return "", fmt.Errorf("field '%s' not found in JSON", part)
			}
		default:
			return "", fmt.Errorf("cannot navigate into non-object at path '%s'", part)
		}
	}

	// Convert result to string
	switch v := current.(type) {
	case string:
		return v, nil
	case float64:
		return fmt.Sprintf("%.0f", v), nil
	case bool:
		return fmt.Sprintf("%t", v), nil
	case nil:
		return "", nil
	default:
		// For complex objects, return as JSON
		bytes, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to marshal result: %w", err)
		}
		return string(bytes), nil
	}
}

// base64Decode decodes a base64 encoded string
func base64Decode(encoded string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}
	return string(decoded), nil
}

// base64Encode encodes a string as base64
func base64Encode(input string) (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(input)), nil
}

// extractYAMLPath extracts a value from YAML using a simple path syntax
// Supports paths like ".field", ".nested.field"
func extractYAMLPath(yamlStr, path string) (string, error) {
	if !strings.HasPrefix(path, ".") {
		return "", fmt.Errorf("YAML path must start with '.'")
	}

	var data interface{}
	if err := yaml.Unmarshal([]byte(yamlStr), &data); err != nil {
		return "", fmt.Errorf("invalid YAML: %w", err)
	}

	// Remove the leading dot
	path = strings.TrimPrefix(path, ".")
	
	// Split path into components
	parts := strings.Split(path, ".")
	
	current := data
	for _, part := range parts {
		if part == "" {
			continue
		}

		// Handle array indexing like "field[0]"
		if strings.Contains(part, "[") && strings.Contains(part, "]") {
			// For now, we'll implement basic array access
			// This is a simplified implementation - a full YAML path would be more complex
			return "", fmt.Errorf("array indexing not yet implemented in YAML path")
		}

		// Navigate to the next level
		switch v := current.(type) {
		case map[string]interface{}:
			if val, exists := v[part]; exists {
				current = val
			} else {
				return "", fmt.Errorf("field '%s' not found in YAML", part)
			}
		default:
			return "", fmt.Errorf("cannot navigate into non-object at path '%s'", part)
		}
	}

	// Convert result to string
	switch v := current.(type) {
	case string:
		return v, nil
	case int:
		return fmt.Sprintf("%d", v), nil
	case float64:
		return fmt.Sprintf("%.0f", v), nil
	case bool:
		return fmt.Sprintf("%t", v), nil
	case nil:
		return "", nil
	default:
		// For complex objects, return as YAML
		bytes, err := yaml.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to marshal result: %w", err)
		}
		return strings.TrimSpace(string(bytes)), nil
	}
}

// joinValues joins array values with a separator
func joinValues(input, separator string) (string, error) {
	// Try to parse as JSON array first
	var jsonArray []interface{}
	if err := json.Unmarshal([]byte(input), &jsonArray); err == nil {
		// Convert array elements to strings
		var strArray []string
		for _, item := range jsonArray {
			switch v := item.(type) {
			case string:
				strArray = append(strArray, v)
			case float64:
				strArray = append(strArray, fmt.Sprintf("%.0f", v))
			case bool:
				strArray = append(strArray, fmt.Sprintf("%t", v))
			case nil:
				strArray = append(strArray, "")
			default:
				// For complex objects, convert to JSON
				bytes, err := json.Marshal(v)
				if err != nil {
					return "", fmt.Errorf("failed to marshal array element: %w", err)
				}
				strArray = append(strArray, string(bytes))
			}
		}
		return strings.Join(strArray, separator), nil
	}

	// If not JSON array, try splitting by common delimiters and joining with new separator
	input = strings.TrimSpace(input)
	
	// Try common delimiters
	delimiters := []string{"\n", ",", ";", "|", " "}
	for _, delimiter := range delimiters {
		if strings.Contains(input, delimiter) {
			parts := strings.Split(input, delimiter)
			var trimmedParts []string
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					trimmedParts = append(trimmedParts, trimmed)
				}
			}
			if len(trimmedParts) > 1 {
				return strings.Join(trimmedParts, separator), nil
			}
		}
	}

	// If no delimiters found, return original value
	return input, nil
}