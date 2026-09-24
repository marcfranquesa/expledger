package cli

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/marcfranquesa/expledger/internal/catalog"
	"github.com/marcfranquesa/expledger/internal/experiment"
	"github.com/spf13/cobra"
)

func runExperimentCommand(app *application) *cobra.Command {
	var ref string
	cmd := &cobra.Command{
		Use:     "run <id> [--at <ref>]",
		Short:   runDescription,
		Long:    runDetails,
		Example: "  expledger run 20260924-baseline --at main\n  expledger run 20260924-baseline",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return fmt.Errorf("%w; keep experiment parameters in run.sh", err)
			}
			if cmd.Flags().Changed("at") && strings.TrimSpace(ref) == "" {
				return errors.New("--at must name a Git revision")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runExperiment(ctx, cmd, app.repoRoot, args[0], ref)
		},
	}
	cmd.Flags().StringVar(&ref, "at", "", "Project Git `ref` to run (default: last started run's commit)")
	return cmd
}

func runExperiment(ctx context.Context, cli *cobra.Command, root, id, ref string) (result error) {
	gitDir, err := runGit(ctx, root, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return err
	}
	state, err := os.OpenRoot(gitDir)
	if err != nil {
		return err
	}
	defer state.Close()
	for _, dir := range []string{"expledger", "expledger/locks"} {
		if err := makeRunDirectory(state, dir); err != nil {
			return err
		}
	}
	lockPath := filepath.Join("expledger/locks", fmt.Sprintf("%x.lock", sha256.Sum256([]byte(id))))
	unlock, err := lockRun(state, lockPath)
	if err != nil {
		return fmt.Errorf("lock experiment %q: %w", id, err)
	}
	defer func() { result = errors.Join(result, unlock()) }()
	// Read under the lock so a previous invocation cannot change the selected revision.
	record, err := catalog.Read(root, id)
	if err != nil {
		return err
	}
	if ref == "" {
		if record.LastRun == nil {
			return errors.New("first run requires --at <ref> to select the project revision")
		}
		ref = record.LastRun.ProjectCommit
	}
	commit, err := runGit(ctx, root, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	if err != nil {
		return fmt.Errorf("resolve project revision %q (no fallback is used): %w", ref, err)
	}
	source, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer source.Close()
	experimentPath := filepath.Join("experiments", id)
	for _, name := range []string{"expledger.yaml", "run.sh"} {
		info, err := source.Lstat(filepath.Join(experimentPath, name))
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
	// Local outputs live outside both the definition and the disposable checkout.
	commonDir, err := runGit(ctx, root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	outputs, err := os.OpenRoot(commonDir)
	if err != nil {
		return err
	}
	defer outputs.Close()
	for _, dir := range []string{"expledger", "expledger/runs", filepath.Join("expledger/runs", id)} {
		if err := makeRunDirectory(outputs, dir); err != nil {
			return err
		}
	}
	output, err := os.MkdirTemp(filepath.Join(commonDir, "expledger", "runs", id), "run-")
	if err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if _, err := fmt.Fprintf(cli.ErrOrStderr(), "Project commit: %s\nOutput directory: %q\n", commit, output); err != nil {
		return err
	}
	temporary, err := os.MkdirTemp("", "expledger-run-")
	if err != nil {
		return err
	}
	temporary, err = filepath.EvalSymlinks(temporary)
	if err != nil {
		return err
	}
	worktree := filepath.Join(temporary, "project")
	if _, err := runGit(ctx, root, "-c", "core.hooksPath=/dev/null", "-c", "core.sparseCheckout=false", "worktree", "add", "--quiet", "--detach", "--", worktree, commit); err != nil {
		// Git may have partially registered the checkout. Keep the path for recovery.
		return fmt.Errorf("prepare worktree (inspect %s if cleanup is needed): %w", temporary, err)
	}
	cleanup := true
	defer func() {
		if !cleanup {
			result = errors.Join(result, fmt.Errorf("worktree retained for recovery: %s", worktree))
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		// This exact worktree is owned by this run and is dirty after copying inputs.
		if _, err := runGit(cleanupCtx, root, "worktree", "remove", "--force", "--", worktree); err != nil {
			result = errors.Join(result, fmt.Errorf("cleanup failed; worktree retained at %s: %w", worktree, err))
			return
		}
		result = errors.Join(result, os.Remove(temporary))
	}()
	if err := copyRunDefinition(ctx, root, worktree, id); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	process := exec.Command("./run.sh")
	process.Dir = filepath.Join(worktree, experimentPath)
	process.Env = append(runEnvironment(process.Environ()), "EXPLEDGER_PROJECT_DIR="+worktree, "EXPLEDGER_OUTPUT_DIR="+output)
	process.Stdin, process.Stdout, process.Stderr = cli.InOrStdin(), cli.OutOrStdout(), cli.ErrOrStderr()
	safe, err := executeRun(ctx, process, func() error {
		receipt := experiment.RunReceipt{ProjectCommit: commit, StartedAt: time.Now().UTC()}
		if err := catalog.RecordRun(root, id, receipt); err != nil {
			return fmt.Errorf("workload launched at %s but last_run could not be saved; output directory %s: %w", commit, output, err)
		}
		return nil
	})
	cleanup = safe
	return err
}

func makeRunDirectory(root *os.Root, path string) error {
	if err := root.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	info, err := root.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("runner directory %s must not be a file or symlink", path)
	}
	return nil
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
