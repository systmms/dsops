package resolve

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/config"
)

// TestApplyTransformEdgeCases tests transform application with edge cases and error messages.
func TestApplyTransformEdgeCases(t *testing.T) {
	// Create a minimal resolver for testing
	cfg := &config.Config{}
	resolver := New(cfg)

	tests := []struct {
		name          string
		value         string
		transform     string
		expectedValue string
		expectError   bool
		errorContains string
	}{
		// trim transform
		{
			name:          "trim removes leading and trailing whitespace",
			value:         "  hello world  ",
			transform:     "trim",
			expectedValue: "hello world",
		},
		{
			name:          "trim with only whitespace",
			value:         "   \t\n  ",
			transform:     "trim",
			expectedValue: "",
		},
		{
			name:          "trim with newlines",
			value:         "\n\ndata\n\n",
			transform:     "trim",
			expectedValue: "data",
		},

		// multiline_to_single transform
		{
			name:          "multiline to single converts newlines",
			value:         "line1\nline2\nline3",
			transform:     "multiline_to_single",
			expectedValue: "line1\\nline2\\nline3",
		},
		{
			name:          "multiline removes carriage returns",
			value:         "line1\r\nline2\r\n",
			transform:     "multiline_to_single",
			expectedValue: "line1\\nline2\\n",
		},
		{
			name:          "multiline with only carriage returns",
			value:         "data\rmore",
			transform:     "multiline_to_single",
			expectedValue: "datamore",
		},

		// replace transform
		{
			name:          "replace simple substring",
			value:         "hello world",
			transform:     "replace:world:universe",
			expectedValue: "hello universe",
		},
		{
			name:          "replace all occurrences",
			value:         "foo bar foo baz foo",
			transform:     "replace:foo:qux",
			expectedValue: "qux bar qux baz qux",
		},
		{
			name:          "replace with empty string (deletion)",
			value:         "remove_underscores_from_string",
			transform:     "replace:_:",
			expectedValue: "removeunderscoresfromstring",
		},
		{
			name:          "replace with special chars",
			value:         "path/to/file",
			transform:     "replace:/:-",
			expectedValue: "path-to-file",
		},
		{
			name:          "replace missing format error",
			value:         "test",
			transform:     "replace:invalid",
			expectError:   true,
			errorContains: "replace transform requires format",
		},

		// join transform (note: comma in join:, would conflict with transform chaining)
		{
			name:          "join JSON array with semicolon",
			value:         `["a", "b", "c"]`,
			transform:     "join:;",
			expectedValue: "a;b;c",
		},
		{
			name:          "join newline separated values with dash",
			value:         "x\ny\nz",
			transform:     "join:-",
			expectedValue: "x-y-z",
		},

		// json_extract transform
		{
			name:          "json extract simple field",
			value:         `{"key": "value"}`,
			transform:     "json_extract:.key",
			expectedValue: "value",
		},
		{
			name:          "json extract nested field",
			value:         `{"outer": {"inner": "nested"}}`,
			transform:     "json_extract:.outer.inner",
			expectedValue: "nested",
		},
		{
			name:          "json extract invalid path",
			value:         `{"key": "value"}`,
			transform:     "json_extract:noprefix",
			expectError:   true,
			errorContains: "JSON path must start with '.'",
		},

		// yaml_extract transform
		{
			name:          "yaml extract simple field",
			value:         "key: value\n",
			transform:     "yaml_extract:.key",
			expectedValue: "value",
		},
		{
			name:          "yaml extract missing path prefix",
			value:         "key: value\n",
			transform:     "yaml_extract:key",
			expectError:   true,
			errorContains: "YAML path must start with '.'",
		},

		// base64 transforms
		{
			name:          "base64 encode string",
			value:         "hello",
			transform:     "base64_encode",
			expectedValue: "aGVsbG8=",
		},
		{
			name:          "base64 decode valid string",
			value:         "aGVsbG8=",
			transform:     "base64_decode",
			expectedValue: "hello",
		},
		{
			name:          "base64 decode invalid string",
			value:         "not-base64!",
			transform:     "base64_decode",
			expectError:   true,
			errorContains: "base64 decode failed",
		},

		// unknown transform
		{
			name:          "unknown transform returns error",
			value:         "test",
			transform:     "nonexistent_transform",
			expectError:   true,
			errorContains: "unknown transform",
		},
		{
			name:          "almost valid transform name",
			value:         "test",
			transform:     "trimm", // typo
			expectError:   true,
			errorContains: "unknown transform",
		},

		// Edge cases with empty values
		{
			name:          "trim empty string",
			value:         "",
			transform:     "trim",
			expectedValue: "",
		},
		{
			name:          "base64 encode empty string",
			value:         "",
			transform:     "base64_encode",
			expectedValue: "",
		},
		{
			name:          "replace in empty string",
			value:         "",
			transform:     "replace:foo:bar",
			expectedValue: "",
		},

		// Special characters preservation
		{
			name:          "trim preserves internal whitespace",
			value:         "  hello   world  ",
			transform:     "trim",
			expectedValue: "hello   world",
		},
		{
			name:          "replace handles colon in value",
			value:         "key:value:extra",
			transform:     "replace:value:new",
			expectedValue: "key:new:extra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.applyTransform(tt.value, tt.transform)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}
		})
	}
}

