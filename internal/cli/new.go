package cli

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/marcfranquesa/expledger/internal/experiment"
	"github.com/spf13/cobra"
)

func newExperimentCommand(cwd string, now time.Time) *cobra.Command {
	var opts experiment.CreateOptions
	cmd := &cobra.Command{
		Use:     "new <slug>",
		Short:   newDescription,
		Long:    newDetails,
		Example: newExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("title") && strings.TrimSpace(opts.Title) == "" {
				return fmt.Errorf("--title must not be empty")
			}
			root, err := gitRoot(cwd)
			if err != nil {
				return err
			}
			dir, err := experiment.Create(root, args[0], now, opts)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), dir)
			return err
		},
	}
	cmd.Flags().StringVar(&opts.Title, "title", "", "Display `title` (default: derived from slug)")
	cmd.Flags().StringArrayVar(&opts.BasedOn, "based-on", nil, "Parent experiment `id` (repeat for multiple parents)")
	return cmd
}

func gitRoot(cwd string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("find Git working tree (run inside a repository): %w", err)
	}
	return strings.TrimSuffix(strings.TrimSuffix(string(output), "\n"), "\r"), nil
}
