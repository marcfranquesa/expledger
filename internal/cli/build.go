package cli

import (
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
			if err := os.WriteFile(path, body, 0o644); err != nil {
				return fmt.Errorf("write experiment page: %w", err)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), path)
			return err
		},
	}
	cmd.Flags().StringVar(&output, "output", "dist", "Output `directory` (relative to the current directory)")
	return cmd
}
