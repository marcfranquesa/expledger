package catalog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/marcfranquesa/expledger/internal/experiment"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// CreateOptions supplies optional metadata for a new experiment.
type CreateOptions struct {
	Title   string // An empty title is derived from the slug.
	BasedOn []string
}

// Create adds an experiment under root, using the date in now's location.
// Only the named direct parents are validated; ancestors and unrelated records
// are not read. Invalid inputs do not change files. Existing paths are never overwritten.
func Create(root, slug string, now time.Time, opts CreateOptions) (string, error) {
	if !slugPattern.MatchString(slug) {
		return "", fmt.Errorf("invalid slug %q: use lowercase letters, digits, and single hyphens", slug)
	}
	id := now.Format("20060102") + "-" + slug
	title := opts.Title
	if title == "" {
		title = strings.ReplaceAll(slug, "-", " ")
		title = strings.ToUpper(title[:1]) + title[1:]
	}
	record := experiment.Record{
		ID: id, Title: title,
		CreatedAt: now.UTC(),
		BasedOn:   opts.BasedOn,
		Body:      []byte(fmt.Sprintf("\n# %s\n\n## Hypothesis\n\n## Method\n\n## Finding\n", title)),
	}
	data, err := record.Marshal()
	if err != nil {
		return "", err
	}
	for _, parent := range opts.BasedOn {
		if _, err := Read(root, parent); err != nil {
			return "", fmt.Errorf("check parent experiment %q: %w", parent, err)
		}
	}
	fs, err := os.OpenRoot(root)
	if err != nil {
		return "", fmt.Errorf("open project directory: %w", err)
	}
	defer fs.Close()
	if err := fs.Mkdir("experiments", 0755); err != nil && !errors.Is(err, os.ErrExist) {
		return "", fmt.Errorf("create experiments directory: %w", err)
	}
	info, err := fs.Lstat("experiments")
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("experiments must be a directory, not a file or symlink")
	}

	dir := filepath.Join("experiments", id)
	if err := fs.Mkdir(dir, 0755); err != nil {
		return "", fmt.Errorf("create experiment %s: %w", id, err)
	}
	readme := filepath.Join(dir, "README.md")
	f, err := fs.OpenFile(readme, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "", errors.Join(fmt.Errorf("create README: %w", err), fs.Remove(dir))
	}
	_, writeErr := f.Write(data)
	if err := errors.Join(writeErr, f.Close()); err != nil {
		return "", errors.Join(fmt.Errorf("write README: %w", err), fs.Remove(readme), fs.Remove(dir))
	}
	return filepath.Join(root, dir), nil
}
