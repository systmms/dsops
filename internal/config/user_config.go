package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	dserrors "github.com/systmms/dsops/internal/errors"
	"gopkg.in/yaml.v3"
)

// Environment variables and sentinels governing configuration discovery.
const (
	// ProjectConfigEnvVar overrides the default project config path (dsops.yaml).
	ProjectConfigEnvVar = "DSOPS_CONFIG"
	// DefaultProjectConfigFile is the project config file name used when
	// neither --config nor DSOPS_CONFIG is set.
	DefaultProjectConfigFile = "dsops.yaml"
	// UserConfigEnvVar points dsops at a machine-level (user) config file.
	UserConfigEnvVar = "DSOPS_USER_CONFIG"
	// UserConfigDisabled is the sentinel value (for --user-config or
	// DSOPS_USER_CONFIG) that turns user config loading off entirely.
	UserConfigDisabled = "none"
)

// UserConfigOrigin records how the user config path was chosen. It decides
// whether a missing file is an error (explicit origins) or silently ignored
// (the default location).
type UserConfigOrigin string

const (
	// UserConfigOriginNone means no user config is consulted. It is the zero
	// value, so a Config constructed in tests stays hermetic by default.
	UserConfigOriginNone UserConfigOrigin = ""
	// UserConfigOriginFlag means the path came from --user-config.
	UserConfigOriginFlag UserConfigOrigin = "flag"
	// UserConfigOriginEnv means the path came from DSOPS_USER_CONFIG.
	UserConfigOriginEnv UserConfigOrigin = "env"
	// UserConfigOriginDefault means the XDG / home / %APPDATA% default location.
	UserConfigOriginDefault UserConfigOrigin = "default"
)

// UserConfigSpec is the input to Load(): where to look for the machine-level
// config and whether its absence is an error.
type UserConfigSpec struct {
	Path   string
	Origin UserConfigOrigin
}

// Explicit reports whether the user asked for this specific file (flag or
// env), in which case a missing file is a configuration error.
func (s UserConfigSpec) Explicit() bool {
	return s.Origin == UserConfigOriginFlag || s.Origin == UserConfigOriginEnv
}

// StoreScope says which configuration layer declared a store.
type StoreScope string

const (
	// StoreScopeProject means the store was declared in the project dsops.yaml.
	StoreScopeProject StoreScope = "project"
	// StoreScopeUser means the store was declared in the machine-level user config.
	StoreScopeUser StoreScope = "user"
)

// StoreSource records where a named store, service, or legacy provider was
// declared so commands can show provenance.
type StoreSource struct {
	Scope StoreScope
	Path  string // absolute path of the declaring file
}

// UserConfigLookup abstracts the process environment so discovery can be
// tested table-driven and in parallel (t.Setenv is incompatible with
// t.Parallel).
type UserConfigLookup struct {
	Getenv        func(string) string
	UserHomeDir   func() (string, error)
	UserConfigDir func() (string, error) // only consulted on windows
	GOOS          string
}

// DefaultUserConfigLookup returns a lookup backed by the real process
// environment and runtime.
func DefaultUserConfigLookup() UserConfigLookup {
	return UserConfigLookup{
		Getenv:        os.Getenv,
		UserHomeDir:   os.UserHomeDir,
		UserConfigDir: os.UserConfigDir,
		GOOS:          runtime.GOOS,
	}
}

func (lk UserConfigLookup) withDefaults() UserConfigLookup {
	if lk.Getenv == nil {
		lk.Getenv = os.Getenv
	}
	if lk.UserHomeDir == nil {
		lk.UserHomeDir = os.UserHomeDir
	}
	if lk.UserConfigDir == nil {
		lk.UserConfigDir = os.UserConfigDir
	}
	if lk.GOOS == "" {
		lk.GOOS = runtime.GOOS
	}
	return lk
}

// DefaultProjectConfigPath returns the default value for --config: the
// DSOPS_CONFIG environment variable when set, otherwise dsops.yaml.
func DefaultProjectConfigPath(getenv func(string) string) string {
	if getenv == nil {
		getenv = os.Getenv
	}
	if v := getenv(ProjectConfigEnvVar); v != "" {
		return v
	}
	return DefaultProjectConfigFile
}

