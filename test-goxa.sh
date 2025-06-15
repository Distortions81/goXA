#!/usr/bin/env bash
# Re-run this script with bash if not already using bash
if [ -z "${BASH_VERSION:-}" ]; then
    exec bash "$0" "$@"
fi

set -euo pipefail

# Extensive end-to-end testing of goxa covering many CLI combinations.
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

SRC="$TMPDIR/src"
OUT="$TMPDIR/out"
mkdir -p "$SRC" "$OUT"

# Generate random input files. Set TEST_BYTES for quicker runs.
TARGET_BYTES=${TEST_BYTES:-$((20 * 1024 * 1024))}
TOTAL=0
COUNT=0
while [ "$TOTAL" -lt "$TARGET_BYTES" ]; do
  COUNT=$((COUNT + 1))
  SIZE=$(shuf -i 1024-65536 -n 1)
  head -c "$SIZE" </dev/urandom >"$SRC/file_${COUNT}.bin"
  TOTAL=$((TOTAL + SIZE))
  if (( COUNT % 50 == 0 )); then
    mkdir -p "$SRC/dir_$COUNT"
  fi
  if (( COUNT >= 200 )); then
    break
  fi
done

# Hidden file with known perms and timestamp
head -c 4096 </dev/urandom >"$SRC/.hidden"
chmod 600 "$SRC/.hidden"
touch -t 202201010101 "$SRC/file_1.bin"

GOXA="$TMPDIR/goxa"
go build -o "$GOXA" ./

ORIG_NAME=$(basename "$SRC")

"$GOXA" -version >"$TMPDIR/version.txt"
grep -q "goxa v" "$TMPDIR/version.txt"

"$GOXA" -h >"$TMPDIR/help.txt"
grep -q "Usage" "$TMPDIR/help.txt"

validate() {
  local orig=$1
  local extr=$2
  local label=$3

  local of=$(find "$orig" -type f | wc -l)
  local ef=$(find "$extr" -type f | wc -l)
  if [ "$of" -ne "$ef" ]; then
    echo "file count mismatch ($label)" >&2
    exit 1
  fi

  local op=$(stat -c %a "$orig/.hidden")
  local ep=$(stat -c %a "$extr/.hidden")
  if [ "$op" != "$ep" ]; then
    echo "permission mismatch ($label)" >&2
    exit 1
  fi

  local ot=$(stat -c %Y "$orig/file_1.bin")
  local et=$(stat -c %Y "$extr/file_1.bin")
  if [ "$ot" != "$et" ]; then
    echo "mod time mismatch ($label)" >&2
    exit 1
  fi

  local osum=$(sha256sum "$orig/file_1.bin" | cut -d" " -f1)
  local esum=$(sha256sum "$extr/file_1.bin" | cut -d" " -f1)
  if [ "$osum" != "$esum" ]; then
    echo "checksum mismatch ($label)" >&2
    exit 1
  fi
}

# goxa archive with compression
"$GOXA" cpmi -interactive=false -progress=false -arc "$TMPDIR/test.goxa" "$SRC"
"$GOXA" xpmi -interactive=false -progress=false -arc "$TMPDIR/test.goxa" "$OUT/comp"
validate "$SRC" "$OUT/comp/$ORIG_NAME" "compressed"

echo "compressed archive ok"

# no compression
"$GOXA" cpmin -interactive=false -progress=false -arc "$TMPDIR/test_nocomp.goxa" "$SRC"
"$GOXA" xpmi -interactive=false -progress=false -arc "$TMPDIR/test_nocomp.goxa" "$OUT/nocomp"
validate "$SRC" "$OUT/nocomp/$ORIG_NAME" "nocomp"

echo "no compression archive ok"

# Base64 and Base32 encoding
"$GOXA" cpmi -interactive=false -progress=false -arc "$TMPDIR/test_b64.goxa.b64" "$SRC"
"$GOXA" xpmi -interactive=false -progress=false -arc "$TMPDIR/test_b64.goxa.b64" "$OUT/b64"
validate "$SRC" "$OUT/b64/$ORIG_NAME" "base64"

"$GOXA" cpmi -interactive=false -progress=false -arc "$TMPDIR/test_b32.goxa.b32" "$SRC"
"$GOXA" xpmi -interactive=false -progress=false -arc "$TMPDIR/test_b32.goxa.b32" "$OUT/b32"
validate "$SRC" "$OUT/b32/$ORIG_NAME" "base32"

