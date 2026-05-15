package commands

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/internal/resolve"
	"github.com/systmms/dsops/pkg/adapter"
	"github.com/systmms/dsops/pkg/provider"
)

// collectBitwardenAccountInfo gathers AccountInfo snapshots from every
// registered Bitwarden provider declared in cfg. Names that appear in both
// SecretStores and Providers maps are deduplicated. Providers wrapped in
// the SecretStores adapter chain are unwrapped so they show up alongside
// directly-registered ones.
func collectBitwardenAccountInfo(resolver *resolve.Resolver, cfg *config.Config) []providers.BitwardenAccountInfo {
	seen := make(map[string]struct{})
	names := make([]string, 0)
	for name, sc := range cfg.Definition.SecretStores {
		if sc.Type != "bitwarden" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	for name, pc := range cfg.Definition.Providers {
		if pc.Type != "bitwarden" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	out := make([]providers.BitwardenAccountInfo, 0, len(names))
	for _, name := range names {
		p, ok := resolver.GetProvider(name)
		if !ok {
			continue
		}
		if bw, ok := unwrapBitwardenProvider(p); ok {
			out = append(out, bw.AccountInfo())
		}
	}
	return out
}

// unwrapBitwardenProvider walks the adapter chain that the secret-store
// registry applies (SecretStore -> ProviderToSecretStoreAdapter -> legacy
// provider) so a Bitwarden provider configured under either `providers:`
// or `secretStores:` can be discovered by `dsops doctor`.
func unwrapBitwardenProvider(p provider.Provider) (*providers.BitwardenProvider, bool) {
	for {
		if bw, ok := p.(*providers.BitwardenProvider); ok {
			return bw, true
		}
		ssa, ok := p.(*adapter.SecretStoreToProviderAdapter)
		if !ok {
			return nil, false
		}
		ps, ok := ssa.SecretStore().(*adapter.ProviderToSecretStoreAdapter)
		if !ok {
			return nil, false
		}
		p = ps.Provider()
	}
}

// renderBitwardenAccounts writes a per-Bitwarden-provider block to out.
// Surfaces appDataDir / server / email / observed status per instance and
// adds a "⚠ shared with: <other>" warning when two providers point at the
// same appDataDir (SPEC-026 FR-007).
func renderBitwardenAccounts(out io.Writer, infos []providers.BitwardenAccountInfo) {
	if len(infos) == 0 {
		return
	}

	dirOwners := map[string][]string{}
	for _, info := range infos {
		if info.AppDataDir != "" {
			dirOwners[info.AppDataDir] = append(dirOwners[info.AppDataDir], info.Name)
		}
	}

	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "Bitwarden accounts:")

	for _, info := range infos {
		_, _ = fmt.Fprintf(out, "  provider %s:\n", info.Name)
		_, _ = fmt.Fprintf(out, "    appDataDir: %s\n", valueOrUnset(info.AppDataDir))
		_, _ = fmt.Fprintf(out, "    server:     %s\n", valueOrUnset(info.Server))
		// Prefer the live observed identity; fall back to the configured
		// expectation; finally to <unset>. This is the field operators
		// actually want to see in doctor output.
		emailValue := info.ObservedEmail
		if emailValue == "" {
			emailValue = info.Email
		}
		emailLine := valueOrUnset(emailValue)
		if info.ObservedStatus != "" {
			emailLine = fmt.Sprintf("%s   (status: %s)", emailLine, info.ObservedStatus)
		}
		_, _ = fmt.Fprintf(out, "    email:      %s\n", emailLine)

		if info.AppDataDir == "" {
			continue
		}
		others := make([]string, 0, len(dirOwners[info.AppDataDir]))
		for _, name := range dirOwners[info.AppDataDir] {
			if name != info.Name {
				others = append(others, name)
			}
		}
		if len(others) > 0 {
			sort.Strings(others)
			_, _ = fmt.Fprintf(out, "    ⚠ shared with: %s\n", strings.Join(others, ", "))
		}
	}
}

func valueOrUnset(s string) string {
	if s == "" {
		return "<unset>"
	}
	return s
}
