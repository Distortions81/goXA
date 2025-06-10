# GoXA Archive Format (v2)

This document provides a compact description of the binary format used by the `goxa` archiver. All integer fields are little-endian.

## Layout

```
[Header]
[Per-file data]
[Trailer]
```

The header contains metadata for empty directories and files. Actual file contents follow the header and end with a trailer containing the block index.

### Header
- Magic bytes `GOXA`
- Version (uint16)
- Feature flags (uint32)
- Block size (`uint32`)
- Trailer offset (`uint64`)
- Empty directory entries
- File entries

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

Multiple flags may be combined.

### Empty Directories

```
[Empty Dir Count: uint64]
[Empty Dir Entries...]
```
Each entry optionally stores mode and mod time (controlled by flags) followed by
`[Path Length: uint16][UTF‑8 Path]`. Paths longer than 65,535 bytes cannot be
stored.

### Files

```
[File Count: uint64]
[File Entries...]
```
Each file entry contains:
* Uncompressed size (`uint64`)
* Optional mode (`uint32`) and mod time (`int64`)
* `[Path Length: uint16][UTF‑8 Path]`
* Type byte (`0`=file, `1`=symlink, `2`=hardlink, `3`=other)
* `[Link Target Length: uint16][Target]` for links

### Per-file Data

For every file:
1. Optional 32‑byte BLAKE2b checksum when `fChecksums` is set.
2. File contents, compressed according to the compression flag. Gzip is used by default when no flag is set.

### Example Layout

```
[Magic][Version][Flags]
[Empty Dir Count][Dirs]
[File Count][Files]
[Checksums and Data]
[Trailer]
```

## Trailer Format

Version 2 archives always use block mode. The header includes the block size and trailer offset fields:

```
[Block Size: uint32]
[Trailer Offset: uint64]
```

Files are compressed in fixed-size blocks (default 512&nbsp;KiB). When
`fNoCompress` is set the block size becomes `0` and each file is stored as a
single block. After all file data comes a trailer containing a block index for
each file followed by a 32‑byte BLAKE2b‑256 checksum of the trailer.

Trailer layout:

```
[Block Count: uint32]
[ [Offset uint64][Size uint32] ... ]
[Trailer Checksum: 32 bytes]
```

A 32‑byte BLAKE2b‑256 checksum of the header (including the trailer offset) is
stored at the end of the header. Offsets in both the header and trailer are
absolute within the archive.

### Notes
- Directories that contain files are implied; only empty ones are listed.
- Compression and checksums apply per file.
- Special file entries contain metadata but no data.

See `create()` in the source for a reference implementation.
