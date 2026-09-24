package experiment

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// List reads immediate experiment directories and orders records by descending ID.
// A missing experiments directory is an empty list.
func List(root string) ([]Record, error) {
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
	var records []Record
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		readme := filepath.Join("experiments", entry.Name(), "README.md")
		data, err := project.ReadFile(readme)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", readme, err)
		}
		record, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", readme, err)
		}
		records = append(records, record)
	}
	sort.SliceStable(records, func(i, j int) bool { return records[i].ID > records[j].ID })
	return records, nil
}
