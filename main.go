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
			fmt.Println(version)
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
	var speedOpt string
	flagSet.StringVar(&compression, "comp", "zstd", "compression: gzip|zstd|lz4|s2|snappy|brotli|xz|none")
	flagSet.StringVar(&speedOpt, "speed", "fastest", "compression speed: fastest|default|better|best")
	flagSet.StringVar(&format, "format", "goxa", "archive format: tar|goxa")
	flagSet.StringVar(&sel, "files", "", "comma-separated list of files and directories to extract")
	flagSet.IntVar(&fileRetries, "retries", 3, "retries when file changes during read (0=never give up)")
	flagSet.IntVar(&fileRetryDelay, "retrydelay", 5, "delay between retries in seconds")
	flagSet.BoolVar(&failOnChange, "failonchange", false, "treat file change after retries as fatal")
	var showVer bool
	flagSet.BoolVar(&showVer, "version", false, "print version and exit")
	flagSet.Parse(os.Args[2:])
	if showVer {
		fmt.Println(version)
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
	fmt.Println("Usage:")
	fmt.Println("  goxa [mode] [flags] -arc=FILE [paths...]")

	fmt.Println("\nModes:")
	fmt.Println("  c - create an archive")
	fmt.Println("  l - list contents")
	fmt.Println("  j - JSON list")
	fmt.Println("  x - extract files")

	fmt.Println("\nFlags:")
	fmt.Print("  a = Absolute paths          ")
	fmt.Println("p = File permissions")
	fmt.Print("  m = Modification times      ")
	fmt.Println("s = Enable checksums")
	fmt.Print("  b = Block checksums         ")
	fmt.Println("n = Disable compression")
	fmt.Print("  i = Hidden files            ")
	fmt.Println("o = Special files")
	fmt.Print("  u = Use archive flags       ")
	fmt.Println("v = Verbose logging")
	fmt.Println("  f = Force overwrite/ignore errors")

	fmt.Println("\nExtra flags:")
	fmt.Println("  -arc=FILE       Archive file name")
	fmt.Println("  -stdout         Output archive to stdout")
	fmt.Println("  -files=LIST     Files/directories to extract")
	fmt.Println("  -progress=false Disable progress display")
	fmt.Println("  -comp=ALG       Compression algorithm (gzip, zstd, lz4, s2, snappy, brotli, xz, none)")
	fmt.Println("  -speed=LEVEL    Compression speed (fastest, default, better, best)")
	fmt.Println("  -format=FORMAT  Archive format (goxa or tar)")
	fmt.Println("  -retries=N      Retries when a file changes during read (0=never give up)")
	fmt.Println("  -retrydelay=N   Delay between retries in seconds")
	fmt.Println("  -failonchange   Treat changed files as fatal errors")
	fmt.Println("  -version        Print version and exit")
	fmt.Println("  (append .b32 or .b64 to -arc for Base32 or Base64 encoding)")

	fmt.Println("\nExamples:")
	fmt.Println("  goxa -version                                 # print version")
	fmt.Println("  goxa c -arc=mybackup.goxa myStuff/            # create archive")
	fmt.Println("  goxa x -arc=mybackup.goxa                     # extract to folder")
	fmt.Println("  goxa l -arc=mybackup.goxa                     # list contents")
	fmt.Println("  goxa c -arc=mybackup.tar.gz myStuff/          # create tar.gz")
	fmt.Println("  goxa x -arc=mybackup.tar.xz                   # extract tar.xz")
	fmt.Println("  goxa c -arc=mybackup.goxa.b64 myStuff/        # create Base64 encoded")
	fmt.Println("  goxa x -arc=mybackup.goxa.b64                 # extract encoded archive")
}
