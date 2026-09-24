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
		Schema: experiment.Schema, ID: id, Title: title,
		CreatedAt: now.UTC(),
		BasedOn:   opts.BasedOn,
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
	var created []string
	cleanup := func(cause error) (string, error) {
		for _, path := range created {
			cause = errors.Join(cause, fs.Remove(path))
		}
		return "", errors.Join(cause, fs.Remove(dir))
	}
	// Publish metadata last so discovery ignores an unfinished experiment.
	for _, file := range []struct {
		name string
		data []byte
		mode os.FileMode
	}{
		{"README.md", []byte(fmt.Sprintf("# %s\n\n## Hypothesis\n\n## Method\n\n## Finding\n", title)), 0644},
		{"run.sh", []byte("#!/bin/sh\nprintf '%s\\n' 'Configure run.sh with the experiment command and fixed parameters.' >&2\nexit 1\n"), 0755},
		{"expledger.yaml", data, 0644},
	} {
		path := filepath.Join(dir, file.name)
		f, err := fs.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, file.mode)
		if err != nil {
			return cleanup(fmt.Errorf("create %s: %w", path, err))
		}
		created = append(created, path)
		_, writeErr := f.Write(file.data)
		if err := errors.Join(writeErr, f.Close()); err != nil {
			return cleanup(fmt.Errorf("write %s: %w", path, err))
		}
	}
	return filepath.Join(root, dir), nil
}
