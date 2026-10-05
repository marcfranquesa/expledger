# ExpLedger

ExpLedger records coding experiments as folders in your project and runs
their `run.sh` scripts in place.

```text
experiments/20260924-baseline/
├── experiment.yaml
├── README.md
├── run.sh
└── ... supporting files
```

ExpLedger discovers only folders containing `experiment.yaml` with
`schema: expledger/v1`.

The [record format](docs/format.md) defines metadata, parent references, and
filesystem rules. The [running guide](docs/running.md) covers entrypoints and
execution behavior.

ExpLedger is in active development. Expect breaking changes.

## Commands

| Name | Description |
| --- | --- |
| `expledger init [--remote-url <url>]` | Initialize the current directory as a project. |
| `expledger new <slug> [flags]` | Create a dated folder with metadata, notes, and an executable `run.sh` stub. |
| `expledger run <id>` | Execute the experiment's `run.sh` in place without changing metadata. |
| `expledger list` | List experiment IDs and titles, newest first. |
| `expledger validate <id>` | Validate an experiment's `experiment.yaml` and check that its ID matches the folder name. |
| `expledger build [--output <directory>]` | Generate a static snapshot in `dist/index.html` by default. |
| `expledger serve [--port <port>] [--source <root> ...]` | Browse live experiments across configured projects. |
| `expledger help [command]` | Show help and available flags. |

Initialize once from your project directory:

```sh
expledger init
expledger new baseline
```

Commands find the nearest `expledger.yaml` in the current directory or its
parents. There is no Git fallback or implicit initialization. A local-only
project has a root config containing `{}`. To enable View remote links, initialize
with the full browser URL of the published **experiments directory**:

```sh
expledger init --remote-url https://github.com/owner/repo/tree/main/experiments
```

For an existing project, edit `remote_url` in the root `expledger.yaml` explicitly.
`build` and `serve` append an escaped experiment ID to that URL. Any HTTP(S)
host is supported; queries, fragments, and credentials are rejected. An absent
or empty URL means no remote links. ExpLedger does not infer a branch or contact
the remote.

## Migration

Rename each existing `experiments/<id>/expledger.yaml` to
`experiments/<id>/experiment.yaml`, leaving its contents unchanged, then run
`expledger init` at the project root. Migration is manual; the old experiment
filename is not supported. An old experiment marker encountered during upward
root discovery produces a filename-migration error.

ExpLedger no longer records commits, dirty state, or `last_run`. Historical
`last_run` values remain accepted as custom metadata and are never updated;
they are not current run provenance.

## Experiment graph

Choose **Graph** in either `build` or `serve` to explore `based_on` lineage.
Descendants appear above ancestors, with arrows pointing upward. Independent
lineages favor their newest terminal descendant, so a late-imported ancestor
does not promote an older branch. Cycles are treated as a unit. Wide rows wrap;
shared routes reduce edge clutter. Missing parents and cycles use dashed styling.

Hover or focus previews direct connections; clicking pins selection and opens
full metadata, remote links, and navigable parents/children. Click the selected
node again, empty space, or Escape to clear it. Search by title or ID. Scroll or
drag to pan; Ctrl/Command + wheel zooms around the pointer. The graph starts at
75%; +/− and 100% adjust zoom without switching layouts. Double-click centers a
node at 100%; Enter or Space selects and centers a focused node.

The page is self-contained and works offline. The list remains the default and
works without JavaScript. Dependency structure takes precedence over date order;
parent IDs in the details panel provide a text alternative to bundled arrows.

Inspired by [Lab Exp's experiment graph](https://github.com/rsoatto/lab-exp),
with an independent implementation using ExpLedger's metadata and renderer.

## Installation

Building requires Go 1.26+; the clone command below uses Git. The installed CLI
has no Git dependency.

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

For multi-project browsing, set `sources: [., ../experiment-worktree]` in
`expledger.yaml`, or repeat `serve --source`. Paths resolve from that config's
directory. The first source wins duplicate IDs; the browser refreshes every few
seconds while retaining the selected view and graph scroll position. See
[browsing multiple projects](docs/running.md#browsing-multiple-projects) for errors,
refresh behavior, and CLI overrides. Other commands remain current-project only.
