#!/bin/bash
set -euo pipefail

experiment_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
project_dir="$(CDPATH= cd -- "$experiment_dir/../.." && pwd)"
scratch_dir="$(mktemp -d "${TMPDIR:-/tmp}/expledger-monitor-bench.XXXXXX")"
trap 'rm -rf "$scratch_dir"' EXIT
export GOCACHE="${GOCACHE:-$scratch_dir/go-cache}"
artifacts_dir="$experiment_dir/artifacts"
export EXPLEDGER_MONITOR_ARTIFACTS="${EXPLEDGER_MONITOR_ARTIFACTS:-$scratch_dir/html}"
mkdir -p "$artifacts_dir" "$EXPLEDGER_MONITOR_ARTIFACTS"
cd "$project_dir"

source_hash() {
  rg --files internal cmd go.mod go.sum | LC_ALL=C sort | while IFS= read -r source_path; do
    shasum -a 256 "$source_path"
  done | shasum -a 256
}
source_before="$(source_hash)"
{
  date -u '+UTC %Y-%m-%dT%H:%M:%SZ'
  git rev-parse HEAD
  git status --short
  git diff --stat
  go version
  uname -mrs
  printf '%s\n' "source_sha256_before=$source_before"
  printf '%s\n' 'Timing: go test -count=1 -run ^$ -bench ^BenchmarkMonitor -benchtime 3x -benchmem ./internal/report ./internal/cli (three process runs)'
  printf '%s\n' 'Heap observation: EXPLEDGER_MONITOR_MEMORY=1 go test -count=1 -run ^TestMonitorMemory$ -v ./internal/cli'
} > "$artifacts_dir/environment.txt"

for batch in 1 2 3; do
  go test -count=1 -run '^$' -bench '^BenchmarkMonitor' -benchtime 3x -benchmem \
    ./internal/report ./internal/cli 2>&1 | tee "$artifacts_dir/timing-$batch.txt"
done

EXPLEDGER_MONITOR_MEMORY=1 go test -count=1 -run '^TestMonitorMemory$' -v \
  ./internal/cli 2>&1 | tee "$artifacts_dir/heap.txt"

source_after="$(source_hash)"
printf '%s\n' "source_sha256_after=$source_after" >> "$artifacts_dir/environment.txt"
if [ "$source_before" != "$source_after" ]; then
  printf '%s\n' 'Source changed during measurement; treat the results as provisional.' >&2
  exit 1
fi
