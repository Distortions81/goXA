package goxa

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestAllChecksums(t *testing.T) {
	cases := []struct {
		name   string
		ctype  uint8
		length uint8
	}{
		{"crc32", sumCRC32, 4},
		{"crc16", sumCRC16, 2},
		{"xxhash", sumXXHash, 8},
		{"sha256", sumSHA256, 32},
		{"blake3", sumBlake3, 32},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			root := filepath.Join(tempDir, "root")
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			content := []byte("checksum test")
			if err := os.WriteFile(filepath.Join(root, "file.txt"), content, 0o644); err != nil {
				t.Fatalf("write file: %v", err)
			}

			archivePath = filepath.Join(tempDir, "test.goxa")
			features = fChecksums
			compType = compGzip
			checksumType = tc.ctype
			checksumLength = tc.length
			protoVersion = protoVersion2
			toStdOut = false
			doForce = false

			if err := create([]string{root}); err != nil {
				t.Fatalf("create failed: %v", err)
			}

			f, err := os.Open(archivePath)
			if err != nil {
				t.Fatalf("open archive: %v", err)
			}
			defer f.Close()
			var magicBytes [4]byte
			if err := binary.Read(f, binary.LittleEndian, &magicBytes); err != nil {
				t.Fatalf("read magic: %v", err)
			}
			var ver uint16
			if err := binary.Read(f, binary.LittleEndian, &ver); err != nil {
				t.Fatalf("read version: %v", err)
			}
			var flags BitFlags
			if err := binary.Read(f, binary.LittleEndian, &flags); err != nil {
				t.Fatalf("read flags: %v", err)
			}
			var comp uint8
			if err := binary.Read(f, binary.LittleEndian, &comp); err != nil {
				t.Fatalf("read comp type: %v", err)
			}
			var csum uint8
			if err := binary.Read(f, binary.LittleEndian, &csum); err != nil {
				t.Fatalf("read checksum type: %v", err)
			}
			var clen uint8
			if err := binary.Read(f, binary.LittleEndian, &clen); err != nil {
				t.Fatalf("read checksum length: %v", err)
			}
			if csum != tc.ctype || clen != tc.length {
				t.Fatalf("checksum header mismatch")
			}

			os.RemoveAll(root)
			dest := filepath.Join(tempDir, "out")
			if err := os.MkdirAll(dest, 0o755); err != nil {
				t.Fatalf("mkdir dest: %v", err)
			}

			features = fChecksums
			compType = compGzip
			checksumType = tc.ctype
			checksumLength = tc.length
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
