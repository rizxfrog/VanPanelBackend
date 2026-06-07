//go:build windows

package fs

import (
	"os"

	filemodel "github.com/rizxfrog/VanPanelBackend/internal/files/model"
)

func applyOwner(item *filemodel.FileInfo, info os.FileInfo) {
	item.User = "-"
	item.Group = "-"
}
