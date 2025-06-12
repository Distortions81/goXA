package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"

	"path/filepath"
	"strings"
)

const makeProfile = false

func main() {

	if makeProfile {
		f, err := os.Create("perf.pprof")
		if err != nil {
			log.Fatalf("could not create CPU profile: %v", err)
		}
		defer f.Close()

		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatalf("could not start CPU profile: %v", err)
		}
		defer pprof.StopCPUProfile()
	}

	if len(os.Args) < 2 {
		showUsage()
		fmt.Println("\nError: No mode specified.")
		return
	}

	if strings.HasPrefix(os.Args[1], "-") {
		fs := flag.NewFlagSet("goxa", flag.ExitOnError)
		var showVer bool
		var showHelp bool
		fs.BoolVar(&showVer, "version", false, "print version and exit")
		fs.BoolVar(&showHelp, "help", false, "show help")
		fs.BoolVar(&showHelp, "h", false, "show help")
		fs.Parse(os.Args[1:])
		if showVer {
			fmt.Println("goxa v" + appVersion)
			return
		}
		showUsage()
		return
	}

	cmd := strings.ToLower(os.Args[1])
	cmdLetter := cmd[0]
	if !strings.ContainsRune("cljx", rune(cmdLetter)) {
		showUsage()
		fmt.Printf("\nError: Unknown mode: %s\n", cmd)
		return
	}
	opts := ""
	if len(cmd) > 1 {
		opts = cmd[1:]
	}
	flagSet := flag.NewFlagSet("goxa", flag.ExitOnError)
	var sel string
	var format string
	flagSet.StringVar(&archivePath, "arc", defaultArchiveName, "archive file name (extension not required)")
	flagSet.BoolVar(&toStdOut, "stdout", false, "output archive data to stdout")
	flagSet.BoolVar(&progress, "progress", true, "show progress bar")
	flagSet.BoolVar(&interactiveMode, "interactive", true, "prompt when archive uses extra flags")
	var speedOpt string
	var sumOpt string
	flagSet.StringVar(&compression, "comp", "zstd", "compression: gzip|zstd|lz4|s2|snappy|brotli|xz|none")
	flagSet.StringVar(&speedOpt, "speed", "fastest", "compression speed: fastest|default|better|best")
	flagSet.StringVar(&sumOpt, "sum", "blake3", "checksum: crc32|crc16|xxhash|sha256|blake3")
	flagSet.StringVar(&format, "format", "goxa", "archive format: tar|goxa")
	flagSet.StringVar(&sel, "files", "", "comma-separated list of files and directories to extract")
	var fecData int
	var fecParity int
	var fecLevel string
	flagSet.IntVar(&fecData, "fec-data", fecDataShards, "FEC data shards")
	flagSet.IntVar(&fecParity, "fec-parity", fecParityShards, "FEC parity shards")
	flagSet.StringVar(&fecLevel, "fec-level", "", "FEC redundancy preset: low|medium|high")
	flagSet.IntVar(&fileRetries, "retries", 3, "retries when file changes during read (0=never give up)")
	flagSet.IntVar(&fileRetryDelay, "retrydelay", 5, "delay between retries in seconds")
	flagSet.BoolVar(&failOnChange, "failonchange", false, "treat file change after retries as fatal")
	flagSet.BoolVar(&bombCheck, "bombcheck", true, "detect extremely compressed files")
	var showVer bool
	flagSet.BoolVar(&showVer, "version", false, "print version and exit")
	flagSet.Parse(os.Args[2:])
	quietMode = toStdOut || cmdLetter == 'j'
	if quietMode {
		progress = false
	}
	switch fecLevel {
	case "low":
		fecDataShards, fecParityShards = 10, 3
	case "medium":
		fecDataShards, fecParityShards = 8, 4
	case "high":
		fecDataShards, fecParityShards = 5, 5
	case "":
		fecDataShards = fecData
		fecParityShards = fecParity
	default:
		log.Fatalf("invalid fec-level: %s", fecLevel)
	}
	if showVer {
		fmt.Println("goxa v" + appVersion)
		return
	}

	detFmt, noComp := detectFormatFromExt(archivePath)
	if detFmt != "" {
		format = detFmt
		if detFmt == "tar" {
			if noComp {
				features.Set(fNoCompress)
			} else {
				features.Clear(fNoCompress)
			}
		}
	}

	if cmdLetter != 'c' {
		if fFmt, fNoComp, ok := detectFormatFromHeader(archivePath); ok {
			format = fFmt
			if fFmt == "tar" {
				if fNoComp {
					features.Set(fNoCompress)
				} else {
					features.Clear(fNoCompress)
				}
			}
		}
	}

	if sel != "" {
		parts := strings.Split(sel, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			extractList = append(extractList, filepath.Clean(p))
		}
	}

	// Clean up archive name will occur after options are parsed

	//Options
	for _, letter := range opts {
		switch letter {

		case 'a':
			features.Set(fAbsolutePaths)
		case 'p':
			features.Set(fPermissions)
		case 'm':
			features.Set(fModDates)
		case 's':
			features.Set(fChecksums)
		case 'b':
			features.Set(fChecksums)
			features.Set(fBlockChecksums)
		case 'n':
			features.Set(fNoCompress)
		case 'i':
			features.Set(fIncludeInvis)
		case 'o':
			features.Set(fSpecialFiles)
		case 'u':
			useArchiveFlags = true
		case 'v':
			verboseMode = true
		case 'f':
			doForce = true
		default:
			continue
		}
	}

	switch strings.ToLower(compression) {
	case "gzip":
		compType = compGzip
	case "zstd":
		compType = compZstd
	case "lz4":
		compType = compLZ4
	case "s2":
		compType = compS2
	case "snappy":
		compType = compSnappy
	case "brotli":
		compType = compBrotli
	case "xz":
		if strings.ToLower(format) == "tar" {
			tarUseXz = true
		} else {
			compType = compXZ
		}
	case "none":
		features.Set(fNoCompress)
		compType = compGzip
	default:
		log.Fatalf("Unknown compression: %s", compression)
	}

	switch strings.ToLower(speedOpt) {
	case "fastest":
		compSpeed = SpeedFastest
	case "default":
		compSpeed = SpeedDefault
	case "better":
		compSpeed = SpeedBetterCompression
	case "best":
		compSpeed = SpeedBestCompression
	default:
		log.Fatalf("Unknown speed: %s", speedOpt)
	}

	switch strings.ToLower(sumOpt) {
	case "crc32":
		checksumType = sumCRC32
		checksumLength = 4
	case "crc16":
		checksumType = sumCRC16
		checksumLength = 2
	case "xxhash":
		checksumType = sumXXHash
		checksumLength = 8
	case "sha256":
		checksumType = sumSHA256
		checksumLength = 32
	case "blake3":
		checksumType = sumBlake3
		checksumLength = 32
	default:
		log.Fatalf("Unknown checksum: %s", sumOpt)
	}

	if cmdLetter == 'c' && !hasKnownArchiveExt(archivePath) {
		if strings.ToLower(format) == "tar" {
			if features.IsNotSet(fNoCompress) {
				if tarUseXz {
					archivePath += ".tar.xz"
				} else {
					archivePath += ".tar.gz"
				}
			} else {
				archivePath += ".tar"
			}
		} else {
			archivePath += ".goxa"
		}
	}

	//Modes
	switch cmdLetter {
	case 'c':
		if strings.ToLower(format) == "tar" {
			if err := createTar(flagSet.Args()); err != nil {
				log.Fatalf("tar create failed: %v", err)
			}
			return
		}
		create(flagSet.Args())
	case 'l':
		if strings.ToLower(format) == "tar" {
			log.Fatalf("list not supported for tar format")
		}
		extract(flagSet.Args(), true, false)
	case 'j':
		if strings.ToLower(format) == "tar" {
			log.Fatalf("list not supported for tar format")
		}
		extract(flagSet.Args(), true, true)
	case 'x':
		if archivePath == defaultArchiveName {
			log.Fatal("You must specify an archive to extract.")
		}
		if len(flagSet.Args()) > 0 {
			if features.IsSet(fAbsolutePaths) {
				log.Fatal("Destination specified in conjunction with absolute path mode, stopping.")
			}
		}
		if strings.ToLower(format) == "tar" {
			dest := ""
			if len(flagSet.Args()) > 0 {
				dest = flagSet.Args()[0]
			}
			if err := extractTar(dest); err != nil {
				log.Fatalf("tar extract failed: %v", err)
			}
			return
		}
		extract(flagSet.Args(), false, false)
	default:
		showUsage()
		doLog(false, "Unknown mode: %c", cmd[0])
		return
	}
}

