#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
preview_dir=$(mktemp -d "${TMPDIR:-/tmp}/expledger-preview.XXXXXX")
server_pid=

cleanup() {
    trap - EXIT HUP INT TERM
    if [ -n "$server_pid" ]; then
        kill "$server_pid" 2>/dev/null || true
        wait "$server_pid" 2>/dev/null || true
    fi
    rm -rf "$preview_dir"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

cd "$repo_root"
go build -o "$preview_dir/expledger" ./cmd/expledger
cp -R testdata/project "$preview_dir/project"
git -C "$preview_dir/project" init -q -b preview
git -C "$preview_dir/project" remote add origin https://github.com/example/expledger-preview.git

cd "$preview_dir/project"
"$preview_dir/expledger" serve "$@" &
server_pid=$!
wait "$server_pid"
server_pid=
