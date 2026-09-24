---
name: expledger
description: Use ExpLedger whenever running coding experiments, including hypothesis tests, benchmarks, and comparisons of implementations or configurations.
---

# ExpLedger

Before running a coding experiment, create a record in the target Git working tree:

```sh
expledger new descriptive-slug --title "Experiment title"
```

- Reuse the record when resuming the same experiment. For follow-ups, add
  `--based-on <parent-id>` using an existing record's YAML `id`; repeat for multiple parents.
- Edit the README in the directory printed by the command:
  - **Hypothesis:** the question and expected outcome, written before running.
  - **Method:** exact commands, inputs, configuration, and code revision or local changes.
  - **Finding:** observed results, failures or uncertainty, and the conclusion.
- Preserve the record's `id` and existing notes. Link supporting artifacts and
  include the README path in the final response.

Use `expledger new --help` for flags. ExpLedger creates records; run experiments
with the project's normal tools.
