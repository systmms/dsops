package protocol

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FakeCertificateHandler implements CertificateHandler for testing
type FakeCertificateHandler struct {
	generateResponse *CertificateResult
	generateError    error
	verifyError      error
	revokeError      error
	listResponse     []CertificateInfo
	listError        error
}

func (h *FakeCertificateHandler) GenerateCertificate(ctx context.Context, req CertificateRequest, config AdapterConfig) (*CertificateResult, error) {
	if h.generateError != nil {
		return nil, h.generateError
	}
	return h.generateResponse, nil
}

func (h *FakeCertificateHandler) VerifyCertificate(ctx context.Context, cert []byte, config AdapterConfig) error {
	return h.verifyError
}

func (h *FakeCertificateHandler) RevokeCertificate(ctx context.Context, serial string, config AdapterConfig) error {
	return h.revokeError
}

func (h *FakeCertificateHandler) ListCertificates(ctx context.Context, config AdapterConfig) ([]CertificateInfo, error) {
	if h.listError != nil {
		return nil, h.listError
	}
	return h.listResponse, nil
}

// TestCertificateAdapterBasics tests basic adapter properties
func TestCertificateAdapterBasics(t *testing.T) {
	adapter := NewCertificateAdapter()

	assert.Equal(t, "Certificate Protocol Adapter", adapter.Name())
	assert.Equal(t, AdapterTypeCertificate, adapter.Type())
}

// TestCertificateAdapterCapabilities tests capability reporting
func TestCertificateAdapterCapabilities(t *testing.T) {
	adapter := NewCertificateAdapter()
	caps := adapter.Capabilities()

	expectedActions := []string{"create", "verify", "rotate", "revoke", "list"}
	assert.Equal(t, expectedActions, caps.SupportedActions)

	expectedRequired := []string{"type"}
	assert.Equal(t, expectedRequired, caps.RequiredConfig)

	assert.True(t, caps.Features["x509"])
	assert.True(t, caps.Features["rsa"])
	assert.True(t, caps.Features["ecdsa"])
	assert.True(t, caps.Features["chain"])
	assert.True(t, caps.Features["revocation"])
}

