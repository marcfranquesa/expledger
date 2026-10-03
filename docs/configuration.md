---
title: Configuration
weight: 20
---

# Configuration

The initialized project root contains `expledger.yaml`, exactly one YAML mapping
with optional settings:

```yaml
remote_url: https://github.com/owner/repo/tree/main/experiments
sources:
  - .
  - ../experiment-worktree
  - /Users/me/projects/another-worktree
```

Set this when initializing a project:

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
is appended as an escaped path segment. No provider or branch is inferred.
Unknown settings, duplicate keys, non-mappings, and multiple documents are errors.

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
copy on the next successful refresh. Source configs supply remote URLs, but their
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
