#!/usr/bin/env bash
# Re-run this script with bash if not already using bash
if [ -z "${BASH_VERSION:-}" ]; then
    echo "🔁 Not running under bash. Re-executing with bash..."
    exec bash "$0" "$@"
fi

set -euo pipefail

# ---- config ---------------------------------------------------------------
SIZE_GB=32
MOUNTPOINT="$(pwd)/RamDisk"
ARCHIVE_SUBDIR="testFiles"
SOURCE="$HOME/$ARCHIVE_SUBDIR"
GOXA_OUTPUT="$MOUNTPOINT/${ARCHIVE_SUBDIR}.goxa"
TAR_OUTPUT="$MOUNTPOINT/${ARCHIVE_SUBDIR}.tar.gz"
MOUNTED=0

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
echo "📆 Archiving with goXA to $GOXA_OUTPUT..."
go build
GOXA_TIME="$(mktemp)"
/usr/bin/time -f "%e %U %S" -o "$GOXA_TIME" \
  ./goXA ci -arc="$GOXA_OUTPUT" "$MOUNTPOINT/$ARCHIVE_SUBDIR"
read -r goxa_real goxa_user goxa_sys < "$GOXA_TIME"
goxa_cpu=$(awk "BEGIN {print $goxa_user + $goxa_sys}")
rm -f "$GOXA_TIME"

# ---- tar archive + timing -------------------------------------------------
echo "📆 Creating tar.gz archive to $TAR_OUTPUT..."
TAR_TIME="$(mktemp)"
/usr/bin/time -f "%e %U %S" -o "$TAR_TIME" \
  tar -czf "$TAR_OUTPUT" -C "$MOUNTPOINT" "$ARCHIVE_SUBDIR"
read -r tar_real tar_user tar_sys < "$TAR_TIME"
tar_cpu=$(awk "BEGIN {print $tar_user + $tar_sys}")
rm -f "$TAR_TIME"

# ---- decompression test --------------------------------------------------
echo "\n📂 Benchmarking decompression..."
GOXA_EXTRACT="$MOUNTPOINT/extracted_goxa"
TAR_EXTRACT="$MOUNTPOINT/extracted_tar"
mkdir -p "$GOXA_EXTRACT" "$TAR_EXTRACT"

# goXA extract
echo "📂 Extracting with goXA to $GOXA_EXTRACT..."
GOXA_X_TIME="$(mktemp)"
/usr/bin/time -f "%e %U %S" -o "$GOXA_X_TIME" \
  ./goXA xu -arc="$GOXA_OUTPUT" "$GOXA_EXTRACT"
read -r goxa_x_real goxa_x_user goxa_x_sys < "$GOXA_X_TIME"
goxa_x_cpu=$(awk "BEGIN {print $goxa_x_user + $goxa_x_sys}")
rm -f "$GOXA_X_TIME"

# tar extract
echo "📂 Extracting with tar to $TAR_EXTRACT..."
TAR_X_TIME="$(mktemp)"
/usr/bin/time -f "%e %U %S" -o "$TAR_X_TIME" \
  tar -xzf "$TAR_OUTPUT" -C "$TAR_EXTRACT"
read -r tar_x_real tar_x_user tar_x_sys < "$TAR_X_TIME"
tar_x_cpu=$(awk "BEGIN {print $tar_x_user + $tar_x_sys}")
rm -f "$TAR_X_TIME"

# ---- size summary ---------------------------------------------------------
echo "\n✅ Archives created:"
ls -lh "$GOXA_OUTPUT" "$TAR_OUTPUT"

# ---- compression performance ---------------------------------------------
echo "\n📊 Comparing compression performance..."
if (( $(awk "BEGIN {exit !($goxa_cpu < $tar_cpu)}") )); then
    cpu_winner="goXA"
    cpu_loser="tar"
    cpu_savings=$(awk "BEGIN {print ($tar_cpu - $goxa_cpu) / $tar_cpu * 100}")
