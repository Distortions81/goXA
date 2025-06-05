package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"math"
	"os"
	"runtime"
	"sync"
	"time"

	gzip "github.com/klauspost/pgzip"
	"github.com/remeh/sizedwaitgroup"

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
		if err := WriteString(&header, folder.Path); err != nil {
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
		if err := WriteString(&header, file.Path); err != nil {
			log.Fatalf("write string failed: %v", err)
		}
	}

	//Save end of header, so we can update offsets later
	offsetsLocation := uint64(header.Len())

	const ThreadedMode = false
	if ThreadedMode {
		//Write spacer for file offsets by block (experimental)
		for _, file := range files {
			blocks := int(math.Ceil(float64(file.Size) / float64(blockSize)))
			for i := 0; i < blocks; i++ {
				binary.Write(&header, binary.LittleEndian, uint64(0))
			}
		}
	} else {
		//Write spacer for file offsets
		for range files {
			binary.Write(&header, binary.LittleEndian, uint64(0))
		}
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

// WIP
func writeEntriesThreaded(offsetLoc uint64, bf *BufferedFile, files []FileEntry) {

	totalBlocks := 0
	wg := sizedwaitgroup.New(runtime.NumCPU())
	for _, entry := range files {

		wg.Add()
		go func(entry FileEntry) {
			defer wg.Done()

			file, err := os.Open(entry.SrcPath)
			if err != nil {
				if doForce {
					//Soldier on even if read fails
					doLog(false, "\nUnable to open file: %v (continuing)", entry.Path)
					return
				} else {
					log.Fatalf("Unable to open file: %v", entry.Path)
				}
			}
			entry.NumBlocks = uint64(math.Ceil(float64(entry.Size) / float64(blockSize)))
			rbuf := make([]byte, blockSize)

			for blockNum := uint64(0); blockNum < entry.NumBlocks; blockNum++ {
				readBuf := bytes.NewBuffer(rbuf)
				io.Copy(readBuf, file)

				//Compress here

				curPos := writeBlock(readBuf.Bytes(), bf)
				entry.BlockOffset[blockNum] = offsetLoc + curPos

				totalBlocks++
			}

		}(entry)
	}
	wg.Wait()

	//End of BlockIndexOffset region
	blockIndexOffset := totalBlocks * 8

	//Shutup Compiler for the moment
	if blockIndexOffset == 0 {
		//
	}

	//Update blockOffsets in archive here
	var writtenBlock uint64
	for _, entry := range files {
		for block := uint64(0); block < entry.NumBlocks; block++ {
			newOffset := blockIndexOffset + int(entry.BlockOffset[block])
			_, err := bf.Seek(int64(offsetLoc+writtenBlock), io.SeekStart)
			if err != nil {
				log.Fatal("Failed seeking within archive file.")
			}
			binary.Write(bf, binary.LittleEndian, uint64(newOffset))
			writtenBlock++
		}
	}

}

var currentWritePos uint64
var writeMutex sync.Mutex

func writeBlock(data []byte, bf *BufferedFile) uint64 {
	dataLen := len(data)

	writeMutex.Lock()
	defer writeMutex.Unlock()

	currentWritePos += uint64(dataLen)

	_, err := bf.Write(data)
	if err != nil {
		log.Fatalf("Unable to write block to archive: %v", err)
	}
	return currentWritePos
}
