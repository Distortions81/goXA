# GoXA -- Go eXpress Archive
<img src="https://github.com/Distortions81/goXA/blob/main/Xango.png?raw=true" alt="Xango the Archivist" width="300"/>

## Xango the Pangolin Archivist
GoXA is a gopher-friendly archiver written in Go. It's quick and simple, and still new enough that the paint is drying. Expect the odd bump and let the busy gophers know if you hit one.

### New format coming soon, with data blocks for better multi-threaded compression.

## Features

- [x] Fast archive creation and extraction
- [x] Optional gzip compression
- [x] Optional BLAKE2b-256 checksums
- [x] Preserve permissions and modification times
- [x] Empty directory support
- [x] Fully documented binary format ([FILE-FORMAT.md](FILE-FORMAT.md))
- [x] Optional support for symlinks and other special files
- [x] Clean, dependency-free Go code

## File Format

See [file-format.md](file-format.md) for the full binary format.

## Install

With Go 1.20+:

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
| `a` | Store absolute paths |
| `p` | Preserve permissions |
| `m` | Preserve modification times |
| `s` | Enable BLAKE2b checksums |
| `n` | Disable compression |
| `i` | Include hidden files |
| `o` | Include special files |
| `v` | Verbose logging |
| `f` | Force overwrite / ignore errors |

Paths default to relative. Using `a` when extracting restores absolute paths.

### Extra Flags

| Flag | Description |
|------|-------------|
| `-arc=` | Archive file name |
| `-stdout` | Output archive to stdout |
| `-progress=false` | Disable progress display |

Progress shows transfer speed and the current file being processed.

### Examples

```bash
goxa c -arc=mybackup.goxa myStuff/
goxa capmsif -arc=mybackup.goxa ~/
goxa x -arc=mybackup.goxa
goxa l -arc=mybackup.goxa
```

## Roadmap

- [x] Format documentation
- [x] Working relative path support
- [x] Add modes for non-files (symlinks, devices)
- [ ] Random-access extraction mode
- [ ] Multi-threaded archive optimization
- [ ] Additional compression formats
- [ ] Go 1.24+ os.Root directory jails
- [ ] Archive signatures for optional additional security
- [ ] Archive comment field
- [ ] Encrypted archives

## Security Notes

- Paths are sanitized during extraction, but `-a` lets archives write wherever they like. Use with care on unknown files.
- Symlinks are not resolved, so sneaky links can sidestep your destination folder.
- Size fields use `int64`; absurdly huge or corrupted sizes might crash the extractor.
- The flag parser shortens the options string as it goes; unusual flags might confuse it.

## License

MIT License.

## Author

- https://github.com/Distortions81

---

**GoXA** — fast, clean, and gopher-approved archiving.
