package goxa

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestUnicodeFilenames(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "ユニコード")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	fileName := "文件.txt"
	filePath := filepath.Join(root, fileName)
	content := []byte("hello unicode")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	archivePath = filepath.Join(tempDir, "test.goxa")
	features = 0
	protoVersion = protoVersion2
	toStdOut = false
	doForce = false

	cwd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	if err := create([]string{root}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	dest := filepath.Join(tempDir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("failed to create dest: %v", err)
	}

	extract([]string{dest}, false, false)

	extracted := filepath.Join(dest, filepath.Base(root), fileName)
	data, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Fatalf("content mismatch")
	}
}
