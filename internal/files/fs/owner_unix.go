//go:build !windows

package fs

import (
	"os"
	"os/user"
	"strconv"
	"syscall"

	filemodel "github.com/GoSimplicity/AI-CloudOps/internal/files/model"
)

func applyOwner(item *filemodel.FileInfo, info os.FileInfo) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}
	item.User = strconv.FormatUint(uint64(stat.Uid), 10)
	item.Group = strconv.FormatUint(uint64(stat.Gid), 10)
	if u, err := user.LookupId(item.User); err == nil {
		item.User = u.Username
	}
	if g, err := user.LookupGroupId(item.Group); err == nil {
		item.Group = g.Name
	}
}