echo "encoding tests ok"

# FEC encoding with high redundancy
"$GOXA" cpmi -interactive=false -progress=false -fec-level=high -arc "$TMPDIR/test_fec.goxaf" "$SRC"
"$GOXA" xpmi -interactive=false -progress=false -arc "$TMPDIR/test_fec.goxaf" "$OUT/fec"
validate "$SRC" "$OUT/fec/$ORIG_NAME" "fec"

echo "fec archive ok"

# Tar formats
"$GOXA" cpmi -interactive=false -progress=false -arc "$TMPDIR/test.tar.gz" "$SRC"
"$GOXA" xpmi -interactive=false -progress=false -arc "$TMPDIR/test.tar.gz" "$OUT/targz"
validate "$SRC" "$OUT/targz/$ORIG_NAME" "targz"

"$GOXA" cpmi -interactive=false -progress=false -arc "$TMPDIR/test.tar.xz" "$SRC"
"$GOXA" xpmi -interactive=false -progress=false -arc "$TMPDIR/test.tar.xz" "$OUT/tarxz"
validate "$SRC" "$OUT/tarxz/$ORIG_NAME" "tarxz"

"$GOXA" cpmin -interactive=false -progress=false -arc "$TMPDIR/test.tar" "$SRC"
"$GOXA" xpmi -interactive=false -progress=false -arc "$TMPDIR/test.tar" "$OUT/tar"
validate "$SRC" "$OUT/tar/$ORIG_NAME" "tar"

echo "tar format tests ok"

# Listing and JSON output
"$GOXA" l -interactive=false -progress=false -arc "$TMPDIR/test.goxa" >"$TMPDIR/list.txt"
grep -q "file_1.bin" "$TMPDIR/list.txt"

"$GOXA" j -interactive=false -progress=false -stdout -arc "$TMPDIR/test.goxa" >"$TMPDIR/list.json"
python3 -m json.tool "$TMPDIR/list.json" >/dev/null
grep -q "file_1.bin" "$TMPDIR/list.json"

echo "listing commands ok"

# Extract specific file
mkdir -p "$OUT/partial"
"$GOXA" xpmi -interactive=false -progress=false -arc "$TMPDIR/test.goxa" -files "$ORIG_NAME/file_1.bin" "$OUT/partial"
if [ ! -f "$OUT/partial/$ORIG_NAME/file_1.bin" ]; then
  echo "selected file missing" >&2
  exit 1
fi
if [ "$(find "$OUT/partial/$ORIG_NAME" -type f | wc -l)" -ne 1 ]; then
  echo "unexpected files extracted" >&2
  exit 1
fi

echo "partial extraction ok"

# Stdout archive
"$GOXA" cpmi -interactive=false -progress=false -stdout -arc dummy "$SRC" >"$TMPDIR/stdout.goxa"
"$GOXA" xpmi -interactive=false -progress=false -arc "$TMPDIR/stdout.goxa" "$OUT/stdout"
validate "$SRC" "$OUT/stdout/$ORIG_NAME" "stdout"

echo "stdout handling ok"

# Advanced options
"$GOXA" cpmi -interactive=false -progress=false -sum=crc32 -comp=gzip -speed=better -threads=2 -block=65536 -bombcheck=false -spacecheck=false -noflush -arc "$TMPDIR/test_opts.goxa" "$SRC"
"$GOXA" xpmi -interactive=false -progress=false -arc "$TMPDIR/test_opts.goxa" "$OUT/opts"
validate "$SRC" "$OUT/opts/$ORIG_NAME" "opts"

echo "option flags ok"

# Force tar format and ensure extension
"$GOXA" cpmi -interactive=false -progress=false -format=tar -arc "$TMPDIR/force" "$SRC"
if [ ! -f "$TMPDIR/force.tar.gz" ]; then
  echo "forced tar file missing" >&2
  exit 1
fi
"$GOXA" xpmi -interactive=false -progress=false -arc "$TMPDIR/force.tar.gz" "$OUT/force"
validate "$SRC" "$OUT/force/$ORIG_NAME" "force"

echo "forced format ok"

# Expect failure on missing archive
if "$GOXA" x -interactive=false -progress=false -arc "$TMPDIR/missing.goxa" "$OUT/miss" 2>/dev/null; then
  echo "missing archive unexpectedly succeeded" >&2
  exit 1
fi

echo "failure case ok"

go test ./...

echo "all tests passed"
