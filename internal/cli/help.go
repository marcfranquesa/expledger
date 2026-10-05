package cli

const rootDescription = "Manage experiment records in an initialized project"

const newDescription = "Create an experiment folder, metadata, README, and run.sh"

const newDetails = `Create experiment.yaml, README.md, and executable run.sh in experiments/YYYYMMDD-<slug>/ at the
project root.
The date uses your local time. Slugs contain lowercase letters, digits, and
single hyphens between words. Existing experiments are never overwritten.
Validate only the direct parents named by --based-on. Each must have a valid
experiment.yaml whose id exactly matches its experiment folder name.`

const runDescription = "Run an experiment's run.sh in its existing folder"

const runDetails = `Execute ./run.sh from experiments/<id>/ with the current checkout and environment.
Keep workload parameters in run.sh or its input files; extra arguments are rejected.
The script chooses output paths. Metadata is not changed and no provenance is
recorded. Git is not required.
The shebang selects the runtime. Standard streams and exit status are forwarded.
Ctrl+C stops the workload and its ordinary child processes. Requires macOS or
Linux; supports batch workloads, not interactive terminal input or daemons.`

const newExample = `  expledger new my-idea
  expledger new my-idea --title "My experiment" --based-on 20260920-baseline
  expledger new combined --based-on 20260920-baseline --based-on 20260921-alternative`

const listDescription = "List experiment IDs and titles"

const listDetails = `Read experiments/*/experiment.yaml at the project root.
Display IDs and titles by created_at, newest first.
Each YAML id must exactly match its experiment folder name.
IDs containing whitespace, quotes, or non-printable characters are double-quoted,
using Go-style escapes such as \t and \n.
Title whitespace is collapsed to spaces; other non-printable characters are escaped.
Ignore directories without experiment.yaml. Report invalid metadata with its path.`

const validateDescription = "Validate experiment metadata"

const validateDetails = `Check experiments/<id>/experiment.yaml at the project root.
Require schema: expledger/v1 and valid id, title, and created_at fields.
The YAML id must match the experiment folder name. README.md is independent.
Report the first error with its file path; no files are changed.`

const buildDescription = "Build a static experiment web page"

const buildDetails = `Write a self-contained index.html snapshot, newest experiments first.
Create the output directory if needed and replace its index.html on each build.
Other files in the directory are left unchanged. The root expledger.yaml's optional
remote_url supplies the full browser URL of the remote experiments directory.
An escaped experiment ID is appended; omit remote_url for a local-only catalog.
Only the current project is included; sources applies only to serve.
The generated page needs no running ExpLedger server and does not poll.`

const serveDescription = "Browse experiments in a local web page"

const serveDetails = `Serve experiments on 127.0.0.1 with live list and lineage views.
Poll configuration and records every few seconds without reloading the page.
Use sources in expledger.yaml, or repeat --source to replace that list.
Paths name project roots; relative paths resolve from the current project config
directory. No ~ or environment-variable expansion is performed.
The first source wins each ID, including its relationships and remote links.
Other commands remain scoped to the current project. Failed refreshes keep the
last valid view until recovery. Press Ctrl+C to stop the server.`
