package main

const (
	magic   = "GOXA"
	version = 1

	readBuffer         = 1000 * 1000 * 1 //MiB
	writeBuffer        = readBuffer
	defaultArchiveName = "archive.goxa"
	checksumSize       = 32
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

	fTop //Do not use, move or delete
)

var (
	flagNames = []string{"None", "Absolute Paths", "Permissions", "Mod Dates", "Checksums", "No Compress", "Include Invis", "Special Files", "Zstd", "LZ4", "S2", "Snappy", "Brotli", "Unknown"}
)

// Entry Types
const (
	entryFile uint8 = iota
	entrySymlink
	entryHardlink
	entryOther
)
