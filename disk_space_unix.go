//go:build !windows

package goxa

import "golang.org/x/sys/unix"

func getDiskSpace(path string) (free uint64, total uint64, err error) {
	var stat unix.Statfs_t
	if err = unix.Statfs(path, &stat); err != nil {
		return
	}
	free = stat.Bavail * uint64(stat.Bsize)
	total = stat.Blocks * uint64(stat.Bsize)
	return
}
