package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

func initProjectCommand(app *application) *cobra.Command {
	var remote string
	cmd := &cobra.Command{
		Use: "init", Short: "Initialize an ExpLedger project in the current directory",
		Long:    "Create expledger.yaml in the current directory. Existing valid config is left unchanged.\nUse --remote-url for the full browser URL of the remote experiments directory.",
		Example: "  expledger init\n  expledger init --remote-url https://github.com/owner/repo/tree/main/experiments",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateRemoteURL(remote); err != nil {
				return err
			}
			path := filepath.Join(app.cwd, "expledger.yaml")
			file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if errors.Is(err, os.ErrExist) {
				config, err := readProjectConfig(path)
				if err != nil {
					return fmt.Errorf("read %s: %w", path, err)
				}
				if cmd.Flags().Changed("remote-url") && remote != config.RemoteURL {
					return fmt.Errorf("%s already exists with a different remote_url; edit it explicitly", path)
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), path)
				return err
			}
			if err != nil {
				return fmt.Errorf("create %s: %w", path, err)
			}
			data, err := yaml.Marshal(projectConfig{RemoteURL: remote})
			if err == nil {
				_, err = file.Write(data)
			}
			if err := errors.Join(err, file.Close()); err != nil {
				return fmt.Errorf("write %s: %w", path, errors.Join(err, os.Remove(path)))
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), path)
			return err
		},
	}
	cmd.Flags().StringVar(&remote, "remote-url", "", "Full browser `URL` of the remote experiments directory")
	return cmd
}
