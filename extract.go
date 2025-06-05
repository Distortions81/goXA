package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/remeh/sizedwaitgroup"
	"golang.org/x/crypto/blake2b"
)

var skippedFiles, checksumCount atomic.Int64

func extract(destinations []string, listOnly bool) {

	var destination string
	//Clean destination
	if len(destinations) > 0 {
		destination = path.Clean(destinations[0]) + "/"

		if destination != "" {
			os.Mkdir(destination, os.ModePerm)
		}
	} else { //use pwd if none specified
		pwd, _ := os.Getwd()
		pwd = path.Clean(pwd)

		archiveName := path.Base(archivePath)
		archiveName = strings.TrimRight(archiveName, ".goxa")
		destination = path.Clean(pwd + "/" + archiveName + "/")
	}

	//Create reader
	arc, err := NewBinReader(archivePath)
	if err != nil {
		log.Fatalf("extract: Could not open the archive file: %v", err)
	}
	doLog(false, "Opening archive: %v", archivePath)
	if !listOnly {
		doLog(false, "Destination: %v", path.Clean(destination))
	}

	//Read header
	readMagic := make([]byte, 4)
	if err := binary.Read(arc, binary.LittleEndian, &readMagic); err != nil {
		log.Fatalf("extract: failed to read magic: %v", err)
	}
	if string(readMagic) != magic {
		log.Fatal("extract: File does not appear to be a goxa archive")
	}

	var readVersion uint16
	if err := binary.Read(arc, binary.LittleEndian, &readVersion); err != nil {
		log.Fatalf("extract: failed to read version: %v", err)
	}
	if readVersion != version {
		log.Fatalf("extract: Archive is of an unsupported version: %v", readVersion)
	}

	var lfeat BitFlags
	if err := binary.Read(arc, binary.LittleEndian, &lfeat); err != nil {
		log.Fatalf("extract: failed to read feature flags: %v", err)
	}
	showFeatures(lfeat)

	//Empty Directories
	var numEmptyDirs uint64
	if err := binary.Read(arc, binary.LittleEndian, &numEmptyDirs); err != nil {
		log.Fatalf("extract: failed to read empty directory count: %v", err)
	}

	dirList := make([]FileEntry, numEmptyDirs)
	for n := range numEmptyDirs {
		var fileMode uint32
		var modTime int64
		if lfeat.IsSet(fPermissions) {
			if err := binary.Read(arc, binary.LittleEndian, &fileMode); err != nil {
				log.Fatalf("extract: failed to read directory mode: %v", err)
			}
		}
		if lfeat.IsSet(fModDates) {
			if err := binary.Read(arc, binary.LittleEndian, &modTime); err != nil {
				log.Fatalf("extract: failed to read directory mod time: %v", err)
			}
		}

		pathName, err := ReadString(arc)
		if err != nil {
			log.Fatalf("extract: failed to read directory path: %v", err)
		}

		newDirEntry := FileEntry{Path: pathName, Mode: os.FileMode(fileMode), ModTime: time.Unix(modTime, 0).UTC()}
		dirList[n] = newDirEntry
	}

	//Files
	var numFiles uint64
	if err := binary.Read(arc, binary.LittleEndian, &numFiles); err != nil {
		log.Fatalf("extract: failed to read file count: %v", err)
	}

	fileList := make([]FileEntry, numFiles)
	for n := range numFiles {
		var fileSize uint64
		var fileMode uint32
		var modTime int64

		if err := binary.Read(arc, binary.LittleEndian, &fileSize); err != nil {
			log.Fatalf("extract: failed to read file size: %v", err)
		}
		if lfeat&fPermissions != 0 {
			if err := binary.Read(arc, binary.LittleEndian, &fileMode); err != nil {
				log.Fatalf("extract: failed to read file mode: %v", err)
			}
		}
		if lfeat&fModDates != 0 {
			if err := binary.Read(arc, binary.LittleEndian, &modTime); err != nil {
				log.Fatalf("extract: failed to read file mod time: %v", err)
			}
		}

		pathName, err := ReadString(arc)
		if err != nil {
			log.Fatalf("extract: failed to read file path: %v", err)
		}

		newEntry := FileEntry{Path: pathName, Size: fileSize, Mode: fs.FileMode(fileMode), ModTime: time.Unix(modTime, 0).UTC()}
		fileList[n] = newEntry
	}

	if listOnly {
		fileCount := 0
		byteCount := 0
		for _, item := range dirList {
			fmt.Printf("%v\n", item.Path)
		}
		for _, item := range fileList {
			fileCount++
			byteCount += int(item.Size)
			fmt.Printf("%v\n", item.Path)
		}
		fmt.Printf("%v files, %v\n", fileCount, humanize.Bytes(uint64(byteCount)))

		return
	}

	//File offsets
	for n := range numFiles {
		var fileOffset uint64
		if err := binary.Read(arc, binary.LittleEndian, &fileOffset); err != nil {
			log.Fatalf("extract: failed to read file offset: %v", err)
		}
		fileList[n].Offset = fileOffset
	}

	doLog(false, "Read index: %v files.", len(fileList))

	for _, item := range dirList {
		perms := os.FileMode(0644)
		if lfeat.IsSet(fPermissions) {
			perms = item.Mode
		}
		os.MkdirAll(item.Path, perms)
	}
	arc.Close()

	if lfeat.IsNotSet(fNoCompress) {
		wg := sizedwaitgroup.New(runtime.NumCPU())
		for f := range fileList {
			wg.Add()
			go func(item *FileEntry) {
				defer wg.Done()
				handleFile(destination, lfeat, item)
			}(&fileList[f])
		}
		wg.Wait()
	} else {
		for f := range fileList {
			handleFile(destination, lfeat, &fileList[f])
		}
	}

	if lfeat.IsSet(fChecksums) && int(checksumCount.Load()) == int(numFiles-uint64(skippedFiles.Load())) {
		doLog(false, "All checksums verified.")
	}
}