// TestCertificateAdapterValidate tests configuration validation
func TestCertificateAdapterValidate(t *testing.T) {
	adapter := NewCertificateAdapter()

	tests := []struct {
		name          string
		config        AdapterConfig
		expectError   bool
		errorContains string
	}{
		{
			name: "valid_self_signed",
			config: AdapterConfig{
				Connection: map[string]string{
					"type": "self-signed",
				},
			},
			expectError: false,
		},
		{
			name: "valid_acme",
			config: AdapterConfig{
				Connection: map[string]string{
					"type":           "acme",
					"acme_directory": "https://acme-v02.api.letsencrypt.org/directory",
				},
			},
			expectError: false,
		},
		{
			name:          "nil_connection",
			config:        AdapterConfig{},
			expectError:   true,
			errorContains: "connection configuration is required",
		},
		{
			name: "missing_type",
			config: AdapterConfig{
				Connection: map[string]string{},
			},
			expectError:   true,
			errorContains: "certificate type is required",
		},
		{
			name: "empty_type",
			config: AdapterConfig{
				Connection: map[string]string{
					"type": "",
				},
			},
			expectError:   true,
			errorContains: "certificate type is required",
		},
		{
			name: "unsupported_type",
			config: AdapterConfig{
				Connection: map[string]string{
					"type": "venafi",
				},
			},
			expectError:   true,
			errorContains: "unsupported certificate type",
		},
		{
			name: "acme_missing_directory",
			config: AdapterConfig{
				Connection: map[string]string{
					"type": "acme",
				},
			},
			expectError:   true,
			errorContains: "acme_directory is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.Validate(tt.config)
			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestBuildCertificateRequest tests certificate request building
func TestBuildCertificateRequest(t *testing.T) {
	adapter := NewCertificateAdapter()

	tests := []struct {
		name      string
		operation Operation
		config    AdapterConfig
		check     func(t *testing.T, req CertificateRequest)
	}{
		{
			name: "default_values",
			operation: Operation{
				Action:     "create",
				Parameters: map[string]interface{}{},
			},
			config: AdapterConfig{},
			check: func(t *testing.T, req CertificateRequest) {
				assert.Equal(t, 365, req.ValidityDays)
				assert.Equal(t, 2048, req.KeySize)
				assert.Empty(t, req.CommonName)
			},
		},
		{
			name: "with_common_name",
			operation: Operation{
				Action: "create",
				Parameters: map[string]interface{}{
					"common_name": "example.com",
				},
			},
			config: AdapterConfig{},
			check: func(t *testing.T, req CertificateRequest) {
				assert.Equal(t, "example.com", req.CommonName)
			},
		},
		{
			name: "with_dns_names_array",
			operation: Operation{
				Action: "create",
				Parameters: map[string]interface{}{
					"dns_names": []string{"example.com", "www.example.com"},
				},
			},
			config: AdapterConfig{},
			check: func(t *testing.T, req CertificateRequest) {
				assert.Equal(t, []string{"example.com", "www.example.com"}, req.DNSNames)
			},
		},
		{
			name: "with_single_dns_name",
			operation: Operation{
				Action: "create",
				Parameters: map[string]interface{}{
					"dns_name": "single.example.com",
				},
			},
			config: AdapterConfig{},
			check: func(t *testing.T, req CertificateRequest) {
				assert.Equal(t, []string{"single.example.com"}, req.DNSNames)
			},
		},
		{
			name: "with_custom_validity",
			operation: Operation{
				Action: "create",
				Parameters: map[string]interface{}{
					"validity_days": float64(90),
				},
			},
			config: AdapterConfig{},
			check: func(t *testing.T, req CertificateRequest) {
				assert.Equal(t, 90, req.ValidityDays)
			},
		},
		{
			name: "with_custom_key_size",
			operation: Operation{
				Action: "create",
				Parameters: map[string]interface{}{
					"key_size": float64(4096),
				},
			},
			config: AdapterConfig{},
			check: func(t *testing.T, req CertificateRequest) {
				assert.Equal(t, 4096, req.KeySize)
			},
		},
		{
			name: "with_organization_array",
			operation: Operation{
				Action: "create",
				Parameters: map[string]interface{}{
					"organization": []string{"Example Corp", "IT Dept"},
				},
			},
			config: AdapterConfig{},
			check: func(t *testing.T, req CertificateRequest) {
				assert.Equal(t, []string{"Example Corp", "IT Dept"}, req.Organization)
			},
		},
		{
			name: "with_organization_string",
			operation: Operation{
				Action: "create",
				Parameters: map[string]interface{}{
					"organization": "Single Org",
				},
			},
			config: AdapterConfig{},
			check: func(t *testing.T, req CertificateRequest) {
				assert.Equal(t, []string{"Single Org"}, req.Organization)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := adapter.buildCertificateRequest(tt.operation, tt.config)
			tt.check(t, req)
		})
	}
}

// TestSelfSignedHandlerGenerateCertificate tests self-signed certificate generation
func TestSelfSignedHandlerGenerateCertificate(t *testing.T) {
	handler := &SelfSignedHandler{}
	ctx := context.Background()

	tests := []struct {
		name        string
		request     CertificateRequest
		expectError bool
	}{
		{
			name: "basic_certificate",
			request: CertificateRequest{
				CommonName:   "test.example.com",
				ValidityDays: 30,
				KeySize:      2048,
			},
			expectError: false,
		},
		{
			name: "with_dns_names",
			request: CertificateRequest{
				CommonName:   "test.example.com",
				DNSNames:     []string{"test.example.com", "www.test.example.com"},
				ValidityDays: 365,
				KeySize:      2048,
			},
			expectError: false,
		},
		{
			name: "with_organization",
			request: CertificateRequest{
				CommonName:   "corporate.example.com",
				Organization: []string{"Example Corp"},
				Country:      []string{"US"},
				ValidityDays: 365,
				KeySize:      2048,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.GenerateCertificate(ctx, tt.request, AdapterConfig{})

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				// Verify certificate PEM
				block, _ := pem.Decode(result.Certificate)
				require.NotNil(t, block)
				assert.Equal(t, "CERTIFICATE", block.Type)

				// Parse certificate to verify
				cert, err := x509.ParseCertificate(block.Bytes)
				require.NoError(t, err)
				assert.Equal(t, tt.request.CommonName, cert.Subject.CommonName)

				// Verify private key PEM
				keyBlock, _ := pem.Decode(result.PrivateKey)
				require.NotNil(t, keyBlock)
				assert.Equal(t, "RSA PRIVATE KEY", keyBlock.Type)

				// Verify serial number
				assert.NotEmpty(t, result.SerialNumber)

				// Verify validity period
				assert.False(t, result.NotBefore.IsZero())
				assert.False(t, result.NotAfter.IsZero())
				assert.True(t, result.NotAfter.After(result.NotBefore))
			}
		})
	}
}

// TestSelfSignedHandlerVerifyCertificate tests certificate verification
func TestSelfSignedHandlerVerifyCertificate(t *testing.T) {
	handler := &SelfSignedHandler{}
	ctx := context.Background()

	t.Run("valid_certificate", func(t *testing.T) {
		// Generate a valid certificate first
		req := CertificateRequest{
			CommonName:   "test.example.com",
			ValidityDays: 365,
			KeySize:      2048,
		}
		result, err := handler.GenerateCertificate(ctx, req, AdapterConfig{})
		require.NoError(t, err)

		// Verify it
		err = handler.VerifyCertificate(ctx, result.Certificate, AdapterConfig{})
		assert.NoError(t, err)
	})

	t.Run("invalid_pem", func(t *testing.T) {
		err := handler.VerifyCertificate(ctx, []byte("not valid pem"), AdapterConfig{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode")
	})

	t.Run("expired_certificate", func(t *testing.T) {
		// Generate a certificate that's already expired (negative validity)
		req := CertificateRequest{
			CommonName:   "expired.example.com",
			ValidityDays: -1, // Will be expired
			KeySize:      2048,
		}
		result, err := handler.GenerateCertificate(ctx, req, AdapterConfig{})
		require.NoError(t, err)

		err = handler.VerifyCertificate(ctx, result.Certificate, AdapterConfig{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})
}

// TestSelfSignedHandlerRevokeCertificate tests certificate revocation
func TestSelfSignedHandlerRevokeCertificate(t *testing.T) {
	handler := &SelfSignedHandler{}
	ctx := context.Background()

	// Self-signed handler doesn't do real revocation
	err := handler.RevokeCertificate(ctx, "12345", AdapterConfig{})
	assert.NoError(t, err)
}

// TestSelfSignedHandlerListCertificates tests certificate listing
func TestSelfSignedHandlerListCertificates(t *testing.T) {
	handler := &SelfSignedHandler{}
	ctx := context.Background()

	// Self-signed handler returns empty list
	certs, err := handler.ListCertificates(ctx, AdapterConfig{})
	assert.NoError(t, err)
	assert.Empty(t, certs)
}

// TestACMEHandlerNotImplemented tests ACME handler placeholder
func TestACMEHandlerNotImplemented(t *testing.T) {
	handler := &ACMEHandler{}
	ctx := context.Background()

	t.Run("generate", func(t *testing.T) {
		_, err := handler.GenerateCertificate(ctx, CertificateRequest{}, AdapterConfig{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not yet implemented")
	})

	t.Run("verify", func(t *testing.T) {
		err := handler.VerifyCertificate(ctx, []byte{}, AdapterConfig{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not yet implemented")
	})

	t.Run("revoke", func(t *testing.T) {
		err := handler.RevokeCertificate(ctx, "123", AdapterConfig{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not yet implemented")
	})

	t.Run("list", func(t *testing.T) {
		_, err := handler.ListCertificates(ctx, AdapterConfig{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not yet implemented")
	})
}

// TestCertificateAdapterExecute tests full execute operations
func TestCertificateAdapterExecute(t *testing.T) {
	adapter := NewCertificateAdapter()

	t.Run("unsupported_action", func(t *testing.T) {
		config := AdapterConfig{
			Connection: map[string]string{
				"type": "self-signed",
			},
		}
		operation := Operation{
			Action: "unknown",
			Target: "certificate",
		}

		_, err := adapter.Execute(context.Background(), operation, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported action")
	})

	t.Run("unsupported_certificate_type", func(t *testing.T) {
		// Bypass validation by setting type after validation
		adapter := NewCertificateAdapter()
		config := AdapterConfig{
			Connection: map[string]string{
				"type": "self-signed",
			},
		}

		// Replace handler map to simulate unknown type
		adapter.handlers = map[string]CertificateHandler{}

		operation := Operation{
			Action: "create",
			Target: "certificate",
		}

		_, err := adapter.Execute(context.Background(), operation, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported certificate type")
	})

	t.Run("verify_missing_certificate_parameter", func(t *testing.T) {
		config := AdapterConfig{
			Connection: map[string]string{
				"type": "self-signed",
			},
		}
		operation := Operation{
			Action:     "verify",
			Target:     "certificate",
			Parameters: map[string]interface{}{},
		}

		_, err := adapter.Execute(context.Background(), operation, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "certificate parameter is required")
	})

	t.Run("revoke_missing_serial_parameter", func(t *testing.T) {
		config := AdapterConfig{
			Connection: map[string]string{
				"type": "self-signed",
			},
		}
		operation := Operation{
			Action:     "revoke",
			Target:     "certificate",
			Parameters: map[string]interface{}{},
		}

		_, err := adapter.Execute(context.Background(), operation, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "serial_number parameter is required")
	})

	t.Run("create_success", func(t *testing.T) {
		config := AdapterConfig{
			Connection: map[string]string{
				"type": "self-signed",
			},
		}
		operation := Operation{
			Action: "create",
			Target: "certificate",
			Parameters: map[string]interface{}{
				"common_name":   "test.example.com",
				"validity_days": float64(30),
			},
		}

		result, err := adapter.Execute(context.Background(), operation, config)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, "create", result.Data["action"])
		assert.NotEmpty(t, result.Data["certificate"])
		assert.NotEmpty(t, result.Data["private_key"])
		assert.NotEmpty(t, result.Data["serial_number"])
	})

	t.Run("verify_success", func(t *testing.T) {
		// First create a certificate
		config := AdapterConfig{
			Connection: map[string]string{
				"type": "self-signed",
			},
		}
		createOp := Operation{
			Action: "create",
			Target: "certificate",
			Parameters: map[string]interface{}{
				"common_name": "test.example.com",
			},
		}
		createResult, err := adapter.Execute(context.Background(), createOp, config)
		require.NoError(t, err)

		// Now verify it
		verifyOp := Operation{
			Action: "verify",
			Target: "certificate",
			Parameters: map[string]interface{}{
				"certificate": createResult.Data["certificate"],
			},
		}
		result, err := adapter.Execute(context.Background(), verifyOp, config)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.True(t, result.Data["verified"].(bool))
	})

	t.Run("rotate_success", func(t *testing.T) {
		config := AdapterConfig{
			Connection: map[string]string{
				"type": "self-signed",
			},
		}
		operation := Operation{
			Action: "rotate",
			Target: "certificate",
			Parameters: map[string]interface{}{
				"common_name":       "rotated.example.com",
				"old_serial_number": "12345",
			},
		}

		result, err := adapter.Execute(context.Background(), operation, config)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, "rotate", result.Data["action"])
		assert.NotEmpty(t, result.Data["certificate"])
	})

	t.Run("revoke_success", func(t *testing.T) {
		config := AdapterConfig{
			Connection: map[string]string{
				"type": "self-signed",
			},
		}
		operation := Operation{
			Action: "revoke",
			Target: "certificate",
			Parameters: map[string]interface{}{
				"serial_number": "12345",
			},
		}

		result, err := adapter.Execute(context.Background(), operation, config)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, "revoke", result.Data["action"])
		assert.True(t, result.Data["revoked"].(bool))
	})

	t.Run("list_success", func(t *testing.T) {
		config := AdapterConfig{
			Connection: map[string]string{
				"type": "self-signed",
			},
		}
		operation := Operation{
			Action: "list",
			Target: "certificates",
		}

		result, err := adapter.Execute(context.Background(), operation, config)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, "list", result.Data["action"])
		assert.Equal(t, 0, result.Data["count"])
	})
}
