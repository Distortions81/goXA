package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

type shortWriter struct {
	w   io.Writer
	max int
}

func (sw *shortWriter) Write(p []byte) (int, error) {
	if len(p) > sw.max {
		p = p[:sw.max]
	}
	return sw.w.Write(p)
}

func TestWriteStringNormal(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteString(&buf, "hello"); err != nil {
		t.Fatalf("WriteString returned error: %v", err)
	}
	want := make([]byte, 2)
	binary.LittleEndian.PutUint16(want, uint16(len("hello")))
	want = append(want, []byte("hello")...)
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("unexpected output: %v", buf.Bytes())
	}
}

func TestWriteStringShortWrite(t *testing.T) {
	var underlying bytes.Buffer
	sw := &shortWriter{w: &underlying, max: 2}
	if err := WriteString(sw, "hello"); err != nil {
		t.Fatalf("WriteString returned error: %v", err)
	}
	want := make([]byte, 2)
	binary.LittleEndian.PutUint16(want, uint16(len("hello")))
	want = append(want, []byte("hello")...)
	if !bytes.Equal(underlying.Bytes(), want) {
		t.Errorf("unexpected output with short writer: %v", underlying.Bytes())
	}
}
