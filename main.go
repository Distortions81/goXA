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
	defer startProfile()()

	if len(os.Args) < 2 {
		showUsage()
		fmt.Println("\nError: No mode specified.")
		return
	}

	if strings.HasPrefix(os.Args[1], "-") {
		if handleTopLevel(os.Args[1:]) {
			return
		}
	}

	cmdLetter, opts := parseCommand(os.Args[1])
	if !strings.ContainsRune("cljx", rune(cmdLetter)) {
		showUsage()
		fmt.Printf("\nError: Unknown mode: %s\n", os.Args[1])
		return
	}

	flagSet, mflags := initFlags()
	flagSet.Parse(os.Args[2:])

	quietMode = toStdOut || cmdLetter == 'j'
	if quietMode {
		progress = false
	}

	configureFEC(mflags)
	if mflags.showVer {
		fmt.Println("goxa v" + appVersion)
		return
	}

	mflags.format = detectArchiveFormat(cmdLetter, mflags.format)
	buildExtractList(mflags.sel)

	applyModeOptions(opts)

	configureCompression(mflags.format)
	configureSpeed(mflags.speedOpt)
	configureChecksum(mflags.sumOpt)

	ensureArchiveExtension(cmdLetter, mflags.format)

	runMode(cmdLetter, flagSet.Args(), mflags.format)
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
	fmt.Println("  m  preserve modification times  s  disable checksums")
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
	fmt.Println("  -spacecheck=false disable free space check")
	fmt.Println("  -noflush        skip final disk flush")
	fmt.Println("  -version        print program version")
	fmt.Println("  -pgo            run built-in PGO training (10k files ~2GB, s-curve around 150KB)")
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
	fmt.Println("  goxa -pgo                                     # generate default.pgo using 10k files")
	fmt.Println("  goxa c -arc=backup.goxa dir/                  # create archive")
	fmt.Println("  goxa x -arc=backup.goxa                       # extract to folder")
	fmt.Println("  goxa c -arc=backup.tar.gz dir/                # create tar.gz")
	fmt.Println("  goxa c -arc=backup.goxa.b64 dir/              # Base64 encoded archive")
	fmt.Println("  goxa c -arc=backup.goxaf dir/                 # FEC encoded archive")
}

type flagSettings struct {
	sel       string
	format    string
	speedOpt  string
	sumOpt    string
	fecData   int
	fecParity int
	fecLevel  string
	showVer   bool
}

