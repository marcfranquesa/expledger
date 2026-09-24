package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func copyRunDefinition(ctx context.Context, source, destination, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	src, err := os.OpenRoot(source)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenRoot(destination)
	if err != nil {
		return err
	}
	defer dst.Close()
	if err := makeRunDirectory(dst, "experiments"); err != nil {
		return err
	}
	path := filepath.Join("experiments", id)
	// Replacement avoids retaining inputs deleted since the historical revision.
	if err := dst.RemoveAll(path); err != nil {
		return err
	}
	return fs.WalkDir(src.FS(), filepath.ToSlash(path), func(name string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return dst.Mkdir(name, 0o755)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("experiment input %s must be a regular file or directory; symlinks and special files are unsupported", name)
		}
		input, err := src.Open(name)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := dst.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(output, runInput{ctx, input})
		return errors.Join(copyErr, output.Chmod(info.Mode().Perm()), output.Close())
	})
}

// Check cancellation between reads, including when a definition contains large inputs.
type runInput struct {
	ctx context.Context
	io.Reader
}

func (r runInput) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}
