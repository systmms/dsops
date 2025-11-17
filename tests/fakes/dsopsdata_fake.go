// Package fakes provides test doubles for dsops testing.
package fakes

import (
	"github.com/systmms/dsops/internal/dsopsdata"
)

// FakeDsopsDataRepository creates a mock dsops-data repository for testing.
// This provides pre-configured service definitions without needing real data files.
type FakeDsopsDataRepository struct {
	*dsopsdata.Repository
}

// NewFakeDsopsDataRepository creates a new fake repository with empty maps.
func NewFakeDsopsDataRepository() *FakeDsopsDataRepository {
	return &FakeDsopsDataRepository{
		Repository: &dsopsdata.Repository{
			ServiceTypes:     make(map[string]*dsopsdata.ServiceType),
			ServiceInstances: make(map[string]*dsopsdata.ServiceInstance),
			RotationPolicies: make(map[string]*dsopsdata.RotationPolicy),
			Principals:       make(map[string]*dsopsdata.Principal),
		},
	}
}

// AddServiceType adds a service type to the repository.
func (f *FakeDsopsDataRepository) AddServiceType(st *dsopsdata.ServiceType) {
	f.ServiceTypes[st.Metadata.Name] = st
}

// AddServiceInstance adds a service instance to the repository.
func (f *FakeDsopsDataRepository) AddServiceInstance(si *dsopsdata.ServiceInstance) {
	key := si.Metadata.Type + "/" + si.Metadata.ID
	f.ServiceInstances[key] = si
}

// AddRotationPolicy adds a rotation policy to the repository.
func (f *FakeDsopsDataRepository) AddRotationPolicy(rp *dsopsdata.RotationPolicy) {
	f.RotationPolicies[rp.Metadata.Name] = rp
}

// AddPrincipal adds a principal to the repository.
func (f *FakeDsopsDataRepository) AddPrincipal(p *dsopsdata.Principal) {
	f.Principals[p.Metadata.Name] = p
}

// Clear resets the repository to empty state.
func (f *FakeDsopsDataRepository) Clear() {
	f.ServiceTypes = make(map[string]*dsopsdata.ServiceType)
	f.ServiceInstances = make(map[string]*dsopsdata.ServiceInstance)
	f.RotationPolicies = make(map[string]*dsopsdata.RotationPolicy)
	f.Principals = make(map[string]*dsopsdata.Principal)
}

// WithPostgreSQLServiceType adds a pre-configured PostgreSQL service type.
func (f *FakeDsopsDataRepository) WithPostgreSQLServiceType() *FakeDsopsDataRepository {
	f.AddServiceType(&dsopsdata.ServiceType{
		APIVersion: "dsops.io/v1alpha1",
		Kind:       "ServiceType",
		Metadata: struct {
			Name        string `yaml:"name" json:"name"`
			Description string `yaml:"description,omitempty" json:"description,omitempty"`
			Category    string `yaml:"category,omitempty" json:"category,omitempty"`
		}{
			Name:        "postgresql",
			Description: "PostgreSQL database service",
			Category:    "database",
		},
		Spec: struct {
			CredentialKinds []dsopsdata.CredentialKind `yaml:"credentialKinds" json:"credentialKinds"`
			Defaults        struct {
				RateLimit        string `yaml:"rateLimit,omitempty" json:"rateLimit,omitempty"`
				RotationStrategy string `yaml:"rotationStrategy,omitempty" json:"rotationStrategy,omitempty"`
			} `yaml:"defaults,omitempty" json:"defaults,omitempty"`
		}{
			CredentialKinds: []dsopsdata.CredentialKind{
				{
					Name:         "password",
					Description:  "Database user password",
					Capabilities: []string{"rotate", "verify", "revoke"},
					Constraints: struct {
						MaxActive interface{} `yaml:"maxActive,omitempty" json:"maxActive,omitempty"`
						TTL       string      `yaml:"ttl,omitempty" json:"ttl,omitempty"`
						Format    string      `yaml:"format,omitempty" json:"format,omitempty"`
					}{
						MaxActive: 2,
						TTL:       "90d",
						Format:    "alphanumeric",
					},
				},
				{
					Name:         "connection_string",
					Description:  "Full PostgreSQL connection string",
					Capabilities: []string{"read", "rotate"},
					Constraints: struct {
						MaxActive interface{} `yaml:"maxActive,omitempty" json:"maxActive,omitempty"`
						TTL       string      `yaml:"ttl,omitempty" json:"ttl,omitempty"`
						Format    string      `yaml:"format,omitempty" json:"format,omitempty"`
					}{
						Format: "postgresql://user:password@host:port/db",
					},
				},
			},
			Defaults: struct {
				RateLimit        string `yaml:"rateLimit,omitempty" json:"rateLimit,omitempty"`
				RotationStrategy string `yaml:"rotationStrategy,omitempty" json:"rotationStrategy,omitempty"`
			}{
				RotationStrategy: "two-secret",
			},
		},
	})
	return f
}

