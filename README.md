# ExpLedger

## Install

Requires Go 1.26+ and access to this private repository.

```sh
git clone git@github.com:marcfranquesa/expledger.git
cd expledger
go install ./cmd/expledger
```

Make sure Go's binary installation directory is on your `PATH`.

## Commands

| Name | Description |
| --- | --- |
| `expledger new <slug>` | Create a dated experiment folder and README at the Git working tree root. |
| `expledger help` | Show usage. Also available as `expledger --help` or `expledger -h`. |

## Usage

From anywhere inside a Git working tree:

```sh
expledger new my-idea
```

This creates `experiments/YYYYMMDD-my-idea/README.md` at that working tree's root and prints its path. The date uses your local time. Slugs contain lowercase letters, digits, and single hyphens between words. Existing experiments are never overwritten.

The README contains YAML `id` and `title` fields, followed by Markdown sections for the hypothesis, method, and finding. The ID stays fixed; the title and prose can be edited. An optional `based_on` list refers to parent experiment IDs:

```yaml
---
id: 20260924-my-idea
title: My idea
based_on:
  - 20260920-baseline
---
```

`based_on` describes experiment ancestry. References are stored but are not yet resolved or checked for cycles. Independent experiments omit this field. There is no status field.

## Development

```sh
go test ./...
go vet ./...
go run ./cmd/expledger --help
```

The executable entry point is in `cmd/expledger`. `internal/cli` handles commands and Git root discovery; `internal/experiment` handles directory creation and README encoding and parsing. Tests live beside the code they exercise.

Front matter contains one YAML mapping with explicit values; aliases and merge keys are unsupported. The parser preserves the Markdown body byte for byte and retains unknown metadata values. Re-encoding metadata normalizes YAML formatting, and YAML comments are not guaranteed to survive. The `new` command only creates files; it never rewrites an existing README.
