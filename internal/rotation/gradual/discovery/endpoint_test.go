package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndpointProvider_Name(t *testing.T) {
	t.Parallel()

	provider := NewEndpointProvider()
	assert.Equal(t, "endpoint", provider.Name())
}

func TestEndpointProvider_Discover(t *testing.T) {
	t.Parallel()

	t.Run("invalid config type", func(t *testing.T) {
		t.Parallel()

		provider := NewEndpointProvider()
		ctx := context.Background()

		_, err := provider.Discover(ctx, "invalid")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid config type for endpoint discovery")
	})

	t.Run("missing endpoint URL", func(t *testing.T) {
		t.Parallel()

		provider := NewEndpointProvider()
		ctx := context.Background()

		_, err := provider.Discover(ctx, Config{
			Type:     "endpoint",
			Endpoint: "",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "endpoint URL is required")
	})

	t.Run("successful discovery", func(t *testing.T) {
		t.Parallel()

		// Create mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request headers
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			assert.Equal(t, "dsops/1.0", r.Header.Get("User-Agent"))

			response := EndpointResponse{
				Instances: []EndpointInstance{
					{
						ID:       "instance-1",
						Endpoint: "http://instance1.example.com",
						Labels:   map[string]string{"env": "prod"},
					},
					{
						ID:       "instance-2",
						Endpoint: "http://instance2.example.com",
						Labels:   map[string]string{"env": "prod", "canary": "true"},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		provider := NewEndpointProvider()
		ctx := context.Background()

		instances, err := provider.Discover(ctx, Config{
			Type:     "endpoint",
			Endpoint: server.URL,
		})

		require.NoError(t, err)
		assert.Len(t, instances, 2)
		assert.Equal(t, "instance-1", instances[0].ID)
		assert.Equal(t, "http://instance1.example.com", instances[0].Endpoint)
		assert.Equal(t, "prod", instances[0].Labels["env"])
		assert.Equal(t, "instance-2", instances[1].ID)
	})

	t.Run("skip instances without ID", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := EndpointResponse{
				Instances: []EndpointInstance{
					{ID: "instance-1", Endpoint: "http://instance1.example.com"},
					{ID: "", Endpoint: "http://no-id.example.com"},       // Should be skipped
					{ID: "instance-2", Endpoint: "http://instance2.example.com"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		provider := NewEndpointProvider()
		ctx := context.Background()

		instances, err := provider.Discover(ctx, Config{
			Endpoint: server.URL,
		})

		require.NoError(t, err)
		assert.Len(t, instances, 2)
		assert.Equal(t, "instance-1", instances[0].ID)
		assert.Equal(t, "instance-2", instances[1].ID)
	})

	t.Run("no valid instances returns error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := EndpointResponse{
				Instances: []EndpointInstance{
					{ID: "", Endpoint: "http://no-id.example.com"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		provider := NewEndpointProvider()
		ctx := context.Background()

		_, err := provider.Discover(ctx, Config{
			Endpoint: server.URL,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no valid instances")
	})

	t.Run("empty instances returns error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := EndpointResponse{
				Instances: []EndpointInstance{},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		provider := NewEndpointProvider()
		ctx := context.Background()

		_, err := provider.Discover(ctx, Config{
			Endpoint: server.URL,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no valid instances")
	})

	t.Run("non-200 status code", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("Internal Server Error"))
		}))
		defer server.Close()

		provider := NewEndpointProvider()
		ctx := context.Background()

		_, err := provider.Discover(ctx, Config{
			Endpoint: server.URL,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "non-200 status: 500")
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("not valid json"))
		}))
		defer server.Close()

		provider := NewEndpointProvider()
		ctx := context.Background()

		_, err := provider.Discover(ctx, Config{
			Endpoint: server.URL,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse JSON")
	})

	t.Run("connection error", func(t *testing.T) {
		t.Parallel()

		provider := NewEndpointProvider()
		ctx := context.Background()

		_, err := provider.Discover(ctx, Config{
			Endpoint: "http://localhost:99999", // Invalid port
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to query endpoint")
	})

	t.Run("context cancellation", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// This will hang forever without context cancellation
			<-r.Context().Done()
		}))
		defer server.Close()

		provider := NewEndpointProvider()
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := provider.Discover(ctx, Config{
			Endpoint: server.URL,
		})

		require.Error(t, err)
	})
}

func TestEndpointProvider_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "invalid config type",
			config:  "invalid",
			wantErr: true,
			errMsg:  "invalid config type for endpoint discovery",
		},
		{
			name: "valid http endpoint",
			config: Config{
				Type:     "endpoint",
				Endpoint: "http://discovery.example.com/instances",
			},
			wantErr: false,
		},
		{
			name: "valid https endpoint",
			config: Config{
				Type:     "endpoint",
				Endpoint: "https://discovery.example.com/instances",
			},
			wantErr: false,
		},
		{
			name: "empty type is valid",
			config: Config{
				Type:     "",
				Endpoint: "http://discovery.example.com",
			},
			wantErr: false,
		},
		{
			name: "invalid discovery type",
			config: Config{
				Type:     "kubernetes",
				Endpoint: "http://discovery.example.com",
			},
			wantErr: true,
			errMsg:  "invalid discovery type for endpoint provider",
		},
		{
			name: "missing endpoint URL",
			config: Config{
				Type:     "endpoint",
				Endpoint: "",
			},
			wantErr: true,
			errMsg:  "endpoint URL is required",
		},
		{
			name: "invalid URL scheme",
			config: Config{
				Type:     "endpoint",
				Endpoint: "ftp://discovery.example.com",
			},
			wantErr: true,
			errMsg:  "must be a valid HTTP or HTTPS URL",
		},
		{
			name: "file URL not allowed",
			config: Config{
				Type:     "endpoint",
				Endpoint: "file:///etc/passwd",
			},
			wantErr: true,
			errMsg:  "must be a valid HTTP or HTTPS URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := NewEndpointProvider()
			err := provider.Validate(tt.config)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