// WithStripeServiceType adds a pre-configured Stripe API service type.
func (f *FakeDsopsDataRepository) WithStripeServiceType() *FakeDsopsDataRepository {
	f.AddServiceType(&dsopsdata.ServiceType{
		APIVersion: "dsops.io/v1alpha1",
		Kind:       "ServiceType",
		Metadata: struct {
			Name        string `yaml:"name" json:"name"`
			Description string `yaml:"description,omitempty" json:"description,omitempty"`
			Category    string `yaml:"category,omitempty" json:"category,omitempty"`
		}{
			Name:        "stripe",
			Description: "Stripe payment processing API",
			Category:    "api",
		},
		Spec: struct {
			CredentialKinds []dsopsdata.CredentialKind `yaml:"credentialKinds" json:"credentialKinds"`
			Defaults        struct {
				RateLimit        string `yaml:"rateLimit,omitempty" json:"rateLimit,omitempty"`
				RotationStrategy string `yaml:"rotationStrategy,omitempty" json:"rotationStrategy,omitempty"`
			} `yaml:"defaults,omitempty" json:"defaults,omitempty"`
		}{
			CredentialKinds: []dsopsdata.CredentialKind{
				{
					Name:         "api_key",
					Description:  "Stripe API key (secret key)",
					Capabilities: []string{"rotate", "verify", "revoke"},
					Constraints: struct {
						MaxActive interface{} `yaml:"maxActive,omitempty" json:"maxActive,omitempty"`
						TTL       string      `yaml:"ttl,omitempty" json:"ttl,omitempty"`
						Format    string      `yaml:"format,omitempty" json:"format,omitempty"`
					}{
						MaxActive: "unlimited",
						Format:    "sk_live_*",
					},
				},
				{
					Name:         "webhook_secret",
					Description:  "Stripe webhook signing secret",
					Capabilities: []string{"rotate"},
					Constraints: struct {
						MaxActive interface{} `yaml:"maxActive,omitempty" json:"maxActive,omitempty"`
						TTL       string      `yaml:"ttl,omitempty" json:"ttl,omitempty"`
						Format    string      `yaml:"format,omitempty" json:"format,omitempty"`
					}{
						MaxActive: 1,
						Format:    "whsec_*",
					},
				},
			},
			Defaults: struct {
				RateLimit        string `yaml:"rateLimit,omitempty" json:"rateLimit,omitempty"`
				RotationStrategy string `yaml:"rotationStrategy,omitempty" json:"rotationStrategy,omitempty"`
			}{
				RateLimit:        "100/s",
				RotationStrategy: "overlap",
			},
		},
	})
	return f
}

