package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"

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
	flagSet := flag.NewFlagSet("goxa", flag.ExitOnError)
	flagSet.StringVar(&archivePath, "arc", defaultArchiveName, "archive file name (extension not required)")
	flagSet.BoolVar(&toStdOut, "stdout", false, "output archive data to stdout")
	flagSet.BoolVar(&progress, "progress", true, "show progress bar")
	flagSet.Parse(os.Args[2:])

	//Clean up archive name
	archivePath = removeExtension(archivePath)
	archivePath = archivePath + ".goxa"

	//Options
	for _, letter := range cmd {
		switch letter {

		case 'a':
			features.Set(fAbsolutePaths)
		case 'p':
			features.Set(fPermissions)
		case 'm':
			features.Set(fModDates)
		case 's':
			features.Set(fChecksums)
		case 'n':
			features.Set(fNoCompress)
		case 'i':
			features.Set(fIncludeInvis)
		case 'o':
			features.Set(fSpecialFiles)
		case 'v':
			verboseMode = true
		case 'f':
			doForce = true
		default:
			continue
		}
		cmd = cmd[:len(cmd)-1]
	}

	if len(cmd) == 0 {
		showUsage()
		log.Fatal("No mode specified")
	}

	//Modes
	switch cmd[0] {
	case 'c':
		create(flagSet.Args())
	case 'l':
		extract(flagSet.Args(), true)
	case 'x':
		if archivePath == defaultArchiveName {
			log.Fatal("You must specify an archive to extract.")
		}
		if len(flagSet.Args()) > 0 {
			if features.IsSet(fAbsolutePaths) {
				log.Fatal("Destination specified in conjunction with absolute path mode, stopping.")
			}
		}
		extract(flagSet.Args(), false)
	default:
		showUsage()
		doLog(false, "Unknown mode: %c", cmd[0])
		return
	}
}

func showUsage() {
	fmt.Println("Usage: goxa [c|l|x][apmsniveo] -arc=arcFile [input paths/files...] or [destination]")
	fmt.Println("Output archive to stdout: -stdout, No progress bar: -progress=false")
	fmt.Println("\nModes:")
	fmt.Println("  c = Create a new archive. Requires input paths or files")
	fmt.Println("  l = List archive contents. Requires -arc")
	fmt.Println("  x = Extract files from archive. Requires -arc")

	fmt.Println("\nOptions:")
	fmt.Print("  a = Absolute paths	")
	fmt.Println("  p = Permissions")
	fmt.Print("  m = Modification date	")
	fmt.Println("  s = Sums")
	fmt.Print("  n = No-compression	")
	fmt.Println("  i = Include dotfiles")
	fmt.Print("  o = Special files          ")
	fmt.Println("  v = Verbose logging")
	fmt.Print("  f = Force (overwrite files and ignore read errors)")
	fmt.Println()
	fmt.Println("  goxa c -arc=arcFile myStuff		(similar to zip)")
	fmt.Println("  goxa cpmi -arc=arcFile myStuff	(similar to tar -czf)")
	fmt.Println("")
	fmt.Println("  goxa x -arc=arcFile			(similar to unzip)")
	fmt.Println("  goxa xpmi -arc=arcFile		(similar to tar -xzf)")
}
