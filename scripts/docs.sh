#!/bin/sh
set -eu
cd "$(dirname "$0")/../docs-site"
required=$(cat hugo-version)
installed=$(hugo version)
case "$installed" in
  "hugo v${required}-"*|"hugo v${required}+"*|"hugo v${required} "*) ;;
  *) echo "Documentation requires Hugo $required (see docs-site/README.md)" >&2; exit 1 ;;
esac
case "$installed" in
  *+extended*) ;;
  *) echo "Documentation requires Hugo extended (see docs-site/README.md)" >&2; exit 1 ;;
esac
go run -mod=readonly ./generate
exec hugo "$@"
