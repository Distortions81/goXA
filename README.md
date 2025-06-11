# GoXA -- Go eXpress Archive
<img src="https://github.com/Distortions81/goXA/blob/main/Xango.png?raw=true" alt="Xango the Archivist" width="300"/>

## Xango the Pangolin Archivist
GoXA is a friendly archiver written in Go. It's fast and straightforward, though still maturing—please report any issues you find.

## Features

- [x] Fast archive creation and extraction
- [x] Multiple compression formats (gzip, zstd, lz4, s2, snappy, brotli, xz; defaults to zstd)
- [x] Standard tar archive support (auto-detected from extension or archive header)
- [x] Optional checksums (per-file or per-block; crc16, crc32, xxhash, sha-256, or blake3; default blake3)
- [x] Preserve permissions and modification times
- [x] Fully documented binary format ([FILE-FORMAT.md](FILE-FORMAT.md))
- [x] Optional support for symlinks and other special files
- [x] Block-based format for fast compression (single block when uncompressed)
- [x] Automatic format detection from file extension or archive header
- [x] Output archives directly to stdout for easy piping
- [x] Selective extraction with the `-files` flag
- [x] Progress bar showing transfer speed and current file
- [x] Pure Go code with no runtime dependencies once compiled.
- [x] Optional base32/base64 encoding via `.b32` or `.b64` file suffixes

## File Format

See [FILE-FORMAT.md](FILE-FORMAT.md) for the full binary format.

## Install

With Go 1.24+:

```bash
go install github.com/Distortions81/goXA@latest
```

Or build from source:

```bash
git clone https://github.com/Distortions81/goXA.git
cd goXA
go build
```

This creates the `goxa` binary.

## Usage

```bash
goxa [mode] [flags] -arc=archiveFile [paths...]
```

`mode`: `c` (create), `l` (list), `x` (extract)

`flags`: any combination of:

| Flag | Description |
|------|-------------|
| `a` | Absolute paths |
| `p` | File permissions |
| `m` | Modification times |
| `s` | Enable checksums |
| `b` | Per-block checksums |
| `n` | Disable compression |
| `i` | Hidden files |
| `o` | Special files |
| `u` | Use flags from archive |
| `v` | Verbose logging |
| `f` | Overwrite files / ignore read errors |

Paths are stored relative by default. Use `a` to store and restore absolute paths. Extraction only restores permissions, modification times, hidden files, or special files when `p`, `m`, `i`, or `o` are given (or `u` to use the archive's flags).

### Extra Flags

| Flag | Description |
|------|-------------|
| `-arc=` | Archive file name |
| `-stdout` | Output archive to stdout |
| `-files` | Comma-separated list of files and directories to extract |
| `-progress=false` | Disable progress display |
| `-comp=` | Compression algorithm (gzip, zstd, lz4, s2, snappy, brotli, xz, none) |
| `-speed=` | Compression speed (fastest, default, better, best) |
| `-format=` | Archive format (`goxa` or `tar`) |

Progress shows transfer speed and the current file being processed.

`xz` compression is only available when `-format=tar`.
Snappy does not support configurable compression levels; `-speed` has no effect when using snappy.

### Examples

```bash
goxa c -arc=mybackup.goxa myStuff/
goxa capmsif -arc=mybackup.goxa ~/
goxa x -arc=mybackup.goxa
goxa xu -arc=mybackup.goxa     # use flags in archive (aka auto)
goxa l -arc=mybackup.goxa
goxa c -arc=mybackup.tar.gz myStuff/
goxa x -arc=mybackup.tar.gz
goxa c -arc=mybackup.tar.xz myStuff/
goxa x -arc=mybackup.tar.xz
goxa c -arc=mybackup.goxa -stdout myStuff/ | ssh host "cat > backup.goxa"
goxa x -arc=mybackup.goxa -files=file.txt,dir/
```

## Roadmap

- [x] Format documentation
- [x] Working relative path support
- [x] Add modes for non-files (symlinks, devices)
- [ ] Multi-threaded archive optimization
- [ ] Archive signatures for optional additional security
- [ ] Archive comment field
- [ ] Encrypted archives

## Security Notes

- Paths are sanitized during extraction, but `-a` (`absolute paths`) allows the archive to write anywhere. Use with care on unknown files.
- With `-o` (`special files`) symlinks are not resolved, so sneaky links can sidestep your destination folder.
- `-u` (`use flags from archive`) applies whatever options were set when the archive was created, which may enable absolute paths, permissions, mod dates and special files.
- Size fields use `int64`; maximum individual file size is about 9,223 petabytes (~8&nbsp;EiB).

## License

MIT License.

## Author

- https://github.com/Distortions81

---

**GoXA** — fast, clean, and gopher-approved archiving.
