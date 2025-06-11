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
- Compression type (`uint8`)
- Checksum type (`uint8`)
- Checksum length (`uint8`)
- Block size (`uint32`)
- Trailer offset (`uint64`)
- Archive size (`uint64`)
- Empty directory entries
- File entries

### Compression Types

| Value | Algorithm |
|-------|-----------|
| 0 | gzip |
| 1 | zstd |
| 2 | lz4 |
| 3 | s2 |
| 4 | snappy |
| 5 | brotli |

### Checksum Types

| Value | Algorithm |
|-------|-----------|
| 0 | CRC32 |
| 1 | CRC16 |
| 2 | XXHash3 |
| 3 | SHA‑256 |
| 4 | Blake3 |

Blake3 is used by default when checksums are enabled.

### Feature Flags

| Flag            | Value | Purpose                                   |
|-----------------|-------|-------------------------------------------|
| `fNone`         | 0x1   | Reserved                                  |
| `fAbsolutePaths`| 0x2   | Store absolute paths                      |
| `fPermissions`  | 0x4   | Preserve permissions                      |
| `fModDates`     | 0x8   | Preserve modification times               |
| `fChecksums`    | 0x10  | Include checksums                         |
| `fNoCompress`   | 0x20  | Disable compression                       |
| `fIncludeInvis` | 0x40  | Include hidden files                      |
| `fSpecialFiles` | 0x80  | Archive symlinks and other special files  |
| `fBlockChecksums` | 0x100 | Store per-block checksums                |

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
1. Optional checksum (length given in the header) when `fChecksums` is set. When `fBlockChecksums` is set, a checksum precedes each block instead of one per file.
2. File contents, compressed according to the compression type. Zstd is used by default when compression is enabled and no other type is selected.

### Example Layout

```
[Magic][Version][Flags][CompType]
[Block Size][Trailer Offset][Archive Size]
[Empty Dir Count][Dirs]
[File Count][Files]
[Checksums and Data]
[Trailer]
```

## Trailer Format

Version 2 archives always use block mode. The header includes the block size, trailer offset, and archive size fields:

```
[Block Size: uint32]
[Trailer Offset: uint64]
```

Files are compressed in fixed-size blocks (default 512&nbsp;KiB). When
`fNoCompress` is set the block size becomes `0` and each file is stored as a
single block. After all file data comes a trailer containing a block index for
each file followed by a checksum of the trailer.

Trailer layout:

```
[Block Count: uint32]
[ [Offset uint64][Size uint32] ... ]
[Trailer Checksum: checksum length from header]
```

A checksum of the header (including the trailer offset) is stored at the end of
the header. Offsets in both the header and trailer are
absolute within the archive.

### Notes
- Directories that contain files are implied; only empty ones are listed.
- Compression and checksums apply per file.
- Special file entries contain metadata but no data.

See `create()` in the source for a reference implementation.
