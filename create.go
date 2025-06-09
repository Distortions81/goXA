package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"os"
	"time"

	brotli "github.com/andybalholm/brotli"
	"github.com/dustin/go-humanize"
	"github.com/golang/snappy"
	"github.com/klauspost/compress/s2"
	"github.com/klauspost/compress/zstd"
	gzip "github.com/klauspost/pgzip"
	lz4 "github.com/pierrec/lz4/v4"
	"golang.org/x/crypto/blake2b"
)

func compressor(w io.Writer) io.WriteCloser {
	switch {
	case features.IsSet(fZstd):
		zw, err := zstd.NewWriter(w)
		if err != nil {
			log.Fatalf("zstd init failed: %v", err)
		}
		return zw
	case features.IsSet(fLZ4):
		return lz4.NewWriter(w)
	case features.IsSet(fS2):
		return s2.NewWriter(w)
	case features.IsSet(fSnappy):
		return snappy.NewBufferedWriter(w)
	case features.IsSet(fBrotli):
		return brotli.NewWriter(w)
	default:
		return gzip.NewWriter(w)
	}
}

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

	if version >= version2 && features.IsSet(fBlock) {
		header := writeHeaderV2(emptyDirs, files, 0, features)
		headerLen := len(header)
		bf.Write(header)
		trailerOffset := writeEntriesV2(headerLen, bf, files)
		trailer := writeTrailer(files)
		bf.Write(trailer)
		finalHeader := writeHeaderV2(emptyDirs, files, trailerOffset, features)
		if len(finalHeader) != headerLen {
			log.Fatalf("header size mismatch")
		}
		if _, err := bf.Seek(0, io.SeekStart); err != nil {
			log.Fatalf("seek start: %v", err)
		}
		bf.Write(finalHeader)
	} else {
		offsetsLoc, header := writeHeader(emptyDirs, files)
		bf.Write(header)
		writeEntries(offsetsLoc, bf, files)
	}

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
			zw := compressor(cw)
			if _, err := io.Copy(zw, br); err != nil {
				log.Fatalf("compress copy failed: %v", err)
			}
			if err := zw.Close(); err != nil {
				log.Fatalf("compress close failed: %v", err)
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

func writeHeaderV2(emptyDirs, files []FileEntry, trailerOffset uint64, flags BitFlags) []byte {
	var header bytes.Buffer

	binary.Write(&header, binary.LittleEndian, []byte(magic))
	binary.Write(&header, binary.LittleEndian, uint16(version))
	binary.Write(&header, binary.LittleEndian, flags)
	binary.Write(&header, binary.LittleEndian, blockSize)
	binary.Write(&header, binary.LittleEndian, trailerOffset)

	binary.Write(&header, binary.LittleEndian, uint64(len(emptyDirs)))
	for _, folder := range emptyDirs {
		if flags.IsSet(fPermissions) {
			binary.Write(&header, binary.LittleEndian, uint32(folder.Mode))
		}
		if flags.IsSet(fModDates) {
			binary.Write(&header, binary.LittleEndian, int64(folder.ModTime.Unix()))
		}
		if err := WriteLPString(&header, folder.Path); err != nil {
			log.Fatalf("write string failed: %v", err)
		}
	}

	binary.Write(&header, binary.LittleEndian, uint64(len(files)))
	for _, file := range files {
		binary.Write(&header, binary.LittleEndian, uint64(file.Size))
		if flags.IsSet(fPermissions) {
			binary.Write(&header, binary.LittleEndian, uint32(file.Mode))
		}
		if flags.IsSet(fModDates) {
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
	for _, file := range files {
		binary.Write(&header, binary.LittleEndian, file.Offset)
	}

	h, _ := blake2b.New256(nil)
	h.Write(header.Bytes())
	header.Write(h.Sum(nil))
	return header.Bytes()
}

func writeEntriesV2(headerLen int, bf *BufferedFile, files []FileEntry) uint64 {
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

	cOffset := uint64(headerLen)
	buf := make([]byte, blockSize)

	for i := range files {
		entry := &files[i]
		p.file.Store(entry.Path)
		if entry.Type != entryFile {
			entry.Offset = 0
			continue
		}

		f, err := os.Open(entry.SrcPath)
		if err != nil {
			if doForce {
				doLog(false, "\nUnable to open file: %v (continuing)", entry.Path)
				continue
			}
			log.Fatalf("Unable to open file: %v", entry.Path)
		}

		var checksum []byte
		if features.IsSet(fChecksums) {
			h.Reset()
			if _, err := io.Copy(h, f); err != nil {
				log.Fatalf("checksum compute failed: %v", err)
			}
			checksum = h.Sum(nil)
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				log.Fatalf("seek reset failed: %v", err)
			}
			if _, err := bf.Write(checksum); err != nil {
				log.Fatalf("writing checksum failed: %v", err)
			}
		}

		entry.Offset = cOffset
		if features.IsSet(fChecksums) {
			cOffset += checksumSize
		}

		br := NewBufferedFile(f, writeBuffer, p)
		var blocks []Block
		for {
			n, err := io.ReadFull(br, buf)
			if n > 0 {
				bOff := cOffset
				if features.IsSet(fNoCompress) {
					if _, err := bf.Write(buf[:n]); err != nil {
						log.Fatalf("copy failed: %v", err)
					}
					cOffset += uint64(n)
					blocks = append(blocks, Block{Offset: bOff, Size: uint32(n)})
				} else {
					cw := &countingWriter{w: bf}
					zw := compressor(cw)
					if _, err := zw.Write(buf[:n]); err != nil {
						log.Fatalf("compress copy failed: %v", err)
					}
					if err := zw.Close(); err != nil {
						log.Fatalf("compress close failed: %v", err)
					}
					cOffset += uint64(cw.Count())
					blocks = append(blocks, Block{Offset: bOff, Size: uint32(cw.Count())})
				}
			}
			if err == io.EOF {
				break
			}
			if err == io.ErrUnexpectedEOF {
				break
			}
			if err != nil {
				log.Fatalf("read block failed: %v", err)
			}
		}
		br.Close()
		f.Close()
		entry.Blocks = blocks
	}
	return cOffset
}

func writeTrailer(files []FileEntry) []byte {
	var trailer bytes.Buffer
	for _, f := range files {
		binary.Write(&trailer, binary.LittleEndian, uint32(len(f.Blocks)))
		for _, b := range f.Blocks {
			binary.Write(&trailer, binary.LittleEndian, b.Offset)
			binary.Write(&trailer, binary.LittleEndian, b.Size)
		}
	}
	h, _ := blake2b.New256(nil)
	h.Write(trailer.Bytes())
	trailer.Write(h.Sum(nil))
	return trailer.Bytes()
}
