# ExpLedger

## Install

Requires Go 1.26+.

```sh
git clone git@github.com:marcfranquesa/expledger.git
cd expledger
go install ./cmd/expledger
```

Make sure Go's binary installation directory is on your `PATH`.

## Commands

| Name | Description |
| --- | --- |
| `expledger new <slug> [flags]` | Create a dated experiment folder and README at the Git working tree root. Use `--title` for a custom title and repeat `--based-on` for parent experiment IDs. |
| `expledger list` | List experiment IDs and titles in descending ID order. |
| `expledger help [command]` | Show general or command-specific help. Also available with `--help` or `-h`, such as `expledger new --help`. |

## Usage

From anywhere inside a Git working tree:

```sh
expledger new my-idea
```

This creates `experiments/YYYYMMDD-my-idea/README.md` at that working tree's root and prints its path. The date uses your local time. Slugs contain lowercase letters, digits, and single hyphens between words. Existing experiments are never overwritten.

Set a display title and record parent experiments with flags:

```sh
expledger new my-idea --title "My experiment" --based-on 20260920-baseline
expledger new combined --based-on 20260920-baseline --based-on 20260921-alternative
expledger new --help
```

`--title` defaults to a title derived from the slug. Each `--based-on` takes one parent experiment ID; repeat the flag for multiple parents. Both flags require nonempty values.

Running `expledger` without arguments shows help. Help works outside a Git repository. Experiment commands resolve the Git working tree root before running and report an error outside a repository.

The README contains YAML `id`, `title`, and `created_at` fields, followed by Markdown sections for the hypothesis, method, and finding. The YAML `id` must exactly match the experiment folder name, including case. The ID and creation time stay fixed; the title and prose can be edited. An optional `based_on` list refers to parent experiment IDs:

```yaml
---
id: 20260924-my-idea
title: My idea
created_at: 2026-09-24T14:30:00.123456789Z
based_on:
  - 20260920-baseline
---
```

`created_at` records creation time as an RFC3339 timestamp in UTC, preserving fractional seconds. Older records may omit it; their creation time remains unknown and is not filled in when read or re-encoded. The date in the experiment ID still uses local time.

`based_on` describes experiment ancestry. When `--based-on` is supplied, the catalog is loaded and validated once, then reused to resolve every parent ID before any files are created. Missing or invalid records anywhere in the catalog block parent lookup. References are not checked for cycles. Independent experiments omit this field. There is no status field.

```sh
expledger list
```

This reads the README in each experiment directory and prints an ID/title table. A missing or empty `experiments/` directory reports "No experiments found." Missing or malformed experiment READMEs report an error with the file path. ID mismatches report the README path, YAML ID, and expected folder name without modifying files. Files and symlinked directories directly under `experiments/` are ignored; experiment artifacts are not scanned recursively.

## Agent skill

After installing the CLI, link the [skill](skills/expledger/SKILL.md) from a
persistent ExpLedger checkout. Run one of these from that checkout:

**Per repository** (replace `/path/to/project`):

```sh
mkdir -p "/path/to/project/.agents/skills"
ln -s "$PWD/skills/expledger" "/path/to/project/.agents/skills/expledger"
```

**Global** (all your repositories):

```sh
mkdir -p "$HOME/.agents/skills"
ln -s "$PWD/skills/expledger" "$HOME/.agents/skills/expledger"
```

## Development

```sh
go test ./...
go vet ./...
go run ./cmd/expledger --help
```

The executable entry point is in `cmd/expledger`. Tests live beside the code they exercise.

- `internal/cli`: command arguments, orchestration, and terminal output.
- `internal/catalog`: discovery and validation (`List`), lookup of loaded records (`Lookup`), and directory-existence checks (`Exists`) for overwrite protection.
- `internal/experiment`: individual records, IDs, creation, and YAML parsing.

Front matter contains one YAML mapping with string keys and explicit values; aliases and merge keys are unsupported. The parser preserves the Markdown body byte for byte and retains unknown metadata values. Re-encoding metadata normalizes YAML formatting, and YAML comments are not guaranteed to survive. The `new` command only creates files; it never rewrites an existing README.
