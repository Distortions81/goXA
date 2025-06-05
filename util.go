package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/blake2b"
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

func Blake2b512FromPtr(dataPtr *[]byte) ([]byte, error) {
	// Create a new BLAKE2b-512 hasher (no key)
	h, err := blake2b.New512(nil)
	if err != nil {
		log.Fatal(err)
	}

	// Feed it the bytes
	h.Write(*dataPtr)

	// Return the digest
	return h.Sum(nil), nil
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
		info, err := os.Stat(root)
		if err != nil {
			return nil, nil, err
		}
		root = filepath.Clean(root)

		// File case
		if !info.IsDir() {
			if features.IsSet(fIncludeInvis) || !strings.HasPrefix(info.Name(), ".") {
				metaData := gatherMeta(storedPath(root, root), root, info)
				files = append(files, metaData)
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
				files = append(files, gatherMeta(storedPath(root, path), path, info))
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
	return entry
}
