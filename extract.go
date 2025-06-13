package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	gzip "github.com/klauspost/pgzip"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	brotli "github.com/andybalholm/brotli"
	"github.com/dustin/go-humanize"
	"github.com/golang/snappy"
	"github.com/klauspost/compress/s2"
	"github.com/klauspost/compress/zstd"
	lz4 "github.com/pierrec/lz4/v4"
	"github.com/remeh/sizedwaitgroup"
	"github.com/ulikunitz/xz"
)

func decompressor(r io.Reader, cType uint8) (io.ReadCloser, error) {
	switch cType {
	case compZstd:
		zr, err := zstd.NewReader(r, zstd.WithDecoderConcurrency(runtime.NumCPU()))
		if err != nil {
			return nil, err
		}
		return zr.IOReadCloser(), nil
	case compLZ4:
		return io.NopCloser(lz4.NewReader(r)), nil
	case compS2:
		return io.NopCloser(s2.NewReader(r)), nil
	case compSnappy:
		return io.NopCloser(snappy.NewReader(r)), nil
	case compBrotli:
		return io.NopCloser(brotli.NewReader(r)), nil
	case compXZ:
		xr, err := xz.NewReader(r)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(xr), nil
	default:
		gr, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		return gr, nil
	}
}

var skippedFiles, checksumCount atomic.Int64

func isZipBomb(f *FileEntry) (uint64, float64, bool) {
	if len(f.Blocks) == 0 {
		return 0, 0, false
	}
	var comp uint64
	for _, b := range f.Blocks {
		comp += b.Size
	}
	if comp == 0 || f.Size < zipBombMinSize {
		return comp, 0, false
	}
	ratio := float64(f.Size) / float64(comp)
	return comp, ratio, ratio > zipBombRatio
}

func compName(t uint8) string {
	switch t {
	case compZstd:
		return "zstd"
	case compLZ4:
		return "lz4"
	case compS2:
		return "s2"
	case compSnappy:
		return "snappy"
	case compBrotli:
		return "brotli"
	case compXZ:
		return "xz"
	default:
		return "gzip"
	}
}

func checksumName(t uint8) string {
	switch t {
	case sumCRC32:
		return "crc32"
	case sumCRC16:
		return "crc16"
	case sumXXHash:
		return "xxhash"
	case sumSHA256:
		return "sha256"
	case sumBlake3:
		return "blake3"
	default:
		return "unknown"
	}
}

func entryName(t uint8) string {
	switch t {
	case entryFile:
		return "file"
	case entrySymlink:
		return "symlink"
	case entryHardlink:
		return "hardlink"
	default:
		return "other"
	}
}

