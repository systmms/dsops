package protocol

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// CertificateAdapter implements the Adapter interface for certificate operations
type CertificateAdapter struct {
	// Handlers for different certificate protocols
	handlers map[string]CertificateHandler
}

// CertificateHandler defines the interface for specific certificate protocol handlers
type CertificateHandler interface {
	GenerateCertificate(ctx context.Context, req CertificateRequest, config AdapterConfig) (*CertificateResult, error)
	VerifyCertificate(ctx context.Context, cert []byte, config AdapterConfig) error
	RevokeCertificate(ctx context.Context, serial string, config AdapterConfig) error
	ListCertificates(ctx context.Context, config AdapterConfig) ([]CertificateInfo, error)
}

// CertificateRequest contains parameters for certificate generation
type CertificateRequest struct {
	CommonName         string
	Organization       []string
	OrganizationalUnit []string
	Country            []string
	Province           []string
	Locality           []string
	DNSNames           []string
	EmailAddresses     []string
	IPAddresses        []string
	ValidityDays       int
	KeySize            int
}

// CertificateResult contains the generated certificate and key
type CertificateResult struct {
	Certificate []byte
	PrivateKey  []byte
	CertificateChain []byte
	SerialNumber string
	NotBefore    time.Time
	NotAfter     time.Time
}

// CertificateInfo contains certificate metadata
type CertificateInfo struct {
	SerialNumber string
	Subject      string
	Issuer       string
	NotBefore    time.Time
	NotAfter     time.Time
	DNSNames     []string
	Status       string
}

// NewCertificateAdapter creates a new certificate protocol adapter
func NewCertificateAdapter() *CertificateAdapter {
	return &CertificateAdapter{
		handlers: map[string]CertificateHandler{
			"self-signed": &SelfSignedHandler{},
			"acme":        &ACMEHandler{},
			// Additional handlers can be added here (Venafi, AWS ACM, etc.)
		},
	}
}

// Name returns the adapter name
func (a *CertificateAdapter) Name() string {
	return "Certificate Protocol Adapter"
}

// Type returns the adapter type
func (a *CertificateAdapter) Type() AdapterType {
	return AdapterTypeCertificate
}

// Execute performs a certificate operation
func (a *CertificateAdapter) Execute(ctx context.Context, operation Operation, config AdapterConfig) (*Result, error) {
	// Validate configuration
	if err := a.Validate(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Get the appropriate handler
	certType := strings.ToLower(config.Connection["type"])
	handler, ok := a.handlers[certType]
	if !ok {
		return nil, fmt.Errorf("unsupported certificate type: %s", certType)
	}
	
	// Execute the operation
	switch operation.Action {
	case "create":
		return a.executeCreate(ctx, handler, operation, config)
	case "verify":
		return a.executeVerify(ctx, handler, operation, config)
	case "rotate":
		return a.executeRotate(ctx, handler, operation, config)
	case "revoke":
		return a.executeRevoke(ctx, handler, operation, config)
	case "list":
		return a.executeList(ctx, handler, operation, config)
	default:
		return nil, fmt.Errorf("unsupported action: %s", operation.Action)
	}
}

// Validate checks if the configuration is valid
func (a *CertificateAdapter) Validate(config AdapterConfig) error {
	if config.Connection == nil {
		return fmt.Errorf("connection configuration is required")
	}
	
	certType, ok := config.Connection["type"]
	if !ok || certType == "" {
		return fmt.Errorf("certificate type is required")
	}
	
	// Validate based on certificate type
	switch strings.ToLower(certType) {
	case "self-signed":
		// No additional validation needed
	case "acme":
		if config.Connection["acme_directory"] == "" {
			return fmt.Errorf("acme_directory is required for ACME certificates")
		}
	default:
		return fmt.Errorf("unsupported certificate type: %s", certType)
	}
	
	return nil
}

// Capabilities returns what this adapter can do
func (a *CertificateAdapter) Capabilities() Capabilities {
	return Capabilities{
		SupportedActions: []string{"create", "verify", "rotate", "revoke", "list"},
		RequiredConfig:   []string{"type"},
		OptionalConfig:   []string{"common_name", "dns_names", "validity_days", "key_size"},
		Features: map[string]bool{
			"x509":         true,
			"rsa":          true,
			"ecdsa":        true,
			"chain":        true,
			"revocation":   true,
		},
	}
}

// executeCreate creates a new certificate
func (a *CertificateAdapter) executeCreate(ctx context.Context, handler CertificateHandler, operation Operation, config AdapterConfig) (*Result, error) {
	// Build certificate request from operation parameters
	req := a.buildCertificateRequest(operation, config)
	
	// Generate certificate
	certResult, err := handler.GenerateCertificate(ctx, req, config)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to generate certificate: %v", err),
		}, err
	}
	
	return &Result{
		Success: true,
		Data: map[string]interface{}{
			"action":        "create",
			"target":        operation.Target,
			"certificate":   string(certResult.Certificate),
			"private_key":   string(certResult.PrivateKey),
			"serial_number": certResult.SerialNumber,
			"not_before":    certResult.NotBefore.Format(time.RFC3339),
			"not_after":     certResult.NotAfter.Format(time.RFC3339),
		},
		Metadata: map[string]string{
			"certificate_type": config.Connection["type"],
		},
	}, nil
}

