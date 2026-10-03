---
title: Running experiments
weight: 30
---

# Running experiments

`expledger new` creates an executable `experiments/<id>/run.sh` stub that exits
with configuration guidance. Replace it with the experiment's commands, using a
shebang and fixed parameters. For example, with `experiment.py` beside it:

```sh
#!/bin/sh
exec uv run --locked python experiment.py --seed 42
```

Follow [Getting started](getting-started.md) to initialize a project and create a
record, then prepare the code and dependencies you want. Git is not required. Run:

```sh
expledger run <id>
```

ExpLedger executes `./run.sh` directly from the existing experiment directory.
The script inherits your environment and writes results wherever it chooses.
Keep parameters in the script or nearby configuration; additional arguments and
revision-selection options are rejected. Both `run.sh` and `experiment.yaml` must
be regular files, and `run.sh` must be executable. Supporting files are left to
the script.

Running is supported on macOS and Linux. Standard input from a pipe or file, both
output streams, and the workload's exit status are forwarded. Ctrl-C stops the
workload and its ordinary children. Use batch workloads; interactive terminal input, job control,
and daemons are unsupported. Children must keep the workload's process group and
user identity so ExpLedger can stop them.

## Metadata and reproducibility

Running does not change metadata or record automatic provenance. Existing
historical `last_run` fields are left byte-for-byte unchanged and do not describe
new runs. There is no automatic commit, dirty-state, timestamp, or outcome record.

To reproduce a result, prepare the desired code yourself and retain experiment
inputs, dependency lockfiles, datasets, and environment details in your notes.
The runner does not create checkouts, copy files, or relocate outputs.

See the [CLI reference](https://marcfranquesa.github.io/expledger/docs/reference/expledger/) for commands
and flags, and [Experiment format](format.md) for metadata and filesystem rules.

## Experiment graph

Choose **Graph** in either `build` or `serve` to explore `based_on` lineage.
Descendants appear above ancestors, with arrows pointing upward. Independent
lineages favor their newest terminal descendant, so a late-imported ancestor
does not promote an older branch. Cycles are treated as a unit. Wide rows wrap;
shared routes reduce edge clutter. Missing parents and cycles use dashed styling.

Hover or focus previews direct connections; clicking pins selection and opens
full metadata, remote links, and navigable parents/children. Click the selected
node again, empty space, or Escape to clear it. Search by title or ID. Scroll or
drag to pan; Ctrl/Command + wheel zooms around the pointer. The graph starts at
75%; +/− and 100% adjust zoom without switching layouts. Double-click centers a
node at 100%; Enter or Space selects and centers a focused node.

The page is self-contained and works offline. The list remains the default and
works without JavaScript. Dependency structure takes precedence over date order;
parent IDs in the details panel provide a text alternative to bundled arrows.

Inspired by [Lab Exp's experiment graph](https://github.com/rsoatto/lab-exp),
with an independent implementation using ExpLedger's metadata and renderer.


## Browsing multiple projects

Configure `sources` in the current project's `expledger.yaml` (see
[Configuration](configuration.md#sources-for-live-browsing)), then run `expledger serve`.
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
