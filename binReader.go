package main

import (
        "bufio"
        "encoding/binary"
        "fmt"
        "io"
        "os"
        "unicode/utf8"
)

// Read string length, then the string
func ReadString(w io.Reader) (string, error) {
        var stringLength uint16
        if err := binary.Read(w, binary.LittleEndian, &stringLength); err != nil {
                return "", err
        }
        stringData := make([]byte, stringLength)
        if _, err := io.ReadFull(w, stringData); err != nil {
                return "", err
        }
        if !utf8.Valid(stringData) {
                return "", fmt.Errorf("invalid UTF-8 string")
        }
        return string(stringData), nil
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
