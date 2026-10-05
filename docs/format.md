# Experiment record format

Each experiment has `experiments/<id>/experiment.yaml` under the initialized project
root. This file is the sole source of ExpLedger metadata. `README.md` holds
independent notes; other files can hold scripts, artifacts, or another tool's
metadata. `new` creates a README and an executable `run.sh` stub; neither file is
required for catalog reads.

```yaml
schema: expledger/v1
id: 20260924-baseline
title: Baseline model
created_at: 2026-09-24T12:00:00Z
based_on:
  - 20260920-reference
seed: 123
```

The metadata file contains exactly one YAML mapping. Standard YAML document
markers and LF or CRLF line endings are accepted. README contents, including
another tool's front matter, are never parsed or updated by ExpLedger.

| Field | Contract |
| --- | --- |
| `schema` | Required string, exactly `expledger/v1`. Missing, foreign, and unsupported versions are errors. |
| `id` | Required nonblank string, exactly matching the experiment folder's name, including case and whitespace. |
| `title` | Required nonblank string. It does not determine the ID or folder name. |
| `created_at` | Required nonzero RFC3339 timestamp with a timezone, such as `2026-09-24T12:00:00Z` or `2026-09-24T08:00:00-04:00`. Quoted timestamps are accepted. |
| `based_on` | Optional list of nonblank strings naming direct parent experiments. |
| Other keys | Custom metadata with string keys; accepted but not used or changed by ExpLedger. |

Timestamp precision is nanoseconds. Additional fractional-second digits are
truncated when read. Re-encoding preserves the represented instant and supported
timezone offset; offsets must be whole minutes and within RFC3339's range.
The zero timestamp `0001-01-01T00:00:00Z` is rejected. YAML aliases and merge keys
are unsupported, including within custom metadata.

## Project configuration

The project root contains `expledger.yaml`, exactly one YAML mapping with optional settings:

```yaml
remote_url: https://github.com/owner/repo/tree/main/experiments
sources:
  - .
  - ../experiment-worktree
  - /Users/me/projects/another-worktree
```

Use `{}` for a local-only project. `remote_url` must be a string; an empty string
also disables links. A configured value is the full HTTP(S) browser URL of the
remote experiments directory, with an ASCII hostname or IP address and optional
port. Credentials, queries, and fragments are rejected. A trailing slash is
optional. Existing escaped path segments are retained, and each experiment ID
is appended as an escaped path segment. No provider or branch is inferred.
Unknown settings, duplicate keys, non-mappings, and multiple documents are errors.

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
copy on the next successful refresh. Source configs supply remote URLs, but their
own source lists are never followed. Every selected source must be readable and
valid, including records shadowed by an earlier source. An error fails the whole
snapshot; there are no partial results. Existing per-project filesystem boundaries
apply independently to each root. Aggregation does not copy or synchronize files.

Commands walk the current directory and its parents, selecting the nearest
`expledger.yaml`. An invalid or unreadable nearest config is an error; discovery
does not fall through to a parent. No marker means an error asking you to run
`expledger init`. Help works without a project. `init` always targets the current
directory, allowing nested project boundaries. It never overwrites an existing
config or experiment; a valid existing config is accepted unchanged unless an
explicit `--remote-url` conflicts, in which case edit the file yourself.

To migrate, rename each old `experiments/<id>/expledger.yaml` to
`experiment.yaml` without changing its contents, then initialize the root.
An old experiment marker (`schema: expledger/v1`) encountered during upward
discovery produces a filename-migration error. There is no dual-name support.
Historical `last_run` values are ordinary custom metadata, not current provenance;
ExpLedger neither validates them as receipts nor updates them.

## IDs and creation

`new <slug>` generates `YYYYMMDD-<slug>` using the local date. Slugs use lowercase
ASCII letters, digits, and single hyphens between words. New records store
`created_at` in UTC, so its UTC date can differ from the date in the ID.

Existing IDs need not follow the generated date/slug convention. Spaces and
Unicode are supported. An ID must be one nonblank directory name: `.`, `..`,
slashes, backslashes, and NUL bytes are rejected. The YAML ID must equal the
actual directory entry, even on a case-insensitive filesystem.

Creation writes a plain Markdown notes template, the YAML metadata file, and an
executable `run.sh` stub that exits with an error until replaced with a workload.
It never overwrites an existing experiment path. An occupied directory,
file, or symlink is an error. Invalid metadata and parent references are rejected
before creating the child. An omitted title is derived from the slug;
`new baseline-model` uses `Baseline model`.

## Parent references and validation

`new --based-on <id>` requires each named parent to have valid `experiment.yaml`
metadata and a matching directory ID. Repeat the flag to record multiple parents. Only named
direct parents are validated; unrelated experiments and their ancestors are not
read. Parent order is retained.

Parsing checks the document's metadata. Catalog reads and `validate <id>` also
check its folder identity. They do not resolve its `based_on` references. A readable
record may therefore refer to an absent ancestor. These operations do not rewrite
the record or automatically repair its metadata.

## Discovery and preservation

`list` reads immediate experiment directories containing `experiment.yaml` and
sorts by `created_at`, newest first. Equal timestamps retain directory-name order.
A missing `experiments` directory is an empty catalog. Loose files, nested artifact directories, and
symlinked child directories are not discovered as experiments. Directories without
`experiment.yaml` are ignored, regardless of their README contents. A present but unreadable or invalid metadata file fails the operation
with no partial list; a dangling metadata symlink is an error. `build` and `serve`
use the same catalog rules.

The `experiments` directory and directly requested experiment directories must
be real directories. Metadata must resolve to a regular file. Relative metadata
symlinks that stay within the project root are readable; absolute links and paths
that escape the root are rejected.
Running requires `experiment.yaml` and executable `run.sh` to be regular files,
not symlinks. Supporting files are not scanned by the runner.

The internal record codec retains only standard fields. Catalog reads and runs
do not modify metadata, including custom fields and historical `last_run` values.

The internal packages keep these boundaries: `experiment` owns the record and
codec; `catalog` owns reading, discovery, and creation; `web` owns rendering and
HTTP handling; `cli` owns project configuration and command orchestration.
Records remain ordinary files; no separate index or database is required.
