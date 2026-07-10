// Package profiles provides model profile loading and resolution.
package profiles

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadResult is returned by Loader.Load.
type LoadResult struct {
	Profiles []*Profile
	Warnings []string
}

// Loader reads model profile YAML files from a directory.
type Loader struct {
	dir string
}

// NewLoader creates a loader for an explicit profile directory.
// If dir is empty, Load() will search for a "profiles" directory.
func NewLoader(dir string) *Loader {
	return &Loader{dir: dir}
}

// NewDefaultLoader creates a loader that discovers the built-in profiles/
// directory by searching upward from the executable and current working
// directory.
func NewDefaultLoader() *Loader {
	return &Loader{}
}

// Load reads every *.yaml and *.yml file in the configured directory.
// Invalid profiles are skipped with a warning; a missing directory returns
// only the generic fallback profile so the runtime keeps working.
func (l *Loader) Load() (*LoadResult, error) {
	dir := l.dir
	if dir == "" {
		dir = findProfilesDir()
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return &LoadResult{
			Profiles: []*Profile{GenericFallback()},
			Warnings: []string{fmt.Sprintf("profile directory %q not found; using generic fallback", dir)},
		}, nil
	}

	var profiles []*Profile
	var warnings []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to read profile %s: %v", path, err))
			continue
		}

		var p Profile
		if err := yaml.Unmarshal(data, &p); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to parse profile %s: %v", path, err))
			continue
		}

		if err := Validate(&p); err != nil {
			warnings = append(warnings, fmt.Sprintf("skipping invalid profile %s: %v", path, err))
			continue
		}

		profiles = append(profiles, &p)
	}

	if len(profiles) == 0 {
		return &LoadResult{
			Profiles: []*Profile{GenericFallback()},
			Warnings: append(warnings, fmt.Sprintf("no valid profiles found in %q; using generic fallback", dir)),
		}, nil
	}

	return &LoadResult{Profiles: profiles, Warnings: warnings}, nil
}

// findProfilesDir searches upward from the executable directory and the
// current working directory for a directory named "profiles" that contains
// the generic-local.yaml marker file.
func findProfilesDir() string {
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		if dir := searchProfilesDir(filepath.Dir(exe)); dir != "" {
			candidates = append(candidates, dir)
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		if dir := searchProfilesDir(cwd); dir != "" {
			candidates = append(candidates, dir)
		}
	}
	for _, c := range candidates {
		if c != "" {
			return c
		}
	}
	return "profiles"
}

// searchProfilesDir walks up to 6 parent directories looking for a "profiles"
// directory that contains the generic-local.yaml marker.
func searchProfilesDir(start string) string {
	current := start
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(current, "profiles")
		marker := filepath.Join(candidate, "generic-local.yaml")
		if fi, err := os.Stat(marker); err == nil && !fi.IsDir() {
			return candidate
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return ""
}
