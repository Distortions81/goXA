
# GoXA -- Go eXpress Archive

**GoXA** is a custom archival format and tool written in Go. It provides a simple, efficient, and feature-rich alternative to traditional archival formats like tar or zip, with an emphasis on data integrity, flexibility, and extensibility.
> ⚠️ **Early Development**: This project is still experimental and under active development. Expect bugs, untested behavior, and breaking changes. Use at your own risk.<br>
> See the [issues list](https://github.com/Distortions81/goXA/issues)


## Features

- ✅ Fast archive creation and extraction
- ✅ Per-file compression (gzip, optional)
- ✅ Per-file checksums (BLAKE2b-256, optional)
- ✅ Preservation of permissions and modification timestamps (optional)
- ✅ Empty directory support
- ✅ Simple, fully documented binary format ([file-format.md](file-format.md))
- ✅ Clean Go codebase, easy to extend
- ✅ No external dependencies, self-contained

## File Format

The full GoXA binary file format is documented here: [file-format.md](file-format.md).

## Install

You can build GoXA easily using Go 1.20+:

```bash
git clone https://github.com/Distortions81/goXA.git
cd goXA
go build
```

This will produce the `goxa` binary.

## Usage

### Command Syntax

```
goxa [mode][options] -arc=archiveFile [additional arguments]
```

- `mode` (required): one of:
  - `c` = create
  - `l` = list contents
  - `x` = extract

- `options` (optional): any combination of the following single-character flags:

| Flag | Description |
|------|-------------|
| `a` | Store absolute paths |
| `p` | Preserve file/directory permissions |
| `m` | Preserve modification timestamps |
| `s` | Enable BLAKE2b checksums |
| `n` | Disable compression |
| `i` | Include invisible files |
| `v` | Verbose logging |
| `f` | Force overwrite existing files / ignore read errors |

Paths are stored relative to the given inputs by default. Use `a` to preserve
absolute paths during creation and extraction.

### Additional Global Flags

| Flag | Description |
|------|-------------|
| `-arc=` | Specify archive file name |
| `-stdout` | Output archive to stdout |
| `-progress=false` | Disable progress bar display |

### Examples

**Create Archive:**

```bash
goxa c -arc=mybackup.goxa myStuff/
```

**Full backup (like tar+gz):**

```bash
goxa capmsif -arc=mybackup.goxa ~/
```

**Extract Archive:**

```bash
goxa x -arc=mybackup.goxa
```

**List Archive Contents:**

```bash
goxa l -arc=mybackup.goxa
```

## Roadmap Ideas

- ✅ Format documentation (complete)
- 🛠 Working relative path support
- 🛠 Add modes to allow non-files (symlinks, devices)
- 🛠 Random-access extraction mode
- 🛠 Multi-threaded archive optimization (blocks, v2 format)
- 🛠 Additional compression formats
- 🛠 Go 1.24+ os.Root directory jails
- 🛠 Archive signatures for optional additional security
- 🛠 Archive comment field
- 🛠 Encrypted archives

## License

This project is licensed under the MIT License.

## Author

- https://github.com/Distortions81

---

**GoXA** — fast, clean, reliable archiving in Go.
