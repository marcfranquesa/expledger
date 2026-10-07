// Package cli handles ExpLedger commands and their terminal output.
package cli

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/marcfranquesa/expledger/internal/version"
	"github.com/spf13/cobra"
)

type application struct {
	cwd         string
	projectRoot string
	config      projectConfig
	now         time.Time
}

// Streams connects the command and its workload to the caller's input and output.
// Nil input is empty; nil outputs are discarded.
// Streams remain caller-owned. Custom readers and writers must finish or unblock
// independently of Run returning, including on cancellation; Run does not close them.
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
	root := NewCommand(cwd, now)
	root.SetArgs(append([]string{}, args...))
	root.SetIn(streams.In)
	root.SetOut(streams.Out)
	root.SetErr(streams.Err)
	return root.ExecuteContext(ctx)
}

// NewCommand constructs the CLI without discovering a project or executing work.
// Each invocation returns independent commands and flag state.
func NewCommand(cwd string, now time.Time) *cobra.Command {
	app := &application{cwd: cwd, now: now}
	root := &cobra.Command{
		Use:           "expledger",
		Version:       version.String(),
		Short:         rootDescription,
		Long:          rootDetails,
		Example:       "  expledger init\n  expledger new baseline\n  expledger list",
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
			if cmd.Name() == "init" {
				return nil
			}
			var err error
			app.projectRoot, app.config, err = discoverProject(cwd)
			return err
		},
	}
	root.AddCommand(initProjectCommand(app), newExperimentCommand(app), listExperimentsCommand(app), validateExperimentCommand(app), buildExperimentsCommand(app), serveExperimentsCommand(app), runExperimentCommand(app))
	return root
}
