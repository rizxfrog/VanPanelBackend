package fs

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	filemodel "github.com/rizxfrog/VanPanelBackend/internal/files/model"
)

var (
	ErrTooLarge = errors.New("file too large")
	ErrBinary   = errors.New("binary file unsupported")
)

type ListOptions struct {
	Page       int
	Size       int
	Search     string
	ShowHidden bool
	SortBy     string
	SortOrder  string
}

type LocalFS struct{}

func NewLocalFS() *LocalFS {
	return &LocalFS{}
}

func (l *LocalFS) Info(pathValue string) (filemodel.FileInfo, error) {
	info, err := os.Lstat(pathValue)
	if err != nil {
		return filemodel.FileInfo{}, err
	}
	return buildInfo(pathValue, info), nil
}

func (l *LocalFS) List(pathValue string, opts ListOptions) ([]filemodel.FileInfo, int, error) {
	entries, err := os.ReadDir(pathValue)
	if err != nil {
		return nil, 0, err
	}

	items := make([]filemodel.FileInfo, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !opts.ShowHidden && strings.HasPrefix(name, ".") {
			continue
		}
		if opts.Search != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(opts.Search)) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		items = append(items, buildInfo(filepath.Join(pathValue, name), info))
	}

	sort.SliceStable(items, func(i, j int) bool {
		left, right := items[i], items[j]
		desc := strings.EqualFold(opts.SortOrder, "desc") || strings.EqualFold(opts.SortOrder, "descending")
		switch opts.SortBy {
		case "size":
			if desc {
				return left.Size > right.Size
			}
			return left.Size < right.Size
		case "mod_time":
			if desc {
				return left.ModTime.After(right.ModTime)
			}
			return left.ModTime.Before(right.ModTime)
		default:
			if left.IsDir != right.IsDir {
				return left.IsDir
			}
			if desc {
				return left.Name > right.Name
			}
			return left.Name < right.Name
		}
	})

	total := len(items)
	page, size := opts.Page, opts.Size
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 50
	}
	start := (page - 1) * size
	if start >= total {
		return []filemodel.FileInfo{}, total, nil
	}
	end := start + size
	if end > total {
		end = total
	}
	return items[start:end], total, nil
}

func (l *LocalFS) ReadText(pathValue string, maxBytes int64) (string, error) {
	info, err := os.Stat(pathValue)
	if err != nil {
		return "", err
	}
	if info.Size() > maxBytes {
		return "", ErrTooLarge
	}
	data, err := os.ReadFile(pathValue)
	if err != nil {
		return "", err
	}
	if DetectBinary(data) {
		return "", ErrBinary
	}
	return string(data), nil
}

func (l *LocalFS) WriteText(pathValue string, content string) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(pathValue); err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(pathValue, []byte(content), mode)
}

func (l *LocalFS) Create(pathValue string, isDir bool) error {
	if isDir {
		return os.MkdirAll(pathValue, 0o755)
	}
	file, err := os.OpenFile(pathValue, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	return file.Close()
}

func (l *LocalFS) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func (l *LocalFS) Delete(pathValue string) error {
	return os.RemoveAll(pathValue)
}

func (l *LocalFS) Copy(src, dst string, overwrite bool) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dst); err == nil && !overwrite {
		return os.ErrExist
	}
	if info.IsDir() {
		return copyDir(src, dst, overwrite)
	}
	return copyFile(src, dst, info.Mode().Perm(), overwrite)
}

func (l *LocalFS) Chmod(pathValue string, mode uint32) error {
	return os.Chmod(pathValue, os.FileMode(mode))
}

func (l *LocalFS) Chown(pathValue string, uid int, gid int) error {
	return os.Chown(pathValue, uid, gid)
}

func copyDir(src, dst string, overwrite bool) error {
	return filepath.Walk(src, func(pathValue string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, pathValue)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyFile(pathValue, target, info.Mode().Perm(), overwrite)
	})
}

func copyFile(src, dst string, mode os.FileMode, overwrite bool) error {
	if _, err := os.Stat(dst); err == nil && !overwrite {
		return os.ErrExist
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func DetectBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	mimeType := http.DetectContentType(data)
	if strings.HasPrefix(mimeType, "text/") {
		return false
	}
	n := len(data)
	if n > 1024 {
		n = 1024
	}
	for i := 0; i < n; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

func buildInfo(pathValue string, info os.FileInfo) filemodel.FileInfo {
	mode := info.Mode()
	item := filemodel.FileInfo{
		Name:        info.Name(),
		Path:        filepath.Clean(pathValue),
		Extension:   filepath.Ext(info.Name()),
		Size:        info.Size(),
		IsDir:       info.IsDir(),
		IsSymlink:   mode&os.ModeSymlink != 0,
		IsHidden:    strings.HasPrefix(info.Name(), "."),
		Mode:        strconv.FormatUint(uint64(mode.Perm()), 8),
		ModTime:     info.ModTime(),
		MimeType:    mime.TypeByExtension(filepath.Ext(info.Name())),
		Editable:    !info.IsDir(),
		Previewable: !info.IsDir(),
	}
	applyOwner(&item, info)
	return item
}
