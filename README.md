# GoXA — Go eXpress Archive
"An archive isn’t only storage—it’s a promise to the future." – Unknown

<img src="https://github.com/Distortions81/goXA/blob/main/Xango.png?raw=true" alt="Xango the Archivist" width="300"/>

## Xango the Pangolin Archivist
GoXA is a small archiver written in Go. It's quick and friendly, though still learning new tricks. Let us know if you spot a bug.

## Features

- Fast archive creation and extraction
- Compression: gzip, zstd, lz4, s2, snappy, brotli or xz (default zstd)
- Tar compatibility (auto-detected by filename or header)
- Optional checksums: CRC32, CRC16, XXHash3, SHA-256 or Blake3 (default Blake3)
- Preserve permissions and modification times
- Fully documented binary format ([FILE-FORMAT.md](FILE-FORMAT.md))
- Archive symlinks and other special files
- Block-based compression for speed
- Automatic format detection
- Stream archives to stdout
- Selective extraction with `-files`
- Progress bar with transfer speed and current file
- Pure Go code—no runtime deps once built
- Base32/64 encoding when the archive filename ends with `.b32` or `.b64`

## File Format

See [FILE-FORMAT.md](FILE-FORMAT.md) for the full binary format.
The JSON structure emitted by `j` mode is described in
[JSON-LIST-FORMAT.md](JSON-LIST-FORMAT.md).

## Install

With Go 1.24+:
```bash
go install github.com/Distortions81/goXA@latest
```

To build from source:
```bash
git clone https://github.com/Distortions81/goXA.git
cd goXA
go build
```

The script `./install.sh` builds and installs the binary and man page for you.
See `goxa.1` for the full command reference.

## Usage

```bash
goxa [mode] [flags] -arc=FILE [paths...]
```

Modes are:

* `c` – create an archive
* `l` – list contents
* `j` – JSON list
* `x` – extract files

Flags (combine as needed):

| Flag | Description |
|------|-------------|
| `a` | Absolute paths |
| `p` | File permissions |
| `m` | Modification times |
| `s` | Enable checksums |
| `b` | Per-block checksums |
| `n` | Disable compression |
| `i` | Hidden files |
| `o` | Special files |
| `u` | Use flags from archive |
| `v` | Verbose logging |
| `f` | Overwrite files / ignore read errors |

Paths are stored relative by default. Use `a` to store and restore absolute paths. Extraction only restores permissions, modification times, hidden files, or special files when `p`, `m`, `i`, or `o` are given (or `u` to use the archive's flags).

### Extra Flags

| Flag | Description |
|------|-------------|
| `-arc=` | Archive file name |
| `-stdout` | Output archive to stdout |
| `-files` | Comma-separated list of files and directories to extract |
| `-progress=false` | Disable progress display |
| `-comp=` | Compression algorithm (gzip, zstd, lz4, s2, snappy, brotli, xz, none) |
| `-speed=` | Compression speed (fastest, default, better, best) |
| `-format=` | Archive format (`goxa` or `tar`) |
| `-retries=` | Retries when a file changes during read (0=never give up) |
| `-retrydelay=` | Delay between retries in seconds |
| `-failonchange` | Treat changed files as fatal errors |
| `-version` | Print version and exit |

Progress shows transfer speed and the current file being processed.
Snappy does not support configurable compression levels; `-speed` has no effect when using snappy.

### Base32/Base64 Archives

Appending `.b32` or `.b64` to the archive filename encodes the output in Base32
or Base64. The same suffix triggers automatic decoding when extracting or
listing. For example:

```bash
goxa c -arc=mybackup.goxa.b64 myStuff/  # create Base64 encoded archive
goxa x -arc=mybackup.goxa.b64           # extract encoded archive
goxa c -arc=mybackup.goxa.b32 myStuff/  # create Base32 encoded archive
```

### Examples

```bash
goxa -version                                 # print version
goxa c -arc=mybackup.goxa myStuff/            # create archive
goxa capmsif -arc=mybackup.goxa ~/            # create using all flags
goxa x -arc=mybackup.goxa                     # extract to folder
goxa xu -arc=mybackup.goxa                    # extract using archive flags
goxa l -arc=mybackup.goxa                     # list contents
goxa j -arc=mybackup.goxa > listing.json      # JSON listing
goxa c -arc=mybackup.tar.gz myStuff/          # create tar.gz
goxa x -arc=mybackup.tar.gz                   # extract tar.gz
goxa c -arc=mybackup.tar.xz myStuff/          # create tar.xz
goxa x -arc=mybackup.tar.xz                   # extract tar.xz
goxa c -arc=mybackup.goxa -stdout myStuff/ | ssh host "cat > backup.goxa"  # stream over SSH
goxa x -arc=mybackup.goxa -files=file.txt,dir/ # selective extract
```

## Roadmap

- [ ] Multi-threaded archive optimization
- [ ] Archive signatures for optional additional security
- [ ] Archive comment field
- [ ] Encrypted archives
- [ ] Create goxa library

## Testing

`go test ./...` runs the built-in unit and integration tests. The
`test-goxa.sh` script builds the CLI and performs real-world archive
creation and extraction checks.

## Security Notes

- Paths are sanitized during extraction, but `-a` (`absolute paths`) allows the archive to write anywhere. Use with care on unknown files.
- With `-o` (`special files`) symlinks are not resolved, so sneaky links can sidestep your destination folder.
 - `-u` (`use flags from archive`) applies whatever options were set when the archive was created, which may enable absolute paths, permissions, modification times and special files.
- Size fields use `int64`; maximum individual file size is about 9,223 petabytes (~8&nbsp;EiB).

## License

MIT License.

## Author

- https://github.com/Distortions81
- AI-assisted contributions using OpenAI's ChatGPT

---

**GoXA** — fast, clean, and gopher-approved archiving.
