package fakes

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// SecretsManagerAPI defines the interface for AWS Secrets Manager operations
// This matches the subset of methods used by AWSSecretsManagerProvider
type SecretsManagerAPI interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	DescribeSecret(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error)
	ListSecrets(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error)
	UpdateSecret(ctx context.Context, params *secretsmanager.UpdateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretOutput, error)
	UpdateSecretVersionStage(ctx context.Context, params *secretsmanager.UpdateSecretVersionStageInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretVersionStageOutput, error)
}

// SSMAPI defines the interface for AWS SSM Parameter Store operations
// This matches the subset of methods used by AWSSSMProvider
type SSMAPI interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	DescribeParameters(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error)
}

// FakeSecretsManagerClient is a mock implementation of SecretsManagerAPI
type FakeSecretsManagerClient struct {
	// Secrets maps secret names to their data
	Secrets map[string]*SecretData
	// Errors maps secret names to errors to return
	Errors map[string]error
	// GetSecretValueFunc allows custom behavior for GetSecretValue
	GetSecretValueFunc func(ctx context.Context, params *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error)
	// DescribeSecretFunc allows custom behavior for DescribeSecret
	DescribeSecretFunc func(ctx context.Context, params *secretsmanager.DescribeSecretInput) (*secretsmanager.DescribeSecretOutput, error)
	// ListSecretsFunc allows custom behavior for ListSecrets
	ListSecretsFunc func(ctx context.Context, params *secretsmanager.ListSecretsInput) (*secretsmanager.ListSecretsOutput, error)
	// UpdateSecretFunc allows custom behavior for UpdateSecret
	UpdateSecretFunc func(ctx context.Context, params *secretsmanager.UpdateSecretInput) (*secretsmanager.UpdateSecretOutput, error)
	// UpdateSecretVersionStageFunc allows custom behavior for UpdateSecretVersionStage
	UpdateSecretVersionStageFunc func(ctx context.Context, params *secretsmanager.UpdateSecretVersionStageInput) (*secretsmanager.UpdateSecretVersionStageOutput, error)
}

// SecretData holds the data for a mock secret
type SecretData struct {
	SecretString  *string
	SecretBinary  []byte
	VersionId     *string
	VersionStages []string
	CreatedDate   *time.Time
	Description   *string
	KmsKeyId      *string
	RotationEnabled *bool
	RotationLambdaARN *string
	RotationRules *types.RotationRulesType
	LastChangedDate *time.Time
	VersionIdsToStages map[string][]string
	ReplicationStatus []types.ReplicationStatusType
}

// NewFakeSecretsManagerClient creates a new mock Secrets Manager client
func NewFakeSecretsManagerClient() *FakeSecretsManagerClient {
	return &FakeSecretsManagerClient{
		Secrets: make(map[string]*SecretData),
		Errors:  make(map[string]error),
	}
}

// AddSecret adds a secret to the mock client
func (f *FakeSecretsManagerClient) AddSecret(name string, data *SecretData) {
	f.Secrets[name] = data
}

// AddSecretString adds a string secret to the mock client
func (f *FakeSecretsManagerClient) AddSecretString(name, value string) {
	now := time.Now()
	versionId := "v1-abc123"
	f.Secrets[name] = &SecretData{
		SecretString:  aws.String(value),
		VersionId:     aws.String(versionId),
		VersionStages: []string{"AWSCURRENT"},
		CreatedDate:   &now,
		LastChangedDate: &now,
		VersionIdsToStages: map[string][]string{
			versionId: {"AWSCURRENT"},
		},
	}
}

// AddSecretBinary adds a binary secret to the mock client
func (f *FakeSecretsManagerClient) AddSecretBinary(name string, value []byte) {
	now := time.Now()
	versionId := "v1-abc123"
	f.Secrets[name] = &SecretData{
		SecretBinary:  value,
		VersionId:     aws.String(versionId),
		VersionStages: []string{"AWSCURRENT"},
		CreatedDate:   &now,
		LastChangedDate: &now,
		VersionIdsToStages: map[string][]string{
			versionId: {"AWSCURRENT"},
		},
	}
}

// AddError configures the mock to return an error for a specific secret
func (f *FakeSecretsManagerClient) AddError(name string, err error) {
	f.Errors[name] = err
}

// GetSecretValue mocks the GetSecretValue operation
func (f *FakeSecretsManagerClient) GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	if f.GetSecretValueFunc != nil {
		return f.GetSecretValueFunc(ctx, params)
	}

	secretName := aws.ToString(params.SecretId)

	// Check for configured errors
	if err, exists := f.Errors[secretName]; exists {
		return nil, err
	}

	// Check if secret exists
	data, exists := f.Secrets[secretName]
	if !exists {
		return nil, &types.ResourceNotFoundException{
			Message: aws.String(fmt.Sprintf("Secrets Manager can't find the specified secret: %s", secretName)),
		}
	}

	return &secretsmanager.GetSecretValueOutput{
		ARN:           aws.String(fmt.Sprintf("arn:aws:secretsmanager:us-east-1:123456789012:secret:%s", secretName)),
		Name:          params.SecretId,
		SecretString:  data.SecretString,
		SecretBinary:  data.SecretBinary,
		VersionId:     data.VersionId,
		VersionStages: data.VersionStages,
		CreatedDate:   data.CreatedDate,
	}, nil
}

