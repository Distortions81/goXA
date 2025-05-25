package main

import (
	"io/fs"
	"time"
)

var (
	archivePath                              string
	verboseMode, doForce, toStdOut, progress bool
	features                                 BitFlags
)

type FileEntry struct {
	Offset  uint64
	Path    string
	Size    uint64
	Mode    fs.FileMode
	ModTime time.Time

	//Block mode
	NumBlocks   uint64
	BlockOffset []uint64
}
