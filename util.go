package main

import (
	"bytes"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"compress/gzip"

	"github.com/ulikunitz/xz"
)

func fileExists(filePath string) (bool, error) {
	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func removeExtension(filename string) string {
	extension := filepath.Ext(filename)
	return filename[:len(filename)-len(extension)]
}

// detectEncodingFromExt checks for .b32, .b64 or .fec suffixes.
// It returns the filename without the encoding extension and the encoding type.
func detectEncodingFromExt(name string) (string, string) {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".b32") {
		return name[:len(name)-4], "b32"
	}
	if strings.HasSuffix(lower, ".b64") {
		return name[:len(name)-4], "b64"
	}
	if strings.HasSuffix(lower, ".fec") {
		return name[:len(name)-4], "fec"
	}
	return name, ""
}

// detectFormatFromExt inspects the archive filename to infer the format.
// It returns "tar" or "goxa" and whether the tar archive is uncompressed.
func detectFormatFromExt(name string) (string, bool) {
	var enc string
	name, enc = detectEncodingFromExt(name)
	if enc != "" {
		encode = enc
	}
	lower := strings.ToLower(name)
	tarUseXz = false
	if strings.HasSuffix(lower, ".tar.gz") {
		return "tar", false
	}
	if strings.HasSuffix(lower, ".tar.xz") {
		tarUseXz = true
		return "tar", false
	}
	if strings.HasSuffix(lower, ".tar") {
		return "tar", true
	}
	if strings.HasSuffix(lower, ".goxa") {
		return "goxa", false
	}
	return "", false
}

// stripArchiveExt removes a known archive extension from name.
func stripArchiveExt(name string) string {
	name, _ = detectEncodingFromExt(name)
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return name[:len(name)-len(".tar.gz")]
	case strings.HasSuffix(lower, ".tar.xz"):
		return name[:len(name)-len(".tar.xz")]
	case strings.HasSuffix(lower, ".tar"):
		return name[:len(name)-len(".tar")]
	case strings.HasSuffix(lower, ".goxa"):
		return name[:len(name)-len(".goxa")]
	default:
		return name
	}
}

func hasKnownArchiveExt(name string) bool {
	name, _ = detectEncodingFromExt(name)
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tar.xz") || strings.HasSuffix(lower, ".tar") || strings.HasSuffix(lower, ".goxa")
}

// decodeIfNeeded decodes a Base32, Base64 or FEC encoded archive to a temporary file
// and returns the new filename and a cleanup function.
func decodeIfNeeded(name string) (string, func(), error) {
	if encode == "" {
		return name, func() {}, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	tmp, err := os.CreateTemp("", "goxa_dec_*")
	if err != nil {
		return "", nil, err
	}

	var r io.Reader = f
	if encode == "b32" {
		r = base32.NewDecoder(base32.StdEncoding, f)
	} else if encode == "b64" {
		r = base64.NewDecoder(base64.StdEncoding, f)
	} else if encode == "fec" {
		f.Close()
		os.Remove(tmp.Name())
		return decodeWithFEC(name)
	}
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", nil, err
	}
	tmp.Close()
	return tmp.Name(), func() { os.Remove(tmp.Name()) }, nil
}

// detectTarHeader reports whether buf appears to be a tar archive by checking
// for the ustar magic at the expected offset.
func detectTarHeader(buf []byte) bool {
	if len(buf) < 262 {
		return false
	}
	return string(buf[257:262]) == "ustar"
}

// detectFormatFromHeader attempts to identify the archive format based on the
// file's magic bytes. The returned bool indicates success.
func detectFormatFromHeader(name string) (string, bool, bool) {
	path := name
	cleanup := func() {}
	if encode != "" {
		var err error
		path, cleanup, err = decodeIfNeeded(name)
		if err != nil {
			return "", false, false
		}
		defer cleanup()
	}
	f, err := os.Open(path)
	if err != nil {
		return "", false, false
	}
	defer f.Close()

	hdr := make([]byte, 6)
	n, _ := io.ReadFull(f, hdr)
	hdr = hdr[:n]

	if n >= 4 && string(hdr[:4]) == magic {
		return "goxa", false, true
	}

	if n >= 2 && hdr[0] == 0x1f && hdr[1] == 0x8b { // gzip
		if _, err := f.Seek(0, io.SeekStart); err == nil {
			gr, err := gzip.NewReader(f)
			if err == nil {
				buf := make([]byte, 512)
				m, _ := io.ReadFull(gr, buf)
				gr.Close()
				if detectTarHeader(buf[:m]) {
					return "tar", false, true
				}
			}
		}
	}

	if n >= 6 && bytes.Equal(hdr, []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}) { // xz
		if _, err := f.Seek(0, io.SeekStart); err == nil {
			xr, err := xz.NewReader(f)
			if err == nil {
				buf := make([]byte, 512)
				m, _ := io.ReadFull(xr, buf)
				if detectTarHeader(buf[:m]) {
					tarUseXz = true
					return "tar", false, true
				}
			}
		}
	}

	if _, err := f.Seek(0, io.SeekStart); err == nil {
		buf := make([]byte, 512)
		m, _ := io.ReadFull(f, buf)
		if detectTarHeader(buf[:m]) {
			return "tar", true, true
		}
	}

	return "", false, false
}

