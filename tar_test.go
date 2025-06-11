package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

type tarFileSpec struct {
	rel  string
	data []byte
	perm fs.FileMode
}

func setupTarTree(t *testing.T, root string) []tarFileSpec {
	old := syscall.Umask(0)
	defer syscall.Umask(old)
	files := []tarFileSpec{
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

func tarCheckFile(t *testing.T, path string, expect []byte) {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %v: %v", path, err)
	}
	if !bytes.Equal(data, expect) {
		t.Fatalf("content mismatch for %v", path)
	}
}

func TestTarBasic(t *testing.T) {
	cases := []struct {
		name   string
		noComp bool
	}{
		{"compress", false},
		{"nocompress", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			root := filepath.Join(tempDir, "root")
			specs := setupTarTree(t, root)

			archivePath = filepath.Join(tempDir, "test.tar")
			features = 0
			if tc.noComp {
				features = fNoCompress
			}
			if err := createTar([]string{root}); err != nil {
				t.Fatalf("createTar failed: %v", err)
			}

			os.RemoveAll(root)
			dest := filepath.Join(tempDir, "out")
			os.MkdirAll(dest, 0o755)
			if err := extractTar(dest); err != nil {
				t.Fatalf("extractTar failed: %v", err)
			}

			base := filepath.Join(dest, filepath.Base(root))
			for _, sp := range specs {
				tarCheckFile(t, filepath.Join(base, sp.rel), sp.data)
			}
		})
	}
}

func TestTarModTime(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "root")
	filePath := filepath.Join(root, "file.txt")
	os.MkdirAll(root, 0o755)
	content := []byte("hi")
	os.WriteFile(filePath, content, 0o644)
	modTime := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	os.Chtimes(filePath, modTime, modTime)

	archivePath = filepath.Join(tempDir, "test.tar")
	features = 0
	if err := createTar([]string{root}); err != nil {
		t.Fatalf("createTar: %v", err)
	}
	os.RemoveAll(root)
	dest := filepath.Join(tempDir, "out")
	os.MkdirAll(dest, 0o755)
	if err := extractTar(dest); err != nil {
		t.Fatalf("extractTar: %v", err)
	}
	info, err := os.Stat(filepath.Join(dest, filepath.Base(root), "file.txt"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.ModTime().UTC().Truncate(time.Second) != modTime {
		t.Fatalf("mod time mismatch")
	}
}
