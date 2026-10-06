package catalog

import (
	"fmt"
	"os"
	"path/filepath"

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
