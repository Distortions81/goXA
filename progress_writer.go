package goxa

import "io"

// progressWriter wraps an io.Writer and increments progress counters for bytes written.
type progressWriter struct {
	w io.Writer
	p *progressData
}

func (pw progressWriter) Write(b []byte) (int, error) {
	n, err := pw.w.Write(b)
	if pw.p != nil {
		pw.p.current.Add(int64(n))
		pw.p.written.Add(int64(n))
	}
	return n, err
}
