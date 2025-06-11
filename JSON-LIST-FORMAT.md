# Extended List JSON Format

The `j` mode of `goxa` prints a JSON document describing the archive without extracting it. The structure mirrors the information found in the header and trailer but omits internal offsets and block data.

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
    { "path": "emptyDir", "mode": 493, "modTime": 1672671845 }
  ],
  "files": [
    { "path": "file.txt", "type": "file", "size": 12,
      "mode": 420, "modTime": 1672671845 }
  ]
}
```

* `version` – archive format version (`uint16`)
* `flags` – feature flags as an array of names
* `compression` – compression algorithm name
* `checksum` – checksum algorithm name
* `checksumLength` – checksum size in bytes
* `blockSize` – compression block size
* `archiveSize` – total archive size in bytes
* `dirs` – array of empty directory entries
* `files` – array of file entries
* `modTime` – seconds since the Unix epoch

Each directory may include `mode` and `modTime` when stored. File entries contain a `path`, `type`, and `size` (except for `other` types). Symlinks and hardlinks include a `link` field with the target path. Optional `mode` and `modTime` fields appear when present in the archive.

`flags`, `compression`, and `checksum` correspond to the tables in [FILE-FORMAT.md](FILE-FORMAT.md).
The recognized flag names are:

- "Absolute Paths" – store absolute paths
- "Permissions" – preserve permissions
 - "Modification Times" – preserve modification times
 - "Checksums" – include checksums
 - "No Compress" – disable compression
 - "Hidden Files" – include hidden files
- "Special Files" – archive symlinks and other special files
- "Block Checksums" – store per-block checksums

"None" is reserved and does not correspond to a feature. "Unknown" may appear when future flags are encountered.

