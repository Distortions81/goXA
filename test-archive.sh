#!/usr/bin/env bash
set -euo pipefail

# Temporary working directory for the test
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

SRC="$TMPDIR/src"
OUT="$TMPDIR/out"
mkdir -p "$SRC"

# Generate random data spread across many files. Use TEST_BYTES env var to
# override the default size for quicker runs.
TARGET_BYTES=${TEST_BYTES:-$((3 * 1024 * 1024 * 1024))}
TOTAL=0
COUNT=0
while [ "$TOTAL" -lt "$TARGET_BYTES" ]; do
  COUNT=$((COUNT + 1))
  SIZE=$(shuf -i 1024-1048576 -n 1)
  head -c "$SIZE" </dev/random >"$SRC/file_${COUNT}.bin"
  TOTAL=$((TOTAL + SIZE))
  # Create a subdirectory occasionally
  if (( COUNT % 100 == 0 )); then
    mkdir -p "$SRC/dir_$COUNT"
  fi
  # Limit to a few thousand files
  if (( COUNT >= 4000 )); then
    break
  fi
done

# Hidden file with distinct permissions
head -c 4096 </dev/random >"$SRC/.hidden"
chmod 600 "$SRC/.hidden"

# Set explicit mod time on first file
touch -t 202201010101 "$SRC/file_1.bin"

# Build CLI
GOXA="$TMPDIR/goxa"
go build -o "$GOXA" ./

# Create archive with permissions, mod times, checksums and invis files
"$GOXA" cpmsi -arc "$TMPDIR/test.goxa" "$SRC"

# Preserve source for comparison
ORIG_NAME=$(basename "$SRC")
mv "$SRC" "$SRC.orig"

# Extract
"$GOXA" xpmsi -arc "$TMPDIR/test.goxa" "$OUT"
EXTRACTED="$OUT/$ORIG_NAME"

# Basic validations
orig_files=$(find "$SRC.orig" -type f | wc -l)
extr_files=$(find "$EXTRACTED" -type f | wc -l)
if [ "$orig_files" -ne "$extr_files" ]; then
  echo "file count mismatch" >&2
  exit 1
fi

# Check hidden file permissions
orig_perm=$(stat -c %a "$SRC.orig/.hidden")
extr_perm=$(stat -c %a "$EXTRACTED/.hidden")
if [ "$orig_perm" != "$extr_perm" ]; then
  echo "permission mismatch" >&2
  exit 1
fi

# Check mod time preservation
orig_time=$(stat -c %Y "$SRC.orig/file_1.bin")
extr_time=$(stat -c %Y "$EXTRACTED/file_1.bin")
if [ "$orig_time" != "$extr_time" ]; then
  echo "mod time mismatch" >&2
  exit 1
fi

# Spot check file contents using checksums
orig_sum=$(sha256sum "$SRC.orig/file_1.bin" | cut -d" " -f1)
extr_sum=$(sha256sum "$EXTRACTED/file_1.bin" | cut -d" " -f1)
if [ "$orig_sum" != "$extr_sum" ]; then
  echo "checksum mismatch" >&2
  exit 1
fi

echo "archive create/extract large test passed"
