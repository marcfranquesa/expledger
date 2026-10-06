package catalog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/marcfranquesa/expledger/internal/experiment"
)

// Read loads one experiment's metadata and requires its ID to match its relative path.
// It does not load parent experiment metadata or change any files.
func Read(root, id string, layout Layout) (experiment.Record, error) {
	if err := layout.Validate(); err != nil {
		return experiment.Record{}, err
	}
	layout = layout.WithDefaults()
	if err := validateID(id); err != nil {
		return experiment.Record{}, err
	}
	project, err := os.OpenRoot(root)
	if err != nil {
		return experiment.Record{}, fmt.Errorf("open project directory: %w", err)
	}
	defer project.Close()
	if err := checkExperimentDirectory(project, layout.ExperimentsDir, id); err != nil {
		return experiment.Record{}, err
	}
	return readRecord(project, layout.ExperimentsDir, id)
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

func readRecord(project *os.Root, directory, id string) (experiment.Record, error) {
	metadata := filepath.Join(filepath.FromSlash(directory), filepath.FromSlash(id), "experiment.yaml")
	info, err := project.Stat(metadata)
	if err != nil {
		return experiment.Record{}, fmt.Errorf("read %s: %w", metadata, err)
	}
	if !info.Mode().IsRegular() {
		return experiment.Record{}, fmt.Errorf("read %s: metadata must be a regular file", metadata)
	}
	data, err := project.ReadFile(metadata)
	if err != nil {
		return experiment.Record{}, fmt.Errorf("read %s: %w", metadata, err)
	}
	return parseRecord(metadata, id, data)
}

func parseRecord(metadata, id string, data []byte) (experiment.Record, error) {
	record, err := experiment.Parse(data)
	if err != nil {
		return experiment.Record{}, fmt.Errorf("parse %s: %w", metadata, err)
	}
	if record.ID != id {
		return experiment.Record{}, fmt.Errorf("invalid %s: YAML id %q must match experiment path %q", metadata, record.ID, id)
	}
	return record, nil
}
