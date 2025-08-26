package protocol_test

import (
	"context"
	"testing"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/pkg/protocol"
)

func TestRegistry(t *testing.T) {
	t.Run("RegisterAndRetrieveAdapters", func(t *testing.T) {
		registry := protocol.NewRegistry()
		
		// Register all adapters
		require.NoError(t, registry.Register(protocol.NewSQLAdapter()))
		require.NoError(t, registry.Register(protocol.NewHTTPAPIAdapter()))
		require.NoError(t, registry.Register(protocol.NewNoSQLAdapter()))
		require.NoError(t, registry.Register(protocol.NewCertificateAdapter()))
		
		// Test retrieval by type
		sqlAdapter, err := registry.Get(protocol.AdapterTypeSQL)
		assert.NoError(t, err)
		assert.NotNil(t, sqlAdapter)
		assert.Equal(t, protocol.AdapterTypeSQL, sqlAdapter.Type())
		
		// Test retrieval by protocol string
		httpAdapter, err := registry.GetByProtocol("http-api")
		assert.NoError(t, err)
		assert.NotNil(t, httpAdapter)
		assert.Equal(t, protocol.AdapterTypeHTTPAPI, httpAdapter.Type())
		
		// Test listing
		types := registry.List()
		assert.Len(t, types, 4)
	})
	
	t.Run("DuplicateRegistration", func(t *testing.T) {
		registry := protocol.NewRegistry()
		
		// First registration should succeed
		require.NoError(t, registry.Register(protocol.NewSQLAdapter()))
		
		// Second registration should fail
		err := registry.Register(protocol.NewSQLAdapter())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})
	
	t.Run("UnregisteredAdapter", func(t *testing.T) {
		registry := protocol.NewRegistry()
		
		// Try to get unregistered adapter
		_, err := registry.Get(protocol.AdapterTypeSQL)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no adapter registered")
	})
}

func TestHTTPAPIAdapter(t *testing.T) {
	adapter := protocol.NewHTTPAPIAdapter()
	
	t.Run("BasicProperties", func(t *testing.T) {
		assert.Equal(t, "HTTP API Protocol Adapter", adapter.Name())
		assert.Equal(t, protocol.AdapterTypeHTTPAPI, adapter.Type())
		
		caps := adapter.Capabilities()
		assert.Contains(t, caps.SupportedActions, "create")
		assert.Contains(t, caps.SupportedActions, "verify")
		assert.Contains(t, caps.SupportedActions, "rotate")
		assert.Contains(t, caps.RequiredConfig, "base_url")
	})
	
	t.Run("ValidateConfiguration", func(t *testing.T) {
		// Valid config
		validConfig := protocol.AdapterConfig{
			Connection: map[string]string{
				"base_url": "https://api.example.com",
			},
			Auth: map[string]string{
				"type":  "bearer",
				"value": "test-token",
			},
		}
		assert.NoError(t, adapter.Validate(validConfig))
		
		// Missing connection
		invalidConfig := protocol.AdapterConfig{}
		assert.Error(t, adapter.Validate(invalidConfig))
		
		// Missing base_url
		missingURL := protocol.AdapterConfig{
			Connection: map[string]string{},
		}
		assert.Error(t, adapter.Validate(missingURL))
		
		// Invalid auth type
		invalidAuth := protocol.AdapterConfig{
			Connection: map[string]string{
				"base_url": "https://api.example.com",
			},
			Auth: map[string]string{
				"type": "invalid",
			},
		}
		assert.Error(t, adapter.Validate(invalidAuth))
	})
}

func TestSQLAdapter(t *testing.T) {
	adapter := protocol.NewSQLAdapter()
	
	t.Run("BasicProperties", func(t *testing.T) {
		assert.Equal(t, "SQL Protocol Adapter", adapter.Name())
		assert.Equal(t, protocol.AdapterTypeSQL, adapter.Type())
		
		caps := adapter.Capabilities()
		assert.Contains(t, caps.SupportedActions, "create")
		assert.Contains(t, caps.SupportedActions, "verify")
		assert.Contains(t, caps.SupportedActions, "revoke")
		assert.Contains(t, caps.RequiredConfig, "host")
		assert.Contains(t, caps.RequiredConfig, "database")
		assert.True(t, caps.Features["transactions"])
	})
	
	t.Run("ValidateConfiguration", func(t *testing.T) {
		// Valid PostgreSQL config
		validConfig := protocol.AdapterConfig{
			Connection: map[string]string{
				"type":     "postgresql",
				"host":     "localhost",
				"port":     "5432",
				"database": "mydb",
			},
			Auth: map[string]string{
				"username": "user",
				"password": "pass",
			},
		}
		assert.NoError(t, adapter.Validate(validConfig))
		
		// Missing required fields
		invalidConfig := protocol.AdapterConfig{
			Connection: map[string]string{
				"type": "postgresql",
			},
		}
		assert.Error(t, adapter.Validate(invalidConfig))
		
		// Unsupported database type
		unsupportedDB := protocol.AdapterConfig{
			Connection: map[string]string{
				"type":     "oracle",
				"host":     "localhost",
				"port":     "1521",
				"database": "orcl",
			},
			Auth: map[string]string{
				"username": "user",
			},
		}
		assert.Error(t, adapter.Validate(unsupportedDB))
	})
}

