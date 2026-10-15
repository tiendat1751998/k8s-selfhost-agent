//go:build !linux

package main

func getDiskUsage(mountPoint string) (totalBytes, usedBytes, inodesTotal, inodesFree int64, err error) {
	// Fallback disk usage for non-Linux platforms (e.g. Windows / Darwin in development)
	totalBytes = 100 * 1024 * 1024 * 1024 // 100 GiB baseline
	usedBytes = 40 * 1024 * 1024 * 1024   // 40 GiB baseline
	inodesTotal = 10000000
	inodesFree = 7000000
	return totalBytes, usedBytes, inodesTotal, inodesFree, nil
}
