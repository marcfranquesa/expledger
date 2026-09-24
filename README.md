# ExpLedger

ExpLedger records coding experiments as folders in your Git repository and runs
their `run.sh` scripts in place.

```text
experiments/20260924-baseline/
├── expledger.yaml
├── README.md
├── run.sh
└── ... supporting files
```

ExpLedger discovers only folders containing `expledger.yaml` with
`schema: expledger/v1`.

The [record format](docs/format.md) defines metadata, parent references, and
filesystem rules. The [running guide](docs/running.md) covers entrypoints and
recorded project provenance.

ExpLedger is in active development. Expect breaking changes.

## Commands

| Name | Description |
| --- | --- |
| `expledger new <slug> [flags]` | Create a dated folder with metadata, notes, and an executable `run.sh` stub. |
| `expledger run <id>` | Execute the experiment's `run.sh` and record the current project commit and dirty state. |
| `expledger list` | List experiment IDs and titles, newest first. |
| `expledger validate <id>` | Validate an experiment's `expledger.yaml` and check that its ID matches the folder name. |
| `expledger build [--output <directory>]` | Generate a static snapshot in `dist/index.html` by default. |
| `expledger serve [--port <port>]` | Browse experiments locally with links to their GitHub folders. |
| `expledger help [command]` | Show help and available flags. |

`build` and `serve` work in local-only repositories and detached worktrees.
Remote links appear when `origin` points to GitHub and a branch is checked out.

## Installation

Requires Go 1.26+ and Git.

```sh
git clone git@github.com:marcfranquesa/expledger.git
cd expledger
go install ./cmd/expledger
```

Make sure Go's binary installation directory is on your `PATH`.

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

Open a shell in a temporary fixture project to try the installed CLI:

```sh
./scripts/playground.sh
```

Run `expledger list` or other commands there; `exit` returns to your previous shell.
The project stays in `/tmp` until you remove it. Its remote links are placeholders.

Preview the fixture experiments (remote links are placeholders):

```sh
./scripts/preview.sh --port 0
```
