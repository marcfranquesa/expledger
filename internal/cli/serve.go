package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
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
			remoteURLPrefix, branch, err := githubLocation(app.repoRoot)
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
				Handler:           web.NewHandler(app.repoRoot, remoteURLPrefix, branch),
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

func githubLocation(root string) (remoteURLPrefix, branch string, err error) {
	remote, err := gitOutput(root, "remote", "get-url", "origin")
	if err != nil {
		return "", "", fmt.Errorf("serve requires an origin remote pointing to GitHub: %w", err)
	}
	repositoryURL, err := githubRepositoryURL(remote)
	if err != nil {
		return "", "", err
	}
	branch, err = gitOutput(root, "branch", "--show-current")
	if err != nil {
		return "", "", fmt.Errorf("read current Git branch: %w", err)
	}
	if branch == "" {
		return "", "", errors.New("serve requires a checked-out branch; switch to a branch and try again")
	}
	return repositoryURL + "/tree/" + url.PathEscape(branch) + "/experiments/", branch, nil
}

func githubRepositoryURL(remote string) (string, error) {
	if host, path, ok := strings.Cut(remote, ":"); ok && strings.EqualFold(host, "git@github.com") {
		remote = "ssh://git@github.com/" + path
	}
	u, err := url.Parse(remote)
	if err != nil || (u.Scheme != "https" && u.Scheme != "ssh") || !strings.EqualFold(u.Hostname(), "github.com") || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("origin must be a GitHub HTTPS or SSH remote")
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/"), "/")
	if len(parts) != 2 {
		return "", errors.New("origin must point to a GitHub repository (owner/repository)")
	}
	parts[1] = strings.TrimSuffix(parts[1], ".git")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", errors.New("origin must point to a GitHub repository (owner/repository)")
		}
	}
	return "https://github.com/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]), nil
}
