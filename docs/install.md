# Installation

The planned first release is **v0.1.0**. It has not been published yet; the
release installation examples below become available when it is published.

## Release archives

Download the archive for your machine and `checksums.txt` from the same
[GitHub Release](https://github.com/marcfranquesa/expledger/releases).
Archives support macOS (`darwin`) and Linux (`linux`), each on `arm64` and
`amd64`. Apple Silicon uses `darwin_arm64`; Intel Macs use `darwin_amd64`.

For example, on Apple Silicon, after downloading both files:

```sh
shasum -a 256 --ignore-missing -c checksums.txt
tar -xzf expledger_0.1.0_darwin_arm64.tar.gz
mkdir -p "$HOME/.local/bin"
install -m 755 expledger "$HOME/.local/bin/expledger"
```

Confirm the downloaded archive is reported `OK` before extracting it. Each
archive contains `expledger` and the MIT `LICENSE`. Add `$HOME/.local/bin` to
your `PATH` if needed. Archives are not Apple-notarized.

## Install an exact version with Go

With Go meeting the requirement in `go.mod` (currently Go 1.26 or newer):

```sh
go install github.com/marcfranquesa/expledger/cmd/expledger@v0.1.0
```

Go installs the executable into `GOBIN`, or `$GOPATH/bin` (normally
`$HOME/go/bin`) when `GOBIN` is unset. Add that directory to your `PATH`.
The [Go installation reference](https://go.dev/ref/mod#go-install) describes
version-qualified installation, which works outside a project.

```sh
expledger --version
# expledger version v0.1.0
```

`--version` needs neither an initialized project nor Git. Release archives
embed their tag; version-qualified Go installs use embedded module metadata.
Builds without version metadata report `expledger version devel`. Untagged or
dirty checkouts are identified as development builds. Installs of untagged revisions
report `devel` with their Go pseudo-version. No runtime release tool is needed.

## Build the current source

From a checkout:

```sh
go build -o /tmp/expledger ./cmd/expledger
/tmp/expledger --version
```

This is a development build. See the [release policy](releases.md) for version
and compatibility rules, and the [running guide](running.md) for project use.
