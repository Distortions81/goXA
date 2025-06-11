# Extended List JSON Format

The `j` mode of `goxa` prints a JSON document describing the archive
without extracting it. The structure mirrors the information stored in
the header and trailer while omitting internal offsets and block data.

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

`flags` corresponds to the feature bits listed in
[FILE-FORMAT.md](FILE-FORMAT.md). `compression` and `checksum`
match the tables in the same document. Entries for symlinks and
hardlinks include a `link` field containing the target path.
