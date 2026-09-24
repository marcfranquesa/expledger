package cli

const rootDescription = "Manage experiment records in a Git repository"

const newDescription = "Create an experiment folder and README"

const newDetails = `Create experiments/YYYYMMDD-<slug>/README.md at the Git working tree root.
The date uses your local time. Slugs contain lowercase letters, digits, and
single hyphens between words. Existing experiments are never overwritten.
Parent IDs passed with --based-on must exist in the validated catalog.
Each README's YAML id must exactly match its experiment folder name.`

const newExample = `  expledger new my-idea
  expledger new my-idea --title "My experiment" --based-on 20260920-baseline
  expledger new combined --based-on 20260920-baseline --based-on 20260921-alternative`

const listDescription = "List experiment IDs and titles"

const listDetails = `Read experiments/*/README.md at the Git working tree root.
Display IDs and titles in descending ID order (newest first for generated IDs).
Each README's YAML id must exactly match its experiment folder name.
Report missing or invalid experiment READMEs with their file path.`
