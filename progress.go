package main

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
)

const (
	barWidth     = 60
	updatePeriod = time.Second / 8
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
}

func progressTicker(p *progressData) (*progressData, chan struct{}) {
	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(updatePeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				printProgress(p)
			case <-done:
				printProgress(p)
				return
			}
		}
	}()

	return p, done
}

func printProgress(p *progressData) {
	now := time.Now()

	// Compute progress
	progress := 1.0
	if p.total > 0 {
		progress = float64(p.current.Load()) / float64(p.total)
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

	// Format output (80 columns max)
	out := fmt.Sprintf("\r%s %3.2f%% %v/s", bar, progress*100, humanize.Bytes(uint64(speed)))
	if len(out) > 80 {
		out = out[:80]
	}

	// Print only if changed (reduce flicker)
	if out != p.lastPrintStr {
		fmt.Print(out)
		p.lastPrintStr = out
	}
}
