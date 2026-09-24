// Package cli handles ExpLedger commands and their terminal output.
package cli

import (
	"io"
	"time"

	"github.com/spf13/cobra"
)

func Run(args []string, cwd string, now time.Time, stdout io.Writer) error {
	root := &cobra.Command{
		Use:           "expledger",
		Short:         rootDescription,
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}
	root.SetArgs(append([]string{}, args...))
	root.SetOut(stdout)
	root.AddCommand(newExperimentCommand(cwd, now))
	return root.Execute()
}
