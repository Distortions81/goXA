package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

type largeSpec struct {
	rel  string
	data []byte
	perm fs.FileMode
}

func setupLargeBlockTree(t *testing.T, root string) []largeSpec {
	old := syscall.Umask(0)
	defer syscall.Umask(old)
	specs := make([]largeSpec, 0, 3010)
	for i := 0; i < 3000; i++ {
		size := (i%20 + 1) * 1024
		data := bytes.Repeat([]byte{byte(i % 256)}, size)
		rel := filepath.Join("small", fmt.Sprintf("f%04d.bin", i))
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		specs = append(specs, largeSpec{rel: rel, data: data, perm: 0o644})
	}
	bigSizes := []int{1 << 20, 2 << 20, 4 << 20, 8 << 20, 10 << 20}
	for i, sz := range bigSizes {
		data := bytes.Repeat([]byte{byte(i)}, sz)
		rel := filepath.Join("big", fmt.Sprintf("b%02d.bin", i))
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		specs = append(specs, largeSpec{rel: rel, data: data, perm: 0o644})
	}
	return specs
}

func parseArchive(t *testing.T, path string) []FileEntry {
	arc, err := NewBinReader(path)
	if err != nil {
		t.Fatalf("open arc: %v", err)
	}
	defer arc.Close()

	var magicBytes [4]byte
	if err := binary.Read(arc, binary.LittleEndian, &magicBytes); err != nil {
		t.Fatalf("read magic: %v", err)
	}
	if string(magicBytes[:]) != magic {
		t.Fatalf("wrong magic")
	}

	var ver uint16
	if err := binary.Read(arc, binary.LittleEndian, &ver); err != nil {
		t.Fatalf("read ver: %v", err)
	}
	var flags BitFlags
	if err := binary.Read(arc, binary.LittleEndian, &flags); err != nil {
		t.Fatalf("read flags: %v", err)
	}
	var blkSize uint32 = blockSize
	var trailerOff uint64
	if ver >= version2 {
		if err := binary.Read(arc, binary.LittleEndian, &blkSize); err != nil {
			t.Fatalf("read block size: %v", err)
		}
		if err := binary.Read(arc, binary.LittleEndian, &trailerOff); err != nil {
			t.Fatalf("read trailer offset: %v", err)
		}
	}

	var numDirs uint64
	if err := binary.Read(arc, binary.LittleEndian, &numDirs); err != nil {
		t.Fatalf("read dir count: %v", err)
	}
	for n := uint64(0); n < numDirs; n++ {
		if flags.IsSet(fPermissions) {
			arc.Seek(4, io.SeekCurrent)
		}
		if flags.IsSet(fModDates) {
			arc.Seek(8, io.SeekCurrent)
		}
		if _, err := ReadLPString(arc); err != nil {
			t.Fatalf("read dir path: %v", err)
		}
	}

	var numFiles uint64
	if err := binary.Read(arc, binary.LittleEndian, &numFiles); err != nil {
		t.Fatalf("read file count: %v", err)
	}
	files := make([]FileEntry, numFiles)
	for i := range files {
		var size uint64
		var mode uint32
		var mt int64
		if err := binary.Read(arc, binary.LittleEndian, &size); err != nil {
			t.Fatalf("read size: %v", err)
		}
		if flags.IsSet(fPermissions) {
			binary.Read(arc, binary.LittleEndian, &mode)
		}
		if flags.IsSet(fModDates) {
			binary.Read(arc, binary.LittleEndian, &mt)
		}
		path, err := ReadLPString(arc)
		if err != nil {
			t.Fatalf("read path: %v", err)
		}
		var typ uint8
		if err := binary.Read(arc, binary.LittleEndian, &typ); err != nil {
			t.Fatalf("read type: %v", err)
		}
		if typ == entrySymlink || typ == entryHardlink {
			if _, err := ReadLPString(arc); err != nil {
				t.Fatalf("read link: %v", err)
			}
		}
		files[i] = FileEntry{Path: path, Size: size, Mode: fs.FileMode(mode), ModTime: time.Unix(mt, 0).UTC(), Type: typ}
	}
	for i := range files {
		if err := binary.Read(arc, binary.LittleEndian, &files[i].Offset); err != nil {
			t.Fatalf("read offset: %v", err)
		}
	}
	if ver >= version2 {
		var hdrSum [checksumSize]byte
		if _, err := io.ReadFull(arc, hdrSum[:]); err != nil {
			t.Fatalf("read header checksum: %v", err)
		}
		if _, err := arc.Seek(int64(trailerOff), io.SeekStart); err != nil {
			t.Fatalf("seek trailer: %v", err)
		}
		for i := range files {
			var count uint32
			if err := binary.Read(arc, binary.LittleEndian, &count); err != nil {
				t.Fatalf("read block count: %v", err)
			}
			blocks := make([]Block, count)
			for b := uint32(0); b < count; b++ {
				if err := binary.Read(arc, binary.LittleEndian, &blocks[b].Offset); err != nil {
					t.Fatalf("read block off: %v", err)
				}
				if err := binary.Read(arc, binary.LittleEndian, &blocks[b].Size); err != nil {
					t.Fatalf("read block size: %v", err)
				}
			}
			files[i].Blocks = blocks
		}
	}
	return files
}

func TestBlockArchiveLargeFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "root")
	specs := setupLargeBlockTree(t, root)

	archivePath = filepath.Join(tempDir, "test.goxa")
	features = fBlock | fNoCompress
	version = version2
	toStdOut = false
	doForce = false

	if err := create([]string{root}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	files := parseArchive(t, archivePath)
	for _, f := range files {
		if strings.HasPrefix(f.Path, "small/") {
			if len(f.Blocks) != 1 {
				t.Fatalf("small file %s expected 1 block, got %d", f.Path, len(f.Blocks))
			}
		}
		if strings.HasPrefix(f.Path, "big/") {
			exp := int((f.Size + uint64(blockSize) - 1) / uint64(blockSize))
			if len(f.Blocks) != exp {
				t.Fatalf("big file %s expected %d blocks, got %d", f.Path, exp, len(f.Blocks))
			}
		}
	}

	os.RemoveAll(root)
	dest := filepath.Join(tempDir, "out")
	os.MkdirAll(dest, 0o755)
	extract([]string{dest}, false)

	base := filepath.Join(dest, filepath.Base(root))
	for _, sp := range specs {
		checkFile(t, filepath.Join(base, sp.rel), sp.data, sp.perm, false)
	}
}
