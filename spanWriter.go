package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// spanWriter writes to multiple files when the size limit is reached.
// Files are created as 1.archive.goxa, 2.archive.goxa, ... and renamed
// on Close to include the total span count (e.g. 1-3.archive.goxa).
type spanWriter struct {
	base    string
	limit   int64
	files   []*os.File
	cur     int
	written int64
}

type fileInfo struct {
	name string
	size int64
	mode os.FileMode
	mod  time.Time
}

func (fi *fileInfo) Name() string       { return fi.name }
func (fi *fileInfo) Size() int64        { return fi.size }
func (fi *fileInfo) Mode() os.FileMode  { return fi.mode }
func (fi *fileInfo) ModTime() time.Time { return fi.mod }
func (fi *fileInfo) IsDir() bool        { return false }
func (fi *fileInfo) Sys() interface{}   { return nil }

func (sw *spanWriter) Name() string { return sw.base }

func (sw *spanWriter) Read(p []byte) (int, error) {
	return sw.current().Read(p)
}

func newSpanWriter(path string, limit int64) (*spanWriter, error) {
	sw := &spanWriter{base: path, limit: limit}
	if err := sw.newFile(); err != nil {
		return nil, err
	}
	return sw, nil
}

func (sw *spanWriter) newFile() error {
	dir := filepath.Dir(sw.base)
	name := filepath.Base(sw.base)
	fname := filepath.Join(dir, fmt.Sprintf("%d.%s", len(sw.files)+1, name))
	f, err := os.Create(fname)
	if err != nil {
		return err
	}
	sw.files = append(sw.files, f)
	sw.cur = len(sw.files) - 1
	sw.written = 0
	return nil
}

func (sw *spanWriter) current() *os.File { return sw.files[sw.cur] }

func (sw *spanWriter) Stat() (os.FileInfo, error) {
	var size int64
	for _, f := range sw.files {
		st, err := f.Stat()
		if err != nil {
			return nil, err
		}
		size += st.Size()
	}
	st, err := sw.current().Stat()
	if err != nil {
		return nil, err
	}
	return &fileInfo{name: sw.base, size: size, mode: st.Mode(), mod: st.ModTime()}, nil
}

func (sw *spanWriter) Write(p []byte) (int, error) {
	total := 0
	for len(p) > 0 {
		if sw.limit > 0 && sw.written >= sw.limit {
			if err := sw.newFile(); err != nil {
				return total, err
			}
		}
		space := sw.limit - sw.written
		if sw.limit == 0 || int64(len(p)) <= space {
			n, err := sw.current().Write(p)
			sw.written += int64(n)
			total += n
			return total, err
		}
		n, err := sw.current().Write(p[:space])
		sw.written += int64(n)
		total += n
		if err != nil {
			return total, err
		}
		p = p[space:]
		if err := sw.newFile(); err != nil {
			return total, err
		}
	}
	return total, nil
}

func (sw *spanWriter) Seek(offset int64, whence int) (int64, error) {
	if whence != io.SeekStart {
		return 0, fmt.Errorf("spanWriter only supports SeekStart")
	}
	if offset < 0 {
		return 0, fmt.Errorf("negative seek")
	}
	idx := int(offset / sw.limit)
	off := offset % sw.limit
	if idx >= len(sw.files) {
		return 0, fmt.Errorf("seek past written data")
	}
	for i := idx + 1; i < len(sw.files); i++ {
		sw.files[i].Close()
		os.Remove(sw.files[i].Name())
	}
	sw.files = sw.files[:idx+1]
	sw.cur = idx
	sw.written = off
	f := sw.current()
	_, err := f.Seek(off, io.SeekStart)
	return offset, err
}

func (sw *spanWriter) Sync() error {
	for _, f := range sw.files {
		if err := f.Sync(); err != nil {
			return err
		}
	}
	return nil
}

func (sw *spanWriter) Close() error {
	for _, f := range sw.files {
		if err := f.Close(); err != nil {
			return err
		}
	}
	total := len(sw.files)
	dir := filepath.Dir(sw.base)
	name := filepath.Base(sw.base)
	if total == 1 {
		return os.Rename(sw.files[0].Name(), filepath.Join(dir, name))
	}
	for i, f := range sw.files {
		final := filepath.Join(dir, fmt.Sprintf("%d-%d.%s", i+1, total, name))
		if err := os.Rename(f.Name(), final); err != nil {
			return err
		}
	}
	return nil
}
