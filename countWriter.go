package main

import (
	"encoding/binary"
	"io"
)

func WriteString(w io.Writer, s string) error {
	if err := binary.Write(w, binary.LittleEndian, uint16(len(s))); err != nil {
		return err
	}

	data := []byte(s)
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

// a tiny io.Writer that counts bytes passed through
type countingWriter struct {
	w io.Writer
	n int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	m, err := cw.w.Write(p)
	cw.n += int64(m)
	return m, err
}
func (cw *countingWriter) Count() int64 { return cw.n }
