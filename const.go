package main

const (
	magic         = "GOXA"
	appVersion    = "0.0.92"
	protoVersion2 = 2

	defaultArchiveName = "archive.goxa"
	defaultBlockSize   = 512 * 1024 // 512KiB
	// fat32SpanSize is used when -span is specified without a value.
	fat32SpanSize = 4*1024*1024*1024 - 64*1024 // 4GiB - 64KiB
	// defaultSpanSize disables spanning by default (max int64).
	defaultSpanSize = int64(^uint64(0) >> 1)
)

var (
	readBuffer  = int(defaultBlockSize) * 4
	writeBuffer = readBuffer
)

// Checksum types
const (
	sumCRC32 uint8 = iota
	sumCRC16
	sumXXHash
	sumSHA256
	sumBlake3
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
	fBlockChecksums

	fTop //Do not use, move or delete
)

var (
	flagNames = []string{"None", "Absolute Paths", "Permissions", "Modification Times", "Checksums", "No Compress", "Hidden Files", "Special Files", "Block Checksums", "Unknown"}
)

// Entry Types
const (
	entryFile uint8 = iota
	entrySymlink
	entryHardlink
	entryOther
)

// Compression Types
const (
	compGzip uint8 = iota
	compZstd
	compLZ4
	compS2
	compSnappy
	compBrotli
	compXZ
)

// Compression speed levels
const (
	SpeedFastest = iota
	SpeedDefault
	SpeedBetterCompression
	SpeedBestCompression
)

// Zip bomb detection thresholds
const (
	zipBombMinSize = 10 * 1024 * 1024 // 10MiB
	zipBombRatio   = 100              // uncompressed/compressed ratio
)