// DescribeSecret mocks the DescribeSecret operation
func (f *FakeSecretsManagerClient) DescribeSecret(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
	if f.DescribeSecretFunc != nil {
		return f.DescribeSecretFunc(ctx, params)
	}

	secretName := aws.ToString(params.SecretId)

	// Check for configured errors
	if err, exists := f.Errors[secretName]; exists {
		return nil, err
	}

	// Check if secret exists
	data, exists := f.Secrets[secretName]
	if !exists {
		return nil, &types.ResourceNotFoundException{
			Message: aws.String(fmt.Sprintf("Secrets Manager can't find the specified secret: %s", secretName)),
		}
	}

	output := &secretsmanager.DescribeSecretOutput{
		ARN:          aws.String(fmt.Sprintf("arn:aws:secretsmanager:us-east-1:123456789012:secret:%s", secretName)),
		Name:         params.SecretId,
		Description:  data.Description,
		KmsKeyId:     data.KmsKeyId,
		RotationEnabled: data.RotationEnabled,
		RotationLambdaARN: data.RotationLambdaARN,
		RotationRules: data.RotationRules,
		LastChangedDate: data.LastChangedDate,
		CreatedDate:  data.CreatedDate,
		VersionIdsToStages: data.VersionIdsToStages,
		ReplicationStatus: data.ReplicationStatus,
	}

	return output, nil
}

// ListSecrets mocks the ListSecrets operation
func (f *FakeSecretsManagerClient) ListSecrets(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
	if f.ListSecretsFunc != nil {
		return f.ListSecretsFunc(ctx, params)
	}

	// Return empty list for validation purposes
	return &secretsmanager.ListSecretsOutput{
		SecretList: []types.SecretListEntry{},
	}, nil
}

// UpdateSecret mocks the UpdateSecret operation
func (f *FakeSecretsManagerClient) UpdateSecret(ctx context.Context, params *secretsmanager.UpdateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretOutput, error) {
	if f.UpdateSecretFunc != nil {
		return f.UpdateSecretFunc(ctx, params)
	}

	secretName := aws.ToString(params.SecretId)

	// Check for configured errors
	if err, exists := f.Errors[secretName]; exists {
		return nil, err
	}

	// Check if secret exists
	data, exists := f.Secrets[secretName]
	if !exists {
		return nil, &types.ResourceNotFoundException{
			Message: aws.String(fmt.Sprintf("Secrets Manager can't find the specified secret: %s", secretName)),
		}
	}

	// Update secret value
	if params.SecretString != nil {
		data.SecretString = params.SecretString
	}
	if params.SecretBinary != nil {
		data.SecretBinary = params.SecretBinary
	}

	// Generate new version ID
	newVersionId := fmt.Sprintf("v%d-xyz789", len(data.VersionIdsToStages)+1)
	data.VersionId = aws.String(newVersionId)
	data.VersionIdsToStages[newVersionId] = []string{"AWSCURRENT"}

	// Update last changed date
	now := time.Now()
	data.LastChangedDate = &now

	return &secretsmanager.UpdateSecretOutput{
		ARN:       aws.String(fmt.Sprintf("arn:aws:secretsmanager:us-east-1:123456789012:secret:%s", secretName)),
		Name:      params.SecretId,
		VersionId: data.VersionId,
	}, nil
}

// UpdateSecretVersionStage mocks the UpdateSecretVersionStage operation
func (f *FakeSecretsManagerClient) UpdateSecretVersionStage(ctx context.Context, params *secretsmanager.UpdateSecretVersionStageInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretVersionStageOutput, error) {
	if f.UpdateSecretVersionStageFunc != nil {
		return f.UpdateSecretVersionStageFunc(ctx, params)
	}

	secretName := aws.ToString(params.SecretId)

	// Check for configured errors
	if err, exists := f.Errors[secretName]; exists {
		return nil, err
	}

	// Check if secret exists
	_, exists := f.Secrets[secretName]
	if !exists {
		return nil, &types.ResourceNotFoundException{
			Message: aws.String(fmt.Sprintf("Secrets Manager can't find the specified secret: %s", secretName)),
		}
	}

	return &secretsmanager.UpdateSecretVersionStageOutput{
		ARN:  aws.String(fmt.Sprintf("arn:aws:secretsmanager:us-east-1:123456789012:secret:%s", secretName)),
		Name: params.SecretId,
	}, nil
}

