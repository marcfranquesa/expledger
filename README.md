# ExpLedger

ExpLedger records coding experiments as folders with YAML metadata and Markdown notes in your Git repository.

## Commands

| Name | Description |
| --- | --- |
| `expledger new <slug> [flags]` | Create a dated experiment folder and README. |
| `expledger list` | List experiment IDs and titles, newest first. |
| `expledger build [--output <directory>]` | Generate a static snapshot in `dist/index.html` by default. |
| `expledger serve [--port <port>]` | Browse experiments locally with links to their GitHub folders. |
| `expledger help [command]` | Show help and available flags. |

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

Preview the fixture experiments (remote links are placeholders):

```sh
./scripts/preview.sh --port 0
```
