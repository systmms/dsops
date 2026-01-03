package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderer_base64Encode(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty_string", "", ""},
		{"simple_string", "hello", "aGVsbG8="},
		{"with_spaces", "hello world", "aGVsbG8gd29ybGQ="},
		{"special_chars", "user:pass@123!", "dXNlcjpwYXNzQDEyMyE="},
		{"unicode", "héllo wörld", "aMOpbGxvIHfDtnJsZA=="},
		{"newlines", "line1\nline2", "bGluZTEKbGluZTI="},
		{"json_content", `{"key": "value"}`, "eyJrZXkiOiAidmFsdWUifQ=="},
		{"binary_like", "\x00\x01\x02", "AAEC"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := renderer.base64Encode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRenderer_base64Decode(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()

	t.Run("valid_inputs", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			input    string
			expected string
		}{
			{"empty_string", "", ""},
			{"simple_string", "aGVsbG8=", "hello"},
			{"with_spaces", "aGVsbG8gd29ybGQ=", "hello world"},
			{"special_chars", "dXNlcjpwYXNzQDEyMyE=", "user:pass@123!"},
			{"unicode", "aMOpbGxvIHfDtnJsZA==", "héllo wörld"},
			{"newlines", "bGluZTEKbGluZTI=", "line1\nline2"},
			{"json_content", "eyJrZXkiOiAidmFsdWUifQ==", `{"key": "value"}`},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				result, err := renderer.base64Decode(tt.input)
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("invalid_inputs", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			input string
		}{
			{"invalid_chars", "!!!invalid!!!"},
			{"wrong_padding", "aGVsbG8==="},
			{"invalid_length", "abc"},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				_, err := renderer.base64Decode(tt.input)
				assert.Error(t, err)
			})
		}
	})
}

func TestRenderer_base64_roundtrip(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()

	tests := []string{
		"",
		"hello",
		"hello world",
		"special!@#$%^&*()",
		"unicode: héllo wörld 你好",
		"newlines\nand\ttabs",
		`{"json": "content", "nested": {"key": "value"}}`,
	}

	for _, input := range tests {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			encoded := renderer.base64Encode(input)
			decoded, err := renderer.base64Decode(encoded)
			require.NoError(t, err)
			assert.Equal(t, input, decoded)
		})
	}
}

