// Package catalog discovers and locates experiments in a project.
package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"

	"github.com/marcfranquesa/expledger/internal/experiment"
)

// List reads immediate experiment directories and orders records by creation time, newest first.
// Each README's ID must exactly match its directory name.
// A missing experiments directory is an empty list.
func List(root string) ([]experiment.Record, error) {
	project, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open project directory: %w", err)
	}
	defer project.Close()
	info, err := project.Lstat("experiments")
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read experiments directory: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("experiments must be a directory, not a file or symlink")
	}
	entries, err := fs.ReadDir(project.FS(), "experiments")
	if err != nil {
		return nil, fmt.Errorf("read experiments directory: %w", err)
	}
	var records []experiment.Record
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		record, err := readRecord(project, entry.Name())
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	sort.SliceStable(records, func(i, j int) bool { return records[i].CreatedAt.After(records[j].CreatedAt) })
	return records, nil
}
