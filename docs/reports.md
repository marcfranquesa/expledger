---
title: Experiment reports
weight: 45
---

# Experiment reports

Add an optional `report.yaml` beside an experiment's `experiment.yaml` to compose
a report from Markdown and CSV results. `build` bundles each report into its
self-contained HTML snapshot; `serve` refreshes reports as their files change.
Experiments without `report.yaml` remain ordinary catalog entries.

Reports follow the project's configured `experiments_dir` and experiment ID,
including IDs with nested grouping folders. For example, an experiment with ID
`2026-10-06/baseline` under `experiments_dir: trials` keeps its report at
`trials/2026-10-06/baseline/report.yaml`.

```yaml
blocks:
  - type: markdown
    source: introduction.md

  - type: row
    blocks:
      - type: line
        title: Training loss
        source: results/metrics.csv
        x: step
        y: [train_loss, validation_loss]

      - type: line
        title: Throughput
        source: results/metrics.csv
        x: step
        y: [tokens_per_second]

  - type: markdown
    source: conclusions.md
```

Blocks appear in declaration order. Source paths resolve from the experiment
folder, so `source: README.md` can reuse its existing notes. Use local relative
paths; absolute paths, parent traversal, and symlinks that escape the experiment
folder are rejected. Report files are read without being changed.

## Blocks and layout

| Type | Fields | Behavior |
| --- | --- | --- |
| `markdown` | `source` | Renders the local Markdown file as report prose. |
| `line` | `title`, `source`, `x`, `y` | Plots numeric CSV columns using the default chart style. `y` is a list of series names. |
| `row` | `blocks` | Places Markdown or line blocks in equal-width columns, wrapping when needed and stacking in declaration order at screen widths of 700 px or less. |

Rows contain content blocks; nested rows are unsupported. Chart appearance is
shared across reports: series colors, typography, axes, grid, and legend use the
same defaults. Each line block declares its data and title without repeating a
color scheme or layout settings.

Markdown supports CommonMark headings, lists, emphasis, code blocks, and links.
Raw HTML is disabled, and images appear as their alt text. Only report source
contents are bundled; other experiment artifacts are neither bundled nor served.
Relative artifact links appear as a readable label and path. HTTP, HTTPS, email,
and fragment links remain navigable; external links require access to their
destinations.

## CSV results

A line block's `source` names a CSV with a header row. `x` and each `y` entry must
match a column name exactly:

```csv
step,train_loss,validation_loss,tokens_per_second
0,2.40,2.55,11800
100,1.92,2.10,12100
200,1.61,,12400
300,1.42,1.71,12300
```

Use finite numeric values. Rows retain their file order, and `x` must increase
strictly; duplicate or decreasing x values are errors. An empty y value makes a
gap in that series, preserving the distinction between missing and zero.
Multiple blocks may reuse one CSV; a snapshot reads a shared source consistently.

Reports support up to 64 blocks, including rows, eight series per chart, and
10,000 rows per CSV. File-size limits are 256 KiB for the manifest, 1 MiB per
Markdown file, and 8 MiB per CSV, with 16 MiB of unique source contents per report.
Across its charts, a report can select up to 100,000 x and y values.

A missing or empty result CSV displays a results-unavailable state. This lets a
report describe an experiment before it runs. A missing Markdown file, malformed
report declaration, missing CSV column, or invalid numeric value is an error.
`build` fails instead of producing a partial report. During `serve`, a failed
refresh keeps the last valid snapshot and shows an error until the files are
valid again.

## Preview and share

From the initialized project, run:

```sh
expledger serve --port 0
```

Open the printed URL, then choose **View report** from an experiment's list entry
or graph details. Each report has a link within the same `index.html`. Edit the
report's prose, layout, or results to see updates. To save an offline snapshot:

```sh
expledger build
```

The resulting `dist/index.html` contains the report text and plot data and can
be shared as one file. Review the included content before sharing: report source
files become part of that snapshot. Neither command publishes files.
