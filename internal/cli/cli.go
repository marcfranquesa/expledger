// Package cli handles ExpLedger commands and their terminal output.
package cli

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"
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

func gitRoot(cwd string) (string, error) {
	output, err := gitOutput(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			detail := strings.TrimSpace(string(exitErr.Stderr))
			if strings.HasPrefix(detail, "fatal: not a git repository (or any") {
				return "", fmt.Errorf("no Git repository found in %q or its parent directories\nRun expledger from an existing Git repository, or run 'git init' in your project directory first.", cwd)
			}
			if detail != "" {
				return "", fmt.Errorf("find Git working tree: %s: %w", detail, err)
			}
		}
		return "", fmt.Errorf("find Git working tree: %w", err)
	}
	return output, nil
}

func gitOutput(cwd string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	// Keep Git diagnostics stable for error handling across locales.
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	output, err := cmd.Output()
	return strings.TrimSuffix(strings.TrimSuffix(string(output), "\n"), "\r"), err
}

func githubLocation(root string) (remoteURLPrefix, branch string, err error) {
	remote, err := gitOutput(root, "remote", "get-url", "origin")
	if err != nil {
		return "", "", fmt.Errorf("GitHub links require an origin remote pointing to GitHub: %w", err)
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
		return "", "", errors.New("GitHub links require a checked-out branch; switch to a branch and try again")
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
