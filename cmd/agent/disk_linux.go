//go:build linux

package main

import "syscall"

func getDiskUsage(mountPoint string) (totalBytes int64, usedBytes int64, err error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(mountPoint, &stat); err != nil {
		return 0, 0, err
	}
	totalBytes = int64(stat.Blocks) * int64(stat.Bsize)
	freeBytes := int64(stat.Bavail) * int64(stat.Bsize)
	usedBytes = totalBytes - freeBytes
	if usedBytes < 0 {
		usedBytes = 0
	}
	return totalBytes, usedBytes, nil
}
