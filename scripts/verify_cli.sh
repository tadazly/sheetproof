#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
fixture_dir=$(mktemp -d "${TMPDIR:-/tmp}/sheetproof-cli.XXXXXX")
trap 'rm -rf "$fixture_dir"' EXIT

cd "$repo_dir"
GOCACHE="${GOCACHE:-/tmp/sheetproof-gocache}" go run ./cmd/gentestdata --dir "$fixture_dir"
GOCACHE="${GOCACHE:-/tmp/sheetproof-gocache}" go build -o "$fixture_dir/sheetproof" .
"$fixture_dir/sheetproof" diff --left "$fixture_dir/left.xlsx" --right "$fixture_dir/right.xlsx" --format json
