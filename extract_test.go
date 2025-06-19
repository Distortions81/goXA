package goxa

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractFileDirCreationFailure(t *testing.T) {
	tmp := t.TempDir()

	// create a file that will act as the destination root
	destFile := filepath.Join(tmp, "destfile")
	if err := os.WriteFile(destFile, []byte("test"), 0644); err != nil {
		t.Fatalf("setup dest file: %v", err)
	}

	archivePath = filepath.Join(tmp, "dummy.goxa")
	if err := os.WriteFile(archivePath, []byte{}, 0644); err != nil {
		t.Fatalf("setup archive: %v", err)
	}

	doForce = true
	defer func() { doForce = false }()

	item := FileEntry{Path: filepath.Join("sub", "file.txt"), Offset: 1}

	f, _ := os.Open(archivePath)
	defer f.Close()
	_ = extractFile(f, destFile+string(os.PathSeparator), 0, compGzip, &item, &progressData{})

	if _, err := os.Stat(filepath.Join(tmp, "destfile", "sub")); err == nil {
		t.Fatalf("directory should not be created")
	}
}
