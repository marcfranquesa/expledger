// Package catalog creates and reads experiment directories in a project.
package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/marcfranquesa/expledger/internal/experiment"
)

// List walks grouping directories and orders records by creation time, newest first.
// Experiment directories are leaves; metadata IDs must match their relative paths.
// A missing experiments directory is an empty list.
func List(root string, layout Layout) ([]experiment.Record, error) {
	if err := layout.Validate(); err != nil {
		return nil, err
	}
	layout = layout.WithDefaults()
	project, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open project directory: %w", err)
	}
	defer project.Close()
	err = checkDirectory(project, layout.ExperimentsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read experiments directory: %w", err)
	}
	var records []experiment.Record
	err = fs.WalkDir(project.FS(), layout.ExperimentsDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if path == layout.ExperimentsDir || !entry.IsDir() {
			return nil
		}
		metadata := filepath.Join(filepath.FromSlash(path), "experiment.yaml")
		if _, err := project.Lstat(metadata); errors.Is(err, os.ErrNotExist) {
			return nil
		} else if err != nil {
			return fmt.Errorf("read %s: %w", metadata, err)
		}
		id := path[len(layout.ExperimentsDir)+1:]
		if err := validateID(id); err != nil {
			return err
		}
		record, err := readRecord(project, layout.ExperimentsDir, id)
		if err != nil {
			return err
		}
		records = append(records, record)
		return fs.SkipDir
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(records, func(i, j int) bool { return records[i].CreatedAt.After(records[j].CreatedAt) })
	return records, nil
}
