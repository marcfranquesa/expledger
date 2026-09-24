package experiment

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Exists reports whether experiments/id is a directory, without reading its README.
// IDs must be single directory names; files and symlinks return errors.
func Exists(root, id string) (bool, error) {
	if strings.TrimSpace(id) == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\\x00") {
		return false, fmt.Errorf("invalid experiment ID %q: use a single nonempty directory name", id)
	}
	project, err := os.OpenRoot(root)
	if err != nil {
		return false, fmt.Errorf("open project directory: %w", err)
	}
	defer project.Close()
	info, err := project.Lstat("experiments")
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read experiments directory: %w", err)
	}
	if !info.IsDir() {
		return false, errors.New("experiments must be a directory, not a file or symlink")
	}
	info, err = project.Lstat(filepath.Join("experiments", id))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read experiment %q: %w", id, err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("experiment %q must be a directory, not a file or symlink", id)
	}
	return true, nil
}
