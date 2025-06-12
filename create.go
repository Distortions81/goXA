package main

import (
	"bufio"
	"bytes"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	brotli "github.com/andybalholm/brotli"
	"github.com/dustin/go-humanize"
	"github.com/golang/snappy"
	"github.com/klauspost/compress/s2"
	"github.com/klauspost/compress/zstd"
	gzip "github.com/klauspost/pgzip"
	lz4 "github.com/pierrec/lz4/v4"
	"github.com/ulikunitz/xz"
)

func compressor(w io.Writer) io.WriteCloser {
	switch compType {
	case compZstd:
		level := zstd.SpeedFastest
		switch compSpeed {
		case SpeedFastest:
			level = zstd.SpeedFastest
		case SpeedDefault:
			level = zstd.SpeedDefault
		case SpeedBetterCompression:
			level = zstd.SpeedBetterCompression
		case SpeedBestCompression:
			level = zstd.SpeedBestCompression
		}
		zw, err := zstd.NewWriter(w, zstd.WithEncoderLevel(level))
		if err != nil {
			log.Fatalf("zstd init failed: %v", err)
		}
		return zw
	case compLZ4:
		zw := lz4.NewWriter(w)
		lvl := lz4.Fast
		switch compSpeed {
		case SpeedDefault:
			lvl = lz4.Level3
		case SpeedBetterCompression:
			lvl = lz4.Level6
		case SpeedBestCompression:
			lvl = lz4.Level9
		}
		if err := zw.Apply(lz4.CompressionLevelOption(lvl)); err != nil {
			log.Fatalf("lz4 level: %v", err)
		}
		return zw
	case compS2:
		opts := []s2.WriterOption{}
		switch compSpeed {
		case SpeedBetterCompression:
			opts = append(opts, s2.WriterBetterCompression())
		case SpeedBestCompression:
			opts = append(opts, s2.WriterBestCompression())
		}
		return s2.NewWriter(w, opts...)
	case compSnappy:
		return snappy.NewBufferedWriter(w)
	case compBrotli:
		level := brotli.BestSpeed
		switch compSpeed {
		case SpeedDefault:
			level = brotli.DefaultCompression
		case SpeedBetterCompression:
			level = 9
		case SpeedBestCompression:
			level = brotli.BestCompression
		}
		return brotli.NewWriterLevel(w, level)
	case compXZ:
		xzw, err := xz.NewWriter(w)
		if err != nil {
			log.Fatalf("xz init failed: %v", err)
		}
		return xzw
	default:
		lvl := gzip.BestSpeed
		switch compSpeed {
		case SpeedDefault:
			lvl = gzip.DefaultCompression
		case SpeedBetterCompression:
			lvl = 8
		case SpeedBestCompression:
			lvl = gzip.BestCompression
		}
		zw, err := gzip.NewWriterLevel(w, lvl)
		if err != nil {
			log.Fatalf("gzip init: %v", err)
		}
		return zw
	}
}

