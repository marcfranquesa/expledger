package cli

const rootDescription = "Manage experiment records in a Git repository"

const newDescription = "Create an experiment folder, metadata, and README"

const newDetails = `Create expledger.yaml and README.md in experiments/YYYYMMDD-<slug>/ at the
Git working tree root.
The date uses your local time. Slugs contain lowercase letters, digits, and
single hyphens between words. Existing experiments are never overwritten.
Validate only the direct parents named by --based-on. Each must have a valid
expledger.yaml whose id exactly matches its experiment folder name.`

const newExample = `  expledger new my-idea
  expledger new my-idea --title "My experiment" --based-on 20260920-baseline
  expledger new combined --based-on 20260920-baseline --based-on 20260921-alternative`

const listDescription = "List experiment IDs and titles"

const listDetails = `Read experiments/*/expledger.yaml at the Git working tree root.
Display IDs and titles by created_at, newest first.
Each YAML id must exactly match its experiment folder name.
IDs containing whitespace, quotes, or non-printable characters are double-quoted,
using Go-style escapes such as \t and \n.
Title whitespace is collapsed to spaces; other non-printable characters are escaped.
Ignore directories without expledger.yaml. Report invalid metadata with its path.`

const validateDescription = "Validate experiment metadata"

const validateDetails = `Check experiments/<id>/expledger.yaml at the Git working tree root.
Require schema: expledger/v1 and valid id, title, and created_at fields.
The YAML id must match the experiment folder name. README.md is independent.
Report the first error with its file path; no files are changed.`

const buildDescription = "Build a static experiment web page"

const buildDetails = `Write a self-contained index.html snapshot, newest experiments first.
Create the output directory if needed and replace its index.html on each build.
Other files in the directory are left unchanged. GitHub links are included when
origin points to GitHub and a branch is checked out; they assume the experiment
folders are published. Otherwise the catalog is shown without remote links.
The generated page needs no Git checkout or running ExpLedger server.`

const serveDescription = "Browse experiments in a local web page"

const serveDetails = `Serve experiment titles and creation times, newest first, on 127.0.0.1.
Refresh the page to reread experiment metadata. GitHub links are included when
origin points to GitHub and a branch is checked out when the server starts;
they assume the experiment folders are published. Otherwise the catalog is shown
without remote links. Press Ctrl+C to stop the server.`
