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
	return checkDirectoryNames(project, directory, make(map[string]map[string]bool))
}

func checkDirectoryNames(project *os.Root, directory string, children map[string]map[string]bool) error {
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
		names, found := children[parent]
		if !found {
			entries, err := fs.ReadDir(project.FS(), filepath.ToSlash(parent))
			if err != nil {
				return err
			}
			names = make(map[string]bool, len(entries))
			for _, entry := range entries {
				names[entry.Name()] = true
			}
			children[parent] = names
		}
		if !names[component] {
			return fmt.Errorf("read %s: %w", path, os.ErrNotExist)
		}
		parent = path
	}
	return nil
}

// CheckExperimentDirectories validates exact paths and rejects symlink components
// or nested experiments, reading each parent directory's names once per batch.
func CheckExperimentDirectories(project *os.Root, directory string, ids []string) error {
	if !validRelativePath(directory) {
		return errors.New("experiments_dir must be a nonempty relative path without dot segments, backslashes, or empty components")
	}
	for _, id := range ids {
		if err := validateID(id); err != nil {
			return err
		}
	}
	children := make(map[string]map[string]bool)
	for _, id := range ids {
		path := filepath.Join(filepath.FromSlash(directory), filepath.FromSlash(id))
		if err := checkDirectoryNames(project, filepath.ToSlash(path), children); err != nil {
			return fmt.Errorf("read %s: %w", filepath.Join(path, "experiment.yaml"), err)
		}
		if err := checkExperimentAncestors(project, directory, id); err != nil {
			return err
		}
	}
	return nil
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
