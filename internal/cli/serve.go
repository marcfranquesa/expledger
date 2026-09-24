package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/marcfranquesa/expledger/internal/web"
	"github.com/spf13/cobra"
)

func serveExperimentsCommand(app *application) *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "serve",
		Short: serveDescription,
		Long:  serveDetails,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return err
			}
			if port < 0 || port > 65535 {
				return errors.New("--port must be between 0 and 65535")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			repositoryURL, branch, err := githubLocation(app.repoRoot)
			if err != nil {
				return err
			}
			listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
			if err != nil {
				return fmt.Errorf("start experiment server: %w", err)
			}
			defer listener.Close()
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Serving experiments at http://%s\nPress Ctrl+C to stop.\n", listener.Addr()); err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			server := &http.Server{
				Handler:           web.NewHandler(app.repoRoot, repositoryURL, branch),
				ReadHeaderTimeout: 5 * time.Second,
			}
			finished := make(chan error, 1)
			go func() { finished <- server.Serve(listener) }()
			select {
			case err := <-finished:
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return fmt.Errorf("serve experiments: %w", err)
			case <-ctx.Done():
				shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := server.Shutdown(shutdown); err != nil {
					server.Close()
					return fmt.Errorf("stop experiment server: %w", err)
				}
				return nil
			}
		},
	}
	cmd.Flags().IntVar(&port, "port", 8080, "Local port (0 chooses an available port)")
	return cmd
}
