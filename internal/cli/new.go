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
	return &cobra.Command{
		Use:     "new <slug>",
		Short:   newDescription,
		Long:    newDetails,
		Example: newExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := gitRoot(cwd)
			if err != nil {
				return err
			}
			dir, err := experiment.Create(root, args[0], now)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), dir)
			return err
		},
	}
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
