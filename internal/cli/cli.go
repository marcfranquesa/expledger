// Package cli handles ExpLedger commands and their terminal output.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type application struct {
	repoRoot string
	now      time.Time
}

func Run(args []string, cwd string, now time.Time, stdout io.Writer) error {
	app := &application{now: now}
	root := &cobra.Command{
		Use:           "expledger",
		Short:         rootDescription,
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// The help subcommand inherits hooks; --help bypasses them.
			if cmd.Name() == "help" {
				return nil
			}
			var err error
			app.repoRoot, err = gitRoot(cwd)
			return err
		},
	}
	root.SetArgs(append([]string{}, args...))
	root.SetOut(stdout)
	root.AddCommand(newExperimentCommand(app), listExperimentsCommand(app), serveExperimentsCommand(app))
	return root.Execute()
}

func gitRoot(cwd string) (string, error) {
	output, err := gitOutput(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			detail := strings.TrimSpace(string(exitErr.Stderr))
			if strings.HasPrefix(detail, "fatal: not a git repository (or any") {
				return "", fmt.Errorf("no Git repository found in %q or its parent directories\nRun expledger from an existing Git repository, or run 'git init' in your project directory first.", cwd)
			}
			if detail != "" {
				return "", fmt.Errorf("find Git working tree: %s: %w", detail, err)
			}
		}
		return "", fmt.Errorf("find Git working tree: %w", err)
	}
	return output, nil
}

func gitOutput(cwd string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	// Keep Git diagnostics stable for error handling across locales.
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	output, err := cmd.Output()
	return strings.TrimSuffix(strings.TrimSuffix(string(output), "\n"), "\r"), err
}
