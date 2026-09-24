package cli

import (
	"fmt"
	"strings"

	"github.com/marcfranquesa/expledger/internal/catalog"
	"github.com/marcfranquesa/expledger/internal/experiment"
	"github.com/spf13/cobra"
)

func newExperimentCommand(app *application) *cobra.Command {
	var opts experiment.CreateOptions
	cmd := &cobra.Command{
		Use:     "new <slug>",
		Short:   newDescription,
		Long:    newDetails,
		Example: newExample,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return err
			}
			if cmd.Flags().Changed("title") && strings.TrimSpace(opts.Title) == "" {
				return fmt.Errorf("--title must not be empty")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, parent := range opts.BasedOn {
				if _, err := catalog.Read(app.repoRoot, parent); err != nil {
					return fmt.Errorf("check parent experiment %q: %w", parent, err)
				}
			}
			dir, err := experiment.Create(app.repoRoot, args[0], app.now, opts)
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