else
    cpu_winner="tar"
    cpu_loser="goXA"
    cpu_savings=$(awk "BEGIN {print ($goxa_cpu - $tar_cpu) / $goxa_cpu * 100}")
fi

goxa_eff=$(awk "BEGIN {print $goxa_cpu / $goxa_real}")
tar_eff=$(awk "BEGIN {print $tar_cpu / $tar_real}")

is_goxa_faster=$(awk "BEGIN {print ($goxa_real < $tar_real) ? 1 : 0}")
if [[ "$is_goxa_faster" == "1" ]]; then
    faster="goXA"
    wall_speedup=$(awk "BEGIN {print $tar_real / $goxa_real}")
    wall_diff_pct=$(awk "BEGIN {print (1 - $goxa_real / $tar_real) * 100}")
    cpu_speedup=$(awk "BEGIN {print $tar_cpu / $goxa_cpu}")
    cpu_diff_pct=$(awk "BEGIN {print (1 - $goxa_cpu / $tar_cpu) * 100}")
else
    faster="tar"
    wall_speedup=$(awk "BEGIN {print $goxa_real / $tar_real}")
    wall_diff_pct=$(awk "BEGIN {print (1 - $tar_real / $goxa_real) * 100}")
    cpu_speedup=$(awk "BEGIN {print $goxa_cpu / $tar_cpu}")
    cpu_diff_pct=$(awk "BEGIN {print (1 - $tar_cpu / $goxa_cpu) * 100}")
fi

echo "📆 Compression Results:"
echo "📆 goXA: real=${goxa_real}s, cpu=${goxa_cpu}s"
echo "📆 tar:  real=${tar_real}s, cpu=${tar_cpu}s"

# ---- decompression performance -------------------------------------------
echo "\n📊 Comparing decompression performance..."
if (( $(awk "BEGIN {exit !($goxa_x_cpu < $tar_x_cpu)}") )); then
    x_cpu_winner="goXA"
    x_cpu_loser="tar"
    x_cpu_savings=$(awk "BEGIN {print ($tar_x_cpu - $goxa_x_cpu) / $tar_x_cpu * 100}")
else
    x_cpu_winner="tar"
    x_cpu_loser="goXA"
    x_cpu_savings=$(awk "BEGIN {print ($goxa_x_cpu - $tar_x_cpu) / $goxa_x_cpu * 100}")
fi

x_is_goxa_faster=$(awk "BEGIN {print ($goxa_x_real < $tar_x_real) ? 1 : 0}")
if [[ "$x_is_goxa_faster" == "1" ]]; then
    x_faster="goXA"
    x_wall_speedup=$(awk "BEGIN {print $tar_x_real / $goxa_x_real}")
    x_wall_diff_pct=$(awk "BEGIN {print (1 - $goxa_x_real / $tar_x_real) * 100}")
    x_cpu_speedup=$(awk "BEGIN {print $tar_x_cpu / $goxa_x_cpu}")
    x_cpu_diff_pct=$(awk "BEGIN {print (1 - $goxa_x_cpu / $tar_x_cpu) * 100}")
else
    x_faster="tar"
    x_wall_speedup=$(awk "BEGIN {print $goxa_x_real / $tar_x_real}")
    x_wall_diff_pct=$(awk "BEGIN {print (1 - $tar_x_real / $goxa_x_real) * 100}")
    x_cpu_speedup=$(awk "BEGIN {print $goxa_x_cpu / $tar_x_cpu}")
    x_cpu_diff_pct=$(awk "BEGIN {print (1 - $tar_x_cpu / $goxa_x_cpu) * 100}")
fi

echo "📆 Decompression Results:"
echo "📆 goXA: real=${goxa_x_real}s, cpu=${goxa_x_cpu}s"
echo "📆 tar:  real=${tar_x_real}s, cpu=${tar_x_cpu}s"