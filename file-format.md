
# Archive File Format Specification

This document describes the GoXA binary file format (v1).

## Overview

The archive format is a custom, feature-rich container for storing files and directories. Features include:

- Optional file permissions and modification timestamps.
- Optional per-file checksums (BLAKE2b-256).
- Optional compression (gzip, per-file).
- Support for empty directories.
- File offset table for random access.

The archive file is written entirely in little-endian format.

## File Structure

The archive consists of:

```
[Header]
[Per-file Data Section]
```

The **Header** contains:

- Magic bytes and version.
- Feature flags.
- Empty directory list.
- File entries (size, metadata, paths).
- Offset table (to locate file data).

The **Per-file Data Section** contains:

- (optional) Checksum (32 bytes, if enabled)
- File data (raw or compressed)

---

## Format Details

### Magic Header

| Field       | Type       | Size (bytes) | Description          |
|-------------|------------|--------------|----------------------|
| Magic Bytes | bytes      | 4            | Format identifier (`GOXA`) |
| Version     | uint16     | 2            | Format version (1) |
| Features    | uint32     | 4            | Bitmask of enabled features |

### Feature Flags

The features field is a 32-bit mask. Possible flags include:

| Flag            | Bit | Description |
|------------------|-----|------------------------------|
| `fNone`          | 0x1 | Reserved |
| `fAbsolutePaths` | 0x2 | Store absolute paths |
| `fPermissions`   | 0x4 | Store file/directory permissions |
| `fModDates`      | 0x8 | Store file modification times |
| `fChecksums`     | 0x10 | Include per-file BLAKE2b checksums |
| `fNoCompress`    | 0x20 | Disable compression |
| `fIncludeInvis`  | 0x40 | Include invisible files |

> Note: multiple flags may be combined.

---

### Empty Directories

Empty directories are stored first.

| Field            | Type   | Description |
|------------------|--------|-------------|
| Empty Dir Count  | uint64 | Number of empty directories |

For each empty directory:

| Field             | Type   | Description |
|-------------------|--------|-------------|
| Mode (optional)   | uint32 | Directory permissions (if `fPermissions`) |
| ModTime (optional)| int64  | UNIX timestamp (if `fModDates`) |
| Path Length       | uint16 | Byte count |
| Path              | string | Path as UTF-8 string |

---

### File Entries

File metadata follows empty directories.

| Field           | Type   | Description |
|-----------------|--------|-------------|
| File Count      | uint64 | Number of files |

For each file:

| Field             | Type   | Description |
|-------------------|--------|-------------|
| Uncompressed Size | uint64 | File size (before compression) |
| Mode (optional)   | uint32 | File permissions (if `fPermissions`) |
| ModTime (optional)| int64  | UNIX timestamp (if `fModDates`) |
| Path Length       | uint16 | Byte count |
| Path              | string | Path as UTF-8 string |

---

### Offset Table

Following the file entries is the offset table.

- Threaded mode is always enabled, so the offset table is preallocated.
- Each file has an 8-byte offset (uint64) indicating its starting position in the data section.
- Offsets are absolute file offsets (relative to beginning of archive file).

---

### Per-file Data Section

For each file:

1. (Optional) **Checksum (BLAKE2b-256):**  
   - 32 bytes if `fChecksums` is set.

2. **File Data:**  
   - Raw data if `fNoCompress` is set.  
   - Otherwise, data is compressed using gzip.

---

## Example Layout

```
[Magic][Version][Features]
[Empty Dir Count][Empty Dir Entries...]
[File Count][File Entries...]
[Offset Table]
[File Checksums and Data...]
```

---

## Notes

- Directories with files are implicit; empty directories are explicitly listed.
- Compression and checksums are applied per file.
- The offset table allows seeking directly to file data without reading entire archive.

---

## Versioning

The current format version is 1.

---

## Reference Implementation

See: `create()` function in source.
