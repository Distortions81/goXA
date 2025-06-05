package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHandleFileNoOverwriteWithoutForce(t *testing.T) {
	tmpDir := t.TempDir()
	dest := tmpDir + string(os.PathSeparator)
	existing := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(existing, []byte("old"), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	doForce = false
	archivePath = "" // not used when open fails
	entry := &FileEntry{Path: "test.txt", Offset: 1}

	err := handleFile(dest, 0, entry)
	if err == nil {
		t.Fatalf("expected error when file exists without force")
	}

	data, err := os.ReadFile(existing)
	if err != nil {
		t.Fatalf("failed reading file: %v", err)
	}
	if string(data) != "old" {
		t.Fatalf("file should not be overwritten")
	}
}
