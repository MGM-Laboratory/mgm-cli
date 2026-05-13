// Package projectfile reads/writes the per-repo .mgm.yaml that pins a working
// directory to a given Infisical project / environment / folder so users don't
// have to re-select on every command.
package projectfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const Filename = ".mgm.yaml"

type ProjectFile struct {
	ProjectID   string `yaml:"project_id,omitempty"`
	ProjectSlug string `yaml:"project_slug,omitempty"`
	Environment string `yaml:"environment,omitempty"`
	Folder      string `yaml:"folder,omitempty"`
}

// Load walks up from cwd looking for .mgm.yaml. Returns nil, nil if none exists.
func Load(start string) (*ProjectFile, string, error) {
	dir := start
	for {
		candidate := filepath.Join(dir, Filename)
		if _, err := os.Stat(candidate); err == nil {
			b, err := os.ReadFile(candidate)
			if err != nil {
				return nil, "", err
			}
			var pf ProjectFile
			if err := yaml.Unmarshal(b, &pf); err != nil {
				return nil, "", fmt.Errorf("parse %s: %w", candidate, err)
			}
			return &pf, candidate, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, "", err
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, "", nil
		}
		dir = parent
	}
}

// Save writes the project file in dir.
func Save(dir string, pf *ProjectFile) (string, error) {
	path := filepath.Join(dir, Filename)
	b, err := yaml.Marshal(pf)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
