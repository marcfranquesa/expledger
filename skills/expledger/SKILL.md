---
name: expledger
description: Use ExpLedger whenever running coding experiments, including hypothesis tests, benchmarks, and comparisons of implementations or configurations.
---

# ExpLedger

Before running a coding experiment, create a record in the target Git working tree:

```sh
expledger new descriptive-slug --title "Experiment title"
```

- Reuse the record when resuming the same experiment. For follow-ups, add
  `--based-on <parent-id>` using the `id` in an existing record's `expledger.yaml`;
  repeat for multiple parents.
- Edit the README in the directory printed by the command:
  - **Hypothesis:** the question and expected outcome, written before running.
  - **Method:** commands, inputs, configuration, project revision, and environment.
  - **Finding:** observed results, failures or uncertainty, and the conclusion.
- Preserve the `id` in `expledger.yaml` and existing notes. ExpLedger reads metadata
  only from `expledger.yaml`; do not add ExpLedger front matter to the README.
  Link supporting artifacts and include the README path in the final response.

Replace the generated executable `run.sh` stub with the experiment's commands.
Keep all parameters in `run.sh` or nearby configuration; do not pass workload
arguments through `expledger run`. Use a shebang and the project's environment
manager, for example:

```sh
#!/bin/sh
exec uv run --locked python experiment.py --seed 42
```

Prepare the intended project checkout and environment, then run:

```sh
expledger run <id>
```

ExpLedger runs the existing `./run.sh` from the experiment directory with your
environment. The script chooses where to write results. Use regular files for
`run.sh` and `expledger.yaml`, and use batch workloads rather than interactive
jobs or daemons.

After launch, `last_run` records the current Git `HEAD`, start time, and whether
Git sees staged, unstaged, or untracked project changes outside `experiments/`.
Ignored files are excluded. Failed and interrupted workloads still count as
launches; the receipt does not record success or select code for future runs.
After a rebase, the next run records the new current `HEAD` automatically.

Keep experiment-branch edits under `experiments/`. Record dependencies, datasets,
and environment details in Method when needed to reproduce the result; a commit
and dirty flag do not capture them. Copy or publish artifacts that need sharing.

Use `expledger new --help`, `expledger run --help`, and the
[running guide](../../docs/running.md) for the command contract.
