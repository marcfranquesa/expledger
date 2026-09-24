package catalog

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/marcfranquesa/expledger/internal/experiment"
)

// Read loads one experiment's README and requires its ID to match the folder name.
// It does not read parent experiments or change any files.
func Read(root, id string) (experiment.Record, error) {
	if strings.TrimSpace(id) == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\\x00") {
		return experiment.Record{}, fmt.Errorf("invalid experiment ID %q: use a single nonempty directory name", id)
	}
	project, err := os.OpenRoot(root)
	if err != nil {
		return experiment.Record{}, fmt.Errorf("open project directory: %w", err)
	}
	defer project.Close()
	for _, path := range []string{"experiments", filepath.Join("experiments", id)} {
		info, err := project.Lstat(path)
		if err != nil {
			return experiment.Record{}, fmt.Errorf("read %s: %w", filepath.Join("experiments", id, "README.md"), err)
		}
		if !info.IsDir() {
			return experiment.Record{}, fmt.Errorf("%s must be a directory, not a file or symlink", path)
		}
	}
	entries, err := fs.ReadDir(project.FS(), "experiments")
	if err != nil {
		return experiment.Record{}, fmt.Errorf("read experiments directory: %w", err)
	}
	// Path lookup can ignore case or Unicode normalization on some filesystems.
	for _, entry := range entries {
		if entry.Name() == id {
			return readRecord(project, id)
		}
	}
	return experiment.Record{}, fmt.Errorf("read %s: %w", filepath.Join("experiments", id, "README.md"), os.ErrNotExist)
}

func readRecord(project *os.Root, id string) (experiment.Record, error) {
	readme := filepath.Join("experiments", id, "README.md")
	data, err := project.ReadFile(readme)
	if err != nil {
		return experiment.Record{}, fmt.Errorf("read %s: %w", readme, err)
	}
	record, err := experiment.Parse(data)
	if err != nil {
		return experiment.Record{}, fmt.Errorf("parse %s: %w", readme, err)
	}
	if record.ID != id {
		return experiment.Record{}, fmt.Errorf("invalid %s: YAML id %q must match folder name %q", readme, record.ID, id)
	}
	return record, nil
}
