package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"unicode/utf8"
)

// ReadLPString reads a length-prefixed string.
func ReadLPString(r io.Reader) (string, error) {
	var stringLength uint16
	if err := binary.Read(r, binary.LittleEndian, &stringLength); err != nil {
		return "", err
	}
	stringData := make([]byte, stringLength)
	if _, err := io.ReadFull(r, stringData); err != nil {
		return "", err
	}
	if !utf8.Valid(stringData) {
		return "", fmt.Errorf("invalid UTF-8 string")
	}
	return string(stringData), nil
}

type BinReader struct {
	file   fileLike
	reader *bufio.Reader
}

func NewBinReader(path string) (*BinReader, error) {
	f, err := newSpanReader(path)
	if err != nil {
		return nil, err
	}
	return &BinReader{
		file:   f,
		reader: bufio.NewReaderSize(f, readBuffer),
	}, nil
}

func (br *BinReader) Read(p []byte) (int, error) {
	return br.reader.Read(p)
}

func (br *BinReader) Close() error {
	return br.file.Close()
}

func (br *BinReader) Seek(offset int64, whence int) (int64, error) {
	// Seek the underlying file
	pos, err := br.file.Seek(offset, whence)
	if err != nil {
		return 0, err
	}

	// Reset the buffered reader to discard its buffer
	br.reader.Reset(br.file)

	return pos, nil
}
