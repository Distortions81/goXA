#!/usr/bin/env bash
set -euo pipefail

# This does some real-world testing and go test checks of goxa

# Temporary working directory for the test
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

SRC="$TMPDIR/src"
OUT="$TMPDIR/out"
mkdir -p "$SRC"

echo "Creating test files..."

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

# --- Test with compression ---
"$GOXA" cpmsi -arc "$TMPDIR/test.goxa" "$SRC"
ORIG_NAME=$(basename "$SRC")
mv "$SRC" "$SRC.orig"
"$GOXA" xpmsi -arc "$TMPDIR/test.goxa" "$OUT"
EXTRACTED="$OUT/$ORIG_NAME"

# Validations
orig_files=$(find "$SRC.orig" -type f | wc -l)
extr_files=$(find "$EXTRACTED" -type f | wc -l)
if [ "$orig_files" -ne "$extr_files" ]; then
  echo "file count mismatch (compressed)" >&2
  exit 1
fi

orig_perm=$(stat -c %a "$SRC.orig/.hidden")
extr_perm=$(stat -c %a "$EXTRACTED/.hidden")
if [ "$orig_perm" != "$extr_perm" ]; then
  echo "permission mismatch (compressed)" >&2
  exit 1
fi

orig_time=$(stat -c %Y "$SRC.orig/file_1.bin")
extr_time=$(stat -c %Y "$EXTRACTED/file_1.bin")
if [ "$orig_time" != "$extr_time" ]; then
  echo "mod time mismatch (compressed)" >&2
  exit 1
fi

orig_sum=$(sha256sum "$SRC.orig/file_1.bin" | cut -d" " -f1)
extr_sum=$(sha256sum "$EXTRACTED/file_1.bin" | cut -d" " -f1)
if [ "$orig_sum" != "$extr_sum" ]; then
  echo "checksum mismatch (compressed)" >&2
  exit 1
fi

echo "archive create/extract with compression passed"

# --- Test without compression (-n) ---
mv "$SRC.orig" "$SRC"  # Restore original for re-archiving
"$GOXA" cpmsin -arc "$TMPDIR/test_nocomp.goxa" "$SRC"
"$GOXA" xpmsi -arc "$TMPDIR/test_nocomp.goxa" "$OUT/nocomp"
EXTRACTED_N="$OUT/nocomp/$ORIG_NAME"

orig_files=$(find "$SRC" -type f | wc -l)
extr_files=$(find "$EXTRACTED_N" -type f | wc -l)
if [ "$orig_files" -ne "$extr_files" ]; then
  echo "file count mismatch (no compression)" >&2
  exit 1
fi

orig_perm=$(stat -c %a "$SRC/.hidden")
extr_perm=$(stat -c %a "$EXTRACTED_N/.hidden")
if [ "$orig_perm" != "$extr_perm" ]; then
  echo "permission mismatch (no compression)" >&2
  exit 1
fi

orig_time=$(stat -c %Y "$SRC/file_1.bin")
extr_time=$(stat -c %Y "$EXTRACTED_N/file_1.bin")
if [ "$orig_time" != "$extr_time" ]; then
  echo "mod time mismatch (no compression)" >&2
  exit 1
fi

orig_sum=$(sha256sum "$SRC/file_1.bin" | cut -d" " -f1)
extr_sum=$(sha256sum "$EXTRACTED_N/file_1.bin" | cut -d" " -f1)
if [ "$orig_sum" != "$extr_sum" ]; then
  echo "checksum mismatch (no compression)" >&2
  exit 1
fi

echo "archive create/extract without compression passed"

go test
