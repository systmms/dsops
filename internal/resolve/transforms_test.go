package resolve

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTransformJSONExtract tests JSON path extraction
func TestTransformJSONExtract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		jsonInput string
		path      string
		want      string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "simple string field",
			jsonInput: `{"key":"value"}`,
			path:      ".key",
			want:      "value",
		},
		{
			name:      "nested field",
			jsonInput: `{"a":{"b":"c"}}`,
			path:      ".a.b",
			want:      "c",
		},
		{
			name:      "deeply nested field",
			jsonInput: `{"level1":{"level2":{"level3":"deep-value"}}}`,
			path:      ".level1.level2.level3",
			want:      "deep-value",
		},
		{
			name:      "number field",
			jsonInput: `{"port":5432}`,
			path:      ".port",
			want:      "5432",
		},
		{
			name:      "boolean field",
			jsonInput: `{"enabled":true}`,
			path:      ".enabled",
			want:      "true",
		},
		{
			name:      "null field",
			jsonInput: `{"value":null}`,
			path:      ".value",
			want:      "",
		},
		{
			name:      "complex object as JSON",
			jsonInput: `{"nested":{"field1":"val1","field2":"val2"}}`,
			path:      ".nested",
			want:      `{"field1":"val1","field2":"val2"}`,
		},
		{
			name:      "invalid JSON",
			jsonInput: `not valid json`,
			path:      ".key",
			wantErr:   true,
			errMsg:    "invalid JSON",
		},
		{
			name:      "missing path prefix",
			jsonInput: `{"key":"value"}`,
			path:      "key",
			wantErr:   true,
			errMsg:    "JSON path must start with '.'",
		},
		{
			name:      "field not found",
			jsonInput: `{"key":"value"}`,
			path:      ".missing",
			wantErr:   true,
			errMsg:    "field 'missing' not found in JSON",
		},
		{
			name:      "navigate into non-object",
			jsonInput: `{"key":"value"}`,
			path:      ".key.nested",
			wantErr:   true,
			errMsg:    "cannot navigate into non-object",
		},
		{
			name:      "array indexing not supported",
			jsonInput: `{"items":["a","b","c"]}`,
			path:      ".items[0]",
			wantErr:   true,
			errMsg:    "array indexing not yet implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := extractJSONPath(tt.jsonInput, tt.path)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestTransformBase64Decode tests base64 decoding
func TestTransformBase64Decode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
		errMsg  string
	}{
		{
			name:  "valid base64",
			input: "c2VjcmV0LXBhc3N3b3JkLTEyMw==",
			want:  "secret-password-123",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "base64 with special characters",
			input: "SGVsbG8sIFdvcmxkIQ==",
			want:  "Hello, World!",
		},
		{
			name:  "base64 with newlines",
			input: "bXVsdGlsaW5lCnN0cmluZw==",
			want:  "multiline\nstring",
		},
		{
			name:    "invalid base64",
			input:   "not-base64!!!",
			wantErr: true,
			errMsg:  "base64 decode failed",
		},
		{
			name:    "incomplete base64 padding",
			input:   "abc",
			wantErr: true,
			errMsg:  "base64 decode failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := base64Decode(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestTransformBase64Encode tests base64 encoding
func TestTransformBase64Encode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple string",
			input: "secret-password-123",
			want:  "c2VjcmV0LXBhc3N3b3JkLTEyMw==",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "string with special characters",
			input: "Hello, World!",
			want:  "SGVsbG8sIFdvcmxkIQ==",
		},
		{
			name:  "multiline string",
			input: "multiline\nstring",
			want:  "bXVsdGlsaW5lCnN0cmluZw==",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := base64Encode(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestTransformYAMLExtract tests YAML path extraction
func TestTransformYAMLExtract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		yamlInput  string
		path       string
		want       string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "simple string field",
			yamlInput:  "key: value",
			path:       ".key",
			want:       "value",
		},
		{
			name: "nested field",
			yamlInput: `a:
  b: c`,
			path: ".a.b",
			want: "c",
		},
		{
			name: "deeply nested field",
			yamlInput: `level1:
  level2:
    level3: deep-value`,
			path: ".level1.level2.level3",
			want: "deep-value",
		},
		{
			name:       "number field",
			yamlInput:  "port: 5432",
			path:       ".port",
			want:       "5432",
		},
		{
			name:       "boolean field",
			yamlInput:  "enabled: true",
			path:       ".enabled",
			want:       "true",
		},
		{
			name:       "null field",
			yamlInput:  "value: null",
			path:       ".value",
			want:       "",
		},
		{
			name: "complex object as YAML",
			yamlInput: `nested:
  field1: val1
  field2: val2`,
			path: ".nested",
			want: "field1: val1\nfield2: val2",
		},
		{
			name:       "invalid YAML",
			yamlInput:  "not: valid: yaml: structure",
			path:       ".key",
			wantErr:    true,
			errMsg:     "invalid YAML",
		},
		{
			name:       "missing path prefix",
			yamlInput:  "key: value",
			path:       "key",
			wantErr:    true,
			errMsg:     "YAML path must start with '.'",
		},
		{
			name:       "field not found",
			yamlInput:  "key: value",
			path:       ".missing",
			wantErr:    true,
			errMsg:     "field 'missing' not found in YAML",
		},
		{
			name:       "navigate into non-object",
			yamlInput:  "key: value",
			path:       ".key.nested",
			wantErr:    true,
			errMsg:     "cannot navigate into non-object",
		},
		{
			name: "array indexing not supported",
			yamlInput: `items:
  - a
  - b
  - c`,
			path:    ".items[0]",
			wantErr: true,
			errMsg:  "array indexing not yet implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := extractYAMLPath(tt.yamlInput, tt.path)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestTransformJoinValues tests joining array values with separator
func TestTransformJoinValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		separator string
		want      string
	}{
		{
			name:      "JSON array of strings",
			input:     `["a","b","c"]`,
			separator: ",",
			want:      "a,b,c",
		},
		{
			name:      "JSON array with different separator",
			input:     `["item1","item2","item3"]`,
			separator: "|",
			want:      "item1|item2|item3",
		},
		{
			name:      "JSON array of numbers",
			input:     `[1,2,3]`,
			separator: ",",
			want:      "1,2,3",
		},
		{
			name:      "JSON array of booleans",
			input:     `[true,false,true]`,
			separator: ",",
			want:      "true,false,true",
		},
		{
			name:      "newline delimited",
			input:     "line1\nline2\nline3",
			separator: ",",
			want:      "line1,line2,line3",
		},
		{
			name:      "comma delimited",
			input:     "a,b,c",
			separator: "|",
			want:      "a|b|c",
		},
		{
			name:      "space delimited",
			input:     "one two three",
			separator: ",",
			want:      "one,two,three",
		},
		{
			name:      "single value",
			input:     "single",
			separator: ",",
			want:      "single",
		},
		{
			name:      "empty string",
			input:     "",
			separator: ",",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := joinValues(tt.input, tt.separator)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestTransformChaining tests applying multiple transforms in sequence
func TestTransformChaining(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		transform string
		want      string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "base64 decode then trim",
			input:     "ICBzZWNyZXQgIA==", // "  secret  " encoded
			transform: "base64_decode,trim",
			want:      "secret",
		},
		{
			name:      "json extract then base64 decode",
			input:     `{"encoded":"SGVsbG8="}`,
			transform: "json_extract:.encoded,base64_decode",
			want:      "Hello",
		},
		{
			name:      "trim then base64 encode",
			input:     "  secret  ",
			transform: "trim,base64_encode",
			want:      "c2VjcmV0",
		},
		{
			name:      "json extract then trim then replace",
			input:     `{"value":" old-value "}`,
			transform: "json_extract:.value,trim,replace:old:new",
			want:      "new-value",
		},
		{
			name:      "pipe separator instead of comma",
			input:     "  secret  ",
			transform: "trim|base64_encode",
			want:      "c2VjcmV0",
		},
		{
			name:      "invalid transform in chain",
			input:     "value",
			transform: "trim,invalid_transform",
			wantErr:   true,
			errMsg:    "unknown transform",
		},
		{
			name:      "json extract with invalid path",
			input:     `{"key":"value"}`,
			transform: "json_extract:.missing,trim",
			wantErr:   true,
			errMsg:    "field 'missing' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a minimal resolver to test applyTransform
			r := &Resolver{}

			result, err := r.applyTransform(tt.input, tt.transform)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestTransformTrim tests trimming whitespace
func TestTransformTrim(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "leading whitespace",
			input: "  value",
			want:  "value",
		},
		{
			name:  "trailing whitespace",
			input: "value  ",
			want:  "value",
		},
		{
			name:  "both sides whitespace",
			input: "  value  ",
			want:  "value",
		},
		{
			name:  "tabs and spaces",
			input: "\t  value  \t",
			want:  "value",
		},
		{
			name:  "no whitespace",
			input: "value",
			want:  "value",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only whitespace",
			input: "   ",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &Resolver{}
			result, err := r.applySingleTransform(tt.input, "trim")
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestTransformMultilineToSingle tests converting multiline to single line
func TestTransformMultilineToSingle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "single newline",
			input: "line1\nline2",
			want:  "line1\\nline2",
		},
		{
			name:  "multiple newlines",
			input: "line1\nline2\nline3",
			want:  "line1\\nline2\\nline3",
		},
		{
			name:  "windows newlines",
			input: "line1\r\nline2",
			want:  "line1\\nline2",
		},
		{
			name:  "no newlines",
			input: "single line",
			want:  "single line",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &Resolver{}
			result, err := r.applySingleTransform(tt.input, "multiline_to_single")
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestTransformReplace tests string replacement
func TestTransformReplace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		transform string
		want      string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "simple replacement",
			input:     "hello world",
			transform: "replace:world:universe",
			want:      "hello universe",
		},
		{
			name:      "multiple occurrences",
			input:     "foo bar foo",
			transform: "replace:foo:baz",
			want:      "baz bar baz",
		},
		{
			name:      "no match",
			input:     "hello world",
			transform: "replace:universe:galaxy",
			want:      "hello world",
		},
		{
			name:      "empty replacement",
			input:     "remove-this",
			transform: "replace:remove-:",
			want:      "this",
		},
		{
			name:      "invalid format - missing separator",
			input:     "value",
			transform: "replace:only_one_part",
			wantErr:   true,
			errMsg:    "replace transform requires format 'replace:from:to'",
		},
		{
			name:      "special characters",
			input:     "path/to/file",
			transform: "replace:/:_",
			want:      "path_to_file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &Resolver{}
			result, err := r.applySingleTransform(tt.input, tt.transform)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestTransformUnknown tests error handling for unknown transforms
func TestTransformUnknown(t *testing.T) {
	t.Parallel()

	r := &Resolver{}
	_, err := r.applySingleTransform("value", "unknown_transform")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown transform")
}
