//go:build linux

package builtin

import (
	"fmt"
	"syscall"
)

func defaultDiskPath() string { return "/" }

func platformDiskUsage(path string) (map[string]any, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil, fmt.Errorf("statfs %s: %w", path, err)
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	available := stat.Bavail * uint64(stat.Bsize)
	used := total - free

	var percent float64
	if total > 0 {
		percent = float64(used) / float64(total) * 100
	}

	return map[string]any{
		"path":           path,
		"totalBytes":     total,
		"usedBytes":      used,
		"freeBytes":      free,
		"availableBytes": available,
		"usedPercent":    fmt.Sprintf("%.1f%%", percent),
	}, nil
}
