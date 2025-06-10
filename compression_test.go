package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAllCompressions(t *testing.T) {
	cases := []struct {
		name string
		flag BitFlags
	}{
		{"gzip", 0},
		{"zstd", fZstd},
		{"lz4", fLZ4},
		{"s2", fS2},
		{"snappy", fSnappy},
		{"brotli", fBrotli},
		{"none", fNoCompress},
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
			extract([]string{dest}, false)

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
