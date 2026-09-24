// Package cli handles ExpLedger commands and their terminal output.
package cli

import (
	"io"
	"time"

	"github.com/spf13/cobra"
)

type application struct {
	cwd      string
	repoRoot string
	now      time.Time
}

func Run(args []string, cwd string, now time.Time, stdout io.Writer) error {
	app := &application{cwd: cwd, now: now}
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
	root.AddCommand(newExperimentCommand(app), listExperimentsCommand(app), validateExperimentCommand(app), buildExperimentsCommand(app), serveExperimentsCommand(app))
	return root.Execute()
}
