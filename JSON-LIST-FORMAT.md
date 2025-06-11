# Extended List JSON Format

The `j` mode of `goxa` prints a JSON document describing the archive without extracting it. The structure mirrors the information found in the header and trailer but omits internal offsets and block data.

```json
{
  "version": 2,
  "flags": 16,
  "compression": "zstd",
  "checksum": "blake3",
  "checksumLength": 32,
  "blockSize": 524288,
  "archiveSize": 12345,
  "dirs": [
    { "path": "emptyDir", "mode": 493, "modTime": "2023-01-02T15:04:05Z" }
  ],
  "files": [
    { "path": "file.txt", "type": "file", "size": 12,
      "mode": 420, "modTime": "2023-01-02T15:04:05Z" }
  ]
}
```

* `version` – archive format version (`uint16`)
* `flags` – feature flags (`uint32`)
* `compression` – compression algorithm name
* `checksum` – checksum algorithm name
* `checksumLength` – checksum size in bytes
* `blockSize` – compression block size
* `archiveSize` – total archive size in bytes
* `dirs` – array of empty directory entries
* `files` – array of file entries

Each directory may include `mode` and `modTime` when stored. File entries contain a `path`, `type`, and `size` (except for `other` types). Symlinks and hardlinks include a `link` field with the target path. Optional `mode` and `modTime` fields appear when present in the archive.

`flags`, `compression`, and `checksum` correspond to the tables in [FILE-FORMAT.md](FILE-FORMAT.md).
