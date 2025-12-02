package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPolicyEnforcer(t *testing.T) {
	t.Parallel()

	t.Run("creates_enforcer_with_config", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			AllowedProviders: []string{"vault", "aws"},
		}
		enforcer := NewPolicyEnforcer(config)
		assert.NotNil(t, enforcer)
		assert.Equal(t, config, enforcer.config)
	})

	t.Run("creates_enforcer_with_nil_config", func(t *testing.T) {
		t.Parallel()
		enforcer := NewPolicyEnforcer(nil)
		assert.NotNil(t, enforcer)
		assert.NotNil(t, enforcer.config)
	})
}

func TestPolicyEnforcer_ValidateProviderType(t *testing.T) {
	t.Parallel()

	t.Run("allows_any_provider_when_no_restrictions", func(t *testing.T) {
		t.Parallel()
		enforcer := NewPolicyEnforcer(nil)
		err := enforcer.ValidateProviderType("any-provider")
		assert.NoError(t, err)
	})

	t.Run("blocks_provider_in_blocked_list", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			BlockedProviders: []string{"insecure-provider", "deprecated"},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateProviderType("insecure-provider")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "blocked by policy")
	})

	t.Run("blocks_provider_case_insensitive", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			BlockedProviders: []string{"BLOCKED"},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateProviderType("blocked")
		assert.Error(t, err)
	})

	t.Run("allows_provider_in_allowed_list", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			AllowedProviders: []string{"vault", "aws", "1password"},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateProviderType("vault")
		assert.NoError(t, err)
	})

	t.Run("rejects_provider_not_in_allowed_list", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			AllowedProviders: []string{"vault", "aws"},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateProviderType("bitwarden")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in allowed list")
	})

	t.Run("allowed_list_is_case_insensitive", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			AllowedProviders: []string{"VAULT"},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateProviderType("vault")
		assert.NoError(t, err)
	})

	t.Run("blocked_takes_precedence_over_allowed", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			AllowedProviders: []string{"vault"},
			BlockedProviders: []string{"vault"},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateProviderType("vault")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "blocked")
	})
}

func TestPolicyEnforcer_ValidateEnvironmentProvider(t *testing.T) {
	t.Parallel()

	t.Run("allows_when_no_environment_rules", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateEnvironmentProvider("production", "vault")
		assert.NoError(t, err)
	})

	t.Run("allows_when_environment_not_configured", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			EnvironmentRules: map[string]*EnvironmentPolicy{
				"staging": {AllowedProviders: []string{"vault"}},
			},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateEnvironmentProvider("production", "anything")
		assert.NoError(t, err)
	})

	t.Run("blocks_provider_for_specific_environment", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			EnvironmentRules: map[string]*EnvironmentPolicy{
				"production": {BlockedProviders: []string{"literal"}},
			},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateEnvironmentProvider("production", "literal")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "blocked for environment")
	})

	t.Run("allows_provider_in_environment_allowed_list", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			EnvironmentRules: map[string]*EnvironmentPolicy{
				"production": {AllowedProviders: []string{"vault", "aws"}},
			},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateEnvironmentProvider("production", "vault")
		assert.NoError(t, err)
	})

	t.Run("rejects_provider_not_in_environment_allowed_list", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			EnvironmentRules: map[string]*EnvironmentPolicy{
				"production": {AllowedProviders: []string{"vault"}},
			},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateEnvironmentProvider("production", "bitwarden")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed for environment")
	})
}