// TestApplyTransformChain tests applying multiple transforms in sequence.
func TestApplyTransformChain(t *testing.T) {
	cfg := &config.Config{}
	resolver := New(cfg)

	// Test that we can chain transforms manually (simulating what the resolver does)
	t.Run("trim then base64_encode", func(t *testing.T) {
		value := "  secret  "
		result, err := resolver.applyTransform(value, "trim")
		require.NoError(t, err)
		result, err = resolver.applyTransform(result, "base64_encode")
		require.NoError(t, err)
		assert.Equal(t, "c2VjcmV0", result)
	})

	t.Run("json_extract then trim", func(t *testing.T) {
		value := `{"password": "  pass123  "}`
		result, err := resolver.applyTransform(value, "json_extract:.password")
		require.NoError(t, err)
		result, err = resolver.applyTransform(result, "trim")
		require.NoError(t, err)
		assert.Equal(t, "pass123", result)
	})

	t.Run("replace then multiline_to_single", func(t *testing.T) {
		value := "line1\nline2\nline3"
		result, err := resolver.applyTransform(value, "replace:line:row")
		require.NoError(t, err)
		result, err = resolver.applyTransform(result, "multiline_to_single")
		require.NoError(t, err)
		assert.Equal(t, "row1\\nrow2\\nrow3", result)
	})
}

// TestTransformWithSpecialValues tests transforms with special characters and unicode.
func TestTransformWithSpecialValues(t *testing.T) {
	cfg := &config.Config{}
	resolver := New(cfg)

	tests := []struct {
		name          string
		value         string
		transform     string
		expectedValue string
	}{
		{
			name:          "trim unicode whitespace",
			value:         "  日本語  ",
			transform:     "trim",
			expectedValue: "日本語",
		},
		{
			name:          "replace unicode characters",
			value:         "Hello 世界",
			transform:     "replace:世界:World",
			expectedValue: "Hello World",
		},
		{
			name:          "base64 encode unicode",
			value:         "こんにちは",
			transform:     "base64_encode",
			expectedValue: "44GT44KT44Gr44Gh44Gv",
		},
		{
			name:          "json extract unicode field",
			value:         `{"名前": "太郎"}`,
			transform:     "json_extract:.名前",
			expectedValue: "太郎",
		},
		{
			name:          "replace with emoji",
			value:         "status: ok",
			transform:     "replace:ok:✅",
			expectedValue: "status: ✅",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.applyTransform(tt.value, tt.transform)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedValue, result)
		})
	}
}
