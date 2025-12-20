package fakes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GCPSecretManagerAPI defines the interface for GCP Secret Manager operations
// This matches the subset of methods used by GCPSecretManagerProvider
type GCPSecretManagerAPI interface {
	AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error)
	GetSecret(ctx context.Context, req *secretmanagerpb.GetSecretRequest) (*secretmanagerpb.Secret, error)
	ListSecrets(ctx context.Context, req *secretmanagerpb.ListSecretsRequest) SecretIterator
	AddSecretVersion(ctx context.Context, req *secretmanagerpb.AddSecretVersionRequest) (*secretmanagerpb.SecretVersion, error)
	DisableSecretVersion(ctx context.Context, req *secretmanagerpb.DisableSecretVersionRequest) (*secretmanagerpb.SecretVersion, error)
}

// SecretIterator defines the interface for iterating over secrets
type SecretIterator interface {
	Next() (*secretmanagerpb.Secret, error)
}

// FakeGCPSecretManagerClient is a mock implementation of GCPSecretManagerAPI
type FakeGCPSecretManagerClient struct {
	// Secrets maps full resource names (projects/X/secrets/Y) to their data
	Secrets map[string]*GCPSecretData
	// Versions maps version resource names (projects/X/secrets/Y/versions/Z) to their data
	Versions map[string]*GCPSecretVersionData
	// Errors maps resource names to errors to return
	Errors map[string]error
	// AccessSecretVersionFunc allows custom behavior for AccessSecretVersion
	AccessSecretVersionFunc func(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error)
	// GetSecretFunc allows custom behavior for GetSecret
	GetSecretFunc func(ctx context.Context, req *secretmanagerpb.GetSecretRequest) (*secretmanagerpb.Secret, error)
	// ListSecretsFunc allows custom behavior for ListSecrets
	ListSecretsFunc func(ctx context.Context, req *secretmanagerpb.ListSecretsRequest) SecretIterator
	// AddSecretVersionFunc allows custom behavior for AddSecretVersion
	AddSecretVersionFunc func(ctx context.Context, req *secretmanagerpb.AddSecretVersionRequest) (*secretmanagerpb.SecretVersion, error)
	// DisableSecretVersionFunc allows custom behavior for DisableSecretVersion
	DisableSecretVersionFunc func(ctx context.Context, req *secretmanagerpb.DisableSecretVersionRequest) (*secretmanagerpb.SecretVersion, error)
}

// GCPSecretData holds the data for a mock GCP secret
type GCPSecretData struct {
	Name        string
	CreateTime  *timestamppb.Timestamp
	Labels      map[string]string
	Topics      []*secretmanagerpb.Topic
	Replication *secretmanagerpb.Replication
}

// GCPSecretVersionData holds version-specific data for a GCP secret
type GCPSecretVersionData struct {
	Name        string
	State       secretmanagerpb.SecretVersion_State
	CreateTime  *timestamppb.Timestamp
	DestroyTime *timestamppb.Timestamp
	Data        []byte
}

// NewFakeGCPSecretManagerClient creates a new mock GCP Secret Manager client
func NewFakeGCPSecretManagerClient() *FakeGCPSecretManagerClient {
	return &FakeGCPSecretManagerClient{
		Secrets:  make(map[string]*GCPSecretData),
		Versions: make(map[string]*GCPSecretVersionData),
		Errors:   make(map[string]error),
	}
}

// AddSecret adds a secret to the mock client
func (f *FakeGCPSecretManagerClient) AddSecret(projectID, secretName string, data *GCPSecretData) {
	fullName := fmt.Sprintf("projects/%s/secrets/%s", projectID, secretName)
	data.Name = fullName
	f.Secrets[fullName] = data
}

