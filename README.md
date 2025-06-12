# 🗜️ goXA – Go eXpress Archive

A fast, portable archiving utility written in Go.

> **Pronounced**:  
> - **Phonetic**: `go-eks-ah`  
> - **IPA**: `/ˈɡoʊ.ɛks.ɑ/`  

<img src="https://github.com/Distortions81/goXA/blob/main/Xango.png?raw=true" alt="Xango the Archivist" width="300"/>

### Xango the Pangolin Archivist

> _"An archive isn’t only storage — it’s a promise to the future."_
---

> ⚠️ **Warning: goXA is not yet complete.**  
> This project is under active development — expect **breaking changes**, incomplete features, and **bugs**.  
> Please report any [issues](https://github.com/Distortions81/goXA/issues).

---

## Features

- Fast archive creation and extraction
- Compression: gzip, zstd, lz4, s2, snappy, brotli or xz (default `zstd`)
- Optional checksums: CRC32, CRC16, XXHash3, SHA-256 or Blake3
- Preserve permissions and modification times
- Archives symlinks and special files
- Automatic format detection
- Progress bar with transfer speed and current file
- Base32, Base64 and FEC `error correcting` encoding when the archive name ends with `.b32`, `.b64` or `.goxaf`
- Fully documented format: see [FILE-FORMAT.md](FILE-FORMAT.md) and [JSON-LIST-FORMAT.md](JSON-LIST-FORMAT.md)

## Installation

With Go 1.24+ you can install directly:

```bash
go install github.com/Distortions81/goXA@latest
```

To build from source:

```bash
git clone https://github.com/Distortions81/goXA.git
cd goXA
go build
```

The script `install.sh` builds the binary and installs the man page.

## Usage

```
goxa MODE[flags] [options] -arc FILE [paths...]
```

`MODE` is one of:

- `c` – create an archive
- `l` – list contents
- `j` – output JSON list
- `x` – extract files

Single letter flags follow the mode, e.g. `goxa cpms -arc=out.goxa dir/`.
Longer options use the usual `-flag=value` form.

### Common Flags

| Flag | Meaning |
|------|---------|
| `a` | store absolute paths |
| `p` | preserve permissions |
| `m` | preserve modification times |
| `s` | include checksums |
| `b` | per-block checksums |
| `n` | disable compression |
| `i` | include hidden files |
| `o` | allow special files |
| `u` | use flags stored in archive |
| `v` | verbose output |
| `f` | force overwrite / ignore read errors |

When extracting, the program prompts if the archive was created with
flags you did not specify. It will ask which missing flags to enable, or
`u` to enable all. Press Enter to continue without them. Use
`-interactive=false` to skip prompts.

### Options

| Option | Description |
|--------|-------------|
| `-arc` | archive file name |
| `-stdout` | write archive to stdout |
| `-files` | comma-separated list to extract |
| `-progress=false` | disable progress display |
| `-interactive=false` | disable prompts for archive flags |
| `-comp` | compression algorithm |
| `-speed` | compression speed level |
| `-format` | force `goxa` or `tar` format |
| `-retries` | retries when a file changes during read |
| `-retrydelay` | seconds to wait between retries |
| `-failonchange` | treat changed files as fatal errors |
| `-bombcheck=false` | disable zip bomb detection |
| `-version` | print program version |
| `-fec-data` | number of FEC data shards |
| `-fec-parity` | number of FEC parity shards |
| `-fec-level` | redundancy preset: low, medium or high |

Progress shows transfer speed and current file. Snappy does not support adjustable levels; `-speed` is ignored when using it.

### Base32 / Base64 / FEC

Appending `.b32` or `.b64` to the archive file encodes the archive in Base32 or Base64. Files ending in `.goxaf` are FEC `error correcting` encoded. FEC archives contain data and parity shards; any missing shards up to the parity count can be reconstructed when extracting. For example, with `-fec-data=10 -fec-parity=3` the archive is split into 13 shards. Any 10 shards are enough to fully recover the data. Presets are:

```
low    -> 10 data / 3 parity
medium -> 8 data / 4 parity
high   -> 5 data / 5 parity
```

Examples:

```bash
goxa c -arc=backup.goxa.b64 mydir/    # Base64 archive
goxa c -arc=backup.goxaf mydir/       # FEC encoded archive
goxa c -arc=backup.goxaf -fec-parity=5 mydir/
```

## General Use Examples

```bash
goxa c -arc=mybackup.goxa myStuff/            # create archive
goxa x -arc=mybackup.goxa                     # extract
goxa l -arc=mybackup.goxa                     # list contents
goxa c -arc=mybackup.tar.gz myStuff/          # create tar.gz
goxa x -arc=mybackup.tar.xz                   # extract tar.xz
goxa c -arc=mybackup.goxa -stdout myStuff/ | ssh host "cat > backup.goxa"
```

## Security Notes

- `-a` allows the archive to write anywhere when extracting.
- `-o` stores symlinks as is; malicious archives can use this to escape directories.
- `-u` applies flags embedded in the archive which may enable the above features.
- Maximum individual file size is roughly 9&nbsp;223&nbsp;PB (int64 limit).

## Testing

Run `go test ./...` for unit tests. The script `test-goxa.sh` performs end‑to‑end tests using real files.

## License

MIT

## Author

<https://github.com/Distortions81>

---

*GoXA — fast, clean and gopher‑approved archiving.*