// ResolveUserConfigPath applies the discovery order for the machine-level
// config: --user-config flag > DSOPS_USER_CONFIG > $XDG_CONFIG_HOME/dsops/config.yaml
// > ~/.config/dsops/config.yaml (every non-Windows OS, including macOS, so
// that home-manager's xdg.configFile output is found) > %APPDATA%\dsops\config.yaml
// on Windows. The value "none" for the flag or the env var disables user
// config entirely. It never returns an error: when no location can be
// determined the result has UserConfigOriginNone.
func ResolveUserConfigPath(flagValue string, flagSet bool, lk UserConfigLookup) UserConfigSpec {
	lk = lk.withDefaults()

	if flagSet && flagValue != "" {
		return explicitUserConfig(flagValue, UserConfigOriginFlag, lk)
	}
	if v := lk.Getenv(UserConfigEnvVar); v != "" {
		return explicitUserConfig(v, UserConfigOriginEnv, lk)
	}

	// XDG applies on every OS when set to an absolute path (the spec says a
	// relative XDG_CONFIG_HOME must be ignored).
	if xdg := lk.Getenv("XDG_CONFIG_HOME"); xdg != "" && filepath.IsAbs(xdg) {
		return UserConfigSpec{Path: filepath.Join(xdg, "dsops", "config.yaml"), Origin: UserConfigOriginDefault}
	}

	if lk.GOOS == "windows" {
		dir, err := lk.UserConfigDir()
		if err != nil || dir == "" {
			return UserConfigSpec{}
		}
		return UserConfigSpec{Path: filepath.Join(dir, "dsops", "config.yaml"), Origin: UserConfigOriginDefault}
	}

	home, err := lk.UserHomeDir()
	if err != nil || home == "" {
		return UserConfigSpec{}
	}
	return UserConfigSpec{Path: filepath.Join(home, ".config", "dsops", "config.yaml"), Origin: UserConfigOriginDefault}
}

func explicitUserConfig(raw string, origin UserConfigOrigin, lk UserConfigLookup) UserConfigSpec {
	value := strings.TrimSpace(raw)
	if strings.EqualFold(value, UserConfigDisabled) {
		return UserConfigSpec{}
	}
	if expanded, err := ExpandHome(value, lk.UserHomeDir); err == nil {
		value = expanded
	}
	if abs, err := filepath.Abs(value); err == nil {
		value = abs
	}
	return UserConfigSpec{Path: value, Origin: origin}
}

