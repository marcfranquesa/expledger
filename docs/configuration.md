---
title: Configuration
weight: 20
---

# Configuration

The initialized project root contains `expledger.yaml`, exactly one YAML mapping
with optional settings:

```yaml
experiments_dir: experiments
experiment_format: "{date:20060102}-{slug}"
remote_url: https://github.com/owner/repo/tree/main/experiments
sources:
  - .
  - ../experiment-worktree
  - /Users/me/projects/another-worktree
```

Set `remote_url` when initializing a project; edit the config for other settings:

```sh
expledger init --remote-url https://github.com/owner/repo/tree/main/experiments
```

For an existing project, edit `remote_url` in its root `expledger.yaml` explicitly.
`build` and `serve` use it for each experiment's View remote link. It must point to
the published **experiments directory**, not the repository home page.

Use `{}` for a local-only project. `remote_url` must be a string; an empty string
also disables links. A configured value is the full HTTP(S) browser URL of the
remote experiments directory, with an ASCII hostname or IP address and optional
port. Credentials, queries, and fragments are rejected. A trailing slash is
optional. Existing escaped path segments are retained, and each experiment ID
is appended with each path segment escaped separately. No provider or branch is inferred.
Unknown settings, duplicate keys, non-mappings, and multiple documents are errors.

## Experiment folders and names

`experiments_dir` selects the folder relative to the project root. It defaults to
`experiments`. Use a nonblank relative path with `/` separators, such as
`research/experiments`; absolute paths, empty components, `.`, `..`, backslashes,
and symlinked directories are rejected. Changing this setting does not move
existing experiments. Move their folders yourself if you want to retain them in
the configured catalog.

`experiment_format` controls IDs for `new` and defaults to
`{date:20060102}-{slug}`. Add these placeholders wherever you want them:

| Placeholder | Value |
| --- | --- |
| `{date}` | Local date using the layout `2006-01-02`, for example `2026-10-06`. |
| `{time}` | Local time using the layout `15-04-05`, for example `14-23-45`. |
| `{slug}` | The positional argument to `new <slug>`, for example `baseline`. |
| `{date:format}` or `{time:format}` | Local date/time formatted with the supplied Go time layout. |

Custom formats use native Go `time.Time.Format` layouts, expressed with the
reference time: `2006` for the year, `01` for the month, `02` for the day,
`15` for the hour (24-hour clock), `04` for the minute, and `05` for the second.
See the [Go time layout documentation](https://pkg.go.dev/time#Time.Format)
for the full syntax. Unrecognized layout text remains literal. Only the text
inside date/time placeholders is interpreted as a layout. Literal text outside
placeholders stays unchanged, so
`baseline-2006-{slug}` retains the literal `2006`.

Placeholders can repeat or be omitted, allowing slug-only, time-only, and literal
IDs. Unknown placeholders, unmatched or malformed braces,
`{slug:format}`, and empty formats such as `{date:}` are rejected.

| `experiment_format` | Example ID for `new baseline` at 14:23:45 on October 6, 2026 |
| --- | --- |
| `{date:20060102}-{slug}` | `20261006-baseline` |
| `{date}/{slug}` | `2026-10-06/baseline` |
| `{date}/{time}-{slug}` | `2026-10-06/14-23-45-baseline` |
| `{date:2006-01-02}/{time:15:04}-{slug}` | `2026-10-06/14:23-baseline` |
| `{time:05:04:15}-{slug}` | `45:23:14-baseline` |
| `{date}/{time}` | `2026-10-06/14-23-45` |
| `baseline-2006-{slug}` | `baseline-2006-baseline` |

For example, customize both settings in the root `expledger.yaml`:

```yaml
experiments_dir: research/trials
experiment_format: "{date}/{time}-{slug}"
```

This creates `research/trials/2026-10-06/14-23-45-baseline/`. Its ID is
`2026-10-06/14-23-45-baseline`; use that full ID for `run`, `validate`, and
`--based-on`. Put its optional `report.yaml` beside `experiment.yaml`, at
`research/trials/2026-10-06/14-23-45-baseline/report.yaml`; a source such as
`results/metrics.csv` resolves inside that experiment folder. The
[report guide](reports.md) covers schema, logging, and live monitoring.
IDs use canonical relative paths with the same path restrictions
as `experiments_dir`. An existing path is an error; ExpLedger does not add a
counter to resolve collisions. Below the first ID component, `experiment.yaml`
is reserved for metadata and cannot name a directory. Changing the format affects
only new experiments.
Keep `remote_url` pointed at the published folder selected by `experiments_dir`.

## Sources for live browsing

`sources` applies only to `serve`. Omit it to browse the current project only.
An explicit list replaces that default; include `.` to retain the current project.
The list must contain at least one nonblank string. Paths name project-root
directories containing `expledger.yaml`; source lookup does not walk up to a parent.
Relative paths resolve from the current project's config directory after resolving
symlinks, even when invoked from a nested directory. Absolute paths are accepted. Neither `~` nor
environment variables are expanded by ExpLedger (quote CLI arguments to prevent
shell expansion).

Sources are read in order. The first copy of an experiment ID wins completely,
including its metadata, `based_on` relationships, and source's `remote_url`.
The catalog shows one entry and graph node per ID, sorted newest first; ties retain
source order and then directory-name order. Removing a winner reveals the next
copy on the next successful refresh. Each source uses its own `experiments_dir`.
Source configs supply remote URLs, but their
own source lists are never followed. Every selected source must be readable and
valid, including records shadowed by an earlier source. An error fails the whole
snapshot; there are no partial results. Existing per-project filesystem boundaries
apply independently to each root. Aggregation does not copy or synchronize files.

## Project discovery and initialization

Commands walk the current directory and its parents, selecting the nearest
`expledger.yaml`. An invalid or unreadable nearest config is an error; discovery
does not fall through to a parent. No marker means an error asking you to run
`expledger init`. Help works without a project. `init` always targets the current
directory, allowing nested project boundaries. It never overwrites an existing
config or experiment; a valid existing config is accepted unchanged unless an
explicit `--remote-url` conflicts, in which case edit the file yourself.

Project configuration is separate from each experiment's
[`experiment.yaml`](format.md). ExpLedger does not require Git or contact the
configured URL.

## Migrating older projects

To migrate, rename each old `experiments/<id>/expledger.yaml` to
`experiment.yaml` without changing its contents, then initialize the root.
An old experiment marker (`schema: expledger/v1`) encountered during upward
discovery produces a filename-migration error. There is no dual-name support.
The [running guide](running.md#metadata-and-reproducibility) explains how historical
`last_run` metadata is treated.
