package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/marcfranquesa/expledger/internal/catalog"
	"github.com/spf13/cobra"
)

func listExperimentsCommand(app *application) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: listDescription,
		Long:  listDetails,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			records, err := catalog.List(app.repoRoot)
			if err != nil {
				return err
			}
			if len(records) == 0 {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "No experiments found.")
				return err
			}
			out := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			if _, err := fmt.Fprintln(out, "ID\tTITLE"); err != nil {
				return err
			}
			for _, record := range records {
				id := strings.Join(strings.Fields(record.ID), " ")
				title := strings.Join(strings.Fields(record.Title), " ")
				if _, err := fmt.Fprintf(out, "%s\t%s\n", id, title); err != nil {
					return err
				}
			}
			return out.Flush()
		},
	}
}
