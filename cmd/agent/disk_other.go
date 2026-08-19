//go:build !linux

package main

func getDiskUsage(mountPoint string) (totalBytes int64, usedBytes int64, err error) {
	// Fallback disk usage for non-Linux platforms (e.g. Windows / Darwin in development)
	totalBytes = 100 * 1024 * 1024 * 1024 // 100 GiB baseline
	usedBytes = 40 * 1024 * 1024 * 1024   // 40 GiB baseline
	return totalBytes, usedBytes, nil
}