// executeVerify verifies a certificate
func (a *CertificateAdapter) executeVerify(ctx context.Context, handler CertificateHandler, operation Operation, config AdapterConfig) (*Result, error) {
	// Get certificate from parameters
	certPEM, ok := operation.Parameters["certificate"].(string)
	if !ok {
		return nil, fmt.Errorf("certificate parameter is required for verify")
	}
	
	// Verify certificate
	err := handler.VerifyCertificate(ctx, []byte(certPEM), config)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("certificate verification failed: %v", err),
		}, err
	}
	
	return &Result{
		Success: true,
		Data: map[string]interface{}{
			"action":   "verify",
			"target":   operation.Target,
			"verified": true,
		},
	}, nil
}

// executeRotate rotates a certificate
func (a *CertificateAdapter) executeRotate(ctx context.Context, handler CertificateHandler, operation Operation, config AdapterConfig) (*Result, error) {
	// For rotation, we create a new certificate and optionally revoke the old one
	req := a.buildCertificateRequest(operation, config)
	
	// Generate new certificate
	certResult, err := handler.GenerateCertificate(ctx, req, config)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to rotate certificate: %v", err),
		}, err
	}
	
	// If old serial number provided, revoke it
	if oldSerial, ok := operation.Parameters["old_serial_number"].(string); ok && oldSerial != "" {
		_ = handler.RevokeCertificate(ctx, oldSerial, config) // Best effort revocation
	}
	
	return &Result{
		Success: true,
		Data: map[string]interface{}{
			"action":        "rotate",
			"target":        operation.Target,
			"certificate":   string(certResult.Certificate),
			"private_key":   string(certResult.PrivateKey),
			"serial_number": certResult.SerialNumber,
			"not_before":    certResult.NotBefore.Format(time.RFC3339),
			"not_after":     certResult.NotAfter.Format(time.RFC3339),
		},
	}, nil
}

// executeRevoke revokes a certificate
func (a *CertificateAdapter) executeRevoke(ctx context.Context, handler CertificateHandler, operation Operation, config AdapterConfig) (*Result, error) {
	// Get serial number from parameters
	serialNumber, ok := operation.Parameters["serial_number"].(string)
	if !ok {
		return nil, fmt.Errorf("serial_number parameter is required for revoke")
	}
	
	// Revoke certificate
	err := handler.RevokeCertificate(ctx, serialNumber, config)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to revoke certificate: %v", err),
		}, err
	}
	
	return &Result{
		Success: true,
		Data: map[string]interface{}{
			"action":        "revoke",
			"target":        operation.Target,
			"serial_number": serialNumber,
			"revoked":       true,
		},
	}, nil
}

// executeList lists certificates
func (a *CertificateAdapter) executeList(ctx context.Context, handler CertificateHandler, operation Operation, config AdapterConfig) (*Result, error) {
	// List certificates
	certs, err := handler.ListCertificates(ctx, config)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to list certificates: %v", err),
		}, err
	}
	
	// Convert to generic format
	items := make([]map[string]interface{}, len(certs))
	for i, cert := range certs {
		items[i] = map[string]interface{}{
			"serial_number": cert.SerialNumber,
			"subject":       cert.Subject,
			"issuer":        cert.Issuer,
			"not_before":    cert.NotBefore.Format(time.RFC3339),
			"not_after":     cert.NotAfter.Format(time.RFC3339),
			"dns_names":     cert.DNSNames,
			"status":        cert.Status,
		}
	}
	
	return &Result{
		Success: true,
		Data: map[string]interface{}{
			"action": "list",
			"target": operation.Target,
			"items":  items,
			"count":  len(items),
		},
	}, nil
}

