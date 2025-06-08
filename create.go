package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"os"
	"time"

	gzip "github.com/klauspost/pgzip"

	"github.com/dustin/go-humanize"
	"golang.org/x/crypto/blake2b"
)

func create(inputPaths []string) error {

	var bf *BufferedFile
	if toStdOut {
		bf = NewBufferedFile(os.Stdout, writeBuffer, &progressData{})
	} else {
		if !doForce {
			found, _ := fileExists(archivePath)
			if found {
				log.Fatalf("create: Archive %v already exists.", archivePath)
			}
		}

		f, err := os.Create(archivePath)
		if err != nil {
			log.Fatalf("create: os.Create: %v", err)
		}
		f.Truncate(0)
		defer f.Close()
		bf = NewBufferedFile(f, writeBuffer, &progressData{})
	}
	doLog(false, "Creating archive: %v, inputs: %v", archivePath, inputPaths)

	emptyDirs, files, err := walkPaths(inputPaths)
	if err != nil {
		return err
	}

	offsetsLoc, header := writeHeader(emptyDirs, files)
	bf.Write(header)

	writeEntries(offsetsLoc, bf, files)

	info, err := bf.file.Stat()
	if err != nil {
		log.Fatalf("create: os.Create: %v", err)
	}

	if err := bf.Close(); err != nil {
		log.Fatalf("create: close failed: %v", err)
	}
	doLog(false, "\nWrote %v, %v containing %v files.", archivePath, humanize.Bytes(uint64(info.Size())), len(files))
	return nil
}

func writeHeader(emptyDirs, files []FileEntry) (uint64, []byte) {
	var header bytes.Buffer

	numFiles := len(files)

	//Start
	binary.Write(&header, binary.LittleEndian, []byte(magic))
	binary.Write(&header, binary.LittleEndian, uint16(version))
	binary.Write(&header, binary.LittleEndian, features)

	//Empty dir info
	binary.Write(&header, binary.LittleEndian, uint64(len(emptyDirs)))
	for _, folder := range emptyDirs {

		if features&fPermissions != 0 {
			binary.Write(&header, binary.LittleEndian, uint32(folder.Mode))
		}
		if features&fModDates != 0 {
			binary.Write(&header, binary.LittleEndian, int64(folder.ModTime.Unix()))
		}
		if err := WriteLPString(&header, folder.Path); err != nil {
			log.Fatalf("write string failed: %v", err)
		}
	}

	//File info
	binary.Write(&header, binary.LittleEndian, uint64(numFiles))
	for _, file := range files {
		binary.Write(&header, binary.LittleEndian, uint64(file.Size))
		if features&fPermissions != 0 {
			binary.Write(&header, binary.LittleEndian, uint32(file.Mode))
		}
		if features&fModDates != 0 {
			binary.Write(&header, binary.LittleEndian, int64(file.ModTime.Unix()))
		}
		if err := WriteLPString(&header, file.Path); err != nil {
			log.Fatalf("write string failed: %v", err)
		}
		header.WriteByte(file.Type)
		if file.Type == entrySymlink || file.Type == entryHardlink {
			if err := WriteLPString(&header, file.Linkname); err != nil {
				log.Fatalf("write string failed: %v", err)
			}
		}
	}

	//Save end of header, so we can update offsets later
	offsetsLocation := uint64(header.Len())

	// Reserve space for file offsets
	for range files {
		binary.Write(&header, binary.LittleEndian, uint64(0))
	}

	doLog(true, "Header size: %v", humanize.Bytes(uint64(header.Len())))
	return offsetsLocation, header.Bytes()
}

func writeEntries(offsetLoc uint64, bf *BufferedFile, files []FileEntry) {
	cOffset := offsetLoc + uint64(len(files))*8
	offsets := make([]uint64, len(files))

	h, err := blake2b.New256(nil)
	if err != nil {
		log.Fatalf("blake2b init failed: %v", err)
	}

	var totalBytes int64
	for _, entry := range files {
		totalBytes += int64(entry.Size)
	}

	p, done, finished := progressTicker(&progressData{total: totalBytes, speedWindowSize: time.Second * 5})
	bf.progress = p
	bf.doCount = true
	defer func() {
		close(done)
		<-finished
	}()

	for i, entry := range files {
		p.file.Store(entry.Path)

		if entry.Type != entryFile {
			offsets[i] = 0
			continue
		}

		file, err := os.Open(entry.SrcPath)
		if err != nil {
			if doForce {
				//Soldier on even if read fails
				doLog(false, "\nUnable to open file: %v (continuing)", entry.Path)
				continue
			} else {
				log.Fatalf("Unable to open file: %v", entry.Path)
			}
		}
		// Compute checksum first without counting progress
		var checksum []byte
		if features.IsSet(fChecksums) {
			h.Reset()
			if _, err := io.Copy(h, file); err != nil {
				log.Fatalf("checksum compute failed: %v", err)
			}
			checksum = h.Sum(nil)
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				log.Fatalf("seek reset failed: %v", err)
			}
			if _, err := bf.Write(checksum); err != nil {
				log.Fatalf("writing checksum failed: %v", err)
			}
		}

		br := NewBufferedFile(file, writeBuffer, p)

		// Write file data (compressed or not)
		var written uint64
		if features.IsSet(fNoCompress) {
			if _, err := io.Copy(bf, br); err != nil {
				log.Fatalf("copy failed: %v", err)
			}
			written = entry.Size
		} else {
			cw := &countingWriter{w: bf}
			gw := gzip.NewWriter(cw)
			if _, err := io.Copy(gw, br); err != nil {
				log.Fatalf("gzip copy failed: %v", err)
			}
			if err := gw.Close(); err != nil {
				log.Fatalf("gzip close failed: %v", err)
			}
			written = uint64(cw.Count())
		}

		br.Close()

		offsets[i] = cOffset
		cOffset += written
		if features.IsSet(fChecksums) {
			cOffset += checksumSize
		}
	}

	// Seek back and write offset table
	if _, err := bf.Seek(int64(offsetLoc), io.SeekStart); err != nil {
		log.Fatalf("seek to offset %d failed: %v", offsetLoc, err)
	}
	for _, off := range offsets {
		if err := binary.Write(bf, binary.LittleEndian, off); err != nil {
			log.Fatalf("writing offset failed: %v", err)
		}
	}
}
