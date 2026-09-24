# Running experiments

`expledger new` creates an executable `experiments/<id>/run.sh` stub that exits
with configuration guidance. Replace it with the experiment's commands, using a
shebang and fixed parameters. For example, with `experiment.py` beside it:

```sh
#!/bin/sh
exec uv run --locked python experiment.py --seed 42
```

Prepare the project checkout and dependencies you want. The repository must have
at least one Git commit. Then run:

```sh
expledger run <id>
```

ExpLedger executes `./run.sh` directly from the existing experiment directory.
The script inherits your environment and writes results wherever it chooses.
Keep parameters in the script or nearby configuration; additional arguments and
revision-selection options are rejected. Both `run.sh` and `expledger.yaml` must
be regular files, and `run.sh` must be executable. Supporting files are left to
the script.

On macOS and Linux, standard input from a pipe or file, both output streams, and
the workload's exit status are forwarded. Ctrl-C stops the workload and its
ordinary children. Use batch workloads; interactive terminal input, job control,
and daemons are unsupported. Children must keep the workload's process group and
user identity so ExpLedger can stop them.

## Recorded provenance

ExpLedger reads Git `HEAD` and project changes before starting the script. Each
launch replaces `last_run` with that commit, a launch timestamp, and
`project_dirty`. The dirty flag covers Git-visible staged,
unstaged, and untracked changes outside `experiments/`; ignored files are excluded.
The commit is the checkout's committed baseline. Even an experiment-only commit
advances its hash without changing project code.
When `project_dirty` is true, reproduction also requires the uncommitted project
changes; the hash alone does not capture them.

A failed or interrupted workload still counts as a launch. Failures before launch
preserve the previous receipt. There is no outcome or run history. The receipt
never selects code: after a rebase, the next run records the new current `HEAD`.
See the [record format](format.md#run-receipt) for field details.

To reproduce a result, prepare the desired revision yourself and retain its
experiment inputs, dependency lockfiles, datasets, and environment details.
The receipt is a Git observation at launch, not a snapshot of all those inputs.
