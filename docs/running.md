# Running experiments

`expledger new` creates an executable `experiments/<id>/run.sh` stub that exits
with configuration guidance. Replace it with the experiment's commands, using a
shebang and fixed parameters. For example, with `experiment.py` beside it:

```sh
#!/bin/sh
exec uv run --locked python experiment.py --seed 42
```

Initialize the project with `expledger init`, then prepare the code and
dependencies you want. Git is not required. Run:

```sh
expledger run <id>
```

ExpLedger executes `./run.sh` directly from the existing experiment directory.
The script inherits your environment and writes results wherever it chooses.
Keep parameters in the script or nearby configuration; additional arguments and
revision-selection options are rejected. Both `run.sh` and `experiment.yaml` must
be regular files, and `run.sh` must be executable. Supporting files are left to
the script.

On macOS and Linux, standard input from a pipe or file, both output streams, and
the workload's exit status are forwarded. Ctrl-C stops the workload and its
ordinary children. Use batch workloads; interactive terminal input, job control,
and daemons are unsupported. Children must keep the workload's process group and
user identity so ExpLedger can stop them.

## Metadata and reproducibility

Running does not change metadata or record automatic provenance. Existing
historical `last_run` fields are left byte-for-byte unchanged and do not describe
new runs. There is no automatic commit, dirty-state, timestamp, or outcome record.

To reproduce a result, prepare the desired code yourself and retain experiment
inputs, dependency lockfiles, datasets, and environment details in your notes.
The runner does not create checkouts, copy files, or relocate outputs.
