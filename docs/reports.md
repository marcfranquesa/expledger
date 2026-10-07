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

The manifest is one YAML mapping containing a nonempty `blocks` list. Each block
must provide all fields shown below. Unknown fields, duplicate keys, YAML aliases,
merge keys, and additional YAML documents are errors; no schema/version field is
needed.

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
Numbers are parsed as 64-bit floating-point values; table and inspection values
use their full parsed precision rather than the rounded axis labels.
The header must have nonempty, unique column names, and every data row must have
the same number of fields. Selected x values cannot be empty; selected y values
may be empty. Unselected columns may contain text. NaN and infinity are errors.
Multiple blocks may reuse one CSV; a snapshot reads a shared source consistently.

Reports support up to 64 blocks, including rows, and eight series per chart.
File-size limits are 256 KiB for the manifest, 1 MiB per Markdown file, and
256 MiB per CSV, with 512 MiB of unique source contents per report. Each logical
CSV record, including quoted multiline fields, is limited to 64 KiB. There is no
fixed row-count limit.

Every completed CSV row is validated, but a chart displays at most 512 recorded
rows. Larger logs are sampled while retaining the first and last rows, each
series's extrema, and additional observations across the log. Sampling keeps
recorded observations; it does not average values or fill gaps. Lines remain
disconnected across every known missing-value interval, even when the missing
rows are omitted from the sample; not every gap boundary is plotted individually.
The chart states how many
rows it plots out of the total. Its **View data** table shows the latest 100 raw
recorded rows, including their exact numeric values and missing values. The table
is a recent window; use the source CSV for the full history.

Across all charts in one report, the combined plot and recent-table data is limited to 100,000
x and y values. Reports with many charts receive smaller plot samples to stay
within that budget. Sparse series may show fewer visible observations as the
sample shrinks; missing intervals still remain disconnected.
The page includes every report in the catalog, so total page size and refresh
work also grow with the number of experiments and charts.

A missing or empty result CSV displays a results-unavailable state. This lets a
report describe an experiment before it runs. A missing Markdown file, malformed
report declaration, missing CSV column, or invalid numeric value is an error.
`build` fails instead of producing a partial report, and `serve` refuses to start
with an invalid initial snapshot. After startup, an invalid completed record in
any included chart fails the whole page refresh. The last valid snapshot remains
visible with an error until the files are valid again.

The browser waits three seconds after each refresh request finishes before
requesting the next snapshot. A request times out after ten seconds. A successful
refresh clears the error and retains the open report, expanded tables, table
scroll, and chart inspection. If a log is reset, inspection is cleared; an
appended log keeps inspection near the previously selected x value.
After the first poll establishes a revision, unchanged polls transfer no page
HTML and keep the current browser view.
The live reader keeps at most 64 cached CSV entries and 200,000 numeric values.
Overflow entries are read without evicting the admitted unchanged entries.
Changed CSV files are streamed and validated again from the beginning; append
refresh time therefore grows with log size. Projects exceeding the cache budget
may reread unchanged sources.

While `serve` reads an actively written log, its final record is omitted until
it is newline-terminated, including a quoted multiline record still being
written. Invalid completed records still fail the refresh. `build` accepts a
valid final record without a trailing newline and rejects malformed input.
Build after the writer finishes, or from an atomically published snapshot, when
you need a complete offline result: a valid EOF record during a write can still
contain only a prefix of the value the writer intends to publish.

## Logging results while an experiment runs

Write metrics from the workload; ExpLedger reads them and never modifies the log.
Since `expledger run <id>` executes in the experiment folder, save this standard
library example as `train.py` there. It writes the `results/metrics.csv` used by
the report above:

```python
import csv
import math
import os
from pathlib import Path
import time

path = Path("results/metrics.csv")
path.parent.mkdir(parents=True, exist_ok=True)
temporary = path.with_name(path.name + ".tmp")
with temporary.open("w", newline="", encoding="utf-8") as stream:
    writer = csv.writer(stream, lineterminator="\n")
    writer.writerow(["step", "train_loss", "validation_loss", "tokens_per_second"])
os.replace(temporary, path)
with path.open("a", newline="", encoding="utf-8") as stream:
    writer = csv.writer(stream, lineterminator="\n")
    for step in range(1000):
        loss = 2.4 * math.exp(-step / 300)
        writer.writerow([step, loss, loss + 0.15, 12000])
        stream.flush()
        time.sleep(0.1)
```

Replace the generated `run.sh` with:

```sh
#!/bin/sh
exec python3 train.py
```

Keep `run.sh` executable. Run `expledger run <id>` in one terminal and
`expledger serve --port 0` from the project in another. Replace the example loop
with your training steps; write one complete, newline-terminated CSV record per
step and flush it so the reader can see it. Use one writer per CSV. The example
starts a fresh log each run; if you resume by appending, keep x values strictly
greater than the previous run's last value.
Publish a fresh header atomically when restarting, as above. Avoid truncating or
overwriting a file during a read: a concurrent in-place rewrite can produce an
inconsistent prefix. Detected changes fail that refresh and retry on the next poll.

For workloads that rewrite a whole metrics snapshot, write a temporary file in
the same directory and publish it with `os.replace` after closing it:

```python
import csv
import os
from pathlib import Path

path = Path("results/metrics.csv")
path.parent.mkdir(parents=True, exist_ok=True)

def publish_metrics(rows):
    temporary = path.with_name(path.name + ".tmp")
    with temporary.open("w", newline="", encoding="utf-8") as stream:
        writer = csv.writer(stream, lineterminator="\n")
        writer.writerow(["step", "train_loss", "validation_loss", "tokens_per_second"])
        writer.writerows(rows)
        stream.flush()
    os.replace(temporary, path)

publish_metrics([[0, 2.4, 2.55, 12000], [100, 1.92, 2.10, 12100]])
```

Publish complete, valid rows; replacing the file prevents readers from seeing a
partly rewritten snapshot. Use this pattern for `report.yaml` or Markdown updates
too when producing them programmatically.

## Preview and share

From the initialized project, run:

```sh
expledger serve --port 0
```

Open the printed URL, then choose **View report** from an experiment's list entry
or graph details. Each report has a link within the same `index.html`. Edit the
report's prose, layout, or results to see updates. The server binds only to
`127.0.0.1`; it does not expose the project to the network. To save an offline
snapshot:

```sh
expledger build
```

The resulting `dist/index.html` contains the report text and plot data and can
be shared as one file. Review the included content before sharing: report source
files become part of that snapshot. Neither command publishes files.

### Monitor a remote project over SSH

On the remote machine, from the initialized project directory:

```sh
expledger serve --port 8080
```

On your local machine, forward a local port to that remote loopback listener:

```sh
ssh -N -L 127.0.0.1:8080:127.0.0.1:8080 user@host
```

Open [http://127.0.0.1:8080](http://127.0.0.1:8080) locally. Replace `user@host` with
your SSH destination and change the first `8080` in the forwarding argument if
that local port is occupied. Keep both commands running; Ctrl-C stops each one.
The remote `serve` command still binds to `127.0.0.1` and has no public host
option. Run the remote workload in another terminal or your usual job system.
See [Running experiments](running.md) for the runner's process and environment
behavior.
