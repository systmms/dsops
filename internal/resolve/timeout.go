package resolve

import (
	"context"
	"fmt"
	"time"

	dserrors "github.com/systmms/dsops/internal/errors"
)

// withProviderTimeout creates a context with timeout for provider operations
func withProviderTimeout(ctx context.Context, timeoutMs int) (context.Context, context.CancelFunc) {
	timeout := time.Duration(timeoutMs) * time.Millisecond
	return context.WithTimeout(ctx, timeout)
}

// isTimeoutError checks if an error is a timeout error and wraps it with helpful context
func isTimeoutError(err error, providerName string, timeoutMs int) error {
	if err == context.DeadlineExceeded {
		return dserrors.UserError{
			Message:    "Provider operation timed out",
			Details:    fmt.Sprintf("Operation exceeded %dms timeout", timeoutMs),
			Suggestion: getTimeoutSuggestion(providerName, timeoutMs),
		}
	}
	return err
}

// getTimeoutSuggestion provides helpful suggestions for timeout errors
func getTimeoutSuggestion(providerName string, timeoutMs int) string {
	timeoutSec := timeoutMs / 1000
	
	switch providerName {
	case "bitwarden":
		if timeoutSec < 10 {
			return "Bitwarden CLI can be slow. Try increasing timeout_ms to 15000 or check 'bw status'"
		}
		return "Check Bitwarden server connectivity. Use 'bw unlock' if vault is locked"
		
	case "1password", "onepassword":
		if timeoutSec < 10 {
			return "1Password CLI can be slow. Try increasing timeout_ms to 15000 or check 'op signin'"
		}
		return "Check 1Password connectivity. Use 'op signin' if session expired"
		
	case "aws", "aws-secretsmanager":
		if timeoutSec < 5 {
			return "AWS API can be slow. Try increasing timeout_ms to 10000"
		}
		return "Check AWS connectivity and credentials. Verify region is correct"
		
	case "gcp", "google-cloud-secret-manager":
		if timeoutSec < 5 {
			return "Google Cloud API can be slow. Try increasing timeout_ms to 10000"
		}
		return "Check Google Cloud connectivity and authentication"
		
	case "azure", "azure-key-vault":
		if timeoutSec < 5 {
			return "Azure API can be slow. Try increasing timeout_ms to 10000"
		}
		return "Check Azure connectivity and authentication"
		
	case "vault", "hashicorp-vault":
		if timeoutSec < 5 {
			return "Vault API can be slow. Try increasing timeout_ms to 10000"
		}
		return "Check Vault connectivity and authentication. Verify VAULT_ADDR"
	}
	
	// Generic suggestions
	if timeoutSec < 10 {
		return "Provider operation timed out. Try increasing timeout_ms in your provider configuration"
	}
	return "Check network connectivity and provider authentication. Consider increasing timeout_ms if provider is consistently slow"
}