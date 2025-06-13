#!/usr/bin/env bash
# Re-run this script with bash if not already using bash
if [ -z "${BASH_VERSION:-}" ]; then
    exec bash "$0" "$@"
fi

set -euo pipefail

# ---- config ---------------------------------------------------------------
SIZE_GB=${SIZE_GB:-64}
ARCHIVE_SUBDIR="testFiles"
SOURCE=${SOURCE:-"$HOME/$ARCHIVE_SUBDIR"}
MOUNTPOINT=${MOUNTPOINT:-$(mktemp -d)}
GOXA_OUTPUT="$MOUNTPOINT/${ARCHIVE_SUBDIR}.goxa"
TAR_OUTPUT="$MOUNTPOINT/${ARCHIVE_SUBDIR}.tar.gz"
MOUNTED=0

# timing helper
time_cmd() {
    local _tmp
    _tmp=$(mktemp)
    /usr/bin/time -f "%e %U %S" -o "$_tmp" "$@" >/dev/null
    read -r _real _user _sys < "$_tmp"
    rm -f "$_tmp"
    echo "$_real" "$(awk -v u="$_user" -v s="$_sys" 'BEGIN{print u + s}')"
}

# ---- cleanup function -----------------------------------------------------
cleanup_on_exit() {
    if [[ $MOUNTED -eq 1 ]]; then
        echo "🪩 Unmounting RAM disk at $MOUNTPOINT..."
        sudo umount "$MOUNTPOINT"
        rmdir "$MOUNTPOINT"
        echo "✅ RAM disk unmounted and removed."
    fi
}
trap cleanup_on_exit EXIT

# ---- if already mounted, unmount and exit --------------------------------
if mountpoint -q "$MOUNTPOINT"; then
    echo "⚠️ RAM disk already mounted at $MOUNTPOINT. Unmounting and exiting..."
    sudo umount "$MOUNTPOINT"
    rmdir "$MOUNTPOINT"
    echo "✅ Unmounted and removed $MOUNTPOINT"
    exit 0
fi

# ---- create and mount RAM disk -------------------------------------------
echo "Creating RAM disk at $MOUNTPOINT with size ${SIZE_GB}GB"
mkdir -p "$MOUNTPOINT"
sudo mount -t tmpfs -o "size=${SIZE_GB}G" tmpfs "$MOUNTPOINT"
MOUNTED=1

# ---- copy source files ---------------------------------------------------
echo "Copying files from $SOURCE to $MOUNTPOINT/$ARCHIVE_SUBDIR..."
mkdir -p "$MOUNTPOINT/$ARCHIVE_SUBDIR"
cp -rv "$SOURCE/." "$MOUNTPOINT/$ARCHIVE_SUBDIR"

# ---- goxa archive + timing ------------------------------------------------
echo "📆 Archiving with goxa to $GOXA_OUTPUT..."
go build
read -r goxa_real goxa_cpu < <(time_cmd ./goxa ci -arc="$GOXA_OUTPUT" "$MOUNTPOINT/$ARCHIVE_SUBDIR")

# ---- tar archive + timing -------------------------------------------------
echo "📆 Creating tar.gz archive to $TAR_OUTPUT..."
read -r tar_real tar_cpu < <(time_cmd tar -czf "$TAR_OUTPUT" -C "$MOUNTPOINT" "$ARCHIVE_SUBDIR")

# ---- decompression test --------------------------------------------------
echo "\n📂 Benchmarking decompression..."
GOXA_EXTRACT="$MOUNTPOINT/extracted_goxa"
TAR_EXTRACT="$MOUNTPOINT/extracted_tar"
mkdir -p "$GOXA_EXTRACT" "$TAR_EXTRACT"

# goxa extract
echo "📂 Extracting with goxa to $GOXA_EXTRACT..."
read -r goxa_x_real goxa_x_cpu < <(time_cmd ./goxa xu -arc="$GOXA_OUTPUT" "$GOXA_EXTRACT")

# tar extract
echo "📂 Extracting with tar to $TAR_EXTRACT..."
read -r tar_x_real tar_x_cpu < <(time_cmd tar -xzf "$TAR_OUTPUT" -C "$TAR_EXTRACT")

# ---- size summary ---------------------------------------------------------
echo "\n✅ Archives created:"
ls -lh "$GOXA_OUTPUT" "$TAR_OUTPUT"

# ---- results --------------------------------------------------------------
echo "\n📊 Compression Results:"
printf 'goxa: real=%ss, cpu=%ss\n' "$goxa_real" "$goxa_cpu"
printf 'tar:  real=%ss, cpu=%ss\n' "$tar_real" "$tar_cpu"

echo "\n📊 Decompression Results:"
printf 'goxa: real=%ss, cpu=%ss\n' "$goxa_x_real" "$goxa_x_cpu"
printf 'tar:  real=%ss, cpu=%ss\n' "$tar_x_real" "$tar_x_cpu"
