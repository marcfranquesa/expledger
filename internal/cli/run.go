package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/marcfranquesa/expledger/internal/catalog"
	"github.com/spf13/cobra"
)

func runExperimentCommand(app *application) *cobra.Command {
	return &cobra.Command{
		Use:     "run <id>",
		Short:   runDescription,
		Long:    runDetails,
		Example: "  expledger run 20260924-baseline",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return fmt.Errorf("%w; keep experiment parameters in run.sh", err)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runExperiment(ctx, cmd, app.projectRoot, args[0], app.config.Layout)
		},
	}
}

func runExperiment(ctx context.Context, cli *cobra.Command, root, id string, layout catalog.Layout) error {
	if _, err := catalog.Read(root, id, layout); err != nil {
		return err
	}
	project, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer project.Close()
	experimentPath := filepath.Join(layout.ExperimentsDir, id)
	for _, name := range []string{"experiment.yaml", "run.sh"} {
		info, err := project.Lstat(filepath.Join(experimentPath, name))
		if err != nil {
			return fmt.Errorf("experiment requires %s: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s must be a regular file, not a symlink", name)
		}
		if name == "run.sh" && info.Mode().Perm()&0o111 == 0 {
			return errors.New("run.sh must be executable; run chmod +x on the experiment's run.sh")
		}
	}
	process := exec.Command("./run.sh")
	process.Dir = filepath.Join(root, experimentPath)
	process.Stdin, process.Stdout, process.Stderr = cli.InOrStdin(), cli.OutOrStdout(), cli.ErrOrStderr()
	return executeRun(ctx, process)
}

type workloadExit struct{ code int }

func (e *workloadExit) Error() string { return fmt.Sprintf("run.sh exited with status %d", e.code) }

// ExitCode preserves a workload's exit status and reports orchestration failures as 1.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exit *workloadExit
	if errors.As(err, &exit) {
		return exit.code
	}
	if errors.Is(err, context.Canceled) {
		return 130
	}
	return 1
}
