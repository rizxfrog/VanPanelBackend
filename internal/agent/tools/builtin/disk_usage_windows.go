//go:build windows

package builtin

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceExW = kernel32.NewProc("GetDiskFreeSpaceExW")
)

func defaultDiskPath() string { return `C:\` }

func platformDiskUsage(path string) (map[string]any, error) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes int64
	ret, _, err := getDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("GetDiskFreeSpaceExW: %w", err)
	}

	used := totalBytes - totalFreeBytes
	var percent float64
	if totalBytes > 0 {
		percent = float64(used) / float64(totalBytes) * 100
	}

	return map[string]any{
		"path":           path,
		"totalBytes":     totalBytes,
		"usedBytes":      used,
		"freeBytes":      totalFreeBytes,
		"availableBytes": freeBytesAvailable,
		"usedPercent":    fmt.Sprintf("%.1f%%", percent),
	}, nil
}