// AddMockSecretVersion adds a secret version to the mock client (helper method for setup)
func (f *FakeGCPSecretManagerClient) AddMockSecretVersion(projectID, secretName, version string, value []byte) {
	// Ensure secret exists
	secretFullName := fmt.Sprintf("projects/%s/secrets/%s", projectID, secretName)
	if _, exists := f.Secrets[secretFullName]; !exists {
		now := timestamppb.New(time.Now())
		f.Secrets[secretFullName] = &GCPSecretData{
			Name:       secretFullName,
			CreateTime: now,
			Labels:     make(map[string]string),
		}
	}

	// Add version
	versionFullName := fmt.Sprintf("projects/%s/secrets/%s/versions/%s", projectID, secretName, version)
	now := timestamppb.New(time.Now())
	f.Versions[versionFullName] = &GCPSecretVersionData{
		Name:       versionFullName,
		State:      secretmanagerpb.SecretVersion_ENABLED,
		CreateTime: now,
		Data:       value,
	}
}

// AddSecretString adds a string secret with latest version to the mock client
func (f *FakeGCPSecretManagerClient) AddSecretString(projectID, secretName, value string) {
	f.AddMockSecretVersion(projectID, secretName, "latest", []byte(value))
	// Also add as version "1" for typical access patterns
	f.AddMockSecretVersion(projectID, secretName, "1", []byte(value))
}

// AddSecretWithLabels adds a secret with labels
func (f *FakeGCPSecretManagerClient) AddSecretWithLabels(projectID, secretName string, labels map[string]string) {
	secretFullName := fmt.Sprintf("projects/%s/secrets/%s", projectID, secretName)
	now := timestamppb.New(time.Now())
	f.Secrets[secretFullName] = &GCPSecretData{
		Name:       secretFullName,
		CreateTime: now,
		Labels:     labels,
	}
}

// AddError configures the mock to return an error for a specific resource
func (f *FakeGCPSecretManagerClient) AddError(resourceName string, err error) {
	f.Errors[resourceName] = err
}

// AccessSecretVersion mocks the AccessSecretVersion operation
func (f *FakeGCPSecretManagerClient) AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	if f.AccessSecretVersionFunc != nil {
		return f.AccessSecretVersionFunc(ctx, req)
	}

	// Check for configured errors
	if err, exists := f.Errors[req.Name]; exists {
		return nil, err
	}

	// Check if version exists
	version, exists := f.Versions[req.Name]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Secret version %s not found", req.Name)
	}

	return &secretmanagerpb.AccessSecretVersionResponse{
		Name: version.Name,
		Payload: &secretmanagerpb.SecretPayload{
			Data: version.Data,
		},
	}, nil
}

// GetSecret mocks the GetSecret operation
func (f *FakeGCPSecretManagerClient) GetSecret(ctx context.Context, req *secretmanagerpb.GetSecretRequest) (*secretmanagerpb.Secret, error) {
	if f.GetSecretFunc != nil {
		return f.GetSecretFunc(ctx, req)
	}

	// Check for configured errors
	if err, exists := f.Errors[req.Name]; exists {
		return nil, err
	}

	// Check if secret exists
	secret, exists := f.Secrets[req.Name]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Secret %s not found", req.Name)
	}

	return &secretmanagerpb.Secret{
		Name:        secret.Name,
		CreateTime:  secret.CreateTime,
		Labels:      secret.Labels,
		Topics:      secret.Topics,
		Replication: secret.Replication,
	}, nil
}

// ListSecrets mocks the ListSecrets operation
func (f *FakeGCPSecretManagerClient) ListSecrets(ctx context.Context, req *secretmanagerpb.ListSecretsRequest) SecretIterator {
	if f.ListSecretsFunc != nil {
		return f.ListSecretsFunc(ctx, req)
	}

	// Extract project from parent (format: projects/PROJECT_ID)
	projectID := strings.TrimPrefix(req.Parent, "projects/")

	// Filter secrets by project
	var secrets []*secretmanagerpb.Secret
	prefix := fmt.Sprintf("projects/%s/secrets/", projectID)
	for name, data := range f.Secrets {
		if strings.HasPrefix(name, prefix) {
			secrets = append(secrets, &secretmanagerpb.Secret{
				Name:        data.Name,
				CreateTime:  data.CreateTime,
				Labels:      data.Labels,
				Topics:      data.Topics,
				Replication: data.Replication,
			})
		}
	}

	return &FakeSecretIterator{
		secrets: secrets,
		index:   0,
	}
}

