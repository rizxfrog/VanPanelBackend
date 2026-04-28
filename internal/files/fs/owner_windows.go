//go:build windows

package fs

import (
	"os"

	filemodel "github.com/GoSimplicity/AI-CloudOps/internal/files/model"
)

func applyOwner(item *filemodel.FileInfo, info os.FileInfo) {
	item.User = "-"
	item.Group = "-"
}