func TestPolicyEnforcer_ValidateSecretValue(t *testing.T) {
	t.Parallel()

	t.Run("allows_any_value_when_no_policy", func(t *testing.T) {
		t.Parallel()
		enforcer := NewPolicyEnforcer(nil)
		err := enforcer.ValidateSecretValue("simple")
		assert.NoError(t, err)
	})

	t.Run("validates_minimum_length", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			SecretComplexity: &ComplexityPolicy{MinLength: 8},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateSecretValue("short")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least 8 characters")

		err = enforcer.ValidateSecretValue("longenough")
		assert.NoError(t, err)
	})

	t.Run("validates_maximum_length", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			SecretComplexity: &ComplexityPolicy{MaxLength: 10},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateSecretValue("this-is-way-too-long")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not exceed 10 characters")

		err = enforcer.ValidateSecretValue("short")
		assert.NoError(t, err)
	})

	t.Run("validates_uppercase_requirement", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			SecretComplexity: &ComplexityPolicy{RequireUpper: true},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateSecretValue("nouppercase")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "uppercase")

		err = enforcer.ValidateSecretValue("HasUppercase")
		assert.NoError(t, err)
	})

	t.Run("validates_lowercase_requirement", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			SecretComplexity: &ComplexityPolicy{RequireLower: true},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateSecretValue("NOLOWERCASE")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "lowercase")

		err = enforcer.ValidateSecretValue("HASLOWERCASEa")
		assert.NoError(t, err)
	})

	t.Run("validates_digit_requirement", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			SecretComplexity: &ComplexityPolicy{RequireDigit: true},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateSecretValue("nodigits")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "digits")

		err = enforcer.ValidateSecretValue("hasdigit1")
		assert.NoError(t, err)
	})

	t.Run("validates_symbol_requirement", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			SecretComplexity: &ComplexityPolicy{RequireSymbol: true},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateSecretValue("nosymbols123")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "symbols")

		err = enforcer.ValidateSecretValue("has@symbol")
		assert.NoError(t, err)
	})

	t.Run("validates_all_complexity_requirements", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			SecretComplexity: &ComplexityPolicy{
				MinLength:     12,
				RequireUpper:  true,
				RequireLower:  true,
				RequireDigit:  true,
				RequireSymbol: true,
			},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateSecretValue("weak")
		assert.Error(t, err)

		err = enforcer.ValidateSecretValue("StrongP@ss123!")
		assert.NoError(t, err)
	})

	t.Run("rejects_forbidden_patterns", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			ForbiddenPatterns: []string{
				"password",
				"^admin",
			},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateSecretValue("mypassword123")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "forbidden pattern")

		err = enforcer.ValidateSecretValue("adminuser")
		assert.Error(t, err)

		err = enforcer.ValidateSecretValue("secureValue!")
		assert.NoError(t, err)
	})

	t.Run("requires_matching_required_patterns", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			RequiredPatterns: []string{
				"^[A-Z]", // Must start with uppercase
			},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateSecretValue("lowercase")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required pattern")

		err = enforcer.ValidateSecretValue("Uppercase")
		assert.NoError(t, err)
	})
}

func TestPolicyEnforcer_ValidateOutputPath(t *testing.T) {
	t.Parallel()

	t.Run("allows_any_path_when_no_restrictions", func(t *testing.T) {
		t.Parallel()
		enforcer := NewPolicyEnforcer(nil)
		err := enforcer.ValidateOutputPath("/any/path/.env")
		assert.NoError(t, err)
	})

	t.Run("blocks_matching_blocked_patterns", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			OutputRestrictions: &OutputPolicy{
				BlockedPaths: []string{
					"/etc/.*",
					".*\\.pem$",
				},
			},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateOutputPath("/etc/secrets")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "blocked pattern")

		err = enforcer.ValidateOutputPath("/tmp/key.pem")
		assert.Error(t, err)

		err = enforcer.ValidateOutputPath("/tmp/.env")
		assert.NoError(t, err)
	})

	t.Run("requires_matching_allowed_patterns", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			OutputRestrictions: &OutputPolicy{
				AllowedPaths: []string{
					"/tmp/.*",
					"^\\.env.*",
				},
			},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateOutputPath("/home/user/.env")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in allowed patterns")

		err = enforcer.ValidateOutputPath("/tmp/secrets.env")
		assert.NoError(t, err)

		err = enforcer.ValidateOutputPath(".env.local")
		assert.NoError(t, err)
	})
}

func TestPolicyEnforcer_ValidateEnvironmentSecretCount(t *testing.T) {
	t.Parallel()

	t.Run("allows_any_count_when_no_limits", func(t *testing.T) {
		t.Parallel()
		enforcer := NewPolicyEnforcer(nil)
		err := enforcer.ValidateEnvironmentSecretCount("production", 1000)
		assert.NoError(t, err)
	})

	t.Run("allows_any_count_when_environment_not_configured", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			EnvironmentRules: map[string]*EnvironmentPolicy{
				"staging": {MaxSecrets: 10},
			},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateEnvironmentSecretCount("production", 1000)
		assert.NoError(t, err)
	})

	t.Run("allows_count_within_limit", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			EnvironmentRules: map[string]*EnvironmentPolicy{
				"production": {MaxSecrets: 50},
			},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateEnvironmentSecretCount("production", 25)
		assert.NoError(t, err)

		err = enforcer.ValidateEnvironmentSecretCount("production", 50)
		assert.NoError(t, err)
	})

	t.Run("rejects_count_exceeding_limit", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			EnvironmentRules: map[string]*EnvironmentPolicy{
				"production": {MaxSecrets: 50},
			},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateEnvironmentSecretCount("production", 51)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exceeding limit of 50")
	})

	t.Run("allows_any_count_when_max_is_zero", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			EnvironmentRules: map[string]*EnvironmentPolicy{
				"production": {MaxSecrets: 0}, // 0 means no limit
			},
		}
		enforcer := NewPolicyEnforcer(config)

		err := enforcer.ValidateEnvironmentSecretCount("production", 1000)
		assert.NoError(t, err)
	})
}

