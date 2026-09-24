#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
project_dir=$(mktemp -d /tmp/expledger-test.XXXXXX)
trap 'rm -rf "$project_dir"' EXIT

cp -R "$repo_root/testdata/project/." "$project_dir"
git -C "$project_dir" init -q -b preview
git -C "$project_dir" remote add origin https://github.com/example/expledger-preview.git

printf '%s\n' "$project_dir"
trap - EXIT
