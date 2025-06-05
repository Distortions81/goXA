package main

import "io"

// countingReader wraps an io.Reader and increments progress current count.
type countingReader struct {
	r io.Reader
	p *progressData
}

func (cr countingReader) Read(b []byte) (int, error) {
	n, err := cr.r.Read(b)
	if cr.p != nil {
		cr.p.current.Add(int64(n))
	}
	return n, err
}
