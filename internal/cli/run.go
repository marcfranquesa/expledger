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
	"time"

	"github.com/marcfranquesa/expledger/internal/catalog"
	"github.com/marcfranquesa/expledger/internal/experiment"
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
			return runExperiment(ctx, cmd, app.repoRoot, args[0])
		},
	}
}

func runExperiment(ctx context.Context, cli *cobra.Command, root, id string) error {
	if _, err := catalog.Read(root, id); err != nil {
		return err
	}
	project, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer project.Close()
	experimentPath := filepath.Join("experiments", id)
	for _, name := range []string{"expledger.yaml", "run.sh"} {
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
	commit, err := runGit(ctx, root, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return fmt.Errorf("read HEAD for run provenance: %w", err)
	}
	status, err := runGit(ctx, root, "status", "--porcelain=v1", "--untracked-files=normal", "--ignore-submodules=none", "--", ".", ":(top,exclude)experiments")
	if err != nil {
		return fmt.Errorf("read project changes: %w", err)
	}
	process := exec.Command("./run.sh")
	process.Dir = filepath.Join(root, experimentPath)
	process.Stdin, process.Stdout, process.Stderr = cli.InOrStdin(), cli.OutOrStdout(), cli.ErrOrStderr()
	return executeRun(ctx, process, func() error {
		receipt := experiment.RunReceipt{ProjectCommit: commit, ProjectDirty: status != "", StartedAt: time.Now().UTC()}
		if err := catalog.RecordRun(root, id, receipt); err != nil {
			return fmt.Errorf("workload launched but last_run could not be saved: %w", err)
		}
		return nil
	})
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
