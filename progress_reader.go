package main

import "io"

// progressReader wraps an io.Reader and increments progress counters.
type progressReader struct {
	r io.Reader
	p *progressData
}

func (pr progressReader) Read(b []byte) (int, error) {
	n, err := pr.r.Read(b)
	if pr.p != nil {
		pr.p.current.Add(int64(n))
	}
	return n, err
}