func create(inputPaths []string) error {

	var bf *BufferedFile
	var tmpPath string
	var outFile *os.File
	if toStdOut && encode == "" {
		bf = NewBufferedFile(os.Stdout, writeBuffer, &progressData{})
		outFile = os.Stdout
	} else {
		if encode != "" || toStdOut {
			f, err := os.CreateTemp("", "goxa_tmp_*")
			if err != nil {
				log.Fatalf("temp create: %v", err)
			}
			tmpPath = f.Name()
			outFile = f
			defer outFile.Close()
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
			outFile = f
			defer outFile.Close()
		}
		bf = NewBufferedFile(outFile, writeBuffer, &progressData{})
	}
	doLog(false, "Creating archive: %v, inputs: %v", archivePath, inputPaths)

	emptyDirs, files, err := walkPaths(inputPaths)
	if err != nil {
		return err
	}

	if spaceCheck {
		var totalBytes int64
		for _, f := range files {
			totalBytes += int64(f.Size)
		}
		outDir := filepath.Dir(outFile.Name())
		free, total, err := getDiskSpace(outDir)
		if err != nil {
			doLog(false, "warning: free space check failed: %v", err)
		} else {
			need := uint64(totalBytes)
			if need > free {
				log.Fatalf("create: insufficient disk space: need %v, available %v", humanize.Bytes(need), humanize.Bytes(free))
			}
			if free-need < total/100 {
				msg := fmt.Sprintf("create would leave %v free", humanize.Bytes(free-need))
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

	if features.IsSet(fNoCompress) {
		blockSize = 0
	} else if blockSize == 0 {
		blockSize = defaultBlockSize
	}

	header := writeHeader(emptyDirs, files, 0, 0, features, compType)
	headerLen := len(header)
	bf.Write(header)
	files, trailerOffset := writeEntries(headerLen, bf, files)
	trailer := writeTrailer(files)
	start := time.Now()
	bf.Write(trailer)
	if time.Since(start) > time.Second {
		fmt.Printf("writing offset table took %v\n", time.Since(start))
	}
	if err := bf.Flush(); err != nil {
		log.Fatalf("flush: %v", err)
	}
	info, err := bf.file.Stat()
	if err != nil {
		log.Fatalf("create: os.Create: %v", err)
	}
	finalHeader := writeHeader(emptyDirs, files, trailerOffset, uint64(info.Size()), features, compType)
	if len(finalHeader) != headerLen {
		log.Fatalf("header size mismatch")
	}
	if _, err := bf.Seek(0, io.SeekStart); err != nil {
		log.Fatalf("seek start: %v", err)
	}
	bf.Write(finalHeader)

	start = time.Now()
	if err := bf.Close(); err != nil {
		log.Fatalf("create: close failed: %v", err)
	}
	if time.Since(start) > time.Second {
		fmt.Println("flushing to disk")
	}

	if encode != "" {
		outFile.Close()
		if encode == "fec" {
			if toStdOut {
				log.Fatalf("FEC encoding not supported with stdout")
			}
			doLog(false, "FEC encoding archive")
			if err := encodeWithFEC(tmpPath, archivePath); err != nil {
				log.Fatalf("fec encode: %v", err)
			}
			os.Remove(tmpPath)
			if st, err := os.Stat(archivePath); err == nil {
				info = st
			}
		} else {
			src, err := os.Open(tmpPath)
			if err != nil {
				log.Fatalf("temp reopen: %v", err)
			}
			defer os.Remove(tmpPath)

			st, err := src.Stat()
			if err != nil {
				src.Close()
				log.Fatalf("stat tmp: %v", err)
			}
			info = st

			var dst io.Writer
			var encW io.WriteCloser
			if toStdOut {
				dst = os.Stdout
			} else {
				f, err := os.Create(archivePath)
				if err != nil {
					log.Fatalf("create output: %v", err)
				}
				defer f.Close()
				dst = f
			}
			if encode == "b32" {
				doLog(false, "Base32 encoding archive")
				encW = base32.NewEncoder(base32.StdEncoding, dst)
			} else {
				doLog(false, "Base64 encoding archive")
				encW = base64.NewEncoder(base64.StdEncoding, dst)
			}

			p, done, finished := progressTicker(&progressData{total: info.Size(), speedWindowSize: time.Second * 5})
			p.file.Store(archivePath)

			if _, err := io.Copy(encW, progressReader{r: src, p: p}); err != nil {
				close(done)
				<-finished
				log.Fatalf("encode copy: %v", err)
			}
			encW.Close()
			src.Close()
			close(done)
			<-finished

			if !toStdOut {
				if st, err := os.Stat(archivePath); err == nil {
					info = st
				}
			}
		}
	}

	doLog(false, "\nWrote %v, %v containing %v files.", archivePath, humanize.Bytes(uint64(info.Size())), len(files))
	return nil
}

func writeHeader(emptyDirs, files []FileEntry, trailerOffset, arcSize uint64, flags BitFlags, cType uint8) []byte {
	var header bytes.Buffer

	binary.Write(&header, binary.LittleEndian, []byte(magic))
	binary.Write(&header, binary.LittleEndian, uint16(protoVersion))
	binary.Write(&header, binary.LittleEndian, flags)
	binary.Write(&header, binary.LittleEndian, cType)
	binary.Write(&header, binary.LittleEndian, checksumType)
	binary.Write(&header, binary.LittleEndian, checksumLength)
	binary.Write(&header, binary.LittleEndian, blockSize)
	binary.Write(&header, binary.LittleEndian, trailerOffset)
	binary.Write(&header, binary.LittleEndian, arcSize)

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
		if protoVersion >= protoVersion2 {
			if file.Changed {
				header.WriteByte(1)
			} else {
				header.WriteByte(0)
			}
		}
	}
	// File offsets are tracked in the trailer only
	h := newHasher(checksumType)
	h.Write(header.Bytes())
	sum := h.Sum(nil)
	if len(sum) < int(checksumLength) {
		pad := make([]byte, int(checksumLength)-len(sum))
		sum = append(sum, pad...)
	}
	header.Write(sum[:checksumLength])
	return header.Bytes()
}

func writeEntries(headerLen int, bf *BufferedFile, files []FileEntry) ([]FileEntry, uint64) {
	h := newHasher(checksumType)

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
	var buf []byte
	if blockSize > 0 {
		buf = make([]byte, blockSize)
	}

	newFiles := make([]FileEntry, 0, len(files))

	for i := range files {
		entry := &files[i]
		p.file.Store(entry.Path)
		if entry.Type != entryFile {
			entry.Offset = 0
			newFiles = append(newFiles, *entry)
			continue
		}

		attempt := 0
		hadChange := false
	retryLoop:
		for {
			attempt++
			startOffset := cOffset

			f, err := os.Open(entry.SrcPath)
			if err != nil {
				if doForce {
					doLog(false, "\nUnable to open file: %v (continuing)", entry.Path)
					break retryLoop
				}
				log.Fatalf("Unable to open file: %v", entry.Path)
			}

			statStart, err := f.Stat()
			if err != nil {
				f.Close()
				if doForce {
					doLog(false, "\nStat failed: %v (continuing)", entry.Path)
					break retryLoop
				}
				log.Fatalf("stat failed: %v", err)
			}

			var checksum []byte
			if features.IsSet(fChecksums) {
				h.Reset()
				if _, err := io.Copy(h, f); err != nil {
					f.Close()
					log.Fatalf("checksum compute failed: %v", err)
				}
				checksum = h.Sum(nil)
				if _, err := f.Seek(0, io.SeekStart); err != nil {
					f.Close()
					log.Fatalf("seek reset failed: %v", err)
				}
				if _, err := bf.Write(checksum); err != nil {
					f.Close()
					log.Fatalf("writing checksum failed: %v", err)
				}
			}

			entry.Offset = cOffset
			if features.IsSet(fChecksums) {
				cOffset += uint64(checksumLength)
			}

			br := NewBufferedFile(f, writeBuffer, p)
			var blocks []Block

			if blockSize == 0 {
				bOff := cOffset
				var written uint64
				if features.IsSet(fNoCompress) {
					if _, err := io.Copy(bf, br); err != nil {
						f.Close()
						log.Fatalf("copy failed: %v", err)
					}
					written = uint64(statStart.Size())
				} else {
					cw := &countingWriter{w: bf}
					zw := compressor(cw)
					if _, err := io.Copy(zw, br); err != nil {
						f.Close()
						log.Fatalf("compress copy failed: %v", err)
					}
					if err := zw.Close(); err != nil {
						f.Close()
						log.Fatalf("compress close failed: %v", err)
					}
					written = uint64(cw.Count())
				}
				cOffset += written
				blocks = append(blocks, Block{Offset: bOff, Size: written})
			} else {
				for {
					n, err := io.ReadFull(br, buf)
					if n > 0 {
						bOff := cOffset
						if features.IsSet(fNoCompress) {
							if _, err := bf.Write(buf[:n]); err != nil {
								f.Close()
								log.Fatalf("copy failed: %v", err)
							}
							cOffset += uint64(n)
							blocks = append(blocks, Block{Offset: bOff, Size: uint64(n)})
						} else {
							cw := &countingWriter{w: bf}
							zw := compressor(cw)
							if _, err := zw.Write(buf[:n]); err != nil {
								f.Close()
								log.Fatalf("compress copy failed: %v", err)
							}
							if err := zw.Close(); err != nil {
								f.Close()
								log.Fatalf("compress close failed: %v", err)
							}
							cOffset += uint64(cw.Count())
							blocks = append(blocks, Block{Offset: bOff, Size: uint64(cw.Count())})
						}
					}
					if err == io.EOF {
						break
					}
					if err == io.ErrUnexpectedEOF {
						break
					}
					if err != nil {
						f.Close()
						log.Fatalf("read block failed: %v", err)
					}
				}
			}
			br.Close()
			f.Close()

			statEnd, err := os.Stat(entry.SrcPath)
			if err == nil && (statEnd.Size() != statStart.Size() || !statEnd.ModTime().Equal(statStart.ModTime())) {
				hadChange = true
				if fileRetries == 0 || attempt < fileRetries {
					doLog(false, "\nFile changed during read: %v (retrying)", entry.Path)
					if _, err := bf.Seek(int64(startOffset), io.SeekStart); err != nil {
						log.Fatalf("seek reset failed: %v", err)
					}
					cOffset = startOffset
					time.Sleep(time.Duration(fileRetryDelay) * time.Second)
					continue retryLoop
				}
				if failOnChange {
					log.Fatalf("File changed during read: %v", entry.Path)
				}
				doLog(false, "\nFile changed during read: %v (skipping)", entry.Path)
				if _, err := bf.Seek(int64(startOffset), io.SeekStart); err != nil {
					log.Fatalf("seek reset failed: %v", err)
				}
				cOffset = startOffset
				break retryLoop
			}

			entry.Size = uint64(statEnd.Size())
			entry.ModTime = statEnd.ModTime()
			entry.Blocks = blocks
			if hadChange {
				entry.Changed = true
			}
			newFiles = append(newFiles, *entry)
			break retryLoop
		}
	}
	return newFiles, cOffset
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
	h := newHasher(checksumType)
	h.Write(trailer.Bytes())
	sum := h.Sum(nil)
	if len(sum) < int(checksumLength) {
		pad := make([]byte, int(checksumLength)-len(sum))
		sum = append(sum, pad...)
	}
	trailer.Write(sum[:checksumLength])
	return trailer.Bytes()
}
