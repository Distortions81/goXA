#!/usr/bin/env bash
set -euo pipefail

# Temporary working directory for the test
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

SRC="$TMPDIR/src"
OUT="$TMPDIR/out"

mkdir -p "$SRC"
echo "hello" > "$SRC/file.txt"

go build -o "$TMPDIR/goxa" ./

"$TMPDIR/goxa" c -arc "$TMPDIR/test.goxa" "$SRC"

rm -rf "$SRC"

"$TMPDIR/goxa" x -arc "$TMPDIR/test.goxa" "$OUT"

EXPECTED="$OUT/$(basename "$SRC")/file.txt"
if [ "$(cat "$EXPECTED")" != "hello" ]; then
  echo "archive extraction failed" >&2
  exit 1
fi

echo "archive create/extract test passed"
