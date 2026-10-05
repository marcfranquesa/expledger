#!/bin/sh
# Verify the real GoReleaser output and execute only the host's binary.
set -eu

expected=${1:?usage: check-release.sh expected-version}
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
dist="$repo_root/release-dist"
check_dir=$(mktemp -d "${TMPDIR:-/tmp}/expledger-release-check.XXXXXX")
trap 'rm -rf "$check_dir"' EXIT HUP INT TERM

cd "$dist"
shasum -a 256 -c checksums.txt
[ "$(wc -l < checksums.txt | tr -d ' ')" = 4 ]
set -- ./*.tar.gz
[ "$#" = 4 ]
for os in darwin linux; do
    for arch in arm64 amd64; do
        set -- "${dist}"/expledger_*_"${os}"_"${arch}".tar.gz
        [ "$#" = 1 ]
        [ -f "$1" ]
        mkdir "$check_dir/$os-$arch"
        tar -xzf "$1" -C "$check_dir/$os-$arch"
        [ "$(tar -tzf "$1" | LC_ALL=C sort)" = "$(printf 'LICENSE\nexpledger\n')" ]
        cmp "$repo_root/LICENSE" "$check_dir/$os-$arch/LICENSE"
        [ -x "$check_dir/$os-$arch/expledger" ]
    done
done

host_os=$(go env GOHOSTOS)
host_arch=$(go env GOHOSTARCH)
mkdir "$check_dir/outside-project"
cd "$check_dir/outside-project"
[ "$(PATH=/nonexistent "$check_dir/$host_os-$host_arch/expledger" --version)" = "expledger version $expected" ]
printf 'Verified four archives, licenses, SHA-256 checksums, and %s/%s version output.\n' "$host_os" "$host_arch"
