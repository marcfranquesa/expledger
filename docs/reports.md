---
title: Experiment reports
weight: 45
---

# Experiment reports

Add an optional `report.yaml` beside an experiment's `experiment.yaml` to compose
a report from Markdown, CSV results, and images. `build` bundles each report into
its self-contained HTML snapshot; `serve` refreshes reports as their files change.
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

  - type: image
    source: results/confusion-matrix.png
    alt: Confusion matrix with 98 and 95 correct predictions, and 2 and 5 errors.
    title: Validation confusion matrix

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
| `image` | `source`, `alt`, optional `title` | Embeds a local PNG or JPEG. `alt` is a required nonempty description; `title`, when present, is a nonempty visible caption. |
| `row` | `blocks` | Places Markdown, line, or image blocks in equal-width columns, wrapping when needed and stacking in declaration order at screen widths of 700 px or less. |

Rows contain content blocks; nested rows are unsupported. Chart appearance is
shared across reports: series colors, typography, axes, grid, and legend use the
same defaults. Each line block declares its data and title without repeating a
color scheme or layout settings.

Markdown supports CommonMark headings, lists, emphasis, code blocks, and links.
Raw HTML is disabled, and Markdown images (`![alt](path)`) appear as their alt text;
use an `image` block to render an image. Only report source
contents are bundled; other experiment artifacts are neither bundled nor served.
Relative artifact links appear as a readable label and path. HTTP, HTTPS, email,
and fragment links remain navigable; external links require access to their
destinations.

## Images

An image block's `source` names a local PNG or JPEG, such as a saved plot,
confusion matrix, or experiment photograph. The format is checked from the file
contents rather than its extension. Animated PNG (APNG), SVG, GIF, WebP, and remote
image URLs are unsupported. Images use the same experiment-relative paths and
confinement rules as other report sources.

Write `alt` text that describes the image's result or purpose. An optional `title`
provides a visible caption. Images preserve their aspect ratio and fit their
column; place an image block inside a `row` to compare it with another image,
Markdown, or a line chart.

Image bytes are embedded in the HTML as data URLs. No image server or network
request is needed, and the snapshot keeps working after the source image changes
or is removed.

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

## Limits and unavailable results

Reports support up to 64 blocks, including rows, eight series per chart, and
10,000 rows per CSV. File-size limits are 256 KiB for the manifest, 1 MiB per
Markdown file, and 8 MiB per CSV or image, with 16 MiB of unique source contents
per report.
Each image supports up to 16,000,000 pixels. A report can embed at most 16 MiB of
raw image bytes, counting an image again each time a block references it. Base64
encoding makes the embedded image data about one-third larger than its raw bytes.
Across its charts, a report can select up to 100,000 x and y values.

A missing or empty result CSV displays a results-unavailable state. This lets a
report describe an experiment before it runs. A missing image displays
**Image unavailable**. An empty, corrupt, unsupported, or oversized image is an
error, as is a missing Markdown file, malformed report declaration, missing CSV
column, or invalid numeric value.
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
report's prose, layout, results, or images to see updates. To save an offline
snapshot:

```sh
expledger build
```

The resulting `dist/index.html` contains the report text, plot data, and images
and can be shared as one file. Review the included content before sharing:
report source files become part of that snapshot. Neither command publishes files.
