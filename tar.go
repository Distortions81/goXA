package main

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
)

// createTar creates a tar archive from the provided paths.
// When fNoCompress is not set, the archive is gzip compressed.
func createTar(paths []string) error {
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	var w io.WriteCloser = f
	if features.IsNotSet(fNoCompress) {
		gw := gzip.NewWriter(f)
		w = gw
		defer f.Close()
		defer gw.Close()
	} else {
		defer f.Close()
	}

	tw := tar.NewWriter(w)
	defer tw.Close()

	for _, root := range paths {
		root = filepath.Clean(root)
		err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			header.Name = storedPath(root, p)
			if info.Mode()&os.ModeSymlink != 0 {
				if link, lerr := os.Readlink(p); lerr == nil {
					header.Linkname = link
				}
			}
			if info.IsDir() && header.Name != "." {
				header.Name += "/"
			}
			if err := tw.WriteHeader(header); err != nil {
				return err
			}
			if info.Mode().IsRegular() {
				file, err := os.Open(p)
				if err != nil {
					return err
				}
				if _, err := io.Copy(tw, file); err != nil {
					file.Close()
					return err
				}
				file.Close()
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// extractTar extracts a tar archive into destination. When fNoCompress is not set,
// the archive is assumed to be gzip compressed.
func extractTar(destination string) error {
	r, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	var src io.Reader = r
	if features.IsNotSet(fNoCompress) {
		gr, err := gzip.NewReader(r)
		if err != nil {
			r.Close()
			return err
		}
		src = gr
		defer gr.Close()
	}
	defer r.Close()

	tr := tar.NewReader(src)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target, err := safeJoin(destination, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
			os.Chtimes(target, hdr.ModTime, hdr.ModTime)
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		case tar.TypeLink:
			if err := os.Link(hdr.Linkname, target); err != nil {
				return err
			}
		default:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			w, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, tr); err != nil {
				w.Close()
				return err
			}
			w.Close()
			os.Chtimes(target, hdr.ModTime, hdr.ModTime)
		}
	}
	return nil
}