// WithGitHubServiceType adds a pre-configured GitHub service type.
func (f *FakeDsopsDataRepository) WithGitHubServiceType() *FakeDsopsDataRepository {
	f.AddServiceType(&dsopsdata.ServiceType{
		APIVersion: "dsops.io/v1alpha1",
		Kind:       "ServiceType",
		Metadata: struct {
			Name        string `yaml:"name" json:"name"`
			Description string `yaml:"description,omitempty" json:"description,omitempty"`
			Category    string `yaml:"category,omitempty" json:"category,omitempty"`
		}{
			Name:        "github",
			Description: "GitHub API and token management",
			Category:    "vcs",
		},
		Spec: struct {
			CredentialKinds []dsopsdata.CredentialKind `yaml:"credentialKinds" json:"credentialKinds"`
			Defaults        struct {
				RateLimit        string `yaml:"rateLimit,omitempty" json:"rateLimit,omitempty"`
				RotationStrategy string `yaml:"rotationStrategy,omitempty" json:"rotationStrategy,omitempty"`
			} `yaml:"defaults,omitempty" json:"defaults,omitempty"`
		}{
			CredentialKinds: []dsopsdata.CredentialKind{
				{
					Name:         "personal_access_token",
					Description:  "GitHub personal access token",
					Capabilities: []string{"rotate", "verify", "revoke"},
					Constraints: struct {
						MaxActive interface{} `yaml:"maxActive,omitempty" json:"maxActive,omitempty"`
						TTL       string      `yaml:"ttl,omitempty" json:"ttl,omitempty"`
						Format    string      `yaml:"format,omitempty" json:"format,omitempty"`
					}{
						TTL:    "365d",
						Format: "ghp_*",
					},
				},
				{
					Name:         "app_token",
					Description:  "GitHub App installation token",
					Capabilities: []string{"rotate", "verify"},
					Constraints: struct {
						MaxActive interface{} `yaml:"maxActive,omitempty" json:"maxActive,omitempty"`
						TTL       string      `yaml:"ttl,omitempty" json:"ttl,omitempty"`
						Format    string      `yaml:"format,omitempty" json:"format,omitempty"`
					}{
						TTL:    "1h",
						Format: "ghs_*",
					},
				},
			},
			Defaults: struct {
				RateLimit        string `yaml:"rateLimit,omitempty" json:"rateLimit,omitempty"`
				RotationStrategy string `yaml:"rotationStrategy,omitempty" json:"rotationStrategy,omitempty"`
			}{
				RateLimit:        "5000/h",
				RotationStrategy: "immediate",
			},
		},
	})
	return f
}

// WithStandardRotationPolicy adds a standard rotation policy.
func (f *FakeDsopsDataRepository) WithStandardRotationPolicy() *FakeDsopsDataRepository {
	f.AddRotationPolicy(&dsopsdata.RotationPolicy{
		APIVersion: "dsops.io/v1alpha1",
		Kind:       "RotationPolicy",
		Metadata: struct {
			Name        string `yaml:"name" json:"name"`
			Description string `yaml:"description,omitempty" json:"description,omitempty"`
		}{
			Name:        "standard-90d",
			Description: "Standard 90-day rotation policy",
		},
		Spec: struct {
			Strategy      string                 `yaml:"strategy" json:"strategy"`
			Schedule      string                 `yaml:"schedule,omitempty" json:"schedule,omitempty"`
			Verification  *dsopsdata.Verification  `yaml:"verification,omitempty" json:"verification,omitempty"`
			Cutover       *dsopsdata.Cutover       `yaml:"cutover,omitempty" json:"cutover,omitempty"`
			Notifications *dsopsdata.Notifications `yaml:"notifications,omitempty" json:"notifications,omitempty"`
			Constraints   *dsopsdata.Constraints   `yaml:"constraints,omitempty" json:"constraints,omitempty"`
		}{
			Strategy: "two-secret",
			Schedule: "0 0 1 */3 *", // Every 3 months
			Verification: &dsopsdata.Verification{
				Method:  "connection",
				Timeout: "30s",
				Retries: 3,
			},
			Cutover: &dsopsdata.Cutover{
				RequireCheck:   true,
				GracePeriod:    "1h",
				RollbackWindow: "24h",
			},
			Notifications: &dsopsdata.Notifications{
				OnSuccess: []string{"slack:ops-channel"},
				OnFailure: []string{"pagerduty:critical", "slack:ops-channel"},
				BeforeExpiry: &dsopsdata.BeforeExpiry{
					Targets: []string{"slack:ops-channel"},
					Advance: "7d",
				},
			},
		},
	})
	return f
}