// buildCertificateRequest builds a certificate request from operation parameters
func (a *CertificateAdapter) buildCertificateRequest(operation Operation, config AdapterConfig) CertificateRequest {
	req := CertificateRequest{
		ValidityDays: 365, // Default
		KeySize:      2048, // Default
	}
	
	// Extract from operation parameters
	if cn, ok := operation.Parameters["common_name"].(string); ok {
		req.CommonName = cn
	}
	
	if dnsNames, ok := operation.Parameters["dns_names"].([]string); ok {
		req.DNSNames = dnsNames
	} else if dnsName, ok := operation.Parameters["dns_name"].(string); ok {
		req.DNSNames = []string{dnsName}
	}
	
	if validity, ok := operation.Parameters["validity_days"].(float64); ok {
		req.ValidityDays = int(validity)
	}
	
	if keySize, ok := operation.Parameters["key_size"].(float64); ok {
		req.KeySize = int(keySize)
	}
	
	// Extract organization info if provided
	if org, ok := operation.Parameters["organization"].([]string); ok {
		req.Organization = org
	} else if org, ok := operation.Parameters["organization"].(string); ok {
		req.Organization = []string{org}
	}
	
	return req
}

// SelfSignedHandler handles self-signed certificate generation
type SelfSignedHandler struct{}

func (h *SelfSignedHandler) GenerateCertificate(ctx context.Context, req CertificateRequest, config AdapterConfig) (*CertificateResult, error) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, req.KeySize)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}
	
	// Create certificate template
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			CommonName:         req.CommonName,
			Organization:       req.Organization,
			OrganizationalUnit: req.OrganizationalUnit,
			Country:            req.Country,
			Province:           req.Province,
			Locality:           req.Locality,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, req.ValidityDays),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              req.DNSNames,
	}
	
	// Generate certificate
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}
	
	// Encode certificate to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	
	// Encode private key to PEM
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	
	return &CertificateResult{
		Certificate:  certPEM,
		PrivateKey:   keyPEM,
		SerialNumber: template.SerialNumber.String(),
		NotBefore:    template.NotBefore,
		NotAfter:     template.NotAfter,
	}, nil
}

func (h *SelfSignedHandler) VerifyCertificate(ctx context.Context, certPEM []byte, config AdapterConfig) error {
	// Decode PEM
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("failed to decode certificate PEM")
	}
	
	// Parse certificate
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}
	
	// Basic validity check
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("certificate not yet valid")
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("certificate has expired")
	}
	
	return nil
}

func (h *SelfSignedHandler) RevokeCertificate(ctx context.Context, serial string, config AdapterConfig) error {
	// Self-signed certificates don't have real revocation
	// In a real implementation, this might update a CRL
	return nil
}

func (h *SelfSignedHandler) ListCertificates(ctx context.Context, config AdapterConfig) ([]CertificateInfo, error) {
	// Self-signed handler doesn't maintain a list
	// In a real implementation, this might query a certificate store
	return []CertificateInfo{}, nil
}

// ACMEHandler handles ACME protocol certificates (placeholder)
type ACMEHandler struct{}

func (h *ACMEHandler) GenerateCertificate(ctx context.Context, req CertificateRequest, config AdapterConfig) (*CertificateResult, error) {
	// This is a placeholder - real implementation would use an ACME client
	return nil, fmt.Errorf("ACME handler not yet implemented")
}

func (h *ACMEHandler) VerifyCertificate(ctx context.Context, cert []byte, config AdapterConfig) error {
	return fmt.Errorf("ACME handler not yet implemented")
}

func (h *ACMEHandler) RevokeCertificate(ctx context.Context, serial string, config AdapterConfig) error {
	return fmt.Errorf("ACME handler not yet implemented")
}

func (h *ACMEHandler) ListCertificates(ctx context.Context, config AdapterConfig) ([]CertificateInfo, error) {
	return nil, fmt.Errorf("ACME handler not yet implemented")
}