// ExpandHome expands a leading "~" or "~/" using homeDir. "~user" forms are
// rejected. Any other path is returned unchanged.
func ExpandHome(path string, homeDir func() (string, error)) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		if strings.HasPrefix(path, "~") {
			return "", fmt.Errorf("~user expansion is not supported in %q", path)
		}
		return path, nil
	}
	if homeDir == nil {
		homeDir = os.UserHomeDir
	}
	home, err := homeDir()
	if err != nil {
		return "", fmt.Errorf("cannot expand %q: %w", path, err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}

// userConfigFile is the restricted schema accepted in the machine-level
// config: only version, secretStores and (legacy) providers.
type userConfigFile struct {
	Version      int                          `yaml:"version"`
	SecretStores map[string]SecretStoreConfig `yaml:"secretStores,omitempty"`
	Providers    map[string]ProviderConfig    `yaml:"providers,omitempty"`
}

var allowedUserConfigKeys = map[string]bool{
	"version":      true,
	"secretStores": true,
	"providers":    true,
}

// StoreSource returns where the named store/service/provider was declared.
func (c *Config) StoreSource(name string) (StoreSource, bool) {
	src, ok := c.StoreSources[name]
	return src, ok
}

// MissingProviderSuggestion builds the remediation hint for an unknown store
// name, naming the project file and, when user config is enabled, the user
// file (even if it does not exist yet, so the user knows where to create it).
func (c *Config) MissingProviderSuggestion(name string) string {
	s := fmt.Sprintf("Add provider '%s' to the 'secretStores:' section of %s", name, c.projectPathForDisplay())
	if c.UserConfig.Path != "" {
		s += fmt.Sprintf(", or to your user config at %s", c.UserConfig.Path)
	}
	return s
}

func (c *Config) projectPathForDisplay() string {
	if c.Path == "" {
		return DefaultProjectConfigFile
	}
	if abs, err := filepath.Abs(c.Path); err == nil {
		return abs
	}
	return c.Path
}

func (c *Config) debugf(format string, args ...interface{}) {
	if c.Logger != nil {
		c.Logger.Debug(format, args...)
	}
}

func (c *Config) warnf(format string, args ...interface{}) {
	if c.Logger != nil {
		c.Logger.Warn(format, args...)
	}
}

// resetLoadState clears everything Load() derives so repeated calls never
// accumulate stale provenance.
func (c *Config) resetLoadState() {
	c.LoadedUserConfigPath = ""
	c.StoreSources = make(map[string]StoreSource)
	c.ShadowedUserStores = nil
	c.LoadWarnings = nil
}

// recordProjectSources marks every name declared by the project file.
func (c *Config) recordProjectSources(def *Definition) {
	src := StoreSource{Scope: StoreScopeProject, Path: c.projectPathForDisplay()}
	for name := range def.SecretStores {
		c.StoreSources[name] = src
	}
	for name := range def.Services {
		c.StoreSources[name] = src
	}
	for name := range def.Providers {
		c.StoreSources[name] = src
	}
}

// loadUserConfig reads and validates the machine-level config. It returns
// (nil, nil) when there is nothing to load.
func (c *Config) loadUserConfig() (*userConfigFile, error) {
	path := c.UserConfig.Path
	if path == "" {
		return nil, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if c.UserConfig.Explicit() {
				return nil, dserrors.ConfigError{
					Field:      "user-config",
					Value:      path,
					Message:    "user configuration file not found",
					Suggestion: fmt.Sprintf("Create it with a 'secretStores:' section, or unset --user-config / %s", UserConfigEnvVar),
				}
			}
			c.debugf("No user config at %s", path)
			return nil, nil
		}
		return nil, dserrors.UserError{
			Message:    fmt.Sprintf("Failed to read user configuration file %s", path),
			Details:    err.Error(),
			Suggestion: "Check file permissions and path",
			Err:        err,
		}
	}
	if info.IsDir() {
		return nil, dserrors.ConfigError{
			Field:      "user-config",
			Value:      path,
			Message:    "user config path is a directory",
			Suggestion: fmt.Sprintf("Point --user-config or %s at a YAML file such as %s", UserConfigEnvVar, filepath.Join(path, "config.yaml")),
		}
	}

	for _, w := range userConfigPermissionWarnings(path, os.Stat, runtime.GOOS) {
		c.LoadWarnings = append(c.LoadWarnings, w)
		c.warnf("%s", w)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, dserrors.UserError{
			Message:    fmt.Sprintf("Failed to read user configuration file %s", path),
			Details:    err.Error(),
			Suggestion: "Check file permissions and path",
			Err:        err,
		}
	}

	return parseUserConfig(path, data)
}

func parseUserConfig(path string, data []byte) (*userConfigFile, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, dserrors.ConfigError{
			Field:      "user-config",
			Value:      path,
			Message:    "invalid YAML syntax in user configuration file",
			Suggestion: "Check for indentation errors, missing quotes, or invalid characters. Use a YAML validator",
		}
	}
	if root.Kind == 0 || len(root.Content) == 0 {
		return &userConfigFile{}, nil // empty file
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return nil, dserrors.ConfigError{
			Field:      "user-config",
			Value:      path,
			Message:    "user configuration must be a YAML mapping",
			Suggestion: "Start the file with 'version: 0' followed by a 'secretStores:' section",
		}
	}

	var disallowed []string
	for i := 0; i+1 < len(doc.Content); i += 2 {
		key := doc.Content[i].Value
		if !allowedUserConfigKeys[key] {
			disallowed = append(disallowed, "'"+key+"'")
		}
	}
	if len(disallowed) > 0 {
		return nil, dserrors.ConfigError{
			Field:   "user-config",
			Value:   path,
			Message: fmt.Sprintf("section %s not allowed in user configuration", strings.Join(disallowed, ", ")),
			Suggestion: fmt.Sprintf("Only 'secretStores:' (and legacy 'providers:') may be declared in %s. Move %s to the project dsops.yaml",
				path, strings.Join(disallowed, ", ")),
		}
	}

	var uf userConfigFile
	if err := doc.Decode(&uf); err != nil {
		return nil, dserrors.ConfigError{
			Field:      "user-config",
			Value:      path,
			Message:    "invalid structure in user configuration file",
			Suggestion: err.Error(),
		}
	}
	if uf.Version != 0 {
		return nil, dserrors.ConfigError{
			Field:      "version",
			Value:      uf.Version,
			Message:    fmt.Sprintf("unsupported configuration version in user configuration file %s", path),
			Suggestion: "Set 'version: 0' at the top of the file",
		}
	}
	for name := range uf.SecretStores {
		if _, dup := uf.Providers[name]; dup {
			return nil, dserrors.ConfigError{
				Field:      "user-config",
				Value:      path,
				Message:    fmt.Sprintf("'%s' is declared in both 'secretStores:' and 'providers:'", name),
				Suggestion: "Keep a single declaration per store name",
			}
		}
	}
	return &uf, nil
}

