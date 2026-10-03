# ExpLedger

ExpLedger records coding experiments as folders in your project and runs their
`run.sh` scripts in place. Each experiment keeps structured metadata in
`experiment.yaml` and independent notes in `README.md`. The installed CLI has no
Git dependency.

**[Documentation](https://marcfranquesa.github.io/expledger/)** — installation,
getting started, configuration, the experiment format, and generated CLI reference.

ExpLedger is in active development. Expect breaking changes. See
[installation](docs/install.md) and the [version and release policy](docs/releases.md).
The planned first release is v0.1.0.

## Development

Requires Go 1.26 or later:

```sh
go test ./...
go vet ./...
go run ./cmd/expledger --help
go install ./cmd/expledger
```

Use `./scripts/playground.sh` to open a shell in an isolated fixture project;
`exit` returns to your previous shell. The project remains in `/tmp` until you
remove it. Use `./scripts/preview.sh --port 0` to browse the fixtures directly.
Fixture remote links are placeholders.

See [the documentation build instructions](docs-site/README.md) to preview or
build the documentation site locally.
