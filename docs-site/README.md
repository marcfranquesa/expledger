# Building the documentation

The authoritative handwritten guides are in `../docs/`. Hugo mounts them directly;
do not copy them into this directory. CLI pages come from the real command tree
through [Cobra's documentation generator](https://pkg.go.dev/github.com/spf13/cobra/doc).
Edit command descriptions, examples, and flags in `../internal/cli/`, then rebuild.

Install Go 1.26+ and **Hugo extended 0.167.0** from the
[official release](https://github.com/gohugoio/hugo/releases/tag/v0.167.0).
The single Hugo version pin is `hugo-version`; the build script checks it and CI
installs it. Hugo Book is pinned to **v0.15.0** in this directory's `go.mod` and
verified by `go.sum`. No Node, Sass installation, or custom theme assets are needed.

From the repository root:

```sh
./scripts/docs.sh --minify --panicOnWarning
./scripts/docs.sh server --bind 127.0.0.1 --port 1313
```

Open the URL printed by Hugo (including `/expledger/`). The second command
regenerates the CLI reference once, then watches guides. Restart it after editing
Go command definitions. The first build requires network access for Go modules.
Output lives in ignored `docs-site/public/`; generated Markdown lives in ignored
`docs-site/generated/`. Neither belongs in Git.

The documentation module uses the local parent module through `replace`; generator
and theme dependencies do not enter the CLI's module or installed binary. Use
`hugo mod get github.com/alex-shpak/hugo-book@<version>` from this directory when
intentionally updating the theme. Plain `go mod tidy` removes the theme dependency
because it has no imported Go package. Conversely, `hugo mod tidy` removes the
Go generator's dependencies. If tidying Hugo dependencies, run this complete
sequence from `docs-site/` to restore them before the readonly build:

```sh
hugo mod tidy
go run -mod=mod ./generate
../scripts/docs.sh --minify --panicOnWarning
```

After changing the root module's Go dependencies, run `go run -mod=mod ./generate`
from `docs-site/` to refresh the generator's module requirements and checksums,
then review `go.mod`/`go.sum` and rebuild. Normal builds use `-mod=readonly` so a
missing dependency update fails in the Documentation check instead of changing
tracked files silently.

## GitHub Pages

Set repository **Settings → Pages → Build and deployment → Source** to
**GitHub Actions**. The site URL is <https://marcfranquesa.github.io/expledger/>;
`hugo.toml` includes the project base path. Ensure the `github-pages` environment
allows deployments from `main` (and retains any desired approval rules).

The Documentation workflow builds every PR with read-only repository permissions.
A push to `main` builds the same site, uploads its artifact, and deploys through a
separate job with Pages and OIDC permissions. PR builds do not upload or deploy a
Pages artifact. No `pull_request_target`, deployment branch, committed HTML, or
manual production build is needed.
