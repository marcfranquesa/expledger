#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
project_dir=$(mktemp -d /tmp/expledger-test.XXXXXX)
trap 'rm -rf "$project_dir"' EXIT

cp -R "$repo_root/testdata/project/." "$project_dir"

cd "$project_dir"
printf 'Test project: %s\nType exit to return to your previous shell.\n' "$project_dir"
trap - EXIT
exec "${SHELL:-/bin/sh}" -i
