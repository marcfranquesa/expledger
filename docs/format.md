---
title: Experiment format
weight: 40
---

# Experiment format

Each experiment has `<experiments_dir>/<id>/experiment.yaml` under the initialized
project root. The directory defaults to `experiments`; see
[Configuration](configuration.md#experiment-folders-and-names) to customize it
and the creation format. This file is the sole source of
ExpLedger metadata. `README.md` holds independent notes; other files can hold
scripts, artifacts, or another tool's
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
| `id` | Required nonblank string, exactly matching the experiment's path relative to `experiments_dir`, including case and whitespace. Use `/` separators for nested paths. |
| `title` | Required nonblank string. It does not determine the ID or folder name. |
| `created_at` | Required nonzero RFC3339 timestamp with a timezone, such as `2026-09-24T12:00:00Z` or `2026-09-24T08:00:00-04:00`. Quoted timestamps are accepted. |
| `based_on` | Optional list of nonblank strings naming direct parent experiments. |
| Other keys | Custom metadata with string keys; accepted but not used or changed by ExpLedger. |

Timestamp precision is nanoseconds. Additional fractional-second digits are
truncated when read. Re-encoding preserves the represented instant and supported
timezone offset; offsets must be whole minutes and within RFC3339's range.
The zero timestamp `0001-01-01T00:00:00Z` is rejected. YAML aliases and merge keys
are unsupported, including within custom metadata.

## IDs and creation

By default, `new <slug>` generates `YYYYMMDD-<slug>` using the local date. Configure
`experiment_format` with `{date}`, `{time}`, and `{slug}` placeholders to use
other dates, timestamps, or nested paths, such as `{date}/{time}-{slug}`. Slugs use lowercase
ASCII letters, digits, and single hyphens between words. New records store
`created_at` in UTC, so its UTC date can differ from the date in the ID.

Existing IDs need not follow the generated date/slug convention. Spaces and
Unicode are supported. An ID must be a canonical nonblank relative path:
absolute paths, empty components, `.`, `..`, backslashes, and NUL bytes are
rejected. `/` separates path components. The YAML ID must equal the actual
relative path, even on a case-insensitive filesystem.

Creation writes a plain Markdown notes template, the YAML metadata file, and an
executable `run.sh` stub that exits with an error until replaced with a workload.
It never overwrites an existing experiment path. An occupied directory,
file, or symlink is an error. Invalid metadata and parent references are rejected
before creating the child. An omitted title is derived from the slug;
`new baseline-model` uses `Baseline model`.

## Parent references and validation

`new --based-on <id>` requires each named parent to have valid `experiment.yaml`
metadata and a matching relative path ID. Repeat the flag to record multiple parents.
Only named direct parents are validated; unrelated experiments and their ancestors
are not read. Parent order is retained.

Parsing checks the document's metadata. Catalog reads and `validate <id>` also
check its folder identity. They do not resolve its `based_on` references. A readable
record may therefore refer to an absent ancestor. These operations do not rewrite
the record or automatically repair its metadata.

## Discovery and preservation

`list` recursively reads experiment directories containing `experiment.yaml` and
sorts by `created_at`, newest first. Equal timestamps retain directory-name order.
A missing configured experiments directory is an empty catalog. Directories
without `experiment.yaml` are explored as grouping folders, regardless of their
README contents. Discovery stops at an experiment folder, so nested artifacts
are not read as experiments. Loose files and symlinked child directories are
ignored. A present but
unreadable or invalid metadata file fails the operation with no partial list;
a dangling metadata symlink is an error. `build` and `serve`
use the same catalog rules.

The configured experiments directory and every directory component of a requested
ID must be real directories. Metadata must resolve to a regular file. Relative metadata
symlinks that stay within the project root are readable; absolute links and paths
that escape the root are rejected.
Running requires `experiment.yaml` and executable `run.sh` to be regular files,
not symlinks. Supporting files are not scanned by the runner.

The internal record codec retains only standard fields. Catalog reads and runs
do not modify metadata, including custom fields and historical `last_run` values.

Records remain ordinary files; no separate index or database is required. See
[Running experiments](running.md) for the execution contract.
