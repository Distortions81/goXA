package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

type fileSpec struct {
	rel  string
	data []byte
	perm fs.FileMode
}

func setupTestTree(t *testing.T, root string) []fileSpec {
	old := syscall.Umask(0)
	defer syscall.Umask(old)
	files := []fileSpec{
		{rel: "dir1/file1.txt", data: []byte("file1"), perm: 0o754},
		{rel: "dir1/.hidden", data: []byte("hidden1"), perm: 0o600},
		{rel: "dir2/file2.txt", data: []byte("file2"), perm: 0o640},
		{rel: ".hiddendir/hfile.txt", data: []byte("hidden2"), perm: 0o600},
		{rel: "rootfile.txt", data: []byte("root"), perm: 0o664},
	}

	for _, f := range files {
		full := filepath.Join(root, f.rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, f.data, f.perm); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	return files
}

func checkFile(t *testing.T, path string, expect []byte, perm fs.FileMode, checkPerm bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %v: %v", path, err)
	}
	if !bytes.Equal(data, expect) {
		t.Fatalf("content mismatch for %v", path)
	}
	if checkPerm {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %v: %v", path, err)
		}
		if info.Mode().Perm() != perm {
			t.Fatalf("perm mismatch for %v: got %o want %o", path, info.Mode().Perm(), perm)
		}
	}
}

func TestArchiveScenarios(t *testing.T) {
	cases := []struct {
		name         string
		createFlags  BitFlags
		extractFlags BitFlags
		expectHidden bool
		checkPerms   bool
	}{
		{"rel_compress", 0, 0, false, false},
		{"rel_nocompress", fNoCompress, 0, false, false},
		{"rel_invis", fIncludeInvis, 0, true, false},
		{"abs_compress", fAbsolutePaths, fAbsolutePaths, false, false},
		{"abs_nocompress", fAbsolutePaths | fNoCompress, fAbsolutePaths, false, false},
		{"abs_invis", fAbsolutePaths | fIncludeInvis, fAbsolutePaths, true, false},
		{"rel_all_flags", fPermissions | fChecksums | fIncludeInvis | fNoCompress, 0, true, true},
		{"abs_all_flags", fAbsolutePaths | fPermissions | fChecksums | fIncludeInvis | fNoCompress, fAbsolutePaths, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			root := filepath.Join(tempDir, "root")
			specs := setupTestTree(t, root)

			var oldUmask int
			if tc.checkPerms {
				oldUmask = syscall.Umask(0)
				defer syscall.Umask(oldUmask)
			}

			archivePath = filepath.Join(tempDir, "test.goxa")
			toStdOut = false
			doForce = false
			features = tc.createFlags

			cwd, _ := os.Getwd()
			os.Chdir(tempDir)
			defer os.Chdir(cwd)

			if err := create([]string{root}); err != nil {
				t.Fatalf("create failed: %v", err)
			}

			os.RemoveAll(root)
			features = tc.extractFlags

			var dest string
			if tc.extractFlags.IsSet(fAbsolutePaths) {
				extract([]string{}, false)
			} else {
				dest = filepath.Join(tempDir, "out")
				if err := os.MkdirAll(dest, 0o755); err != nil {
					t.Fatalf("mkdir dest: %v", err)
				}
				extract([]string{dest}, false)
			}

			var base string
			if tc.extractFlags.IsSet(fAbsolutePaths) {
				base = root
			} else {
				base = filepath.Join(dest, filepath.Base(root))
			}

			for _, sp := range specs {
				hidden := strings.Contains("/"+sp.rel, "/.")
				if hidden && !tc.expectHidden {
					if _, err := os.Stat(filepath.Join(base, sp.rel)); !os.IsNotExist(err) {
						t.Fatalf("hidden file should not exist: %v", sp.rel)
					}
					continue
				}
				checkFile(t, filepath.Join(base, sp.rel), sp.data, sp.perm, tc.checkPerms)
			}
		})
	}
}
