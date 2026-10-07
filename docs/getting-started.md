---
title: Getting started
weight: 10
---

# Getting started

ExpLedger keeps coding experiments as folders in your project. Each folder has
structured metadata, independent notes, and a script you can run in place.
There is no database or automatic Git provenance. ExpLedger is in active
development; expect breaking changes.

## Install

Follow the [installation guide](install.md) to install the CLI and add it to your
`PATH`. The installed CLI has no Git dependency. Running experiment scripts
requires macOS or Linux.

## Create your first experiment

From the project directory where you want to keep experiments:

```sh
expledger init
expledger new baseline
```

Initialization creates a root `expledger.yaml`. The second command prints the
new experiment directory, whose default ID uses your local date:

```text
my-project/
├── expledger.yaml
└── experiments/
    └── YYYYMMDD-baseline/
        ├── experiment.yaml
        ├── README.md
        └── run.sh
```

Customize the base folder with `experiments_dir` and add date/time placeholders
with `experiment_format` in `expledger.yaml`, for example `{date}/{time}-{slug}`; see
[Experiment folders and names](configuration.md#experiment-folders-and-names).

Open that directory's README and describe the hypothesis and method. Replace the
executable `run.sh` stub with your workload, keeping parameters in the script or
nearby configuration. A minimal script looks like:

```sh
#!/bin/sh
printf 'Baseline completed\n'
```

The generated stub deliberately fails until you replace it. If your editor
removes its executable permission, restore it with `chmod +x` on `run.sh`.
See [Running experiments](running.md) for dependency management and execution
behavior.

Use the ID shown by `expledger list` in place of `YYYYMMDD-baseline`:

```sh
expledger list
expledger validate YYYYMMDD-baseline
expledger run YYYYMMDD-baseline
```

Record findings and links to results in the experiment's README after the run.
ExpLedger leaves these notes and the metadata unchanged.

To display prose and result charts in the browser, add an optional `report.yaml`
beside the metadata. [Experiment reports](reports.md) shows how to combine
Markdown, line charts, and rows with shared default styling, including a
[runnable CSV logging example](reports.md#logging-results-while-an-experiment-runs)
for live monitoring.

## Browse and share a catalog

Start a local browser view:

```sh
expledger serve --port 0
```

Open the printed URL. The browser refreshes automatically; press Ctrl-C to stop.
Choose **Graph** to explore parent relationships, or configure multiple sources
to browse projects together. See [Browsing multiple projects](running.md#browsing-multiple-projects)
and [Experiment graph](running.md#experiment-graph) for details.
For a remote workload, use the [SSH forwarding example](reports.md#monitor-a-remote-project-over-ssh).
To create a self-contained HTML snapshot instead:

```sh
expledger build
```

This writes `dist/index.html` relative to your current directory. Configure a
[`remote_url`](configuration.md) to add links to published experiment folders.
Neither command publishes your experiment files.

## Create a follow-up

Record a direct parent when creating a new experiment:

```sh
expledger new variant --title "Alternative configuration" --based-on YYYYMMDD-baseline
```

Replace the parent ID with an existing experiment's ID. This records a
relationship; it does not copy the parent's files. See
[Experiment format](format.md) for metadata and parent validation rules, and the
[CLI reference](https://marcfranquesa.github.io/expledger/docs/reference/expledger/) for all commands and flags.

## Set up the agent skill

After installing the CLI, link the
[agent skill](https://github.com/marcfranquesa/expledger/blob/main/skills/expledger/SKILL.md)
from a persistent ExpLedger checkout. Run one of these from that checkout.

For one repository, replace `/path/to/project`:

```sh
mkdir -p "/path/to/project/.agents/skills"
ln -s "$PWD/skills/expledger" "/path/to/project/.agents/skills/expledger"
```

For all your repositories:

```sh
mkdir -p "$HOME/.agents/skills"
ln -s "$PWD/skills/expledger" "$HOME/.agents/skills/expledger"
```

The link depends on keeping that checkout at its current location. The skill
guides agents to record hypotheses, methods, and findings around experiment runs.
