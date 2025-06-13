package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type spanReader struct {
	files []*os.File
	sizes []int64
	size  int64
	pos   int64
}

func (sr *spanReader) Name() string {
	if len(sr.files) > 0 {
		return sr.files[0].Name()
	}
	return ""
}

func (sr *spanReader) Sync() error {
	for _, f := range sr.files {
		if err := f.Sync(); err != nil {
			return err
		}
	}
	return nil
}

func (sr *spanReader) Stat() (os.FileInfo, error) {
	if len(sr.files) > 0 {
		return sr.files[0].Stat()
	}
	return nil, fmt.Errorf("no span files")
}

func findSpanFiles(base string) ([]string, error) {
	if _, err := os.Stat(base); err == nil {
		return []string{base}, nil
	}
	dir := filepath.Dir(base)
	name := filepath.Base(base)
	// try numbered with total
	matches, _ := filepath.Glob(filepath.Join(dir, "1-*"+name))
	if len(matches) > 0 {
		// parse total from first match
		b := filepath.Base(matches[0])
		parts := strings.SplitN(b, "-", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid span name")
		}
		after := parts[1]
		tstr := strings.SplitN(after, ".", 2)[0]
		total, err := strconv.Atoi(tstr)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, total)
		for i := 1; i <= total; i++ {
			p := filepath.Join(dir, fmt.Sprintf("%d-%d.%s", i, total, name))
			if _, err := os.Stat(p); err != nil {
				return nil, err
			}
			out = append(out, p)
		}
		return out, nil
	}
	// try simple numbered
	if _, err := os.Stat(filepath.Join(dir, "1."+name)); err == nil {
		out := []string{}
		for i := 1; ; i++ {
			p := filepath.Join(dir, fmt.Sprintf("%d.%s", i, name))
			if _, err := os.Stat(p); err != nil {
				break
			}
			out = append(out, p)
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	return nil, os.ErrNotExist
}

func newSpanReader(path string) (*spanReader, error) {
	paths, err := findSpanFiles(path)
	if err != nil {
		return nil, err
	}
	sr := &spanReader{}
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			sr.Close()
			return nil, err
		}
		st, err := f.Stat()
		if err != nil {
			f.Close()
			sr.Close()
			return nil, err
		}
		sr.files = append(sr.files, f)
		sr.sizes = append(sr.sizes, st.Size())
		sr.size += st.Size()
	}
	return sr, nil
}

func (sr *spanReader) Close() error {
	var firstErr error
	for _, f := range sr.files {
		if err := f.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (sr *spanReader) seekIndex(pos int64) (int, int64) {
	off := pos
	for i, s := range sr.sizes {
		if off < s {
			return i, off
		}
		off -= s
	}
	return len(sr.files) - 1, sr.sizes[len(sr.files)-1]
}

func (sr *spanReader) Read(p []byte) (int, error) {
	if sr.pos >= sr.size {
		return 0, io.EOF
	}
	idx, off := sr.seekIndex(sr.pos)
	total := 0
	for len(p) > 0 && idx < len(sr.files) {
		f := sr.files[idx]
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			return total, err
		}
		n, err := f.Read(p)
		sr.pos += int64(n)
		total += n
		if err != nil {
			if err == io.EOF {
				idx++
				off = 0
				err = nil
				if len(p) == 0 {
					return total, nil
				}
				if idx >= len(sr.files) {
					return total, io.EOF
				}
				p = p[n:]
				continue
			}
			return total, err
		}
		if len(p) == n {
			return total, nil
		}
		p = p[n:]
		off = 0
		idx++
	}
	return total, nil
}

func (sr *spanReader) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = sr.pos + offset
	case io.SeekEnd:
		abs = sr.size + offset
	}
	if abs < 0 || abs > sr.size {
		return sr.pos, fmt.Errorf("invalid seek")
	}
	sr.pos = abs
	return abs, nil
}

func (sr *spanReader) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("spanReader is read-only")
}
