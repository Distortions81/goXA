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
	encode                                   string
	compType                                 uint8 = compZstd
	compSpeed                                int   = SpeedFastest
	checksumType                             uint8 = defaultChecksumType
	checksumLength                           uint8 = defaultChecksumLen
	tarUseXz                                 bool
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

type ListEntry struct {
	Path     string      `json:"path"`
	Type     string      `json:"type"`
	Size     uint64      `json:"size,omitempty"`
	Mode     fs.FileMode `json:"mode,omitempty"`
	ModTime  time.Time   `json:"modTime,omitempty"`
	Linkname string      `json:"link,omitempty"`
}

type ArchiveListing struct {
	Version        uint16      `json:"version"`
	Flags          BitFlags    `json:"flags"`
	Compression    string      `json:"compression"`
	Checksum       string      `json:"checksum"`
	ChecksumLength uint8       `json:"checksumLength"`
	BlockSize      uint32      `json:"blockSize"`
	ArchiveSize    uint64      `json:"archiveSize"`
	Dirs           []ListEntry `json:"dirs"`
	Files          []ListEntry `json:"files"`
}
