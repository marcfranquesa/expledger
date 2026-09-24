package cli

const rootDescription = "Manage experiment records in a Git repository"

const newDescription = "Create an experiment folder and README"

const newDetails = `Create experiments/YYYYMMDD-<slug>/README.md at the Git working tree root.
The date uses your local time. Slugs contain lowercase letters, digits, and
single hyphens between words. Existing experiments are never overwritten.
Parent IDs are recorded in based_on; their existence is not checked.`

const newExample = `  expledger new my-idea
  expledger new my-idea --title "My experiment" --based-on 20260920-baseline
  expledger new combined --based-on 20260920-baseline --based-on 20260921-alternative`
