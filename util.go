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

func walkPaths(roots []string) (dirs []FileEntry, files []FileEntry, err error) {
	type dirState struct{ entryCount int }
	states := make(map[string]*dirState)

	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			return nil, nil, err
		}

		// File case
		if !info.IsDir() {
			if features.IsSet(fIncludeInvis) || !strings.HasPrefix(info.Name(), ".") {
				metaData := gatherMeta(root, info)

				if metaData.Mode&os.ModeSymlink != 0 {
					files = append(files, metaData)
				}
			}
			continue
		}

		// Directory case
		states[root] = &dirState{}
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

			parent := filepath.Dir(path)
			if st, ok := states[parent]; ok {
				st.entryCount++
			}

			if d.IsDir() {
				states[path] = &dirState{}
			} else {
				info, err := d.Info()
				if err != nil {
					return err
				}
				files = append(files, gatherMeta(path, info))
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
			info, err := os.Stat(path)
			if err != nil {
				return nil, nil, err
			}
			dirs = append(dirs, gatherMeta(path, info))
		}
	}

	// Sort for deterministic ordering
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Path < dirs[j].Path })
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	return dirs, files, nil
}

// gatherMeta pulls the common metadata for a path.
func gatherMeta(path string, info os.FileInfo) FileEntry {
	entry := FileEntry{
		Path:    path,
		Size:    uint64(info.Size()),
		ModTime: info.ModTime(),
	}
	if features.IsSet(fPermissions) {
		entry.Mode = info.Mode()
	}
	return entry
}
