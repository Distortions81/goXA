package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	gx "goxa"
)

func TestCLIJSONList(t *testing.T) {
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

	resetGlobals()
	os.Args = []string{"goxa", "j", "-arc=" + archive, "-progress=false", "-stdout"}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	std := os.Stdout
	os.Stdout = w
	main()
	w.Close()
	os.Stdout = std
	var buf bytes.Buffer
	io.Copy(&buf, r)

	var listing gx.ArchiveListingOut
	if err := json.Unmarshal(buf.Bytes(), &listing); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if len(listing.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(listing.Files))
	}
	exp := filepath.Join(filepath.Base(root), "file.txt")
	if listing.Files[0].Path != exp {
		t.Fatalf("unexpected path: %s", listing.Files[0].Path)
	}
}
