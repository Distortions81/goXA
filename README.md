# GoXA -- Go eXpress Archive
<img src="https://github.com/Distortions81/goXA/blob/main/Xango.png?raw=true" alt="Xango the Archivist" width="300"/>

## Xango the Pangolin Archivist
GoXA is a gopher-friendly archiver written in Go. It's quick and simple, and still new enough that the paint is drying. Expect the odd bump and let the busy gophers know if you hit one.

## Features

- [x] Fast archive creation and extraction
- [x] Multiple compression formats (gzip, zstd, lz4, s2, snappy, brotli)
- [x] Optional BLAKE2b-256 checksums
- [x] Preserve permissions and modification times
- [x] Fully documented binary format ([file-format.md](file-format.md))
- [x] Optional support for symlinks and other special files
- [x] Pure go code, no dependencies once compiled.

## File Format

See [file-format.md](file-format.md) for the full binary format.

## Install

With Go 1.24+:

```bash
go install github.com/Distortions81/goXA@latest
```

Or build from source:

```bash
git clone https://github.com/Distortions81/goXA.git
cd goXA
go build
```

This creates the `goxa` binary.

## Usage

```bash
goxa [mode] [flags] -arc=archiveFile [paths...]
```

`mode`: `c` (create), `l` (list), `x` (extract)

`flags`: any combination of:

| Flag | Description |
|------|-------------|
| `a` | Absolute paths |
| `p` | File permissions |
| `m` | Modification times |
| `s` | Enable BLAKE2b checksums |
| `n` | Disable compression |
| `i` | Hidden files |
| `o` | Special files |
| `u` | Use flags from archive |
| `v` | Verbose logging |
| `f` | Overwrite files / ignore read errors |

Paths default to relative. Using `a` when extracting restores absolute paths if archived with them. By default extraction does not restore permissions, modification times, hidden files, or special files unless `p`, `m`, `i`, or `o` are specified (or `u` to use flags from archive).

### Extra Flags

| Flag | Description |
|------|-------------|
| `-arc=` | Archive file name |
| `-stdout` | Output archive to stdout |
| `-files` | Comma-separated list of files and directories to extract |
| `-progress=false` | Disable progress display |
| `-comp=` | Compression algorithm (gzip, zstd, lz4, s2, snappy, brotli, none) |

Progress shows transfer speed and the current file being processed.

### Examples

```bash
goxa c -arc=mybackup.goxa myStuff/
goxa capmsif -arc=mybackup.goxa ~/
goxa x -arc=mybackup.goxa
goxa xu -arc=mybackup.goxa     # use flags in archive (aka auto)
goxa l -arc=mybackup.goxa
```

## Roadmap

- [x] Format documentation
- [x] Working relative path support
- [x] Add modes for non-files (symlinks, devices)
- [ ] Multi-threaded archive optimization
- [ ] Archive signatures for optional additional security
- [ ] Archive comment field
- [ ] Encrypted archives

## Security Notes

- Paths are sanitized during extraction, but -a 'absolute paths' lets archives write wherever they like. Use with care on unknown files.
- When using -o 'special files' symlinks are not resolved, so sneaky links can sidestep your destination folder.
- The -u 'Use flags from in archive' uses whatever flags were used to create the archive. This can allow absolute paths, permissions, mod dates and special files. Use with care on unknown files.
- Size fields use 'int64'; so maximum file size is 9223 petabytes.

## License

MIT License.

## Author

- https://github.com/Distortions81

---

**GoXA** — fast, clean, and gopher-approved archiving.
