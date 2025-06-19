package goxa

import "runtime"

// ResetDefaults resets global configuration variables to their default values.
func ResetDefaults() {
	archivePath = ""
	verboseMode = false
	doForce = false
	toStdOut = false
	progress = false
	quietMode = false
	interactiveMode = true
	features = fChecksums
	useArchiveFlags = false
	compression = ""
	encode = ""
	compType = compZstd
	compSpeed = SpeedFastest
	checksumType = defaultChecksumType
	checksumLength = defaultChecksumLen
	tarUseXz = false
	extractList = nil
	protoVersion = protoVersion2
	blockSize = defaultBlockSize
	threads = runtime.NumCPU()
	fecDataShards = 10
	fecParityShards = 3
	fileRetries = 3
	fileRetryDelay = 5
	failOnChange = false
	bombCheck = true
	spaceCheck = true
	noFlush = false
}
