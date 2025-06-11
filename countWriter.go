package main

import (
	"encoding/binary"
	"fmt"
	"io"
)

// WriteLPString writes a length-prefixed string.
func WriteLPString(w io.Writer, s string) error {
	b := []byte(s)
	if len(b) > 0xFFFF {
		return fmt.Errorf("string too long: %d bytes", len(b))
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(len(b))); err != nil {
		return err
	}

	for len(b) > 0 {
		n, err := w.Write(b)
		b = b[n:]
		if err != nil {
			return err
		}
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
