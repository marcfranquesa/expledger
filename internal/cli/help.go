package cli

const rootDescription = "Manage experiment records in a Git repository"

const newDescription = "Create an experiment folder and README"

const newDetails = `Create experiments/YYYYMMDD-<slug>/README.md at the Git working tree root.
The date uses your local time. Slugs contain lowercase letters, digits, and
single hyphens between words. Existing experiments are never overwritten.
Validate only the direct parents named by --based-on. Each must have a valid
README whose YAML id exactly matches its experiment folder name.`

const newExample = `  expledger new my-idea
  expledger new my-idea --title "My experiment" --based-on 20260920-baseline
  expledger new combined --based-on 20260920-baseline --based-on 20260921-alternative`

const listDescription = "List experiment IDs and titles"

const listDetails = `Read experiments/*/README.md at the Git working tree root.
Display IDs and titles by created_at, newest first.
Each README's YAML id must exactly match its experiment folder name.
Report missing or invalid experiment READMEs with their file path.`

const validateDescription = "Validate an experiment README"

const validateDetails = `Check experiments/<id>/README.md at the Git working tree root.
Validate YAML front matter, including required id, title, and created_at fields.
The YAML id must match the experiment folder name. Markdown is unrestricted.
Report the first error with its file path; no files are changed.`

const buildDescription = "Build a static experiment web page"

const buildDetails = `Write a self-contained index.html snapshot, newest experiments first.
Create the output directory if needed and replace its index.html on each build.
Other files in the directory are left unchanged. GitHub links use the origin
remote and current branch; they assume the experiment folders are published.
The generated page needs no Git checkout or running ExpLedger server.`

const serveDescription = "Browse experiments in a local web page"

const serveDetails = `Serve experiment titles and creation times, newest first, on 127.0.0.1.
Refresh the page to reread experiment metadata. GitHub links use the origin
remote and branch checked out when the server starts; they assume the experiment
folders are already published. Press Ctrl+C to stop the server.`
