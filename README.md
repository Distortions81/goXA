# goXA

**goXA** is a fast, flexible file archiver written in Go — similar in spirit to `tar` and `gzip`, but designed with a simpler CLI and modern Go-based internals.

> ⚠️ **Early Development**: This project is still experimental and under active development. Expect bugs, untested behavior, and breaking changes. Use at your own risk.

---

## 🔧 Features

- Create, list, and extract archives
- Optional compression (gzip)
- Optional metadata: permissions, mod time, checksums, etc.
- Verbose and minimal modes
- Works with stdout for scripting and piping
- Single binary — no dependencies

---

## 🚀 Usage

```
Usage: goxa [c|l|x][apmsnive] -arc=arcFile [input paths/files...] or [destination]
Output archive to stdout: -stdout, No progress bar: -progress=false

Modes:
  c = Create a new archive. Requires input paths or files
  l = List archive contents. Requires -arc
  x = Extract files from archive. Requires -arc

Options:
  a = Absolute paths      p = Permissions
  m = Modification date   s = Sums
  n = No-compression      i = Include dotfiles
  v = Verbose logging     f = Force (overwrite files and ignore read errors)
```

---

## 🧪 Examples

Create an archive:

```bash
goxa c -arc=arcFile myStuff
# Similar to: zip or tar -cf
```

Create with metadata and compression:

```bash
goxa cpmi -arc=arcFile myStuff
# Similar to: tar -czf
```

List archive contents:

```bash
goxa l -arc=arcFile
```

Extract files:

```bash
goxa x -arc=arcFile
```

Extract with metadata:

```bash
goxa xpmi -arc=arcFile
# Similar to: tar -xzf
```

---

## 📦 Installation

Clone and build:

```bash
git clone https://github.com/Distortions81/goXA.git
cd goXA
go build -o goxa
```
