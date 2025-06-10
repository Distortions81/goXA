# GoXA Archive Format (v1 & v2)

This document provides a compact description of the binary format used by the `goxa` archiver. All integer fields are little-endian.

## Layout

```
[Header]
[Per-file data]
```

The header contains metadata for empty directories and files along with an offset table. Actual file contents follow the header.

### Header
- Magic bytes `GOXA`
- Version (uint16)
- Feature flags (uint32)
- Empty directory entries
- File entries
- Offset table (uint64 per file)

### Feature Flags

| Flag            | Value | Purpose                                   |
|-----------------|-------|-------------------------------------------|
| `fNone`         | 0x1   | Reserved                                  |
| `fAbsolutePaths`| 0x2   | Store absolute paths                      |
| `fPermissions`  | 0x4   | Preserve permissions                      |
| `fModDates`     | 0x8   | Preserve modification times               |
| `fChecksums`    | 0x10  | Include BLAKE2b-256 checksums             |
| `fNoCompress`   | 0x20  | Disable compression                       |
| `fIncludeInvis` | 0x40  | Include hidden files                      |
| `fSpecialFiles` | 0x80  | Archive symlinks and other special files  |
| `fZstd`         | 0x100 | Use zstd compression                      |
| `fLZ4`          | 0x200 | Use lz4 compression                       |
| `fS2`           | 0x400 | Use s2 compression                        |
| `fSnappy`       | 0x800 | Use snappy compression                    |
| `fBrotli`       | 0x1000| Use brotli compression                    |
| `fBlock`        | 0x2000| Enable block mode (v2 archives)           |

Multiple flags may be combined.

### Empty Directories

```
[Empty Dir Count: uint64]
[Empty Dir Entries...]
```
Each entry optionally stores mode and mod time (controlled by flags) followed by path length and the UTF‑8 path.

### Files

```
[File Count: uint64]
[File Entries...]
```
Each file entry contains:
- Uncompressed size (uint64)
- Optional mode and mod time
- Path length and UTF‑8 path
- Type byte (file, symlink, hardlink, etc.)
- Link target for links

### Offset Table

Immediately after the file entries, an 8‑byte offset is stored for each file. These absolute offsets point into the data section.

### Per-file Data

For every file:
1. Optional 32‑byte BLAKE2b checksum when `fChecksums` is set.
2. File contents, compressed according to the compression flag. Gzip is used by default when no flag is set.

### Example Layout

```
[Magic][Version][Flags]
[Empty Dir Count][Dirs]
[File Count][Files]
[Offset Table]
[Checksums and Data]
```

## Version 2 Additions

Version 2 archives introduce block mode indicated by the `fBlock` flag. The header includes two additional fields:

```
[Block Size: uint32]
[Trailer Offset: uint64]
```

Files are compressed in fixed-size blocks (default 512&nbsp;KiB). After all file data comes a trailer containing a block index for each file followed by a 32‑byte checksum of the trailer.

Trailer layout:

```
[Block Count: uint32]
[Block Offsets and Sizes...]
[Trailer Checksum]
```

A 32‑byte checksum of the header (including the trailer offset) is stored at the
end of the header. Offsets in both the header and trailer are absolute within the
archive.

### Notes
- Directories that contain files are implied; only empty ones are listed.
- Compression and checksums apply per file.
- Special file entries contain metadata but no data.

See `create()` in the source for a reference implementation.