// AddSecretVersion mocks the AddSecretVersion operation
func (f *FakeGCPSecretManagerClient) AddSecretVersion(ctx context.Context, req *secretmanagerpb.AddSecretVersionRequest) (*secretmanagerpb.SecretVersion, error) {
	if f.AddSecretVersionFunc != nil {
		return f.AddSecretVersionFunc(ctx, req)
	}

	// Check for configured errors
	if err, exists := f.Errors[req.Parent]; exists {
		return nil, err
	}

	// Ensure secret exists
	if _, exists := f.Secrets[req.Parent]; !exists {
		return nil, status.Errorf(codes.NotFound, "Secret %s not found", req.Parent)
	}

	// Generate new version number
	versionNum := 1
	for versionName := range f.Versions {
		if strings.HasPrefix(versionName, req.Parent+"/versions/") {
			versionNum++
		}
	}

	// Create new version
	versionName := fmt.Sprintf("%s/versions/%d", req.Parent, versionNum)
	now := timestamppb.New(time.Now())
	version := &GCPSecretVersionData{
		Name:       versionName,
		State:      secretmanagerpb.SecretVersion_ENABLED,
		CreateTime: now,
		Data:       req.Payload.Data,
	}

	f.Versions[versionName] = version

	return &secretmanagerpb.SecretVersion{
		Name:       version.Name,
		CreateTime: version.CreateTime,
		State:      version.State,
	}, nil
}

// DisableSecretVersion mocks the DisableSecretVersion operation
func (f *FakeGCPSecretManagerClient) DisableSecretVersion(ctx context.Context, req *secretmanagerpb.DisableSecretVersionRequest) (*secretmanagerpb.SecretVersion, error) {
	if f.DisableSecretVersionFunc != nil {
		return f.DisableSecretVersionFunc(ctx, req)
	}

	// Check for configured errors
	if err, exists := f.Errors[req.Name]; exists {
		return nil, err
	}

	// Check if version exists
	version, exists := f.Versions[req.Name]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Secret version %s not found", req.Name)
	}

	// Update state to disabled
	version.State = secretmanagerpb.SecretVersion_DISABLED

	return &secretmanagerpb.SecretVersion{
		Name:       version.Name,
		CreateTime: version.CreateTime,
		State:      version.State,
	}, nil
}

// FakeSecretIterator is a mock implementation of SecretIterator
type FakeSecretIterator struct {
	secrets []*secretmanagerpb.Secret
	index   int
	err     error
}

// Next returns the next secret in the iteration
func (it *FakeSecretIterator) Next() (*secretmanagerpb.Secret, error) {
	if it.err != nil {
		return nil, it.err
	}

	if it.index >= len(it.secrets) {
		return nil, iterator.Done
	}

	secret := it.secrets[it.index]
	it.index++
	return secret, nil
}

// NewFakeSecretIterator creates a new fake secret iterator
func NewFakeSecretIterator(secrets []*secretmanagerpb.Secret, err error) *FakeSecretIterator {
	return &FakeSecretIterator{
		secrets: secrets,
		index:   0,
		err:     err,
	}
}

// GCP error helpers

// GCPNotFoundError creates a mock GCP not found error
func GCPNotFoundError(resourceName string) error {
	return status.Errorf(codes.NotFound, "Resource %s not found", resourceName)
}

// GCPPermissionDeniedError creates a mock GCP permission denied error
func GCPPermissionDeniedError(message string) error {
	return status.Error(codes.PermissionDenied, message)
}

// GCPUnauthenticatedError creates a mock GCP unauthenticated error
func GCPUnauthenticatedError(message string) error {
	return status.Error(codes.Unauthenticated, message)
}

// GCPInvalidArgumentError creates a mock GCP invalid argument error
func GCPInvalidArgumentError(message string) error {
	return status.Error(codes.InvalidArgument, message)
}

// GCPResourceExhaustedError creates a mock GCP resource exhausted (throttled) error
func GCPResourceExhaustedError() error {
	return status.Errorf(codes.ResourceExhausted, "Quota exceeded")
}
