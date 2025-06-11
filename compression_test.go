package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAllCompressions(t *testing.T) {
	cases := []struct {
		name  string
		ctype uint8
		flag  BitFlags
	}{
		{"gzip", compGzip, 0},
		{"zstd", compZstd, 0},
		{"lz4", compLZ4, 0},
		{"s2", compS2, 0},
		{"snappy", compSnappy, 0},
		{"brotli", compBrotli, 0},
		{"xz", compXZ, 0},
		{"none", compGzip, fNoCompress},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			root := filepath.Join(tempDir, "root")
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			content := []byte("compress test")
			if err := os.WriteFile(filepath.Join(root, "file.txt"), content, 0o644); err != nil {
				t.Fatalf("write file: %v", err)
			}

			archivePath = filepath.Join(tempDir, "test.goxa")
			features = tc.flag
			compType = tc.ctype
			version = version2
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

			features = tc.flag
			compType = tc.ctype
			extract([]string{dest}, false, false)

			extracted := filepath.Join(dest, filepath.Base(root), "file.txt")
			out, err := os.ReadFile(extracted)
			if err != nil {
				t.Fatalf("read extracted: %v", err)
			}
			if !bytes.Equal(out, content) {
				t.Fatalf("content mismatch")
			}
		})
	}
}