// FakeSSMClient is a mock implementation of SSMAPI
type FakeSSMClient struct {
	// Parameters maps parameter names to their data
	Parameters map[string]*ParameterData
	// Errors maps parameter names to errors to return
	Errors map[string]error
	// GetParameterFunc allows custom behavior for GetParameter
	GetParameterFunc func(ctx context.Context, params *ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
	// DescribeParametersFunc allows custom behavior for DescribeParameters
	DescribeParametersFunc func(ctx context.Context, params *ssm.DescribeParametersInput) (*ssm.DescribeParametersOutput, error)
}

// ParameterData holds the data for a mock SSM parameter
type ParameterData struct {
	Name             *string
	Type             ssmtypes.ParameterType
	Value            *string
	Version          int64
	LastModifiedDate *time.Time
	ARN              *string
	DataType         *string
	Tier             ssmtypes.ParameterTier
}

// NewFakeSSMClient creates a new mock SSM client
func NewFakeSSMClient() *FakeSSMClient {
	return &FakeSSMClient{
		Parameters: make(map[string]*ParameterData),
		Errors:     make(map[string]error),
	}
}

// AddParameter adds a parameter to the mock client
func (f *FakeSSMClient) AddParameter(name string, data *ParameterData) {
	f.Parameters[name] = data
}

// AddStringParameter adds a String parameter to the mock client
func (f *FakeSSMClient) AddStringParameter(name, value string) {
	now := time.Now()
	f.Parameters[name] = &ParameterData{
		Name:             aws.String(name),
		Type:             ssmtypes.ParameterTypeString,
		Value:            aws.String(value),
		Version:          1,
		LastModifiedDate: &now,
		ARN:              aws.String(fmt.Sprintf("arn:aws:ssm:us-east-1:123456789012:parameter%s", name)),
		Tier:             ssmtypes.ParameterTierStandard,
	}
}

// AddSecureStringParameter adds a SecureString parameter to the mock client
func (f *FakeSSMClient) AddSecureStringParameter(name, value string) {
	now := time.Now()
	f.Parameters[name] = &ParameterData{
		Name:             aws.String(name),
		Type:             ssmtypes.ParameterTypeSecureString,
		Value:            aws.String(value),
		Version:          1,
		LastModifiedDate: &now,
		ARN:              aws.String(fmt.Sprintf("arn:aws:ssm:us-east-1:123456789012:parameter%s", name)),
		Tier:             ssmtypes.ParameterTierStandard,
	}
}

// AddError configures the mock to return an error for a specific parameter
func (f *FakeSSMClient) AddError(name string, err error) {
	f.Errors[name] = err
}

// GetParameter mocks the GetParameter operation
func (f *FakeSSMClient) GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	if f.GetParameterFunc != nil {
		return f.GetParameterFunc(ctx, params)
	}

	paramName := aws.ToString(params.Name)

	// Check for configured errors
	if err, exists := f.Errors[paramName]; exists {
		return nil, err
	}

	// Check if parameter exists
	data, exists := f.Parameters[paramName]
	if !exists {
		return nil, &ssmtypes.ParameterNotFound{
			Message: aws.String(fmt.Sprintf("Parameter %s not found", paramName)),
		}
	}

	return &ssm.GetParameterOutput{
		Parameter: &ssmtypes.Parameter{
			Name:             data.Name,
			Type:             data.Type,
			Value:            data.Value,
			Version:          data.Version,
			LastModifiedDate: data.LastModifiedDate,
			ARN:              data.ARN,
			DataType:         data.DataType,
		},
	}, nil
}

// DescribeParameters mocks the DescribeParameters operation
func (f *FakeSSMClient) DescribeParameters(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
	if f.DescribeParametersFunc != nil {
		return f.DescribeParametersFunc(ctx, params)
	}

	// If no filters, return all parameters (for validation)
	if len(params.ParameterFilters) == 0 {
		var paramList []ssmtypes.ParameterMetadata
		for _, data := range f.Parameters {
			paramList = append(paramList, ssmtypes.ParameterMetadata{
				Name:             data.Name,
				Type:             data.Type,
				Version:          data.Version,
				LastModifiedDate: data.LastModifiedDate,
				Tier:             data.Tier,
			})
		}
		return &ssm.DescribeParametersOutput{
			Parameters: paramList,
		}, nil
	}

	// Filter by parameter name
	for _, filter := range params.ParameterFilters {
		if aws.ToString(filter.Key) == "Name" && len(filter.Values) > 0 {
			paramName := filter.Values[0]
			data, exists := f.Parameters[paramName]
			if !exists {
				return &ssm.DescribeParametersOutput{
					Parameters: []ssmtypes.ParameterMetadata{},
				}, nil
			}

			return &ssm.DescribeParametersOutput{
				Parameters: []ssmtypes.ParameterMetadata{
					{
						Name:             data.Name,
						Type:             data.Type,
						Version:          data.Version,
						LastModifiedDate: data.LastModifiedDate,
						Tier:             data.Tier,
					},
				},
			}, nil
		}
	}

	return &ssm.DescribeParametersOutput{
		Parameters: []ssmtypes.ParameterMetadata{},
	}, nil
}
