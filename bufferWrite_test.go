package main

import (
	"bufio"
	"errors"
	"os"
	"testing"
)

// errWriter always returns an error on Write
type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write error")
}

func TestBufferedFileClosePropagatesError(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "bf")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	bf := &BufferedFile{
		file:   f,
		writer: bufio.NewWriterSize(errWriter{}, 32),
	}
	bf.writer.WriteByte('a')
	if err := bf.Close(); err == nil {
		t.Fatal("expected close error, got nil")
	}
}
