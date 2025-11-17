package resolve

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractJSONPathEdgeCases tests JSON path extraction edge cases.
func TestExtractJSONPathEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		jsonStr       string
		path          string
		expectedValue string
		expectError   bool
		errorContains string
	}{
		// Basic functionality
		{
			name:          "simple string field",
			jsonStr:       `{"password": "secret123"}`,
			path:          ".password",
			expectedValue: "secret123",
		},
		{
			name:          "nested field",
			jsonStr:       `{"database": {"password": "dbpass"}}`,
			path:          ".database.password",
			expectedValue: "dbpass",
		},
		// Type conversions
		{
			name:          "integer field",
			jsonStr:       `{"port": 5432}`,
			path:          ".port",
			expectedValue: "5432",
		},
		{
			name:          "float field truncates decimals",
			jsonStr:       `{"timeout": 30.5}`,
			path:          ".timeout",
			expectedValue: "30",
		},
		{
			name:          "boolean true",
			jsonStr:       `{"enabled": true}`,
			path:          ".enabled",
			expectedValue: "true",
		},
		{
			name:          "boolean false",
			jsonStr:       `{"disabled": false}`,
			path:          ".disabled",
			expectedValue: "false",
		},
		{
			name:          "null value returns empty string",
			jsonStr:       `{"optional": null}`,
			path:          ".optional",
			expectedValue: "",
		},
		// Edge cases
		{
			name:          "empty string value",
			jsonStr:       `{"empty": ""}`,
			path:          ".empty",
			expectedValue: "",
		},
		{
			name:          "string with special characters",
			jsonStr:       `{"pass": "p@$$w0rd!#%^&*(){}[]"}`,
			path:          ".pass",
			expectedValue: "p@$$w0rd!#%^&*(){}[]",
		},
		{
			name:          "string with unicode",
			jsonStr:       `{"msg": "Hello 世界 🌍 مرحبا"}`,
			path:          ".msg",
			expectedValue: "Hello 世界 🌍 مرحبا",
		},
		{
			name:          "string with escaped quotes",
			jsonStr:       `{"data": "he said \"hello\""}`,
			path:          ".data",
			expectedValue: `he said "hello"`,
		},
		{
			name:          "string with newlines",
			jsonStr:       `{"multi": "line1\nline2\nline3"}`,
			path:          ".multi",
			expectedValue: "line1\nline2\nline3",
		},
		{
			name:          "string with tabs",
			jsonStr:       `{"tabbed": "col1\tcol2\tcol3"}`,
			path:          ".tabbed",
			expectedValue: "col1\tcol2\tcol3",
		},
		{
			name:          "very long string",
			jsonStr:       `{"long": "` + strings.Repeat("x", 10000) + `"}`,
			path:          ".long",
			expectedValue: strings.Repeat("x", 10000),
		},
		// Complex objects returned as JSON
		{
			name:          "nested object returns JSON",
			jsonStr:       `{"config": {"key": "value", "nested": true}}`,
			path:          ".config",
			expectedValue: `{"key":"value","nested":true}`,
		},
		{
			name:          "array returns JSON",
			jsonStr:       `{"items": ["a", "b", "c"]}`,
			path:          ".items",
			expectedValue: `["a","b","c"]`,
		},
		{
			name:          "array of objects",
			jsonStr:       `{"users": [{"name": "alice"}, {"name": "bob"}]}`,
			path:          ".users",
			expectedValue: `[{"name":"alice"},{"name":"bob"}]`,
		},
		// Path edge cases
		{
			name:          "path with empty parts (consecutive dots)",
			jsonStr:       `{"field": "value"}`,
			path:          "..field",
			expectedValue: "value",
		},
		{
			name:          "path with just dot returns whole object",
			jsonStr:       `{"key": "value"}`,
			path:          ".",
			expectedValue: `{"key":"value"}`,
		},
		{
			name:          "deeply nested path",
			jsonStr:       `{"a": {"b": {"c": {"d": {"e": {"f": "deep"}}}}}}`,
			path:          ".a.b.c.d.e.f",
			expectedValue: "deep",
		},
		// Error cases
		{
			name:          "path without leading dot",
			jsonStr:       `{"password": "secret"}`,
			path:          "password",
			expectError:   true,
			errorContains: "JSON path must start with '.'",
		},
		{
			name:          "non-existent field",
			jsonStr:       `{"username": "admin"}`,
			path:          ".password",
			expectError:   true,
			errorContains: "field 'password' not found",
		},
		{
			name:          "navigate into string",
			jsonStr:       `{"name": "value"}`,
			path:          ".name.sub",
			expectError:   true,
			errorContains: "cannot navigate into non-object",
		},
		{
			name:          "navigate into number",
			jsonStr:       `{"count": 42}`,
			path:          ".count.sub",
			expectError:   true,
			errorContains: "cannot navigate into non-object",
		},
		{
			name:          "navigate into boolean",
			jsonStr:       `{"flag": true}`,
			path:          ".flag.sub",
			expectError:   true,
			errorContains: "cannot navigate into non-object",
		},
		{
			name:          "array indexing not supported",
			jsonStr:       `{"items": ["a", "b", "c"]}`,
			path:          ".items[0]",
			expectError:   true,
			errorContains: "array indexing not yet implemented",
		},
		{
			name:          "invalid JSON",
			jsonStr:       `{"broken: json}`,
			path:          ".field",
			expectError:   true,
			errorContains: "invalid JSON",
		},
		{
			name:          "empty JSON object",
			jsonStr:       `{}`,
			path:          ".field",
			expectError:   true,
			errorContains: "field 'field' not found",
		},
		{
			name:          "JSON array at root",
			jsonStr:       `["item1", "item2"]`,
			path:          ".0",
			expectError:   true,
			errorContains: "cannot navigate into non-object",
		},
		{
			name:          "empty path after dot",
			jsonStr:       `{"": "empty key"}`,
			path:          ".",
			expectedValue: `{"":"empty key"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := extractJSONPath(tt.jsonStr, tt.path)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedValue, value)
			}
		})
	}
}

// TestExtractYAMLPathEdgeCases tests YAML path extraction edge cases.
func TestExtractYAMLPathEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		yamlStr       string
		path          string
		expectedValue string
		expectError   bool
		errorContains string
	}{
		// Basic functionality
		{
			name: "simple string field",
			yamlStr: `
password: secret123
`,
			path:          ".password",
			expectedValue: "secret123",
		},
		{
			name: "nested field",
			yamlStr: `
database:
  password: dbpass
`,
			path:          ".database.password",
			expectedValue: "dbpass",
		},
		// Type conversions
		{
			name: "integer field",
			yamlStr: `
port: 5432
`,
			path:          ".port",
			expectedValue: "5432",
		},
		{
			name: "boolean true",
			yamlStr: `
enabled: true
`,
			path:          ".enabled",
			expectedValue: "true",
		},
		{
			name: "boolean false",
			yamlStr: `
disabled: false
`,
			path:          ".disabled",
			expectedValue: "false",
		},
		{
			name: "null value",
			yamlStr: `
optional: null
`,
			path:          ".optional",
			expectedValue: "",
		},
		// YAML-specific edge cases
		{
			name: "multiline string with pipe",
			yamlStr: `
message: |
  line1
  line2
  line3
`,
			path:          ".message",
			expectedValue: "line1\nline2\nline3\n",
		},
		{
			name: "string with special YAML chars",
			yamlStr: `
data: "value: with colons"
`,
			path:          ".data",
			expectedValue: "value: with colons",
		},
		// Error cases
		{
			name: "path without leading dot",
			yamlStr: `
password: secret
`,
			path:          "password",
			expectError:   true,
			errorContains: "YAML path must start with '.'",
		},
		{
			name: "non-existent field",
			yamlStr: `
username: admin
`,
			path:          ".password",
			expectError:   true,
			errorContains: "field 'password' not found",
		},
		{
			name: "navigate into string",
			yamlStr: `
name: value
`,
			path:          ".name.sub",
			expectError:   true,
			errorContains: "cannot navigate into non-object",
		},
		{
			name: "invalid YAML with bad syntax",
			yamlStr: `
broken:
  - item1
  - item2
  key: value
`,
			path:          ".broken",
			expectError:   true,
			errorContains: "invalid YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := extractYAMLPath(tt.yamlStr, tt.path)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedValue, value)
			}
		})
	}
}

// TestJoinValuesEdgeCases tests joinValues with various input formats and separators.
func TestJoinValuesEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		separator     string
		expectedValue string
		expectError   bool
	}{
		// JSON array inputs
		{
			name:          "JSON array of strings",
			input:         `["a", "b", "c"]`,
			separator:     ",",
			expectedValue: "a,b,c",
		},
		{
			name:          "JSON array with custom separator",
			input:         `["host1", "host2", "host3"]`,
			separator:     ";",
			expectedValue: "host1;host2;host3",
		},
		{
			name:          "JSON array with space separator",
			input:         `["word1", "word2", "word3"]`,
			separator:     " ",
			expectedValue: "word1 word2 word3",
		},
		{
			name:          "JSON array with newline separator",
			input:         `["line1", "line2", "line3"]`,
			separator:     "\n",
			expectedValue: "line1\nline2\nline3",
		},
		{
			name:          "JSON array with numbers",
			input:         `[1, 2, 3]`,
			separator:     "-",
			expectedValue: "1-2-3",
		},
		{
			name:          "JSON array with floats",
			input:         `[1.5, 2.7, 3.9]`,
			separator:     ",",
			expectedValue: "2,3,4", // Truncated to integers
		},
		{
			name:          "JSON array with booleans",
			input:         `[true, false, true]`,
			separator:     "|",
			expectedValue: "true|false|true",
		},
		{
			name:          "JSON array with null values",
			input:         `["a", null, "c"]`,
			separator:     ",",
			expectedValue: "a,,c",
		},
		{
			name:          "JSON array with mixed types",
			input:         `["text", 42, true, null]`,
			separator:     ":",
			expectedValue: "text:42:true:",
		},
		{
			name:          "JSON array with nested objects",
			input:         `["a", {"nested": "obj"}, "b"]`,
			separator:     "|",
			expectedValue: `a|{"nested":"obj"}|b`,
		},
		{
			name:          "JSON array with nested arrays",
			input:         `["x", [1,2,3], "y"]`,
			separator:     ",",
			expectedValue: `x,[1,2,3],y`,
		},
		{
			name:          "empty JSON array",
			input:         `[]`,
			separator:     ",",
			expectedValue: "",
		},
		{
			name:          "single element JSON array",
			input:         `["only"]`,
			separator:     ",",
			expectedValue: "only",
		},
		// Newline-delimited inputs
		{
			name:          "newline delimited values",
			input:         "a\nb\nc",
			separator:     ",",
			expectedValue: "a,b,c",
		},
		{
			name:          "newline delimited with spaces",
			input:         "item1\nitem2\nitem3",
			separator:     ";",
			expectedValue: "item1;item2;item3",
		},
		// Comma-delimited inputs
		{
			name:          "comma delimited values",
			input:         "x,y,z",
			separator:     "|",
			expectedValue: "x|y|z",
		},
		{
			name:          "comma delimited with spaces",
			input:         "a, b, c",
			separator:     "-",
			expectedValue: "a-b-c",
		},
		// Semicolon-delimited inputs
		{
			name:          "semicolon delimited values",
			input:         "red;green;blue",
			separator:     ",",
			expectedValue: "red,green,blue",
		},
		// Pipe-delimited inputs
		{
			name:          "pipe delimited values",
			input:         "one|two|three",
			separator:     ",",
			expectedValue: "one,two,three",
		},
		// Space-delimited inputs
		{
			name:          "space delimited values",
			input:         "word1 word2 word3",
			separator:     "-",
			expectedValue: "word1-word2-word3",
		},
		// Edge cases
		{
			name:          "empty string input",
			input:         "",
			separator:     ",",
			expectedValue: "",
		},
		{
			name:          "single value no delimiter",
			input:         "single",
			separator:     ",",
			expectedValue: "single",
		},
		{
			name:          "whitespace trimming",
			input:         "  a , b , c  ",
			separator:     "|",
			expectedValue: "a|b|c",
		},
		{
			name:          "empty separator joins directly",
			input:         `["a", "b", "c"]`,
			separator:     "",
			expectedValue: "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := joinValues(tt.input, tt.separator)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}
		})
	}
}

// TestBase64EncodeDecodeEdgeCases tests base64 encoding/decoding edge cases.
func TestBase64EncodeDecodeEdgeCases(t *testing.T) {
	t.Run("encode empty string", func(t *testing.T) {
		result, err := base64Encode("")
		require.NoError(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("encode simple string", func(t *testing.T) {
		result, err := base64Encode("hello")
		require.NoError(t, err)
		assert.Equal(t, "aGVsbG8=", result)
	})

	t.Run("encode string with special chars", func(t *testing.T) {
		result, err := base64Encode("p@ssw0rd!")
		require.NoError(t, err)
		// Verify round-trip
		decoded, err := base64Decode(result)
		require.NoError(t, err)
		assert.Equal(t, "p@ssw0rd!", decoded)
	})

	t.Run("encode binary-like data", func(t *testing.T) {
		result, err := base64Encode("\x00\x01\x02\xff")
		require.NoError(t, err)
		decoded, err := base64Decode(result)
		require.NoError(t, err)
		assert.Equal(t, "\x00\x01\x02\xff", decoded)
	})

	t.Run("decode valid base64", func(t *testing.T) {
		result, err := base64Decode("aGVsbG8gd29ybGQ=")
		require.NoError(t, err)
		assert.Equal(t, "hello world", result)
	})

	t.Run("decode empty string", func(t *testing.T) {
		result, err := base64Decode("")
		require.NoError(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("decode invalid base64", func(t *testing.T) {
		_, err := base64Decode("not-valid-base64!")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "base64 decode failed")
	})

	t.Run("decode base64 with padding", func(t *testing.T) {
		// Test various padding scenarios
		cases := []struct {
			encoded string
			decoded string
		}{
			{"YQ==", "a"},       // 1 char, 2 padding
			{"YWI=", "ab"},      // 2 chars, 1 padding
			{"YWJj", "abc"},     // 3 chars, no padding
			{"YWJjZA==", "abcd"}, // 4 chars, 2 padding
		}
		for _, c := range cases {
			result, err := base64Decode(c.encoded)
			require.NoError(t, err)
			assert.Equal(t, c.decoded, result)
		}
	})
}

