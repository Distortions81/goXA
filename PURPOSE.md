# Purpose and Design Rationale

GoXA aims to be a modern replacement for classic archivers such as `tar`. The defaults are intentionally safe and predictable so that new users can create and extract archives without fear of damaging their system or losing files.

## Default Safety

* **Checksums enabled** – every file is protected by a Blake3 checksum to detect corruption early while keeping speed high.
* **No dotfiles** – hidden files are skipped unless the `i` flag is used. This prevents accidentally archiving temporary editor artifacts or configuration secrets.
* **Relative paths only** – without `a`, paths are stored relative so extraction cannot write outside the current directory tree.
* **No overwrite by default** – files are never replaced unless you pass `f`. Mistakes are caught instead of silently clobbering data.
* **Zip bomb detection** – extremely compressed files are refused unless `-bombcheck=false` is set, guarding against denial-of-service archives.
* **Space check** – before writing or extracting an archive GoXA verifies there is enough free space so operations do not stop midway.
* **Interactive prompts** – when an archive was created with flags you did not specify, GoXA asks before enabling them. This makes unexpected behaviour explicit.
* **Progress bar** – a clear progress display is shown for interactive runs so you know the program is working or detect possible issues.
* **Disk sync** – output files are flushed to disk before closing so removable drives aren't pulled while data is still in kernel buffers. Syncing ensures data is fully written before devices are detached.
* **`-arc` required** – the archive name is always supplied explicitly which avoids mistakes when scripts are moved between directories.

## Performance Choices

GoXA writes data in large 512KiB blocks by default (see `defaultBlockSize` in `const.go`). A trailer at the end of the file lists all block offsets so readers can jump directly to any part of any file. The layout lends itself to multi-threaded reading and writing, letting GoXA scale with modern
hardware. Older utilities like tar only work with 0.5kb blocks, leading to a lot of overhead. Additionally, goxa uses read and write buffers (default 1MiB) to keep the CPU productive and further reduce overhead.

Together these choices make GoXA safer and faster than traditional tools. Checksums catch errors immediately, the block trailer allows random access and parallelism, and sensible prompts keep the user in control.
