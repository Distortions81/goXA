//go:build windows

package goxa

import "golang.org/x/sys/windows"

func getDiskSpace(path string) (free uint64, total uint64, err error) {
	var avail, tot, freeAll uint64
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	err = windows.GetDiskFreeSpaceEx(p, &avail, &tot, &freeAll)
	return avail, tot, err
}
