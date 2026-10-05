# Running experiments

`expledger new` creates an executable `experiments/<id>/run.sh` stub that exits
with configuration guidance. Replace it with the experiment's commands, using a
shebang and fixed parameters. For example, with `experiment.py` beside it:

```sh
#!/bin/sh
exec uv run --locked python experiment.py --seed 42
```

Initialize the project with `expledger init`, then prepare the code and
dependencies you want. Git is not required. Run:

```sh
expledger run <id>
```

ExpLedger executes `./run.sh` directly from the existing experiment directory.
The script inherits your environment and writes results wherever it chooses.
Keep parameters in the script or nearby configuration; additional arguments and
revision-selection options are rejected. Both `run.sh` and `experiment.yaml` must
be regular files, and `run.sh` must be executable. Supporting files are left to
the script.

On macOS and Linux, standard input from a pipe or file, both output streams, and
the workload's exit status are forwarded. Ctrl-C stops the workload and its
ordinary children. Use batch workloads; interactive terminal input, job control,
and daemons are unsupported. Children must keep the workload's process group and
user identity so ExpLedger can stop them.

## Metadata and reproducibility

Running does not change metadata or record automatic provenance. Existing
historical `last_run` fields are left byte-for-byte unchanged and do not describe
new runs. There is no automatic commit, dirty-state, timestamp, or outcome record.

To reproduce a result, prepare the desired code yourself and retain experiment
inputs, dependency lockfiles, datasets, and environment details in your notes.
The runner does not create checkouts, copy files, or relocate outputs.

## Browsing multiple projects

Configure `sources` in the current project's `expledger.yaml` (see
[the format contract](format.md#project-configuration)), then run `expledger serve`.
To replace the configured list for this server process:

```sh
expledger serve --source . --source ../experiment-worktree --port 8080
```

Repeat `--source` once per root. Relative CLI paths also resolve from the discovered
project config directory. Empty values are errors. The override remains active
until the server stops, while other config changes are still reread.

The browser polls every three seconds after the previous request finishes. Valid
metadata, source order, relationships, and remote-link changes appear without a
restart or page reload. The selected list/graph view, graph zoom, search text, and selected experiment
are retained. Refreshed search results and details use current records. Page and
graph scroll positions are retained within the new content's scroll limits.
If the selected ID disappears from the graph, its selection clears; active drags defer updates
until a later poll. On a failed refresh, the last
valid view remains with an error indicator; the next successful poll recovers.

`new`, `run`, `list`, `validate`, and `build` still operate only on the current
project. A static `build` is an offline snapshot and never polls. `serve` is
read-only, requires no Git, and does not discover worktrees or watch files.
