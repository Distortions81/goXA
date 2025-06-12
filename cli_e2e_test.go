package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func resetGlobals() {
	archivePath = ""
	verboseMode = false
	doForce = false
	toStdOut = false
	progress = false
	quietMode = false
	features = 0
	compression = ""
	extractList = nil
	tarUseXz = false
	protoVersion = protoVersion2
	blockSize = defaultBlockSize
	fileRetries = 3
	fileRetryDelay = 5
	failOnChange = false
	bombCheck = true
}

func TestCLIEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI end-to-end test in short mode")
	}

	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data := []byte("hello")
	if err := os.WriteFile(filepath.Join(root, "file.txt"), data, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	archive := filepath.Join(tempDir, "test.goxa")

	resetGlobals()
	os.Args = []string{"goxa", "c", "-arc=" + archive, "-progress=false", root}
	main()

	dest := filepath.Join(tempDir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}

	resetGlobals()
	os.Args = []string{"goxa", "x", "-arc=" + archive, "-progress=false", dest}
	main()

	extracted := filepath.Join(dest, filepath.Base(root), "file.txt")
	out, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("content mismatch")
	}
}

func TestCLITarAutoDetect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI end-to-end test in short mode")
	}

	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data := []byte("hello")
	if err := os.WriteFile(filepath.Join(root, "file.txt"), data, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	archive := filepath.Join(tempDir, "test.tar.gz")

	resetGlobals()
	os.Args = []string{"goxa", "c", "-arc=" + archive, "-progress=false", root}
	main()

	dest := filepath.Join(tempDir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}

	resetGlobals()
	os.Args = []string{"goxa", "x", "-arc=" + archive, "-progress=false", dest}
	main()

	extracted := filepath.Join(dest, filepath.Base(root), "file.txt")
	out, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("content mismatch")
	}
}

func TestCLITarXzAutoDetect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI end-to-end test in short mode")
	}

	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data := []byte("hello")
	if err := os.WriteFile(filepath.Join(root, "file.txt"), data, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	archive := filepath.Join(tempDir, "test.tar.xz")

	resetGlobals()
	os.Args = []string{"goxa", "c", "-arc=" + archive, "-progress=false", root}
	main()

	dest := filepath.Join(tempDir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}

	resetGlobals()
	os.Args = []string{"goxa", "x", "-arc=" + archive, "-progress=false", dest}
	main()

	extracted := filepath.Join(dest, filepath.Base(root), "file.txt")
	out, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("content mismatch")
	}
}

func TestCLITarHeaderDetect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI end-to-end test in short mode")
	}

	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data := []byte("hello")
	if err := os.WriteFile(filepath.Join(root, "file.txt"), data, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	archive := filepath.Join(tempDir, "test.tar.gz")

	resetGlobals()
	os.Args = []string{"goxa", "c", "-arc=" + archive, "-progress=false", root}
	main()

	noExt := filepath.Join(tempDir, "archive")
	if err := os.Rename(archive, noExt); err != nil {
		t.Fatalf("rename: %v", err)
	}

	dest := filepath.Join(tempDir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}

	resetGlobals()
	os.Args = []string{"goxa", "x", "-arc=" + noExt, "-progress=false", dest}
	main()

	extracted := filepath.Join(dest, filepath.Base(root), "file.txt")
	out, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("content mismatch")
	}
}
