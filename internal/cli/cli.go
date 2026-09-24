// Package cli handles ExpLedger commands and their terminal output.
package cli

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type application struct {
	cwd      string
	repoRoot string
	now      time.Time
}

// Streams connects the command and its workload to the caller's input and output.
// Nil input is empty; nil outputs are discarded.
type Streams struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// Run checks cancellation before running a command and stops ongoing work when ctx ends.
func Run(ctx context.Context, args []string, cwd string, now time.Time, streams Streams) error {
	if streams.In == nil {
		streams.In = strings.NewReader("")
	}
	if streams.Out == nil {
		streams.Out = io.Discard
	}
	if streams.Err == nil {
		streams.Err = io.Discard
	}
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
			if err := cmd.Context().Err(); err != nil {
				return err
			}
			var err error
			if cmd.Name() == "run" {
				app.repoRoot, err = runGit(cmd.Context(), cwd, "rev-parse", "--show-toplevel")
			} else {
				app.repoRoot, err = gitRoot(cwd)
			}
			return err
		},
	}
	root.SetArgs(append([]string{}, args...))
	root.SetIn(streams.In)
	root.SetOut(streams.Out)
	root.SetErr(streams.Err)
	root.AddCommand(newExperimentCommand(app), listExperimentsCommand(app), validateExperimentCommand(app), buildExperimentsCommand(app), serveExperimentsCommand(app), runExperimentCommand(app))
	return root.ExecuteContext(ctx)
}
