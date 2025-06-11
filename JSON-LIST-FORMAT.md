# Extended List JSON Format

The `j` mode of `goxa` prints a JSON document describing the archive without
extracting it. This JSON is intended for tooling and scripts that need to inspect
an archive programmatically. It mirrors the information stored in the header and
trailer but intentionally omits internal offsets and raw block data so that the
structure remains stable across format versions.

```json
{
  "version": 2,
  "flags": ["Checksums"],
  "compression": "zstd",
  "checksum": "blake3",
  "checksumLength": 32,
  "blockSize": 524288,
  "archiveSize": 12345,
  "dirs": [
    { "path": "emptyDir", "mode": 493, "modTime": 1672671845 },
    { "path": "logs", "modTime": 1672671845 }
  ],
  "files": [
    { "path": "file.txt", "type": "file", "size": 12,
      "mode": 420, "modTime": 1672671845 },
    { "path": "link", "type": "symlink", "link": "file.txt" },
    { "path": "special", "type": "other" }
  ]
}
```

### Field Descriptions

* **`version`** – archive format version (`uint16`). Future versions may add
  optional fields, so consumers should ignore unknown properties.
* **`flags`** – array of feature flag names corresponding to those defined in
  the binary format.
* **`compression`** – name of the compression algorithm used for file data.
* **`checksum`** – checksum algorithm identifier.
* **`checksumLength`** – size of each checksum in bytes.
* **`blockSize`** – preferred compression block size. When zero, data is stored
  uncompressed.
* **`archiveSize`** – total archive size in bytes as stored in the header.
* **`dirs`** – list of empty directories in the archive.
* **`files`** – list describing each archived file.
* **`modTime`** – seconds since the Unix epoch.

Each directory may include `mode` and `modTime` when stored. File entries contain a `path`, `type`, and `size` (except for `other` types). Symlinks and hardlinks include a `link` field with the target path. Optional `mode` and `modTime` fields appear when present in the archive.

`flags`, `compression`, and `checksum` correspond to the tables in [FILE-FORMAT.md](FILE-FORMAT.md).
The recognized flag names are:

- "Absolute Paths" – store absolute paths exactly as provided
- "Permissions" – preserve file permissions on extraction
- "Modification Times" – preserve modification times
- "Checksums" – include per-file checksums
- "No Compress" – disable compression for file data
- "Hidden Files" – include files beginning with a dot
- "Special Files" – archive symlinks and other special files
- "Block Checksums" – store per-block checksums

"None" is reserved and does not correspond to a feature. "Unknown" may appear
when future flags are encountered. Tools should treat unknown flags as
informational and continue processing the document. Additional fields may be
introduced by newer versions and should be ignored if unrecognized.

