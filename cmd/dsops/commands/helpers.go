package commands

import (
	"sort"
	
	"github.com/systmms/dsops/internal/config"
)

// getEnvNames returns a sorted list of environment names
func getEnvNames(envs map[string]config.Environment) []string {
	names := make([]string, 0, len(envs))
	for name := range envs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}