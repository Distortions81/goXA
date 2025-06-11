package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
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
		{"rel_invis", fIncludeInvis, fIncludeInvis, true, false},
		{"abs_compress", fAbsolutePaths, fAbsolutePaths, false, false},
		{"abs_nocompress", fAbsolutePaths | fNoCompress, fAbsolutePaths, false, false},
		{"abs_invis", fAbsolutePaths | fIncludeInvis, fAbsolutePaths | fIncludeInvis, true, false},
		{"rel_all_flags", fPermissions | fChecksums | fIncludeInvis | fNoCompress, fPermissions | fIncludeInvis, true, true},
		{"abs_all_flags", fAbsolutePaths | fPermissions | fChecksums | fIncludeInvis | fNoCompress, fAbsolutePaths | fPermissions | fIncludeInvis, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			features = 0
			version = version2

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
			features |= tc.createFlags

			cwd, _ := os.Getwd()
			os.Chdir(tempDir)
			defer os.Chdir(cwd)

			if err := create([]string{root}); err != nil {
				t.Fatalf("create failed: %v", err)
			}

			os.RemoveAll(root)
			features = 0
			features |= tc.extractFlags

			var dest string
			if tc.extractFlags.IsSet(fAbsolutePaths) {
				extract([]string{}, false, false)
			} else {
				dest = filepath.Join(tempDir, "out")
				if err := os.MkdirAll(dest, 0o755); err != nil {
					t.Fatalf("mkdir dest: %v", err)
				}
				extract([]string{dest}, false, false)
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

func TestArchiveParentRelative(t *testing.T) {
	tempDir := t.TempDir()
	parent := filepath.Join(tempDir, "parent")
	root := filepath.Join(parent, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data := []byte("hi")
	if err := os.WriteFile(filepath.Join(root, "file.txt"), data, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	work := filepath.Join(tempDir, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatalf("mkdir work: %v", err)
	}

	archivePath = filepath.Join(tempDir, "test.goxa")
	features = 0
	version = version2
	toStdOut = false
	doForce = false

	cwd, _ := os.Getwd()
	os.Chdir(work)
	defer os.Chdir(cwd)

	relRoot, _ := filepath.Rel(work, root)
	if err := create([]string{relRoot}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	os.RemoveAll(root)
	dest := filepath.Join(tempDir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	extract([]string{dest}, false, false)

	extracted := filepath.Join(dest, filepath.Base(root), "file.txt")
	checkFile(t, extracted, data, 0o644, false)
}

func TestSymlinkAndHardlink(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("requires creating hard links")
	}
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	data := []byte("hi")
	orig := filepath.Join(root, "file.txt")
	if err := os.WriteFile(orig, data, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	os.Symlink("file.txt", filepath.Join(root, "link.txt"))
	os.Link(orig, filepath.Join(root, "hard.txt"))

	archivePath = filepath.Join(tempDir, "test.goxa")
	features = fSpecialFiles
	version = version2
	toStdOut = false
	doForce = false

	if err := create([]string{root}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	os.RemoveAll(root)
	dest := filepath.Join(tempDir, "out")
	os.MkdirAll(dest, 0o755)
	features = fSpecialFiles
	extract([]string{dest}, false, false)

	base := filepath.Join(dest, filepath.Base(root))
	ltarget, err := os.Readlink(filepath.Join(base, "link.txt"))
	if err != nil || ltarget != "file.txt" {
		t.Fatalf("symlink mismatch")
	}
	if _, err := os.Lstat(filepath.Join(base, "hard.txt")); err != nil {
		t.Fatalf("hardlink missing: %v", err)
	}
}

func TestModDatePreservation(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "root")
	filePath := filepath.Join(root, "file.txt")
	dirPath := filepath.Join(root, "empty")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := []byte("hi")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	modTime := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	os.Chtimes(filePath, modTime, modTime)
	os.Chtimes(dirPath, modTime, modTime)

	archivePath = filepath.Join(tempDir, "test.goxa")
	features = fModDates
	toStdOut = false
	doForce = false

	if err := create([]string{root}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	os.RemoveAll(root)
	dest := filepath.Join(tempDir, "out")
	os.MkdirAll(dest, 0o755)
	features = fModDates
	extract([]string{dest}, false, false)

	base := filepath.Join(dest, filepath.Base(root))
	info, err := os.Stat(filepath.Join(base, "file.txt"))
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if info.ModTime().UTC().Truncate(time.Second) != modTime {
		t.Fatalf("file mod time mismatch: got %v want %v", info.ModTime(), modTime)
	}

	dInfo, err := os.Stat(filepath.Join(base, "empty"))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if dInfo.ModTime().UTC().Truncate(time.Second) != modTime {
		t.Fatalf("dir mod time mismatch: got %v want %v", dInfo.ModTime(), modTime)
	}
}

func TestBaseEncoding(t *testing.T) {
	cases := []struct{ enc string }{{"b64"}, {"b32"}}
	for _, tc := range cases {
		tempDir := t.TempDir()
		root := filepath.Join(tempDir, "root")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		data := []byte("hello")
		if err := os.WriteFile(filepath.Join(root, "file.txt"), data, 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		archivePath = filepath.Join(tempDir, "test.goxa."+tc.enc)
		encode = tc.enc
		features = 0
		version = version2
		toStdOut = false
		doForce = false

		if err := create([]string{root}); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		os.RemoveAll(root)
		dest := filepath.Join(tempDir, "out")
		os.MkdirAll(dest, 0o755)
		encode = tc.enc
		extract([]string{dest}, false, false)

		encode = ""

		extracted := filepath.Join(dest, filepath.Base(root), "file.txt")
		checkFile(t, extracted, data, 0o644, false)
	}
}
