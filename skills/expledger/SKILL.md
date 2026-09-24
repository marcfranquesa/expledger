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

Run the first time with an explicit project ref:

```sh
expledger run <id> --at main
```

Later `expledger run <id>` reuses `last_run.project_commit`. Use `--at <ref>` only
when deliberately selecting another project revision. Do not rewrite the receipt
after a rebase or merge: it records the latest launched entrypoint, including a
run that failed or was interrupted, and does not record success.

ExpLedger copies the current experiment into a temporary checkout of that project
commit and runs `./run.sh` from the copied experiment directory. Prepare and use
the environment from `EXPLEDGER_PROJECT_DIR`. Write durable results to
`EXPLEDGER_OUTPUT_DIR`; its actual path is printed to stderr and is local to the
repository's common Git directory, shared by its worktrees. Results left in the temporary
checkout are removed. Copy or publish artifacts that need to be shared.

Keep experiment-branch edits under `experiments/`. The runner uses the current
experiment definition, including uncommitted files, with historical project code;
it does not snapshot dependencies, datasets, or the environment. Record these in
Method when needed to reproduce the result. Avoid symlinks and special files in
the experiment, and use batch workloads rather than interactive jobs or daemons.

Use `expledger new --help`, `expledger run --help`, and the
[running guide](../../docs/running.md) for the command contract.