// WithApplicationPrincipal adds a sample application principal.
func (f *FakeDsopsDataRepository) WithApplicationPrincipal(name string) *FakeDsopsDataRepository {
	f.AddPrincipal(&dsopsdata.Principal{
		APIVersion: "dsops.io/v1alpha1",
		Kind:       "Principal",
		Metadata: struct {
			Name        string            `yaml:"name" json:"name"`
			Description string            `yaml:"description,omitempty" json:"description,omitempty"`
			Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
		}{
			Name:        name,
			Description: "Application service principal",
			Labels: map[string]string{
				"type":        "application",
				"environment": "production",
			},
		},
		Spec: struct {
			Type        string                         `yaml:"type" json:"type"`
			Email       string                         `yaml:"email,omitempty" json:"email,omitempty"`
			Team        string                         `yaml:"team,omitempty" json:"team,omitempty"`
			Environment string                         `yaml:"environment,omitempty" json:"environment,omitempty"`
			Permissions *dsopsdata.PrincipalPermissions `yaml:"permissions,omitempty" json:"permissions,omitempty"`
			Contact     *dsopsdata.PrincipalContact    `yaml:"contact,omitempty" json:"contact,omitempty"`
			Metadata    map[string]interface{}         `yaml:"metadata,omitempty" json:"metadata,omitempty"`
		}{
			Type:        "service-account",
			Team:        "platform",
			Environment: "production",
			Permissions: &dsopsdata.PrincipalPermissions{
				AllowedServices:        []string{"postgresql", "stripe", "github"},
				AllowedCredentialKinds: []string{"password", "api_key"},
				MaxCredentialTTL:       "365d",
			},
		},
	})
	return f
}

// WithServiceInstance adds a sample service instance.
func (f *FakeDsopsDataRepository) WithServiceInstance(serviceType, id, endpoint string) *FakeDsopsDataRepository {
	f.AddServiceInstance(&dsopsdata.ServiceInstance{
		APIVersion: "dsops.io/v1alpha1",
		Kind:       "ServiceInstance",
		Metadata: struct {
			Type        string   `yaml:"type" json:"type"`
			ID          string   `yaml:"id" json:"id"`
			Name        string   `yaml:"name,omitempty" json:"name,omitempty"`
			Description string   `yaml:"description,omitempty" json:"description,omitempty"`
			Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
		}{
			Type:        serviceType,
			ID:          id,
			Name:        id + " instance",
			Description: "Test service instance",
			Tags:        []string{"test", "production"},
		},
		Spec: struct {
			Endpoint        string                        `yaml:"endpoint" json:"endpoint"`
			Auth            string                        `yaml:"auth" json:"auth"`
			CredentialKinds []dsopsdata.InstanceCredential `yaml:"credentialKinds" json:"credentialKinds"`
			Config          map[string]interface{}        `yaml:"config,omitempty" json:"config,omitempty"`
		}{
			Endpoint: endpoint,
			Auth:     "password",
			CredentialKinds: []dsopsdata.InstanceCredential{
				{
					Name:       "password",
					Policy:     "standard-90d",
					Principals: []string{"app-service"},
				},
			},
			Config: map[string]interface{}{
				"ssl": true,
			},
		},
	})
	return f
}

// PrePopulated creates a repository with standard service types and policies.
func PrePopulatedFakeDsopsDataRepository() *FakeDsopsDataRepository {
	return NewFakeDsopsDataRepository().
		WithPostgreSQLServiceType().
		WithStripeServiceType().
		WithGitHubServiceType().
		WithStandardRotationPolicy().
		WithApplicationPrincipal("app-service").
		WithServiceInstance("postgresql", "prod-db", "localhost:5432")
}
