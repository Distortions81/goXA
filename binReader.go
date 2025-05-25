package main

import (
	"bufio"
	"encoding/binary"
	"io"
	"os"
)

// Read string length, then the string
func ReadString(w io.Reader) string {
	var stringLength uint16
	binary.Read(w, binary.LittleEndian, &stringLength)
	stringData := make([]byte, stringLength)
	binary.Read(w, binary.LittleEndian, &stringData)

	return string(stringData)
}

type BinReader struct {
	file   *os.File
	reader *bufio.Reader
}

func NewBinReader(path string) (*BinReader, error) {
	f, err := os.Open(path)
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