func startProfile() func() {
	if !makeProfile {
		return func() {}
	}
	f, err := os.Create("perf.pprof")
	if err != nil {
		log.Fatalf("could not create CPU profile: %v", err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatalf("could not start CPU profile: %v", err)
	}
	return func() {
		pprof.StopCPUProfile()
		f.Close()
	}
}

func handleTopLevel(args []string) bool {
	fs := flag.NewFlagSet("goxa", flag.ExitOnError)
	var showVer bool
	var showHelp bool
	var pgo bool
	fs.BoolVar(&showVer, "version", false, "print version and exit")
	fs.BoolVar(&showHelp, "help", false, "show help")
	fs.BoolVar(&showHelp, "h", false, "show help")
	fs.BoolVar(&pgo, "pgo", false, "run PGO training and exit")
	fs.Parse(args)
	if showVer {
		fmt.Println("goxa v" + appVersion)
		return true
	}
	if pgo {
		runPGOTraining()
		return true
	}
	if showHelp {
		showUsage()
		return true
	}
	showUsage()
	return true
}

func parseCommand(cmd string) (byte, string) {
	cmd = strings.ToLower(cmd)
	if cmd == "" {
		return 0, ""
	}
	letter := cmd[0]
	opts := ""
	if len(cmd) > 1 {
		opts = cmd[1:]
	}
	return letter, opts
}

func initFlags() (*flag.FlagSet, *flagSettings) {
	fs := flag.NewFlagSet("goxa", flag.ExitOnError)
	f := &flagSettings{}
	fs.StringVar(&archivePath, "arc", defaultArchiveName, "archive file name (extension not required)")
	fs.BoolVar(&toStdOut, "stdout", false, "output archive data to stdout")
	fs.BoolVar(&progress, "progress", true, "show progress bar")
	fs.BoolVar(&interactiveMode, "interactive", true, "prompt when archive uses extra flags")
	fs.StringVar(&compression, "comp", "zstd", "compression: gzip|zstd|lz4|s2|snappy|brotli|xz|none")
	fs.StringVar(&f.speedOpt, "speed", "fastest", "compression speed: fastest|default|better|best")
	fs.StringVar(&f.sumOpt, "sum", "blake3", "checksum: crc32|crc16|xxhash|sha256|blake3")
	fs.StringVar(&f.format, "format", "goxa", "archive format: tar|goxa")
	fs.StringVar(&f.sel, "files", "", "comma-separated list of files and directories to extract")
	fs.IntVar(&f.fecData, "fec-data", fecDataShards, "FEC data shards")
	fs.IntVar(&f.fecParity, "fec-parity", fecParityShards, "FEC parity shards")
	fs.StringVar(&f.fecLevel, "fec-level", "", "FEC redundancy preset: low|medium|high")
	fs.IntVar(&fileRetries, "retries", 3, "retries when file changes during read (0=never give up)")
	fs.IntVar(&fileRetryDelay, "retrydelay", 5, "delay between retries in seconds")
	fs.BoolVar(&failOnChange, "failonchange", false, "treat file change after retries as fatal")
	fs.BoolVar(&bombCheck, "bombcheck", true, "detect extremely compressed files")
	fs.BoolVar(&spaceCheck, "spacecheck", true, "verify free disk space before operations")
	fs.BoolVar(&noFlush, "noflush", false, "skip final disk flush")
	fs.BoolVar(&f.showVer, "version", false, "print version and exit")
	return fs, f
}

func configureFEC(f *flagSettings) {
	switch f.fecLevel {
	case "low":
		fecDataShards, fecParityShards = 10, 3
	case "medium":
		fecDataShards, fecParityShards = 8, 4
	case "high":
		fecDataShards, fecParityShards = 5, 5
	case "":
		fecDataShards = f.fecData
		fecParityShards = f.fecParity
	default:
		log.Fatalf("invalid fec-level: %s", f.fecLevel)
	}
}

func detectArchiveFormat(cmdLetter byte, format string) string {
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
	return format
}

func buildExtractList(sel string) {
	if sel == "" {
		return
	}
	parts := strings.Split(sel, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		extractList = append(extractList, filepath.Clean(p))
	}
}

func applyModeOptions(opts string) {
	for _, letter := range opts {
		switch letter {
		case 'a':
			features.Set(fAbsolutePaths)
		case 'p':
			features.Set(fPermissions)
		case 'm':
			features.Set(fModDates)
		case 's':
			features.Clear(fChecksums)
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
}

func configureCompression(format string) {
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
}

func configureSpeed(speed string) {
	switch strings.ToLower(speed) {
	case "fastest":
		compSpeed = SpeedFastest
	case "default":
		compSpeed = SpeedDefault
	case "better":
		compSpeed = SpeedBetterCompression
	case "best":
		compSpeed = SpeedBestCompression
	default:
		log.Fatalf("Unknown speed: %s", speed)
	}
}

func configureChecksum(sum string) {
	switch strings.ToLower(sum) {
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
		log.Fatalf("Unknown checksum: %s", sum)
	}
}

func ensureArchiveExtension(cmdLetter byte, format string) {
	if cmdLetter != 'c' || hasKnownArchiveExt(archivePath) {
		return
	}
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

func runMode(cmdLetter byte, args []string, format string) {
	switch cmdLetter {
	case 'c':
		if strings.ToLower(format) == "tar" {
			if err := createTar(args); err != nil {
				log.Fatalf("tar create failed: %v", err)
			}
			return
		}
		create(args)
	case 'l':
		if strings.ToLower(format) == "tar" {
			log.Fatalf("list not supported for tar format")
		}
		extract(args, true, false)
	case 'j':
		if strings.ToLower(format) == "tar" {
			log.Fatalf("list not supported for tar format")
		}
		extract(args, true, true)
	case 'x':
		if archivePath == defaultArchiveName {
			log.Fatal("You must specify an archive to extract.")
		}
		if len(args) > 0 {
			if features.IsSet(fAbsolutePaths) {
				log.Fatal("Destination specified in conjunction with absolute path mode, stopping.")
			}
		}
		if strings.ToLower(format) == "tar" {
			dest := ""
			if len(args) > 0 {
				dest = args[0]
			}
			if err := extractTar(dest); err != nil {
				log.Fatalf("tar extract failed: %v", err)
			}
			return
		}
		extract(args, false, false)
	default:
		showUsage()
		doLog(false, "Unknown mode: %c", cmdLetter)
	}
}
