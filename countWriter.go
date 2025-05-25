package main

import (
	"encoding/binary"
	"io"
)

func WriteString(w io.Writer, s string) {
	binary.Write(w, binary.LittleEndian, uint16(len(s)))
	w.Write([]byte(s))
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
