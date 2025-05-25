package main

import (
	"bufio"
	"os"
)

type BufferedFile struct {
	doCount  bool
	file     *os.File
	writer   *bufio.Writer
	reader   *bufio.Reader
	progress *progressData
}

func NewBufferedFile(file *os.File, bufSize int, p *progressData) *BufferedFile {
	return &BufferedFile{
		file:     file,
		writer:   bufio.NewWriterSize(file, bufSize),
		reader:   bufio.NewReaderSize(file, bufSize),
		progress: p,
	}
}

func (bf *BufferedFile) Write(p []byte) (int, error) {
	n, err := bf.writer.Write(p)
	if bf.doCount {
		bf.progress.written.Add(int64(n))
	}
	return n, err
}

// Read method
func (bf *BufferedFile) Read(p []byte) (int, error) {
	n, err := bf.reader.Read(p)
	bf.progress.current.Add(int64(n))
	return n, err
}

func (bf *BufferedFile) WriteString(s string) (int, error) {
	return bf.writer.WriteString(s)
}

func (bf *BufferedFile) Flush() error {
	return bf.writer.Flush()
}

func (bf *BufferedFile) Sync() error {
	if err := bf.Flush(); err != nil {
		return err
	}
	return bf.file.Sync()
}

func (bf *BufferedFile) Seek(offset int64, whence int) (int64, error) {
	if err := bf.Flush(); err != nil {
		return 0, err
	}
	off, err := bf.file.Seek(offset, whence)
	if err != nil {
		return off, err
	}
	bf.writer.Reset(bf.file)
	return off, nil
}

func (bf *BufferedFile) Close() error {
	if err := bf.Flush(); err != nil {
		bf.file.Close()
		return err
	}
	return bf.file.Close()
}
