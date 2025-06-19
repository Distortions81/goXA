#!/usr/bin/env bash
set -euo pipefail
PREFIX=${PREFIX:-/usr/local}
BINDIR="$PREFIX/bin"
MANDIR="$PREFIX/share/man/man1"

echo "Building goxa..."
go build -o goxa ./cmd/goxa

echo "Installing binary to $BINDIR"
install -d "$BINDIR"
install -m 755 goxa "$BINDIR/goxa"

echo "Installing man page to $MANDIR"
install -d "$MANDIR"
if command -v gzip >/dev/null; then
    gzip -c goxa.1 > "$MANDIR/goxa.1.gz"
else
    install -m 644 goxa.1 "$MANDIR/goxa.1"
fi

echo "Installation complete."