func doLog(verbose bool, format string, args ...interface{}) {
	if toStdOut || (!verboseMode && verbose) {
		return
	}

	var text string
	if args == nil {
		text = format
	} else {
		text = fmt.Sprintf(format, args...)
	}

	if verbose {
		ctime := time.Now()
		_, filename, line, _ := runtime.Caller(1)
		date := fmt.Sprintf("%2v:%2v.%2v", ctime.Hour(), ctime.Minute(), ctime.Second())
		fmt.Printf("%v: %15v:%5v: %v\n", date, filepath.Base(filename), line, text)
	} else {
		fmt.Println(text)
	}
}

// safeJoin joins base and target, ensuring the result stays within base.
func safeJoin(base, target string) (string, error) {
	cleanBase := filepath.Clean(base)
	cleanTarget := filepath.Clean(target)

	if filepath.IsAbs(cleanTarget) {
		cleanTarget = strings.TrimPrefix(cleanTarget, string(os.PathSeparator))
	}

	joined := filepath.Join(cleanBase, cleanTarget)
	joined = filepath.Clean(joined)

	prefix := cleanBase + string(os.PathSeparator)
	if joined != cleanBase && !strings.HasPrefix(joined, prefix) {
		return "", fmt.Errorf("illegal path: %s", target)
	}

	return joined, nil
}

// storedPath returns the archived path for fullPath based on root and the
// fAbsolutePaths flag. When absolute paths are disabled, paths are stored
// relative to the provided root's basename.
func storedPath(root, fullPath string) string {
	cleanFull := filepath.Clean(fullPath)

	if features.IsSet(fAbsolutePaths) {
		if filepath.IsAbs(cleanFull) {
			return cleanFull
		}
		abs, err := filepath.Abs(cleanFull)
		if err == nil {
			return abs
		}
		return cleanFull
	}

	cleanRoot := filepath.Clean(root)
	base := filepath.Base(cleanRoot)
	if base == "." {
		base = ""
	}

	rel, err := filepath.Rel(cleanRoot, cleanFull)
	if err != nil {
		rel = filepath.Base(cleanFull)
	}
	if rel == "." {
		rel = ""
	}

	if base == "" {
		return filepath.Clean(rel)
	}

	if rel == "" {
		return filepath.Clean(base)
	}

	return filepath.Join(base, rel)
}

func walkPaths(roots []string) (dirs []FileEntry, files []FileEntry, err error) {
	type dirState struct {
		entryCount int
		info       os.FileInfo
	}
	states := make(map[string]*dirState)

	for _, root := range roots {
		info, err := os.Lstat(root)
		if err != nil {
			return nil, nil, err
		}
		root = filepath.Clean(root)

		// File case
		if !info.IsDir() {
			if features.IsSet(fIncludeInvis) || !strings.HasPrefix(info.Name(), ".") {
				if info.Mode().IsRegular() || features.IsSet(fSpecialFiles) {
					metaData := gatherMeta(storedPath(root, root), root, info)
					files = append(files, metaData)
				}
			}
			continue
		}

		// Directory case
		states[storedPath(root, root)] = &dirState{info: info}
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path == root {
				return nil
			}

			name := d.Name()
			if features.IsNotSet(fIncludeInvis) && strings.HasPrefix(name, ".") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			parentKey := storedPath(root, filepath.Dir(path))
			if st, ok := states[parentKey]; ok {
				st.entryCount++
			}

			if d.IsDir() {
				info, err := d.Info()
				if err != nil {
					return err
				}
				states[storedPath(root, path)] = &dirState{info: info}
			} else {
				info, err := d.Info()
				if err != nil {
					return err
				}
				if info.Mode().IsRegular() || features.IsSet(fSpecialFiles) {
					files = append(files, gatherMeta(storedPath(root, path), path, info))
				}
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}

	// Collect only those dirs with zero entries
	for path, st := range states {
		if st.entryCount == 0 {
			dirs = append(dirs, gatherMeta(path, path, st.info))
		}
	}

	// Sort for deterministic ordering
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Path < dirs[j].Path })
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	return dirs, files, nil
}

// gatherMeta pulls the common metadata for a path.
func gatherMeta(path, src string, info os.FileInfo) FileEntry {
	entry := FileEntry{
		Path:    path,
		SrcPath: src,
		Size:    uint64(info.Size()),
		ModTime: info.ModTime(),
	}
	if features.IsSet(fPermissions) {
		entry.Mode = info.Mode()
	}
	switch {
	case info.Mode().IsRegular():
		entry.Type = entryFile
	case info.Mode()&os.ModeSymlink != 0:
		entry.Type = entrySymlink
		entry.Size = 0
		if link, err := os.Readlink(src); err == nil {
			entry.Linkname = link
		}
	default:
		entry.Type = entryOther
		entry.Size = 0
	}
	return entry
}

// isSelected checks if the provided path matches one of the
// user-specified extractList entries. When no list is specified,
// it always returns true.
func isSelected(p string) bool {
	if len(extractList) == 0 {
		return true
	}
	clean := filepath.Clean(p)
	for _, f := range extractList {
		f = strings.TrimSuffix(filepath.Clean(f), string(os.PathSeparator))
		if clean == f || strings.HasPrefix(clean, f+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}
