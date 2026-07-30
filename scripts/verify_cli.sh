#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
fixture_dir=$(mktemp -d "${TMPDIR:-/tmp}/ugxlsx-cli.XXXXXX")
trap 'rm -rf "$fixture_dir"' EXIT

cd "$repo_dir"
GOCACHE="${GOCACHE:-/tmp/ugxlsx-gocache}" go run ./cmd/gentestdata --dir "$fixture_dir"
GOCACHE="${GOCACHE:-/tmp/ugxlsx-gocache}" go build -o "$fixture_dir/ugxlsx" .
"$fixture_dir/ugxlsx" diff --left "$fixture_dir/left.xlsx" --right "$fixture_dir/right.xlsx" --format json
