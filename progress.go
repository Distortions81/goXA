package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
)

const (
	// barWidth sets the number of characters used for the visual progress bar
	// portion of the display. A slightly shorter bar leaves room for the
	// current filename so it remains visible as operations progress.
	barWidth     = 45
	updatePeriod = time.Second / 4
)

type sample struct {
	timestamp time.Time
	bytes     int64
}

type progressData struct {
	current, written atomic.Int64
	total            int64
	speedWindow      []sample
	speedWindowSize  time.Duration
	lastPrintStr     string
	file             atomic.Value
}

func progressTicker(p *progressData) (*progressData, chan struct{}, chan struct{}) {
	done := make(chan struct{})
	finished := make(chan struct{})
	if !progress {
		close(finished)
		return p, done, finished
	}

	go func() {
		ticker := time.NewTicker(updatePeriod)
		defer ticker.Stop()
		defer close(finished)

		for {
			select {
			case <-ticker.C:
				printProgress(p)
			case <-done:
				printProgress(p)
				if progress {
					fmt.Print("\n")
				}
				return
			}
		}
	}()

	return p, done, finished
}

func printProgress(p *progressData) {
	if !progress {
		return
	}
	now := time.Now()

	// Compute progress
	progress := 1.0
	if p.total > 0 {
		progress = float64(p.current.Load()) / float64(p.total)
		if progress > 1 {
			progress = 1
		}
	}
	filled := int(progress * barWidth)
	if filled > barWidth {
		filled = barWidth
	}

	// Add current sample
	var speed float64
	p.speedWindow = append(p.speedWindow, sample{timestamp: now, bytes: p.written.Load()})

	// Remove old samples
	cutoff := now.Add(-p.speedWindowSize)
	i := 0
	for ; i < len(p.speedWindow); i++ {
		if p.speedWindow[i].timestamp.After(cutoff) {
			break
		}
	}
	p.speedWindow = p.speedWindow[i:]

	// Calculate average speed
	var bytesDelta int64
	if len(p.speedWindow) > 1 {
		bytesDelta = p.speedWindow[len(p.speedWindow)-1].bytes - p.speedWindow[0].bytes
		seconds := p.speedWindow[len(p.speedWindow)-1].timestamp.Sub(p.speedWindow[0].timestamp).Seconds()
		speed = float64(bytesDelta) / seconds
	}

	// Build progress bar
	bar := "[" + strings.Repeat("=", filled) + strings.Repeat(" ", barWidth-filled) + "]"

	fileName, _ := p.file.Load().(string)
	fileName = filepath.Base(fileName)
	// Format output (80 columns max)
	out := fmt.Sprintf("\r%s %3.2f%% %v/s %s", bar, progress*100, humanize.Bytes(uint64(speed)), fileName)
	if len(out) > 80 {
		out = out[:80]
	}

	// Print only if changed (reduce flicker)
	if out != p.lastPrintStr {
		fmt.Print(out)
		p.lastPrintStr = out
	}
}
