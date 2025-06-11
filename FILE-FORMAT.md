# GoXA Archive Format (v2)

This document describes the binary layout used by the `goxa` archiver. All integers are little-endian and fields are packed with no padding.

## Layout

```
[Header]
[File data blocks]
[Trailer]
```

The header records every empty directory and file. File data follows, compressed into blocks. A trailer at the end holds a block index and a checksum.

## Header

The header begins with a fixed section:

| Offset | Size | Description |
|-------:|-----:|-------------|
| 0 | 4 | Magic bytes `GOXA` |
| 4 | 2 | Version (`uint16`, currently `2`) |
| 6 | 4 | Feature flags (`uint32`) |
| 10 | 1 | Compression type |
| 11 | 1 | Checksum type |
| 12 | 1 | Checksum length in bytes |
| 13 | 4 | Block size (`uint32`, `0` when uncompressed) |
| 17 | 8 | Trailer offset (`uint64`) |
| 25 | 8 | Archive size (`uint64`) |
| 33 | 8 | Empty directory count (`uint64`) |

Immediately after this count come the empty directory entries, followed by the file count and file entries. A checksum of the header (including the trailer offset) appears after the file entries.

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

Blake3 is the default when checksums are enabled.

### Feature Flags

| Flag | Value | Purpose |
|------|------:|---------|
| `fNone` | 0x1 | Reserved |
| `fAbsolutePaths` | 0x2 | Store absolute paths |
| `fPermissions` | 0x4 | Preserve permissions |
| `fModDates` | 0x8 | Preserve modification times |
| `fChecksums` | 0x10 | Include checksums |
| `fNoCompress` | 0x20 | Disable compression |
| `fIncludeInvis` | 0x40 | Include hidden files |
| `fSpecialFiles` | 0x80 | Archive symlinks and other special files |
| `fBlockChecksums` | 0x100 | Store per-block checksums |

Flags may be combined.

### Empty Directory Entries

```
[Empty Dir Count: uint64]
[Entries...]
```
Each entry optionally stores mode and mod time depending on the flags:
```
[Mode uint32?][ModTime int64?][PathLen uint16][UTF-8 Path]
```
Paths longer than 65,535 bytes cannot be stored.

### File Entries

```
[File Count: uint64]
[Entries...]
```
Each file entry contains:

* Uncompressed size (`uint64`)
* Optional mode (`uint32`) and mod time (`int64`)
* `[PathLen uint16][UTF-8 Path]`
* Type byte (`0`=file, `1`=symlink, `2`=hardlink, `3`=other)
* `[LinkLen uint16][Target]` for symlinks and hardlinks

`entryOther` records only metadata and has no file data.

## Per-file Data

For each file entry the archive stores:
1. A checksum of the entire file when `fChecksums` is set. When `fBlockChecksums` is also set, a checksum precedes each block instead.
2. The file data split into blocks. Each block is compressed using the selected algorithm. Without compression the block size is `0` and each file is stored as one block.

## Trailer

The trailer starts at the offset recorded in the header. Layout:

```
[Block Count uint32]
[ [Offset uint64][Size uint32] ... ]
[Trailer Checksum: checksum length from header]
```

Offsets are absolute from the start of the archive. The trailer checksum covers everything from the `Block Count` field up to the end of the last block entry.

## Notes

- Directories containing files are implied; only empty directories are listed.
- Compression and checksums are applied per file.
- Special file entries contain metadata but no data.
- See `create()` in the source for a working reference implementation.
