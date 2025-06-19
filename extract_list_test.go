package goxa

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractListOption(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "root")
	if err := os.MkdirAll(filepath.Join(root, "sub1"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub2"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub1", "one.txt"), []byte("one"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub2", "two.txt"), []byte("two"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	archivePath = filepath.Join(tempDir, "test.goxa")
	features = 0
	protoVersion = protoVersion2
	toStdOut = false
	doForce = false

	if err := create([]string{root}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	os.RemoveAll(root)
	dest := filepath.Join(tempDir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}

	base := filepath.Base(root)
	extractList = []string{filepath.Join(base, "sub1")}
	defer func() { extractList = nil }()

	extract([]string{dest}, false, false)

	checkFile(t, filepath.Join(dest, base, "sub1", "one.txt"), []byte("one"), 0o644, false)
	if _, err := os.Stat(filepath.Join(dest, base, "sub2", "two.txt")); !os.IsNotExist(err) {
		t.Fatalf("unselected file should not exist")
	}
}
