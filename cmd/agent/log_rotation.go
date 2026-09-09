package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LogRetentionPolicy defines retention criteria for rotating and purging agent logs.
type LogRetentionPolicy struct {
	MaxAge     time.Duration `json:"max_age"`
	MaxBytes   int64         `json:"max_bytes"`
	MaxBackups int           `json:"max_backups"`
}

// DefaultRetentionPolicy returns standard 7-day, 100MB retention configuration.
func DefaultRetentionPolicy() LogRetentionPolicy {
	return LogRetentionPolicy{
		MaxAge:     7 * 24 * time.Hour,
		MaxBytes:   100 * 1024 * 1024,
		MaxBackups: 5,
	}
}

// CleanRotatedLogs scans the configured log directory and removes files exceeding retention criteria.
func (s *LogServer) CleanRotatedLogs(ctx context.Context, policy LogRetentionPolicy) (int, error) {
	s.mu.RLock()
	dir := s.logDir
	s.mu.RUnlock()

	if dir == "" {
		return 0, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("reading log directory for cleanup: %w", err)
	}

	type fileInfo struct {
		name    string
		path    string
		size    int64
		modTime time.Time
	}

	serviceFiles := make(map[string][]fileInfo)
	now := time.Now().UTC()

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".log") && !strings.Contains(name, ".log.") {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		parts := strings.Split(name, ".")
		serviceName := parts[0]

		serviceFiles[serviceName] = append(serviceFiles[serviceName], fileInfo{
			name:    name,
			path:    filepath.Join(dir, name),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
	}

	deletedCount := 0

	for _, files := range serviceFiles {
		sort.Slice(files, func(i, j int) bool {
			return files[i].modTime.After(files[j].modTime)
		})

		for i, f := range files {
			select {
			case <-ctx.Done():
				return deletedCount, ctx.Err()
			default:
			}

			if i == 0 && f.name == filepath.Base(f.path) && !strings.Contains(f.name, ".log.") {
				continue
			}

			shouldDelete := false

			if policy.MaxBackups > 0 && i >= policy.MaxBackups {
				shouldDelete = true
			}

			if policy.MaxAge > 0 && now.Sub(f.modTime) > policy.MaxAge {
				shouldDelete = true
			}

			if shouldDelete {
				if err := os.Remove(f.path); err == nil {
					deletedCount++
				}
			}
		}
	}

	return deletedCount, nil
}