func TestNoSQLAdapter(t *testing.T) {
	adapter := protocol.NewNoSQLAdapter()
	
	t.Run("BasicProperties", func(t *testing.T) {
		assert.Equal(t, "NoSQL Protocol Adapter", adapter.Name())
		assert.Equal(t, protocol.AdapterTypeNoSQL, adapter.Type())
		
		caps := adapter.Capabilities()
		assert.Contains(t, caps.SupportedActions, "create")
		assert.Contains(t, caps.SupportedActions, "list")
		assert.Contains(t, caps.RequiredConfig, "type")
		assert.True(t, caps.Features["json_support"])
	})
	
	t.Run("ValidateConfiguration", func(t *testing.T) {
		// Valid MongoDB config
		validConfig := protocol.AdapterConfig{
			Connection: map[string]string{
				"type": "mongodb",
				"host": "localhost",
				"port": "27017",
			},
		}
		assert.NoError(t, adapter.Validate(validConfig))
		
		// Missing type
		missingType := protocol.AdapterConfig{
			Connection: map[string]string{
				"host": "localhost",
			},
		}
		assert.Error(t, adapter.Validate(missingType))
		
		// Unsupported NoSQL type
		unsupportedType := protocol.AdapterConfig{
			Connection: map[string]string{
				"type": "cassandra",
				"host": "localhost",
				"port": "9042",
			},
		}
		assert.Error(t, adapter.Validate(unsupportedType))
	})
}

func TestCertificateAdapter(t *testing.T) {
	adapter := protocol.NewCertificateAdapter()
	
	t.Run("BasicProperties", func(t *testing.T) {
		assert.Equal(t, "Certificate Protocol Adapter", adapter.Name())
		assert.Equal(t, protocol.AdapterTypeCertificate, adapter.Type())
		
		caps := adapter.Capabilities()
		assert.Contains(t, caps.SupportedActions, "create")
		assert.Contains(t, caps.SupportedActions, "verify")
		assert.Contains(t, caps.SupportedActions, "revoke")
		assert.Contains(t, caps.RequiredConfig, "type")
		assert.True(t, caps.Features["x509"])
	})
	
	t.Run("ValidateConfiguration", func(t *testing.T) {
		// Valid self-signed config
		validConfig := protocol.AdapterConfig{
			Connection: map[string]string{
				"type": "self-signed",
			},
		}
		assert.NoError(t, adapter.Validate(validConfig))
		
		// Valid ACME config
		acmeConfig := protocol.AdapterConfig{
			Connection: map[string]string{
				"type":           "acme",
				"acme_directory": "https://acme-v02.api.letsencrypt.org/directory",
			},
		}
		assert.NoError(t, adapter.Validate(acmeConfig))
		
		// Missing type
		missingType := protocol.AdapterConfig{
			Connection: map[string]string{},
		}
		assert.Error(t, adapter.Validate(missingType))
		
		// ACME without directory
		invalidACME := protocol.AdapterConfig{
			Connection: map[string]string{
				"type": "acme",
			},
		}
		assert.Error(t, adapter.Validate(invalidACME))
	})
}

func TestSelfSignedCertificateGeneration(t *testing.T) {
	adapter := protocol.NewCertificateAdapter()
	ctx := context.Background()
	
	config := protocol.AdapterConfig{
		Connection: map[string]string{
			"type": "self-signed",
		},
	}
	
	operation := protocol.Operation{
		Action: "create",
		Target: "certificate",
		Parameters: map[string]interface{}{
			"common_name":   "test.example.com",
			"dns_names":     []string{"test.example.com", "www.test.example.com"},
			"validity_days": 365.0,
			"key_size":      2048.0,
			"organization":  "Test Org",
		},
		Metadata: map[string]string{
			"purpose": "testing",
		},
	}
	
	result, err := adapter.Execute(ctx, operation, config)
	require.NoError(t, err)
	assert.True(t, result.Success)
	
	// Check that certificate and key were generated
	assert.Contains(t, result.Data, "certificate")
	assert.Contains(t, result.Data, "private_key")
	assert.Contains(t, result.Data, "serial_number")
	
	// Verify the certificate is valid PEM
	certPEM := result.Data["certificate"].(string)
	assert.Contains(t, certPEM, "BEGIN CERTIFICATE")
	assert.Contains(t, certPEM, "END CERTIFICATE")
	
	keyPEM := result.Data["private_key"].(string)
	assert.Contains(t, keyPEM, "BEGIN RSA PRIVATE KEY")
	assert.Contains(t, keyPEM, "END RSA PRIVATE KEY")
}

func TestDefaultRegistry(t *testing.T) {
	// Test that we can use the default registry
	assert.NotNil(t, protocol.DefaultRegistry)
	
	// Register an adapter
	adapter := protocol.NewHTTPAPIAdapter()
	err := protocol.Register(adapter)
	assert.NoError(t, err)
	
	// Get it back
	retrieved, err := protocol.Get(protocol.AdapterTypeHTTPAPI)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, adapter.Name(), retrieved.Name())
	
	// Get by protocol string
	byProtocol, err := protocol.GetByProtocol("http-api")
	assert.NoError(t, err)
	assert.NotNil(t, byProtocol)
}