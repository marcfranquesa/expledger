package cli

import (
	"fmt"
	"path/filepath"

	"github.com/marcfranquesa/expledger/internal/catalog"
	"github.com/spf13/cobra"
)

func validateExperimentCommand(app *application) *cobra.Command {
	return &cobra.Command{
		Use:     "validate <id>",
		Short:   validateDescription,
		Long:    validateDetails,
		Example: "  expledger validate 20260924-my-idea",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := catalog.Read(app.projectRoot, args[0], app.config.Layout); err != nil {
				return err
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Valid: %s\n", filepath.Join(app.config.ExperimentsDir, args[0], "experiment.yaml"))
			return err
		},
	}
}
