# Running experiments

Each runnable experiment has an executable `experiments/<id>/run.sh`. `new`
creates a stub that fails with an explanatory message; replace it with the
workload before running. Existing records remain readable without an entrypoint.

Put parameters in `run.sh` or configuration files beside it. ExpLedger accepts
no workload arguments and executes `./run.sh` directly; the shebang chooses the
interpreter. For example, with `experiment.py` beside the entrypoint:

```sh
#!/bin/sh
exec uv run --locked python experiment.py --seed 42
```

The entrypoint must prepare and use the environment required by the selected
project revision. Use the project's environment manager and lockfile. The
script's working directory is the copied experiment directory.

## Select project code

```sh
expledger run <id> --at main
expledger run <id>
expledger run <id> --at <other-project-ref>
```

The first run requires `--at <ref>`. ExpLedger resolves that ref once to a full
commit ID. Later runs without `--at` use `last_run.project_commit`; selecting a
new ref is explicit. There is no automatic selection from `HEAD`, branch
ancestry, rebases, or merges.

ExpLedger creates a temporary detached Git worktree at the selected commit. It
replaces that checkout's selected experiment directory with a copy of the
current experiment definition, including uncommitted and untracked files. The
invoking checkout stays on its current branch with its project files unchanged.
Symlinks and special files in the experiment are rejected.

The copy includes only the selected experiment directory. Project files and
other experiment directories come from the historical checkout. Shared helpers
outside the selected directory therefore use that revision. Keep experiment
branches limited to `experiments/` as a workflow convention; ExpLedger does not
enforce a branch-diff policy.

The recorded commit must remain available locally. A missing object is an error;
ExpLedger does not substitute a newer revision or remap the hash. Prefer commits
in retained main-branch history when choosing project revisions.

This runs the current experiment definition against pinned project code. It does
not restore the original script or configuration, and does not capture
dependencies, datasets, or the environment. The commit alone is insufficient for
full reproduction.

## Keep results

The workload receives two absolute paths:

| Variable | Meaning |
| --- | --- |
| `EXPLEDGER_PROJECT_DIR` | Root of the prepared historical project checkout. |
| `EXPLEDGER_OUTPUT_DIR` | Unique persistent output directory for this invocation. |

Write every result that must survive to `EXPLEDGER_OUTPUT_DIR`. Files left in the
temporary checkout are removed during cleanup. ExpLedger prints the output path
to stderr and retains the directory when a workload fails.

Outputs live under the repository's common Git directory at
`expledger/runs/<id>/`, with a unique child directory for each invocation. Linked
worktrees share this location, so removing a linked worktree does not delete its
results. These outputs are local,
are not committed or pushed with the experiment, and have no automatic retention
policy. Copy or publish selected results yourself when they must be shared.

## Launches and cancellation

After launching the entrypoint, ExpLedger updates the source experiment's
`last_run` receipt with the project commit and launch timestamp. Nonzero exits
and interruptions still count as launches. Failures before launch preserve the
previous receipt. No exit status or outcome history is stored in metadata.

Only one run of an ID can be active in an invoking worktree at a time. On macOS
and Linux, interruption cancels the workload and its ordinary child processes
before cleanup. Standard input from a pipe or file and both output streams are
forwarded. The runner is intended for batch workloads; reading interactively
from the terminal, interactive job control, and daemon processes are unsupported.
