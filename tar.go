package main

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	gzip "github.com/klauspost/pgzip"

	"github.com/ulikunitz/xz"
)

// createTar creates a tar archive from the provided paths.
// When fNoCompress is not set, the archive is gzip compressed unless
// tarUseXz is true, in which case xz compression is used.
func createTar(paths []string) error {
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	var w io.WriteCloser = f
	if features.IsNotSet(fNoCompress) {
		if tarUseXz {
			xzw, err := xz.NewWriter(f)
			if err != nil {
				f.Close()
				return err
			}
			w = xzw
			defer f.Close()
			defer xzw.Close()
		} else {
			gw := gzip.NewWriter(f)
			if threads < 1 {
				threads = 1
			}
			_ = gw.SetConcurrency(1<<20, threads)
			w = gw
			defer f.Close()
			defer gw.Close()
		}
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

			name := info.Name()
			if features.IsNotSet(fIncludeInvis) && strings.HasPrefix(name, ".") {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 && !info.IsDir() && features.IsNotSet(fSpecialFiles) {
				return nil
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
			if features.IsNotSet(fPermissions) {
				header.Mode = 0
			}
			if features.IsNotSet(fModDates) {
				header.ModTime = time.Time{}
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
// gzip compression is assumed unless tarUseXz is true, in which case xz is used.
func extractTar(destination string) error {
	r, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	var src io.Reader = r
	if features.IsNotSet(fNoCompress) {
		if tarUseXz {
			xr, err := xz.NewReader(r)
			if err != nil {
				r.Close()
				return err
			}
			src = xr
		} else {
			if threads < 1 {
				threads = 1
			}
			gr, err := gzip.NewReaderN(r, 0, threads)
			if err != nil {
				r.Close()
				return err
			}
			src = gr
			defer gr.Close()
		}
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
		var target string
		if features.IsSet(fAbsolutePaths) {
			target = filepath.Clean(hdr.Name)
		} else {
			target, err = safeJoin(destination, hdr.Name)
			if err != nil {
				return err
			}
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			perm := os.FileMode(0755)
			if features.IsSet(fPermissions) {
				perm = os.FileMode(hdr.Mode)
			}
			if err := os.MkdirAll(target, perm); err != nil {
				return err
			}
			if features.IsSet(fModDates) {
				os.Chtimes(target, hdr.ModTime, hdr.ModTime)
			}
		case tar.TypeSymlink:
			if features.IsNotSet(fSpecialFiles) {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		case tar.TypeLink:
			if features.IsNotSet(fSpecialFiles) {
				continue
			}
			if err := os.Link(hdr.Linkname, target); err != nil {
				return err
			}
		default:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			perm := os.FileMode(0644)
			if features.IsSet(fPermissions) {
				perm = os.FileMode(hdr.Mode)
			}
			w, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, tr); err != nil {
				w.Close()
				return err
			}
			w.Close()
			if features.IsSet(fModDates) {
				os.Chtimes(target, hdr.ModTime, hdr.ModTime)
			}
		}
	}
	return nil
}