func handleFile(destination string, lfeat BitFlags, item *FileEntry) {
	if item.Offset == 0 {
		skippedFiles.Add(1)
		return
	}
	var err error
	//Make directories
	dir := path.Dir(destination + item.Path)
	os.MkdirAll(dir, os.ModePerm)

	//Set file perms, if needed
	filePerm := os.FileMode(0644)
	if lfeat.IsSet(fPermissions) {
		filePerm = os.FileMode(item.Mode)
	}

	//Open file
	var newFile *os.File
	if doForce {
		exists, _ := fileExists(destination + item.Path)
		if exists {
			os.Chmod(destination+item.Path, 0644)
		}
		newFile, err = os.OpenFile(destination+item.Path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if exists {
			os.Chmod(destination+item.Path, filePerm)
		}
	} else {
		newFile, err = os.OpenFile(destination+item.Path, os.O_CREATE|os.O_WRONLY, filePerm)
	}
	if err != nil {
		if doForce {
			doLog(false, "Unable to open file: %v :: %v", item.Path, err)
		} else {
			log.Fatalf("Unable to open file: %v :: %v", item.Path, err)
		}
	}

	//Seek to data in archive
	arcB, _ := NewBinReader(archivePath)
	_, err = arcB.Seek(int64(item.Offset), io.SeekStart)
	if err != nil {
		if doForce {
			doLog(false, "Unable to seek archive: %v :: %v", archivePath, err)
		} else {
			log.Fatalf("Unable to seek archive: %v :: %v", archivePath, err)
		}
	}

	//Create buffer and copy
	bf := NewBufferedFile(newFile, writeBuffer, &progressData{})

	//Read checksum
	var expectedChecksum = make([]byte, checksumSize)
	if lfeat.IsSet(fChecksums) {
		arcB.Read(expectedChecksum)
	}

	var src io.Reader = arcB
	if lfeat.IsNotSet(fNoCompress) {
		gzReader, err := gzip.NewReader(arcB)
		if err != nil {
			if doForce {
				doLog(false, "gzip error: Unable to create reader: %v :: %v", item.Path, err)
			} else {
				log.Fatalf("gzip error: Unable to create reader: %v :: %v", item.Path, err)
			}
			return
		}
		defer gzReader.Close()
		src = gzReader
	}

	var writer io.Writer = bf
	var hashSum []byte
	if lfeat.IsSet(fChecksums) {
		hasher, _ := blake2b.New256(nil)
		multiWriter := io.MultiWriter(bf, hasher)
		writer = multiWriter

		_, err = io.CopyN(writer, src, int64(item.Size))
		if err == nil {
			hashSum = hasher.Sum(nil)
		}
	} else {
		_, err = io.CopyN(writer, src, int64(item.Size))
	}

	if err != nil {
		if doForce {
			doLog(false, "Unable to write data: %v :: %v", item.Path, err)
		} else {
			log.Fatalf("Unable to write data to file: %v :: %v", item.Path, err)
		}
	}
	bf.Close()

	if lfeat.IsSet(fChecksums) {
		if bytes.Equal(hashSum, expectedChecksum) {
			checksumCount.Add(1)
		} else {
			if doForce {
				doLog(false, "Checksum mismatch for %v", item.Path)
			} else {
				log.Fatalf("Checksum mismatch for %v", item.Path)
			}
		}
	}
}
