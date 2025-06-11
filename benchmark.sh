#!/usr/bin/env bash
# Re-run this script with bash if not already using bash
if [ -z "${BASH_VERSION:-}" ]; then
    echo "🔁 Not running under bash. Re-executing with bash..."
    exec bash "$0" "$@"
fi

set -euo pipefail

# ---- config ---------------------------------------------------------------
SIZE_GB=24
MOUNTPOINT="$(pwd)/RamDisk"
SOURCE="$HOME/testFiles"
ARCHIVE_SUBDIR="testFiles"
GOXA_OUTPUT="$MOUNTPOINT/${ARCHIVE_SUBDIR}.goxa"
TAR_OUTPUT="$MOUNTPOINT/${ARCHIVE_SUBDIR}.tar.gz"
MOUNTED=0

# ---- cleanup function -----------------------------------------------------
cleanup_on_exit() {
    if [[ $MOUNTED -eq 1 ]]; then
        echo "🧹 Unmounting RAM disk at $MOUNTPOINT..."
        sudo umount "$MOUNTPOINT"
        rmdir "$MOUNTPOINT"
        echo "✅ RAM disk unmounted and removed."
    fi
}
trap cleanup_on_exit EXIT

# ---- check for RAM disk ---------------------------------------------------
if mountpoint -q "$MOUNTPOINT"; then
    echo "⚠️ RAM disk already mounted at $MOUNTPOINT. Unmounting and exiting..."
    sudo umount "$MOUNTPOINT"
    rmdir "$MOUNTPOINT"
    echo "✅ Unmounted and removed $MOUNTPOINT"
    exit 0
fi

# ---- mount RAM disk -------------------------------------------------------
echo "Creating RAM disk at $MOUNTPOINT with size ${SIZE_GB}GB"
mkdir -p "$MOUNTPOINT"
sudo mount -t tmpfs -o "size=${SIZE_GB}G" tmpfs "$MOUNTPOINT"
MOUNTED=1

# ---- copy source files ----------------------------------------------------
echo "Copying files from $SOURCE to $MOUNTPOINT/$ARCHIVE_SUBDIR..."
mkdir -p "$MOUNTPOINT/$ARCHIVE_SUBDIR"
cp -rv "${SOURCE}/." "$MOUNTPOINT/$ARCHIVE_SUBDIR"

# ---- goxa archive ---------------------------------------------------------
echo "📦 Archiving with goXA to $GOXA_OUTPUT..."
{ read -r user sys <<< $(/usr/bin/time -f "%U %S" \
    goxa ci -arc="$GOXA_OUTPUT" "$MOUNTPOINT/$ARCHIVE_SUBDIR" 2>&1 >/dev/null); } \
    && total=$(awk "BEGIN {print $user + $sys}") \
    && echo "🕒 goXA: user=${user}s sys=${sys}s total_cpu=${total}s"

# ---- tar archive ----------------------------------------------------------
echo "📦 Creating tar.gz archive to $TAR_OUTPUT..."
{ read -r user sys <<< $(/usr/bin/time -f "%U %S" \
    tar -czf "$TAR_OUTPUT" -C "$MOUNTPOINT" "$ARCHIVE_SUBDIR" 2>&1 >/dev/null); } \
    && total=$(awk "BEGIN {print $user + $sys}") \
    && echo "🕒 tar:  user=${user}s sys=${sys}s total_cpu=${total}s"
# ---- summary --------------------------------------------------------------
echo ""
echo "✅ Archives created:"
ls -lh "$GOXA_OUTPUT" "$TAR_OUTPUT"
echo ""
echo "RAM disk will now be unmounted automatically."