func TestRenderer_indent(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()

	tests := []struct {
		name     string
		input    string
		prefix   string
		expected string
	}{
		{
			name:     "single_line",
			input:    "hello",
			prefix:   "  ",
			expected: "  hello",
		},
		{
			name:     "multiple_lines",
			input:    "line1\nline2\nline3",
			prefix:   "  ",
			expected: "  line1\n  line2\n  line3",
		},
		{
			name:     "empty_string",
			input:    "",
			prefix:   "  ",
			expected: "",
		},
		{
			name:     "empty_prefix",
			input:    "line1\nline2",
			prefix:   "",
			expected: "line1\nline2",
		},
		{
			name:     "tab_prefix",
			input:    "line1\nline2",
			prefix:   "\t",
			expected: "\tline1\n\tline2",
		},
		{
			name:     "custom_prefix",
			input:    "item1\nitem2",
			prefix:   "- ",
			expected: "- item1\n- item2",
		},
		{
			name:     "with_empty_middle_line",
			input:    "line1\n\nline3",
			prefix:   "  ",
			expected: "  line1\n  \n  line3",
		},
		{
			name:     "trailing_newline",
			input:    "line1\nline2\n",
			prefix:   "  ",
			expected: "  line1\n  line2\n",
		},
		{
			name:     "only_newlines",
			input:    "\n\n",
			prefix:   "  ",
			expected: "  \n  \n",
		},
		{
			name:     "yaml_style_indent",
			input:    "key: value\nnested:\n  inner: data",
			prefix:   "    ",
			expected: "    key: value\n    nested:\n      inner: data",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := renderer.indent(tt.input, tt.prefix)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRenderer_sha256Hash(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty_string",
			input:    "",
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "simple_string",
			input:    "test",
			expected: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		},
		{
			name:     "hello_world",
			input:    "hello world",
			expected: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
		{
			name:     "with_newline",
			input:    "hello\n",
			expected: "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		},
		{
			name:     "json_content",
			input:    `{"key":"value"}`,
			expected: "e43abcf3375244839c012f9633f95862d232a95b00d5bc7348b3098b9fed7f32",
		},
		{
			name:     "unicode",
			input:    "héllo",
			expected: "3c48591d8d098a4538f5e013dfcf406e948eac4d3277b10bf614e295d6068179",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := renderer.sha256Hash(tt.input)
			assert.Equal(t, tt.expected, result)
			// Verify it's always 64 hex characters
			assert.Len(t, result, 64)
		})
	}
}

func TestRenderer_sha256Hash_deterministic(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()
	input := "consistent input"

	// Hash should always produce the same output
	hash1 := renderer.sha256Hash(input)
	hash2 := renderer.sha256Hash(input)
	hash3 := renderer.sha256Hash(input)

	assert.Equal(t, hash1, hash2)
	assert.Equal(t, hash2, hash3)
}

func TestRenderer_sha256Hash_different_inputs(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()

	// Even small differences should produce completely different hashes
	hash1 := renderer.sha256Hash("test1")
	hash2 := renderer.sha256Hash("test2")
	hash3 := renderer.sha256Hash("Test1") // case difference

	assert.NotEqual(t, hash1, hash2)
	assert.NotEqual(t, hash1, hash3)
	assert.NotEqual(t, hash2, hash3)
}

func TestRenderer_marshalJSON(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()

	t.Run("simple_map", func(t *testing.T) {
		t.Parallel()
		input := map[string]string{"key": "value"}
		result, err := renderer.marshalJSON(input)
		require.NoError(t, err)
		assert.Contains(t, string(result), `"key"`)
		assert.Contains(t, string(result), `"value"`)
	})

	t.Run("nested_map", func(t *testing.T) {
		t.Parallel()
		input := map[string]interface{}{
			"outer": map[string]string{
				"inner": "value",
			},
		}
		result, err := renderer.marshalJSON(input)
		require.NoError(t, err)
		assert.Contains(t, string(result), `"outer"`)
		assert.Contains(t, string(result), `"inner"`)
	})

	t.Run("array", func(t *testing.T) {
		t.Parallel()
		input := []string{"a", "b", "c"}
		result, err := renderer.marshalJSON(input)
		require.NoError(t, err)
		assert.Contains(t, string(result), `"a"`)
		assert.Contains(t, string(result), `"b"`)
		assert.Contains(t, string(result), `"c"`)
	})

	t.Run("struct", func(t *testing.T) {
		t.Parallel()
		input := struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}{
			Name:  "test",
			Value: 42,
		}
		result, err := renderer.marshalJSON(input)
		require.NoError(t, err)
		assert.Contains(t, string(result), `"name": "test"`)
		assert.Contains(t, string(result), `"value": 42`)
	})

	t.Run("empty_map", func(t *testing.T) {
		t.Parallel()
		input := map[string]string{}
		result, err := renderer.marshalJSON(input)
		require.NoError(t, err)
		assert.Equal(t, "{}", string(result))
	})

	t.Run("nil_value", func(t *testing.T) {
		t.Parallel()
		result, err := renderer.marshalJSON(nil)
		require.NoError(t, err)
		assert.Equal(t, "null", string(result))
	})

	t.Run("special_characters", func(t *testing.T) {
		t.Parallel()
		input := map[string]string{
			"special": "line1\nline2\ttab\"quote",
		}
		result, err := renderer.marshalJSON(input)
		require.NoError(t, err)
		// JSON should properly escape special characters
		assert.Contains(t, string(result), `\n`)
		assert.Contains(t, string(result), `\t`)
		assert.Contains(t, string(result), `\"`)
	})

	t.Run("pretty_printed", func(t *testing.T) {
		t.Parallel()
		input := map[string]string{"key": "value"}
		result, err := renderer.marshalJSON(input)
		require.NoError(t, err)
		// Should be indented with 2 spaces
		assert.Contains(t, string(result), "\n")
		assert.Contains(t, string(result), "  ")
	})
}

func TestRenderer_marshalJSON_error(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()

	// Channels cannot be marshaled to JSON
	ch := make(chan int)
	_, err := renderer.marshalJSON(ch)
	assert.Error(t, err)
}
