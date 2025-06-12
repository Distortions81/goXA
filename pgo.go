package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"math"
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

	const (
		numFiles   = 10000
		minSize    = 100
		maxSize    = 200 * 1024 * 1024
		totalBytes = 2 * 1024 * 1024 * 1024
		baseSize   = 150 * 1024
	)

	sizes := make([]int64, numFiles)
	upperLog := math.Log(float64(maxSize) / float64(baseSize))
	lowerLog := math.Log(float64(baseSize) / float64(minSize))

	var sum int64
	for i := 0; i < numFiles; i++ {
		t := float64(i) / float64(numFiles-1)
		r := 1 / (1 + math.Exp(-12*(t-0.5)))
		if r >= 0.5 {
			f := (r - 0.5) * 2
			sizes[i] = int64(float64(baseSize) * math.Exp(f*upperLog))
		} else {
			f := (0.5 - r) * 2
			sizes[i] = int64(float64(baseSize) / math.Exp(f*lowerLog))
		}
		if sizes[i] < minSize {
			sizes[i] = minSize
		}
		if sizes[i] > maxSize {
			sizes[i] = maxSize
		}
		sum += sizes[i]
	}

	scale := float64(totalBytes) / float64(sum)
	var scaledSum int64
	for i := range sizes {
		sizes[i] = int64(float64(sizes[i]) * scale)
		scaledSum += sizes[i]
	}
	sizes[numFiles-1] += int64(totalBytes) - scaledSum

	for _, size := range sizes {

		data := make([]byte, size)
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
	}

	fmt.Println("default.pgo written")
}
