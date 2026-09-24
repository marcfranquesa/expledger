package catalog

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/marcfranquesa/expledger/internal/experiment"
)

// Read loads one experiment's metadata and requires its ID to match the folder name.
// It does not read parent experiments or change any files.
func Read(root, id string) (experiment.Record, error) {
	if err := validateID(id); err != nil {
		return experiment.Record{}, err
	}
	project, err := os.OpenRoot(root)
	if err != nil {
		return experiment.Record{}, fmt.Errorf("open project directory: %w", err)
	}
	defer project.Close()
	if err := checkExperimentDirectory(project, id); err != nil {
		return experiment.Record{}, err
	}
	return readRecord(project, id)
}

func checkExperimentDirectory(project *os.Root, id string) error {
	for _, path := range []string{"experiments", filepath.Join("experiments", id)} {
		info, err := project.Lstat(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", filepath.Join("experiments", id, "expledger.yaml"), err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s must be a directory, not a file or symlink", path)
		}
	}
	entries, err := fs.ReadDir(project.FS(), "experiments")
	if err != nil {
		return fmt.Errorf("read experiments directory: %w", err)
	}
	// Path lookup can ignore case or Unicode normalization on some filesystems.
	for _, entry := range entries {
		if entry.Name() == id {
			return nil
		}
	}
	return fmt.Errorf("read %s: %w", filepath.Join("experiments", id, "expledger.yaml"), os.ErrNotExist)
}

func readRecord(project *os.Root, id string) (experiment.Record, error) {
	metadata := filepath.Join("experiments", id, "expledger.yaml")
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
		return experiment.Record{}, fmt.Errorf("invalid %s: YAML id %q must match folder name %q", metadata, record.ID, id)
	}
	return record, nil
}

func validateID(id string) error {
	if strings.TrimSpace(id) == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\\x00") {
		return fmt.Errorf("invalid experiment ID %q: use a single nonempty directory name", id)
	}
	return nil
}