func extract(destinations []string, listOnly bool, jsonList bool) {

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
		archiveName = stripArchiveExt(archiveName)
		destination = path.Clean(pwd + "/" + archiveName + "/")
	}

	//Create reader
	arcPath := archivePath
	cleanup := func() {}
	if encode != "" {
		var err error
		if encode == "fec" {
			arcPath, cleanup, err = decodeWithFEC(archivePath)
		} else {
			arcPath, cleanup, err = decodeIfNeeded(archivePath)
		}
		if err != nil {
			log.Fatalf("extract: decode failed: %v", err)
		}
		defer cleanup()
	}
	arc, err := NewBinReader(arcPath)
	if err != nil {
		log.Fatalf("extract: Could not open the archive file: %v", err)
	}
	doLog(false, "Opening archive: %v", archivePath)
	if !listOnly {
		doLog(false, "Destination: %v", path.Clean(destination))
	}

	headerDone := make(chan struct{})
	go func() {
		timer := time.NewTimer(500 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-headerDone:
			return
		case <-timer.C:
			doLog(false, "Reading archive headers...")
		}
	}()

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
	if readVersion != protoVersion2 {
		log.Fatalf("extract: Archive is of an unsupported version: %v", readVersion)
	}

	var lfeat BitFlags
	if err := binary.Read(arc, binary.LittleEndian, &lfeat); err != nil {
		log.Fatalf("extract: failed to read feature flags: %v", err)
	}
	showFeatures(lfeat)

	ctype := compGzip
	if readVersion >= protoVersion2 {
		if err := binary.Read(arc, binary.LittleEndian, &ctype); err != nil {
			log.Fatalf("extract: failed to read compression type: %v", err)
		}
		if err := binary.Read(arc, binary.LittleEndian, &checksumType); err != nil {
			log.Fatalf("extract: failed to read checksum type: %v", err)
		}
		if err := binary.Read(arc, binary.LittleEndian, &checksumLength); err != nil {
			log.Fatalf("extract: failed to read checksum length: %v", err)
		}
	}

	if useArchiveFlags {
		features |= lfeat
	} else {
		missing := ""
		missingFlags := BitFlags(0)
		if lfeat.IsSet(fPermissions) && features.IsNotSet(fPermissions) {
			missing += "p"
			missingFlags |= fPermissions
		}
		if lfeat.IsSet(fModDates) && features.IsNotSet(fModDates) {
			missing += "m"
			missingFlags |= fModDates
		}
		if lfeat.IsSet(fSpecialFiles) && features.IsNotSet(fSpecialFiles) {
			missing += "o"
			missingFlags |= fSpecialFiles
		}
		if lfeat.IsSet(fIncludeInvis) && features.IsNotSet(fIncludeInvis) {
			missing += "i"
			missingFlags |= fIncludeInvis
		}
		if missing != "" {
			if interactiveMode {
				fmt.Printf("Archive uses flags '%s'. Enable which? (letters or 'u'=all) [none]: ", missing)
				reader := bufio.NewReader(os.Stdin)
				resp, _ := reader.ReadString('\n')
				resp = strings.TrimSpace(strings.ToLower(resp))
				if resp == "u" {
					features |= missingFlags
				} else {
					for _, r := range resp {
						switch r {
						case 'p':
							if missingFlags.IsSet(fPermissions) {
								features.Set(fPermissions)
							}
						case 'm':
							if missingFlags.IsSet(fModDates) {
								features.Set(fModDates)
							}
						case 'o':
							if missingFlags.IsSet(fSpecialFiles) {
								features.Set(fSpecialFiles)
							}
						case 'i':
							if missingFlags.IsSet(fIncludeInvis) {
								features.Set(fIncludeInvis)
							}
						}
					}
				}
			} else {
				doLog(false, "Archive uses flags '%s'.", missing)
			}
		}
	}

	var blkSize uint32 = blockSize
	var trailerOffset uint64
	var arcSize uint64
	if readVersion >= protoVersion2 {
		if err := binary.Read(arc, binary.LittleEndian, &blkSize); err != nil {
			log.Fatalf("extract: failed to read block size: %v", err)
		}
		if err := binary.Read(arc, binary.LittleEndian, &trailerOffset); err != nil {
			log.Fatalf("extract: failed to read trailer offset: %v", err)
		}
		blockSize = blkSize
	}
	if readVersion >= protoVersion2 {
		if err := binary.Read(arc, binary.LittleEndian, &arcSize); err != nil {
			log.Fatalf("extract: failed to read archive size: %v", err)
		}
		info, _ := arc.file.Stat()
		if uint64(info.Size()) != arcSize {
			log.Fatalf("extract: archive size mismatch")
		}
	}

	//Empty Directories
	var numEmptyDirs uint64
	if err := binary.Read(arc, binary.LittleEndian, &numEmptyDirs); err != nil {
		log.Fatalf("extract: failed to read empty directory count: %v", err)
	}

	dirList := make([]FileEntry, numEmptyDirs)
	for n := uint64(0); n < numEmptyDirs; n++ {
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

		pathName, err := ReadLPString(arc)
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
	for n := uint64(0); n < numFiles; n++ {
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

		pathName, err := ReadLPString(arc)
		if err != nil {
			log.Fatalf("extract: failed to read file path: %v", err)
		}
		var ftype uint8
		if err := binary.Read(arc, binary.LittleEndian, &ftype); err != nil {
			log.Fatalf("extract: failed to read file type: %v", err)
		}
		var linkName string
		if ftype == entrySymlink || ftype == entryHardlink {
			linkName, err = ReadLPString(arc)
			if err != nil {
				log.Fatalf("extract: failed to read link target: %v", err)
			}
		}
		var changedFlag uint8
		if readVersion >= protoVersion2 {
			if err := binary.Read(arc, binary.LittleEndian, &changedFlag); err != nil {
				log.Fatalf("extract: failed to read changed flag: %v", err)
			}
		}

		newEntry := FileEntry{Path: pathName, Size: fileSize, Mode: fs.FileMode(fileMode), ModTime: time.Unix(modTime, 0).UTC(), Type: ftype, Linkname: linkName, Changed: changedFlag != 0}
		fileList[n] = newEntry
	}

	if listOnly && !jsonList {
		fileCount := 0
		byteCount := 0
		for _, item := range dirList {
			if isSelected(item.Path) {
				fmt.Printf("%v\n", item.Path)
			}
		}
		for _, item := range fileList {
			if !isSelected(item.Path) {
				continue
			}
			fileCount++
			byteCount += int(item.Size)
			fmt.Printf("%v\n", item.Path)
		}
		fmt.Printf("%v files, %v\n", fileCount, humanize.Bytes(uint64(byteCount)))

		return
	}

	if jsonList {
		out := ArchiveListingOut{
			Version:        readVersion,
			Flags:          flagNamesList(lfeat),
			Compression:    compName(ctype),
			Checksum:       checksumName(checksumType),
			ChecksumLength: checksumLength,
			BlockSize:      blockSize,
			ArchiveSize:    arcSize,
		}
		for _, item := range dirList {
			if isSelected(item.Path) {
				out.Dirs = append(out.Dirs, ListEntryOut{
					Path:    item.Path,
					Type:    "dir",
					Mode:    item.Mode,
					ModTime: item.ModTime.Unix(),
				})
			}
		}
		for _, item := range fileList {
			if !isSelected(item.Path) {
				continue
			}
			out.Files = append(out.Files, ListEntryOut{
				Path:     item.Path,
				Type:     entryName(item.Type),
				Size:     item.Size,
				Mode:     item.Mode,
				ModTime:  item.ModTime.Unix(),
				Linkname: item.Linkname,
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			log.Fatalf("json encode: %v", err)
		}
		return
	}

	if readVersion >= protoVersion2 {
		hdrSum := make([]byte, checksumLength)
		if _, err := io.ReadFull(arc, hdrSum); err != nil {
			log.Fatalf("extract: failed to read header checksum: %v", err)
		}
		var hdrBytes []byte
		hdrBytes = writeHeader(dirList, fileList, trailerOffset, arcSize, lfeat, ctype)
		expect := hdrBytes[len(hdrBytes)-int(checksumLength):]
		if !bytes.Equal(expect, hdrSum) {
			log.Fatalf("extract: header checksum mismatch")
		}

		if _, err := arc.Seek(int64(trailerOffset), io.SeekStart); err != nil {
			log.Fatalf("extract: seek trailer: %v", err)
		}
		for i := range fileList {
			var count uint32
			if err := binary.Read(arc, binary.LittleEndian, &count); err != nil {
				log.Fatalf("extract: read block count: %v", err)
			}
			blocks := make([]Block, count)
			for b := uint32(0); b < count; b++ {
				if err := binary.Read(arc, binary.LittleEndian, &blocks[b].Offset); err != nil {
					log.Fatalf("extract: read block offset: %v", err)
				}
				if err := binary.Read(arc, binary.LittleEndian, &blocks[b].Size); err != nil {
					log.Fatalf("extract: read block size: %v", err)
				}
			}
			fileList[i].Blocks = blocks
			if len(blocks) > 0 {
				off := blocks[0].Offset
				if lfeat.IsSet(fChecksums) {
					off -= uint64(checksumLength)
				}
				fileList[i].Offset = off
			}
		}
		tSum := make([]byte, checksumLength)
		if _, err := io.ReadFull(arc, tSum); err != nil {
			log.Fatalf("extract: read trailer checksum: %v", err)
		}
		trailerBytes := writeTrailer(fileList)
		expectT := trailerBytes[len(trailerBytes)-int(checksumLength):]
		if !bytes.Equal(expectT, tSum) {
			log.Fatalf("extract: trailer checksum mismatch")
		}
	}

	close(headerDone)
	doLog(false, "Read index: %v files.", len(fileList))

	var totalBytes int64
	selectedFiles := 0
	for _, entry := range fileList {
		if !isSelected(entry.Path) {
			continue
		}
		selectedFiles++
		totalBytes += int64(entry.Size)
	}

	if spaceCheck && !listOnly {
		free, total, err := getDiskSpace(destination)
		if err != nil {
			doLog(false, "warning: free space check failed: %v", err)
		} else {
			need := uint64(totalBytes)
			if need > free {
				log.Fatalf("extract: insufficient disk space: need %v, available %v", humanize.Bytes(need), humanize.Bytes(free))
			}
			if free-need < total/100 {
				msg := fmt.Sprintf("extract would leave %v free", humanize.Bytes(free-need))
				if interactiveMode {
					fmt.Printf("%s. Continue? [y/N]: ", msg)
					reader := bufio.NewReader(os.Stdin)
					resp, _ := reader.ReadString('\n')
					resp = strings.TrimSpace(strings.ToLower(resp))
					if resp != "y" && resp != "yes" {
						log.Fatalf("aborted: %s", msg)
					}
				} else {
					log.Fatalf("%s", msg)
				}
			}
		}
	}

	p, done, finished := progressTicker(&progressData{total: totalBytes, speedWindowSize: time.Second * 5})
	defer func() {
		close(done)
		<-finished
	}()

	for _, item := range dirList {
		if !isSelected(item.Path) {
			continue
		}
		perms := os.FileMode(0755)
		if lfeat.IsSet(fPermissions) {
			perms = item.Mode
		}
		var dirPath string
		var err error
		if lfeat.IsSet(fAbsolutePaths) {
			dirPath = filepath.Clean(item.Path)
		} else {
			dirPath, err = safeJoin(destination, item.Path)
			if err != nil {
				if doForce {
					doLog(false, "invalid path: %v", item.Path)
					continue
				}
				log.Fatalf("extract: invalid path %v", item.Path)
			}
		}
		if err := os.MkdirAll(dirPath, perms); err != nil {
			if doForce {
				doLog(false, "unable to create directory %v: %v", dirPath, err)
				continue
			}
			log.Fatalf("extract: unable to create directory %v: %v", dirPath, err)
		}
		if lfeat.IsSet(fModDates) {
			os.Chtimes(dirPath, item.ModTime, item.ModTime)
		}
	}
	arc.Close()

	if lfeat.IsNotSet(fNoCompress) {
		wg := sizedwaitgroup.New(runtime.NumCPU())
		for f := range fileList {
			if !isSelected(fileList[f].Path) {
				continue
			}
			wg.Add()
			go func(item *FileEntry) {
				defer wg.Done()
				_ = extractFile(arcPath, destination, lfeat, ctype, item, p)
			}(&fileList[f])
		}
		wg.Wait()
	} else {
		for f := range fileList {
			if !isSelected(fileList[f].Path) {
				continue
			}
			_ = extractFile(arcPath, destination, lfeat, ctype, &fileList[f], p)
		}
	}

	if lfeat.IsSet(fChecksums) && int(checksumCount.Load()) == selectedFiles-int(skippedFiles.Load()) {
		doLog(false, "All checksums verified.")
	}
}

func extractFile(arcPath, destination string, lfeat BitFlags, ctype uint8, item *FileEntry, p *progressData) error {
	if item.Type == entryOther {
		return nil
	}
	if item.Type == entrySymlink || item.Type == entryHardlink {
		var err error
		var finalPath string
		if lfeat.IsSet(fAbsolutePaths) {
			finalPath = filepath.Clean(item.Path)
		} else {
			finalPath, err = safeJoin(destination, item.Path)
			if err != nil {
				if doForce {
					doLog(false, "invalid path: %v", item.Path)
					skippedFiles.Add(1)
					return nil
				}
				log.Fatalf("invalid path: %v", item.Path)
			}
		}
		if err := os.MkdirAll(filepath.Dir(finalPath), os.ModePerm); err != nil {
			if doForce {
				doLog(false, "unable to create directory %v: %v", filepath.Dir(finalPath), err)
				skippedFiles.Add(1)
				return nil
			}
			log.Fatalf("extract: unable to create directory %v: %v", filepath.Dir(finalPath), err)
		}
		if doForce {
			os.RemoveAll(finalPath)
		}
		if item.Type == entrySymlink {
			return os.Symlink(item.Linkname, finalPath)
		}
		return os.Link(item.Linkname, finalPath)
	}
	if item.Offset == 0 {
		skippedFiles.Add(1)
		return nil
	}
	if item.Changed {
		doLog(false, "warning: %v changed during archiving", item.Path)
	}
	if bombCheck {
		compSize, ratio, bomb := isZipBomb(item)
		if bomb {
			msg := fmt.Sprintf("potential zip bomb: %s expands from %v to %v (x%.0f)", item.Path, humanize.Bytes(compSize), humanize.Bytes(item.Size), ratio)
			if interactiveMode {
				fmt.Printf("%s. Continue? [y/N]: ", msg)
				reader := bufio.NewReader(os.Stdin)
				resp, _ := reader.ReadString('\n')
				resp = strings.TrimSpace(strings.ToLower(resp))
				if resp != "y" && resp != "yes" {
					log.Fatalf("aborted: %s", msg)
				}
			} else {
				log.Fatalf("%s", msg)
			}
		}
	}
	var err error
	var finalPath string
	if lfeat.IsSet(fAbsolutePaths) {
		finalPath = filepath.Clean(item.Path)
	} else {
		finalPath, err = safeJoin(destination, item.Path)
		if err != nil {
			if doForce {
				doLog(false, "invalid path: %v", item.Path)
				skippedFiles.Add(1)
				return nil
			}
			log.Fatalf("invalid path: %v", item.Path)
		}
	}

	//Make directories
	dir := filepath.Dir(finalPath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		if doForce {
			doLog(false, "unable to create directory %v: %v", dir, err)
			skippedFiles.Add(1)
			return nil
		}
		log.Fatalf("extract: unable to create directory %v: %v", dir, err)
	}

	//Set file perms, if needed
	filePerm := os.FileMode(0644)
	if lfeat.IsSet(fPermissions) {
		filePerm = os.FileMode(item.Mode)
	}

	//Open file
	var newFile *os.File
	if doForce {
		exists, _ := fileExists(finalPath)
		if exists {
			os.Chmod(finalPath, 0644)
		}
		newFile, err = os.OpenFile(finalPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if exists {
			os.Chmod(finalPath, filePerm)
		}
	} else {
		newFile, err = os.OpenFile(finalPath, os.O_CREATE|os.O_WRONLY, filePerm)
	}
	if err != nil {
		return err
	}

	closeFile := func() {
		if newFile != nil {
			newFile.Close()
			newFile = nil
		}
	}

	//Seek to data in archive
	arcB, err := NewBinReader(arcPath)
	if err != nil {
		if doForce {
			doLog(false, "unable to open archive reader: %v", err)
			skippedFiles.Add(1)
			closeFile()
			return nil
		}
		closeFile()
		log.Fatalf("unable to open archive reader: %v", err)
	}
	defer arcB.Close()
	_, err = arcB.Seek(int64(item.Offset), io.SeekStart)
	if err != nil {
		if doForce {
			doLog(false, "Unable to seek archive: %v :: %v", arcPath, err)
		} else {
			log.Fatalf("Unable to seek archive: %v :: %v", arcPath, err)
		}
	}

	p.file.Store(item.Path)

	//Create buffer and copy
	bf := NewBufferedFile(newFile, writeBuffer, p)
	bf.doCount = true

	//Read checksum
	expectedChecksum := make([]byte, checksumLength)
	if lfeat.IsSet(fChecksums) {
		if _, err := io.ReadFull(arcB, expectedChecksum); err != nil {
			if doForce {
				doLog(false, "unable to read checksum for %v: %v", item.Path, err)
				skippedFiles.Add(1)
				closeFile()
				return nil
			}
			closeFile()
			log.Fatalf("unable to read checksum for %v: %v", item.Path, err)
		}
	}

	var writer io.Writer = bf
	var hashSum []byte
	hasBlocks := len(item.Blocks) > 0
	if lfeat.IsSet(fChecksums) {
		hasher := newHasher(checksumType)
		writer = io.MultiWriter(bf, hasher)
		if hasBlocks {
			for _, b := range item.Blocks {
				if _, err := arcB.Seek(int64(b.Offset), io.SeekStart); err != nil {
					log.Fatalf("seek block: %v", err)
				}
				r := io.LimitReader(arcB, int64(b.Size))
				if lfeat.IsNotSet(fNoCompress) {
					dec, err := decompressor(r, ctype)
					if err != nil {
						log.Fatalf("decompress setup: %v", err)
					}
					r = dec
					_, err = io.Copy(writer, progressReader{r: r, p: p})
					if err != nil {
						log.Fatalf("copy block: %v", err)
					}
					dec.Close()
				} else {
					_, err = io.Copy(writer, progressReader{r: r, p: p})
					if err != nil {
						log.Fatalf("copy block: %v", err)
					}
				}
			}
		} else {
			var src io.Reader = arcB
			if lfeat.IsNotSet(fNoCompress) {
				dec, err := decompressor(arcB, ctype)
				if err != nil {
					log.Fatalf("decompress error: Unable to create reader: %v :: %v", item.Path, err)
				}
				defer dec.Close()
				src = dec
			}
			src = progressReader{r: src, p: p}
			_, err = io.CopyN(writer, src, int64(item.Size))
			if err != nil {
				if doForce {
					doLog(false, "Unable to write data: %v :: %v", item.Path, err)
				} else {
					log.Fatalf("Unable to write data to file: %v :: %v", item.Path, err)
				}
			}
		}
		hashSum = hasher.Sum(nil)
		if len(hashSum) < int(checksumLength) {
			pad := make([]byte, int(checksumLength)-len(hashSum))
			hashSum = append(hashSum, pad...)
		}
	} else {
		if hasBlocks {
			for _, b := range item.Blocks {
				if _, err := arcB.Seek(int64(b.Offset), io.SeekStart); err != nil {
					log.Fatalf("seek block: %v", err)
				}
				r := io.LimitReader(arcB, int64(b.Size))
				if lfeat.IsNotSet(fNoCompress) {
					dec, err := decompressor(r, ctype)
					if err != nil {
						log.Fatalf("decompress setup: %v", err)
					}
					_, err = io.Copy(writer, progressReader{r: dec, p: p})
					if err != nil {
						log.Fatalf("copy block: %v", err)
					}
					dec.Close()
				} else {
					_, err = io.Copy(writer, progressReader{r: r, p: p})
				}
				if err != nil {
					log.Fatalf("copy block: %v", err)
				}
			}
		} else {
			var src io.Reader = arcB
			if lfeat.IsNotSet(fNoCompress) {
				dec, err := decompressor(arcB, ctype)
				if err != nil {
					log.Fatalf("decompress error: Unable to create reader: %v :: %v", item.Path, err)
				}
				defer dec.Close()
				src = dec
			}
			src = progressReader{r: src, p: p}
			_, err = io.CopyN(writer, src, int64(item.Size))
			if err != nil {
				if doForce {
					doLog(false, "Unable to write data: %v :: %v", item.Path, err)
				} else {
					log.Fatalf("Unable to write data to file: %v :: %v", item.Path, err)
				}
			}
		}
	}
	if err := bf.Close(); err != nil {
		log.Fatalf("extract: close failed: %v", err)
	}
	if lfeat.IsSet(fModDates) {
		os.Chtimes(finalPath, item.ModTime, item.ModTime)
	}

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
	return nil
}
