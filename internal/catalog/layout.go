package catalog

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Layout controls where experiments live and how new experiment IDs are named.
type Layout struct {
	ExperimentsDir   string `yaml:"experiments_dir,omitempty"`
	ExperimentFormat string `yaml:"experiment_format,omitempty"`
}

// WithDefaults fills omitted settings with the original flat experiment layout.
func (l Layout) WithDefaults() Layout {
	if l.ExperimentsDir == "" {
		l.ExperimentsDir = "experiments"
	}
	if l.ExperimentFormat == "" {
		l.ExperimentFormat = "20060102-{name}"
	}
	return l
}

// Validate requires project-relative paths and Go time layouts with an optional {name}.
func (l Layout) Validate() error {
	l = l.WithDefaults()
	if !validRelativePath(l.ExperimentsDir) {
		return fmt.Errorf("experiments_dir must be a nonempty relative path without dot segments, backslashes, or empty components")
	}
	if !validRelativePath(l.ExperimentFormat) || strings.ContainsAny(strings.ReplaceAll(l.ExperimentFormat, "{name}", "name"), "{}") {
		return fmt.Errorf("experiment_format must be a nonempty relative Go time layout; {name} is the only supported placeholder")
	}
	if err := validateID(l.ExperimentFormat); err != nil {
		return fmt.Errorf("experiment_format: %w", err)
	}
	return nil
}

func validRelativePath(value string) bool {
	if value == "." || !fs.ValidPath(value) || !filepath.IsLocal(filepath.FromSlash(value)) || strings.ContainsAny(value, "\\\x00") {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if strings.TrimSpace(part) == "" {
			return false
		}
	}
	return true
}

func validateID(id string) error {
	if !validRelativePath(id) {
		return fmt.Errorf("invalid experiment ID %q: use a nonempty relative path without dot segments, backslashes, or empty components", id)
	}
	// A directory named like a sidecar would make its grouping parent an experiment.
	for _, part := range strings.Split(id, "/")[1:] {
		if strings.EqualFold(part, "experiment.yaml") {
			return fmt.Errorf("invalid experiment ID %q: experiment.yaml is reserved for metadata inside grouping directories", id)
		}
	}
	return nil
}

// checkDirectory rejects symlink components and checks the spelling of each entry.
func checkDirectory(project *os.Root, directory string) error {
	parent := "."
	for _, component := range strings.Split(directory, "/") {
		path := filepath.Join(parent, component)
		info, err := project.Lstat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("%s must be a directory, not a file or symlink", path)
		}
		entries, err := fs.ReadDir(project.FS(), filepath.ToSlash(parent))
		if err != nil {
			return err
		}
		found := false
		for _, entry := range entries {
			if entry.Name() == component {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("read %s: %w", path, os.ErrNotExist)
		}
		parent = path
	}
	return nil
}
