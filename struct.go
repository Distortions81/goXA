package main

import (
	"io/fs"
	"time"
)

var (
	archivePath                              string
	verboseMode, doForce, toStdOut, progress bool
	features                                 BitFlags
	extractList                              []string
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

	//Block mode
	NumBlocks   uint64
	BlockOffset []uint64
}
