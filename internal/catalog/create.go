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

// CreateOptions supplies the layout and optional metadata for a new experiment.
type CreateOptions struct {
	Title   string // An empty title is derived from the slug.
	BasedOn []string
	Layout  Layout
}

// Create adds an experiment under root, formatting its ID in now's location.
// Only the named direct parents are validated; ancestors and unrelated records
// are not read. Invalid inputs do not change files. Existing paths are never overwritten.
func Create(root, slug string, now time.Time, opts CreateOptions) (string, error) {
	if err := opts.Layout.Validate(); err != nil {
		return "", err
	}
	layout := opts.Layout.WithDefaults()
	if !slugPattern.MatchString(slug) {
		return "", fmt.Errorf("invalid slug %q: use lowercase letters, digits, and single hyphens", slug)
	}
	id, err := formatExperimentID(layout.ExperimentFormat, slug, now)
	if err != nil {
		return "", fmt.Errorf("experiment_format: %w", err)
	}
	if err := validateID(id); err != nil {
		return "", err
	}
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
		if _, err := Read(root, parent, layout); err != nil {
			return "", fmt.Errorf("check parent experiment %q: %w", parent, err)
		}
	}
	fs, err := os.OpenRoot(root)
	if err != nil {
		return "", fmt.Errorf("open project directory: %w", err)
	}
	defer fs.Close()
	dir := filepath.Join(filepath.FromSlash(layout.ExperimentsDir), filepath.FromSlash(id))
	parent := filepath.ToSlash(filepath.Dir(dir))
	// Check existing components before creating anything, including on case-folding filesystems.
	path := ""
	for _, part := range strings.Split(parent, "/") {
		path = filepath.Join(path, part)
		if _, err := fs.Lstat(path); errors.Is(err, os.ErrNotExist) {
			break
		} else if err != nil {
			return "", fmt.Errorf("read %s: %w", path, err)
		}
		if err := checkDirectory(fs, filepath.ToSlash(path)); err != nil {
			return "", err
		}
	}
	if err := checkExperimentAncestors(fs, layout.ExperimentsDir, id); err != nil {
		return "", err
	}
	if _, err := fs.Lstat(dir); err == nil {
		return "", fmt.Errorf("create experiment %s: %w", id, os.ErrExist)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("create experiment %s: %w", id, err)
	}
	var created, directories []string
	cleanup := func(cause error) (string, error) {
		for _, path := range created {
			cause = errors.Join(cause, fs.Remove(path))
		}
		for i := len(directories) - 1; i >= 0; i-- {
			cause = errors.Join(cause, fs.Remove(directories[i]))
		}
		return "", cause
	}
	path = ""
	for _, part := range strings.Split(parent, "/") {
		path = filepath.Join(path, part)
		if err := fs.Mkdir(path, 0755); err == nil {
			directories = append(directories, path)
		} else if !errors.Is(err, os.ErrExist) {
			return cleanup(fmt.Errorf("create experiments directory: %w", err))
		}
		if err := checkDirectory(fs, filepath.ToSlash(path)); err != nil {
			return cleanup(err)
		}
	}
	if err := fs.Mkdir(dir, 0755); err != nil {
		return cleanup(fmt.Errorf("create experiment %s: %w", id, err))
	}
	directories = append(directories, dir)
	// Publish metadata last so discovery ignores an unfinished experiment.
	for _, file := range []struct {
		name string
		data []byte
		mode os.FileMode
	}{
		{"README.md", []byte(fmt.Sprintf("# %s\n\n## Hypothesis\n\n## Method\n\n## Finding\n", title)), 0644},
		{"run.sh", []byte("#!/bin/sh\nprintf '%s\\n' 'Configure run.sh with the experiment command and fixed parameters.' >&2\nexit 1\n"), 0755},
		{"experiment.yaml", data, 0644},
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
