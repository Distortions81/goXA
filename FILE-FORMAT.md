# GoXA Archive Format (v2)

This document describes the exact binary layout used by the `goxa` archiver. It details
every byte stored inside a `.goxa` archive so that other implementations can both
create and parse them with perfect fidelity. Unless stated otherwise **all integers
are encoded in little-endian order** and the structures are packed with no padding
between fields. The format intentionally mirrors how Go structures are serialized
in memory so that parsing and generation can be done with minimal overhead.

Version 2 extends the original format with a trailer that lists block offsets and
adds optional per-block checksums. Older archives remain readable but lack these
features.

## Layout

```
[Header]
[File data blocks]
[Trailer]
```

The header records every empty directory and file. File data follows, compressed into blocks. A trailer at the end holds a block index and a checksum.
When an archive is larger than the configured span size the bytes are written
sequentially to additional files named `1-N.archive.goxa`, `2-N.archive.goxa`
and so on. No extra markers are inserted – the data continues exactly where the
previous file ended. Readers must concatenate these files logically.

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

### Header Field Details

* **Magic bytes** – always the ASCII string `GOXA` and used to quickly verify
  that a file is actually a GoXA archive.
* **Version** – current format version. Readers should reject archives with a
  higher version as incompatible.
* **Feature flags** – a bit mask selecting optional features. See the
  [Feature Flags](#feature-flags) table below.
* **Compression type** – index into the compression table. If `fNoCompress` is
  set then file data is stored uncompressed even though this field still holds a
  value.
* **Checksum type** and **length** – define the algorithm used for archive and
  optional per-file checksums. Length is stored separately so future algorithms
  can be supported without new format versions.
* **Block size** – preferred size of compressed blocks. Archives that do not use
  compression set this value to `0`.
* **Trailer offset** – absolute offset to the trailer from the start of the
  archive. This allows a reader to jump directly to the block index when seeking
  within a large archive.
* **Archive size** – total size in bytes of the entire archive file, useful when
  preallocating space on extraction.
* **Empty directory count** – number of entries in the empty directory table that
  follows immediately after this header.

### Compression Types

| Value | Algorithm |
|-------|-----------|
| 0 | gzip |
| 1 | zstd |
| 2 | lz4 |
| 3 | s2 |
| 4 | snappy |
| 5 | brotli |

The choice of compression affects only the file data blocks. Metadata is never
compressed. The algorithms listed above are chosen for their balance of speed
and ratio. When decoding, readers must gracefully handle unknown values by
failing with a clear error message.

### Checksum Types

| Value | Algorithm |
|-------|-----------|
| 0 | CRC32 |
| 1 | CRC16 |
| 2 | XXHash3 |
| 3 | SHA‑256 |
| 4 | Blake3 |

Blake3 is the default when checksums are enabled because it offers strong
cryptographic properties while remaining extremely fast. The checksum type is
used for the header, trailer, and (when enabled) for individual file data.
Readers should expect the length field to match the output size of the selected
algorithm and reject mismatches.

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

The flags control what information is stored for each entry:

* **`fAbsolutePaths`** – paths are written exactly as provided, starting with a
  leading `/`. Without this flag all stored paths are relative.
* **`fPermissions`** – saves the Unix permission bits for directories and files.
  If absent, extracted files will receive default permissions chosen by the
  operating system.
* **`fModDates`** – records the last modification time. When omitted, extraction
  tools typically set the current time for all files.
* **`fChecksums`** – adds a checksum for each file's contents. This helps detect
  corruption but increases archive size.
* **`fNoCompress`** – disables compression for all files even if a compression
  algorithm is specified.
* **`fIncludeInvis`** – includes hidden files (dot files) that would otherwise
  be skipped.
* **`fSpecialFiles`** – allows storing symbolic links and other special file
  types such as device nodes. Without this flag those entries are ignored.
* **`fBlockChecksums`** – when combined with `fChecksums`, adds a checksum before
  every compressed block. This allows detection of corruption in streaming
  scenarios.

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

Empty directories have no associated file data and therefore occupy no space in
the data block section. They are listed only so that an exact directory tree can
be recreated during extraction.

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

File paths are likewise limited to 65,535 bytes. The `size` field records the
uncompressed size of the file data. For symlinks and hardlinks the `link` field
stores the target path as UTF‑8 text and the `size` value is set to zero. Other
special files (for example fifos or device nodes) use `entryOther` and rely on
the feature flags to indicate that such files should be created during
extraction.

## Per-file Data

For each file entry the archive stores:
1. A checksum of the entire file when `fChecksums` is set. When `fBlockChecksums` is also set, a checksum precedes each block instead.
2. The file data split into blocks. Each block is compressed using the selected algorithm. Without compression the block size is `0` and each file is stored as one block.

Checksums appear either once per file or before every block depending on the combination of `fChecksums` and `fBlockChecksums`. They are written using the algorithm and length specified in the header. Block boundaries are independent for each file and no padding is inserted between blocks or between the checksum and the following data.

## Trailer

The trailer starts at the offset recorded in the header. Layout:

```
[Block Count uint32]
[ [Offset uint64][Size uint64] ... ]
[Trailer Checksum: checksum length from header]
```

Offsets are absolute from the start of the archive. The trailer checksum covers everything from the `Block Count` field up to the end of the last block entry.

The block index allows random access to the compressed data. Each entry records the absolute offset and compressed size of one block. When block checksums are enabled an additional checksum of the block immediately follows the block data. Readers should verify the trailer checksum before trusting any offsets.

## Notes

- Directories containing files are implied; only empty directories are listed.
- Compression and checksums are applied per file.
- Special file entries contain metadata but no data.
- See `create()` in the source for a working reference implementation.
- File data blocks are written sequentially in the same order as file entries.
  The trailer exists so that a reader can efficiently locate the data for any
  file without scanning the entire archive.
- When block checksums are enabled, the checksum length is repeated for each
  block so that streams can be verified on the fly.
