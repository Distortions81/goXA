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
	cmd := strings.ToLower(os.Args[1])
	cmdLetter := cmd[0]
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
	flagSet.Parse(os.Args[2:])

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
			log.Fatalf("Unknown compression: %s", compression)
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
	fmt.Println("Usage: goxa [c|l|j|x][apmsnbiveou] -arc=arcFile [-comp=alg] [input paths/files...] or [destination]")
	fmt.Println("Output archive to stdout: -stdout, No progress bar: -progress=false")
	fmt.Println("\nModes:")
	fmt.Println("  c = Create a new archive. Requires input paths or files")
	fmt.Println("  l = List archive contents. Requires -arc")
	fmt.Println("  j = JSON list of archive contents. Requires -arc")
	fmt.Println("  x = Extract files from archive. Requires -arc")

	fmt.Println("\nOptions:")
	fmt.Print("  a = Absolute paths	")
	fmt.Println("  p = Permissions")
	fmt.Print("  m = Modification date	")
	fmt.Println("  s = File sums")
	fmt.Print("  b = Block sums            ")
	fmt.Print("  n = No-compression	")
	fmt.Println("  i = Include dotfiles")
	fmt.Print("  o = Special files          ")
	fmt.Println("  u = Use archive flags")
	fmt.Println("  v = Verbose logging")
	fmt.Print("  f = Force (overwrite files and ignore read errors)")
	fmt.Println("  -comp=gzip|zstd|lz4|s2|snappy|brotli|xz|none")
	fmt.Println("  -speed=fastest|default|better|best")
	fmt.Println("  -format=tar|goxa")
	fmt.Println()
	fmt.Println("  goxa c -arc=arcFile myStuff		(similar to zip)")
	fmt.Println("  goxa cpmi -arc=arcFile myStuff	(similar to tar -czf)")
	fmt.Println("")
	fmt.Println("  goxa x -arc=arcFile			(similar to unzip)")
	fmt.Println("  goxa xpmi -arc=arcFile		(similar to tar -xzf)")
}
