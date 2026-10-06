package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

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

func checkExperimentDirectory(project *os.Root, directory, id string) error {
	path := filepath.Join(directory, filepath.FromSlash(id))
	if err := checkDirectory(project, filepath.ToSlash(path)); err != nil {
		return fmt.Errorf("read %s: %w", filepath.Join(path, "experiment.yaml"), err)
	}
	return checkExperimentAncestors(project, directory, id)
}

func checkExperimentAncestors(project *os.Root, directory, id string) error {
	parts := strings.Split(id, "/")
	path := filepath.FromSlash(directory)
	for _, part := range parts[:len(parts)-1] {
		path = filepath.Join(path, part)
		metadata := filepath.Join(path, "experiment.yaml")
		if _, err := project.Lstat(metadata); err == nil {
			return fmt.Errorf("%s is inside an existing experiment at %s", id, path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read %s: %w", metadata, err)
		}
	}
	return nil
}
