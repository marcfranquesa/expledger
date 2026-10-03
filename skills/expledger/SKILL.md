---
name: expledger
description: Use ExpLedger whenever running coding experiments, including hypothesis tests, benchmarks, and comparisons of implementations or configurations.
---

# ExpLedger

Before running a coding experiment, locate the target project’s root
`expledger.yaml`. If it has no project config, run `expledger init` there.
Then create a record:

```sh
expledger new descriptive-slug --title "Experiment title"
```

- Reuse the record when resuming the same experiment. For follow-ups, add
  `--based-on <parent-id>` using the `id` in an existing record's `experiment.yaml`;
  repeat for multiple parents.
- Edit the README in the directory printed by the command:
  - **Hypothesis:** the question and expected outcome, written before running.
  - **Method:** commands, inputs, configuration, project revision, and environment.
  - **Finding:** observed results, failures or uncertainty, and the conclusion.
- Preserve the `id` in `experiment.yaml` and existing notes. ExpLedger reads metadata
  only from `experiment.yaml`; do not add ExpLedger front matter to the README.
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
`run.sh` and `experiment.yaml`, and use batch workloads rather than interactive
jobs or daemons.

ExpLedger does not read Git or write metadata during a run. Historical `last_run`
values are custom metadata, left unchanged; do not treat them as current
provenance. Record code revisions, dependencies, datasets, environment details,
and results in the notes when needed for reproduction. Copy or publish artifacts
that need sharing.

For remote links, set `remote_url` in the root `expledger.yaml` to the full browser
URL of the published experiments directory. Omit it for local-only projects.
To migrate older projects, rename each experiment's `expledger.yaml` to
`experiment.yaml` without changing contents, then initialize the project root.

Use `expledger new --help`, `expledger run --help`, and the
[running guide](../../docs/running.md) for the command contract.
