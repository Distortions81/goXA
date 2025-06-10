package main

const (
	magic    = "GOXA"
	version1 = 1
	version2 = 2

	readBuffer         = 1000 * 1000 * 1 //MiB
	writeBuffer        = readBuffer
	defaultArchiveName = "archive.goxa"
	checksumSize       = 32
	defaultBlockSize   = 512 * 1024 // 512KiB
	// maxEntries limits how many file or directory entries can be
	// allocated when reading an archive. This prevents huge memory
	// allocations on corrupted archives.
	maxEntries = 1_000_000
	// maxBlocks limits how many blocks can be assigned to a single file.
	// A block is at most blockSize bytes, so any count above the file's
	// theoretical maximum indicates corruption.
	maxBlocks = 1_000_000
)

// Features
const (
	fNone BitFlags = 1 << iota
	fAbsolutePaths
	fPermissions
	fModDates
	fChecksums
	fNoCompress
	fIncludeInvis
	fSpecialFiles
	fZstd
	fLZ4
	fS2
	fSnappy
	fBrotli
	fBlock

	fTop //Do not use, move or delete
)

var (
	flagNames = []string{"None", "Absolute Paths", "Permissions", "Mod Dates", "Checksums", "No Compress", "Include Invis", "Special Files", "Zstd", "LZ4", "S2", "Snappy", "Brotli", "Block", "Unknown"}
)

// Entry Types
const (
	entryFile uint8 = iota
	entrySymlink
	entryHardlink
	entryOther
)
