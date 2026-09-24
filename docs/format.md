# Experiment record format

Each experiment has `experiments/<id>/expledger.yaml` under the Git working
tree root. This file is the sole source of ExpLedger metadata. `README.md` holds
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
| `last_run` | Optional mapping with `project_commit`, `project_dirty`, and `started_at`, recording a launched entrypoint. See below. |
| Other keys | Custom metadata with string keys; accepted but not used by ExpLedger, and preserved when recording a run. |

Timestamp precision is nanoseconds. Additional fractional-second digits are
truncated when read. Re-encoding preserves the represented instant and supported
timezone offset; offsets must be whole minutes and within RFC3339's range.
The zero timestamp `0001-01-01T00:00:00Z` is rejected. YAML aliases and merge keys
are unsupported, including within custom metadata.

## Run receipt

`last_run.project_commit` is the checkout's Git `HEAD` observed before launch,
stored as 40 or 64 lowercase hexadecimal characters. `project_dirty` is a boolean indicating
Git-visible staged, unstaged, or untracked changes outside `experiments/` at
launch; ignored files are excluded. `started_at` is a nonzero RFC3339 timestamp
with a timezone, using the same rules as `created_at`.

`project_commit` and `started_at` are required. Older receipts may omit
`project_dirty`, which is read as `false`; new receipts always write it. Null
values and additional receipt fields are rejected.

```yaml
last_run:
  project_commit: 0123456789abcdef0123456789abcdef01234567
  project_dirty: false
  started_at: 2026-09-24T14:30:00Z
```

Each `run` replaces this receipt when the entrypoint actually launches, including runs
that later fail or are interrupted. A failure before launch leaves the previous
receipt unchanged. The receipt records no outcome or run history; it does not
assert that the experiment succeeded. Existing receipts never affect execution:
each run uses the current checkout and records its current `HEAD`. Experiment-only
commits can advance this hash without changing project code. An experiment without
a receipt remains valid. See [running experiments](running.md) for execution
semantics and reproducibility limits.

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

`new --based-on <id>` requires each named parent to have valid `expledger.yaml`
metadata and a matching directory ID. Repeat the flag to record multiple parents. Only named
direct parents are validated; unrelated experiments and their ancestors are not
read. Parent order is retained.

Parsing checks the document's metadata. Catalog reads and `validate <id>` also
check its folder identity. They do not resolve its `based_on` references. A readable
record may therefore refer to an absent ancestor. These operations do not rewrite
the record or automatically repair its metadata.

## Discovery and preservation

`list` reads immediate experiment directories containing `expledger.yaml` and
sorts by `created_at`, newest first. Equal timestamps retain directory-name order.
A missing `experiments` directory is an empty catalog. Loose files, nested artifact directories, and
symlinked child directories are not discovered as experiments. Directories without
`expledger.yaml` are ignored, regardless of their README contents. A present but unreadable or invalid metadata file fails the operation
with no partial list; a dangling metadata symlink is an error. `build` and `serve`
use the same catalog rules.

The `experiments` directory and directly requested experiment directories must
be real directories. Metadata must resolve to a regular file. Relative metadata
symlinks that stay within the project root are readable; absolute links and paths
that escape the root are rejected.
Running requires `expledger.yaml` and executable `run.sh` to be regular files,
not symlinks. Supporting files are not scanned by the runner.

The internal record codec retains only the standard fields listed above. The
run receipt writer updates `last_run` in the original YAML document, preserving
unrelated custom values and types. Re-encoding may change YAML formatting; exact
original layout is not guaranteed. Catalog reads do not modify metadata.

The internal packages keep these boundaries: `experiment` owns the record and
codec; `catalog` owns reading, discovery, creation, and receipt updates; `web` owns
rendering and HTTP handling; `cli` owns command orchestration and Git context.
Records remain ordinary files; no separate index or database is required.