// mergeUserConfig fills gaps in the project definition from the user file.
// The project always wins: a user store whose name is already used by a
// project secret store, service, or legacy provider is recorded as shadowed
// and otherwise ignored.
func (c *Config) mergeUserConfig(def *Definition, uf *userConfigFile, path string) {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	src := StoreSource{Scope: StoreScopeUser, Path: path}

	taken := func(name string) bool {
		if _, ok := def.SecretStores[name]; ok {
			return true
		}
		if _, ok := def.Services[name]; ok {
			return true
		}
		_, ok := def.Providers[name]
		return ok
	}

	for _, name := range sortedKeys(uf.SecretStores) {
		if taken(name) {
			c.ShadowedUserStores = append(c.ShadowedUserStores, name)
			c.debugf("Secret store '%s' from user config %s is shadowed by the project config", name, path)
			continue
		}
		if def.SecretStores == nil {
			def.SecretStores = make(map[string]SecretStoreConfig)
		}
		def.SecretStores[name] = uf.SecretStores[name]
		c.StoreSources[name] = src
		c.debugf("Secret store '%s' declared by user config %s", name, path)
	}

	for _, name := range sortedKeys(uf.Providers) {
		if taken(name) {
			c.ShadowedUserStores = append(c.ShadowedUserStores, name)
			c.debugf("Provider '%s' from user config %s is shadowed by the project config", name, path)
			continue
		}
		if def.Providers == nil {
			def.Providers = make(map[string]ProviderConfig)
		}
		def.Providers[name] = uf.Providers[name]
		c.StoreSources[name] = src
		c.debugf("Legacy provider '%s' declared by user config %s", name, path)
	}

	sort.Strings(c.ShadowedUserStores)
	c.LoadedUserConfigPath = path
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// userConfigPermissionWarnings reports when the user config file (resolved
// through symlinks, so a nix-store target is what gets checked) or its
// containing directory is writable by group or others. Whoever can edit that
// file can redirect secret resolution, so it deserves a warning; it is not an
// error because bad umasks on CI images should not block resolution. Ownership
// is deliberately not checked: nix-store files are root-owned 0444 and must
// pass. Windows has no comparable permission bits, so it is skipped.
func userConfigPermissionWarnings(path string, stat func(string) (os.FileInfo, error), goos string) []string {
	if goos == "windows" {
		return nil
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}

	const otherWritable = 0o022
	var warnings []string
	if info, err := stat(path); err == nil {
		if perm := info.Mode().Perm(); perm&otherWritable != 0 {
			warnings = append(warnings, fmt.Sprintf(
				"user config %s is writable by other users (mode %04o); anyone who can edit it can redirect secret resolution",
				path, perm))
		}
	}
	parent := filepath.Dir(path)
	if info, err := stat(parent); err == nil && info.IsDir() {
		if perm := info.Mode().Perm(); perm&otherWritable != 0 {
			warnings = append(warnings, fmt.Sprintf(
				"directory %s containing the user config is writable by other users (mode %04o); anyone who can replace the file can redirect secret resolution",
				parent, perm))
		}
	}
	return warnings
}
