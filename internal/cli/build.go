package cli

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/marcfranquesa/expledger/internal/web"
	"github.com/spf13/cobra"
)

func buildExperimentsCommand(app *application) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "build",
		Short: buildDescription,
		Long:  buildDetails,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return err
			}
			if strings.TrimSpace(output) == "" {
				return errors.New("--output must not be empty")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			remoteURLPrefix, branch, err := githubLocation(app.repoRoot)
			if err != nil {
				return err
			}
			body, err := web.Render(app.repoRoot, remoteURLPrefix, branch)
			if err != nil {
				return err
			}
			dir := output
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(app.cwd, dir)
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("create output directory: %w", err)
			}
			path := filepath.Join(dir, "index.html")
			if err := writeSnapshot(path, body); err != nil {
				return fmt.Errorf("write experiment page: %w", err)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), path)
			return err
		},
	}
	cmd.Flags().StringVar(&output, "output", "dist", "Output `directory` (relative to the current directory)")
	return cmd
}

func writeSnapshot(path string, body []byte) error {
	info, err := os.Lstat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	mode := os.FileMode(0o644)
	if info != nil && info.Mode().IsRegular() {
		mode = info.Mode().Perm()
	}
	// Exclusive creation honors the umask without following an existing link.
	temporary := filepath.Join(filepath.Dir(path), ".expledger-"+rand.Text()+".html")
	f, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(body); err != nil {
		return errors.Join(err, f.Close())
	}
	var permissionErr error
	if info != nil && info.Mode().IsRegular() {
		permissionErr = f.Chmod(mode)
	}
	if err := errors.Join(permissionErr, f.Close()); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