func TestPolicyEnforcer_ShouldAudit(t *testing.T) {
	t.Parallel()

	t.Run("returns_false_when_no_audit_config", func(t *testing.T) {
		t.Parallel()
		enforcer := NewPolicyEnforcer(nil)
		assert.False(t, enforcer.ShouldAudit())
	})

	t.Run("returns_false_when_audit_disabled", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			AuditLogging: &AuditPolicy{Enabled: false},
		}
		enforcer := NewPolicyEnforcer(config)
		assert.False(t, enforcer.ShouldAudit())
	})

	t.Run("returns_true_when_audit_enabled", func(t *testing.T) {
		t.Parallel()
		config := &PolicyConfig{
			AuditLogging: &AuditPolicy{Enabled: true},
		}
		enforcer := NewPolicyEnforcer(config)
		assert.True(t, enforcer.ShouldAudit())
	})
}

func TestPolicyEnforcer_GetAuditConfig(t *testing.T) {
	t.Parallel()

	t.Run("returns_nil_when_no_config", func(t *testing.T) {
		t.Parallel()
		enforcer := NewPolicyEnforcer(nil)
		assert.Nil(t, enforcer.GetAuditConfig())
	})

	t.Run("returns_audit_config", func(t *testing.T) {
		t.Parallel()
		auditPolicy := &AuditPolicy{
			Enabled:  true,
			LogPath:  "/var/log/dsops-audit.log",
			LogLevel: "info",
		}
		config := &PolicyConfig{
			AuditLogging: auditPolicy,
		}
		enforcer := NewPolicyEnforcer(config)

		result := enforcer.GetAuditConfig()
		require.NotNil(t, result)
		assert.Equal(t, auditPolicy, result)
		assert.Equal(t, "/var/log/dsops-audit.log", result.LogPath)
		assert.True(t, result.Enabled)
	})
}

func TestComplexityPolicy_struct(t *testing.T) {
	t.Parallel()

	policy := ComplexityPolicy{
		MinLength:     8,
		MaxLength:     64,
		RequireUpper:  true,
		RequireLower:  true,
		RequireDigit:  true,
		RequireSymbol: true,
	}

	assert.Equal(t, 8, policy.MinLength)
	assert.Equal(t, 64, policy.MaxLength)
	assert.True(t, policy.RequireUpper)
	assert.True(t, policy.RequireLower)
	assert.True(t, policy.RequireDigit)
	assert.True(t, policy.RequireSymbol)
}

func TestEnvironmentPolicy_struct(t *testing.T) {
	t.Parallel()

	policy := EnvironmentPolicy{
		AllowedProviders: []string{"vault"},
		BlockedProviders: []string{"literal"},
		RequireApproval:  true,
		MaxSecrets:       100,
	}

	assert.Equal(t, []string{"vault"}, policy.AllowedProviders)
	assert.Equal(t, []string{"literal"}, policy.BlockedProviders)
	assert.True(t, policy.RequireApproval)
	assert.Equal(t, 100, policy.MaxSecrets)
}

func TestOutputPolicy_struct(t *testing.T) {
	t.Parallel()

	policy := OutputPolicy{
		AllowedPaths:     []string{"/tmp/.*"},
		BlockedPaths:     []string{"/etc/.*"},
		RequireGitignore: true,
		MaxTTL:           3600,
	}

	assert.Equal(t, []string{"/tmp/.*"}, policy.AllowedPaths)
	assert.Equal(t, []string{"/etc/.*"}, policy.BlockedPaths)
	assert.True(t, policy.RequireGitignore)
	assert.Equal(t, 3600, policy.MaxTTL)
}

func TestAuditPolicy_struct(t *testing.T) {
	t.Parallel()

	policy := AuditPolicy{
		Enabled:       true,
		LogPath:       "/var/log/audit.log",
		LogLevel:      "warn",
		IncludeValues: false,
	}

	assert.True(t, policy.Enabled)
	assert.Equal(t, "/var/log/audit.log", policy.LogPath)
	assert.Equal(t, "warn", policy.LogLevel)
	assert.False(t, policy.IncludeValues)
}