func showUsage() {
	fmt.Println("goxa - Go eXpress Archive")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  goxa MODE[flags] [options] -arc FILE [paths...]")
	fmt.Println()
	fmt.Println("Modes:")
	fmt.Println("  c   create an archive")
	fmt.Println("  l   list archive contents")
	fmt.Println("  j   output JSON listing")
	fmt.Println("  x   extract files")

	fmt.Println()
	fmt.Println("Flags (append after the mode letter):")
	fmt.Println("  a  store absolute paths         p  preserve permissions")
	fmt.Println("  m  preserve modification times  s  include checksums")
	fmt.Println("  b  per-block checksums          n  disable compression")
	fmt.Println("  i  include hidden files         o  allow special files")
	fmt.Println("  u  use flags from archive       v  verbose output")
	fmt.Println("  f  force overwrite / ignore read errors")

	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -arc FILE       archive file name")
	fmt.Println("  -stdout         write archive to stdout")
	fmt.Println("  -files LIST     comma separated files to extract")
	fmt.Println("  -progress=false disable progress display")
	fmt.Println("  -interactive=false disable prompts for archive flags")
	fmt.Println("  -comp ALG       compression algorithm (gzip, zstd, lz4, s2, snappy, brotli, xz, none)")
	fmt.Println("  -speed LEVEL    compression speed (fastest, default, better, best)")
	fmt.Println("  -sum ALG        checksum algorithm (crc32, crc16, xxhash, sha256, blake3)")
	fmt.Println("  -format FORMAT  archive format (goxa or tar)")
	fmt.Println("  -retries N      retries when file changes during read (0 = never give up)")
	fmt.Println("  -retrydelay N   delay between retries in seconds")
	fmt.Println("  -failonchange   treat changed files as fatal errors")
	fmt.Println("  -bombcheck=false disable zip bomb detection")
	fmt.Println("  -version        print program version")
	fmt.Println("  -fec-data N     number of FEC data shards (default 10)")
	fmt.Println("  -fec-parity N   number of FEC parity shards (default 3)")
	fmt.Println("  -fec-level L    FEC redundancy preset (low, medium, high)")

	fmt.Println()
	fmt.Println("Extensions:")
	fmt.Println("  .b32/.b64       output Base32/Base64 encoded archive")
	fmt.Println("  .goxaf          FEC encoded archive")
	fmt.Println("  .tar, .tar.gz   tar archive (gzipped if .gz)")
	fmt.Println("  .tar.xz         tar archive compressed with xz")

	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  goxa -version                                 # print version")
	fmt.Println("  goxa c -arc=backup.goxa dir/                  # create archive")
	fmt.Println("  goxa x -arc=backup.goxa                       # extract to folder")
	fmt.Println("  goxa c -arc=backup.tar.gz dir/                # create tar.gz")
	fmt.Println("  goxa c -arc=backup.goxa.b64 dir/              # Base64 encoded archive")
	fmt.Println("  goxa c -arc=backup.goxaf dir/                 # FEC encoded archive")
}
