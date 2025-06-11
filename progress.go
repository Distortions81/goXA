package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
	"golang.org/x/term"
)

const (
	// maxBarWidth limits the progress bar size so that extremely wide
	// terminals don't allocate a huge bar. The actual width used is
	// calculated dynamically based on the terminal size and other
	// displayed information.
	maxBarWidth  = 60
	updatePeriod = time.Second / 4
)

func getLineWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 80
}

type sample struct {
	timestamp time.Time
	bytes     int64
}

type progressData struct {
	current, written atomic.Int64
	total            int64
	speedWindow      []sample
	speedWindowSize  time.Duration
	startTime        time.Time
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

	p.startTime = time.Now()

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

	// Calculate average speed for the moving window
	var bytesDelta int64
	if len(p.speedWindow) > 1 {
		bytesDelta = p.speedWindow[len(p.speedWindow)-1].bytes - p.speedWindow[0].bytes
		seconds := p.speedWindow[len(p.speedWindow)-1].timestamp.Sub(p.speedWindow[0].timestamp).Seconds()
		speed = float64(bytesDelta) / seconds
	}

	// Overall average speed since start
	var avgSpeed float64
	if !p.startTime.IsZero() {
		elapsed := now.Sub(p.startTime).Seconds()
		if elapsed > 0 {
			avgSpeed = float64(p.written.Load()) / elapsed
		}
	}

	if progress >= 1 {
		speed = avgSpeed
	}

	fileName, _ := p.file.Load().(string)
	fileName = filepath.Base(fileName)

	// Build the informational part of the line and determine the bar width
	info := fmt.Sprintf(" %3.2f%% %v/s %s", progress*100, humanize.Bytes(uint64(speed)), fileName)
	width := getLineWidth()
	barWidth := width - len(info) - 2 // 2 for the surrounding []
	if barWidth > maxBarWidth {
		barWidth = maxBarWidth
	}
	if barWidth < 0 {
		barWidth = 0
	}

	filled := int(progress * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	bar := "[" + strings.Repeat("=", filled) + strings.Repeat(" ", barWidth-filled) + "]"

	out := bar + info

	// Print only if changed (reduce flicker)
	if out != p.lastPrintStr {
		// Clear the previous line before printing the new progress
		fmt.Printf("\r\033[K%s", out)
		p.lastPrintStr = out
	}
}
