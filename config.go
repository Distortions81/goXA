package main

import (
	"io/fs"
	"time"
)

var (
	archivePath                              string
	verboseMode, doForce, toStdOut, progress bool
	features                                 BitFlags
	useArchiveFlags                          bool
	compression                              string
	extractList                              []string
	version                                  uint16 = version2
	blockSize                                uint32 = defaultBlockSize
)

type FileEntry struct {
	Offset   uint64
	Path     string
	SrcPath  string
	Linkname string
	Type     uint8
	Size     uint64
	Mode     fs.FileMode
	ModTime  time.Time
	Blocks   []Block
}

type Block struct {
	Offset uint64
	Size   uint32
}
