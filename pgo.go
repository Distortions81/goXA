package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/pprof"
)

// runPGOTraining performs a simple compression and decompression
// using default settings and writes a CPU profile to default.pgo.
func runPGOTraining() {
	fmt.Println("Generating default.pgo profile...")
	f, err := os.Create("default.pgo")
	if err != nil {
		log.Fatalf("pgo file: %v", err)
	}
	defer f.Close()
	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatalf("pgo start: %v", err)
	}
	defer pprof.StopCPUProfile()

	data := make([]byte, 5*1024*1024)
	if _, err := rand.Read(data); err != nil {
		log.Fatalf("rand: %v", err)
	}

	var buf bytes.Buffer
	zw := compressor(&buf)
	if _, err := zw.Write(data); err != nil {
		log.Fatalf("compress: %v", err)
	}
	if err := zw.Close(); err != nil {
		log.Fatalf("compress close: %v", err)
	}

	zr, err := decompressor(bytes.NewReader(buf.Bytes()), compType)
	if err != nil {
		log.Fatalf("decompress: %v", err)
	}
	if _, err := io.Copy(io.Discard, zr); err != nil {
		log.Fatalf("decompress copy: %v", err)
	}
	zr.Close()

	fmt.Println("default.pgo written")
}
