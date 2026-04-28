# File Manager Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a local-host operations file manager for VanPanelBackend and VanPanelWebUI, with API fields reserved for future service-tree remote host support.

**Architecture:** Add a new backend `internal/files` module with DTOs, typed errors, configuration, path-policy enforcement, local filesystem adapter, service, and Gin handler. Add a WebUI file workbench route using Ant Design Vue, `requestClient`, a left safe-root/tree panel, central file table, and right drawer for preview/edit/properties.

**Tech Stack:** Go 1.24, Gin, Viper/Wire, standard library filesystem/archive packages, Vue 3 Composition API, TypeScript, Ant Design Vue, Vite.

---

## File Structure

Backend files:

- Create `internal/files/model/types.go`: request/response DTOs, constants, and enums.
- Create `internal/files/service/errors.go`: typed business errors and `IsFileError` helpers.
- Create `internal/files/service/config.go`: file-manager config normalization from `di.GlobalConfig`.
- Create `internal/files/service/path_policy.go`: absolute path resolution and safe-root checks.
- Create `internal/files/service/path_policy_test.go`: path policy unit tests.
- Create `internal/files/fs/local.go`: local filesystem adapter and metadata builders.
- Create `internal/files/fs/archive.go`: zip and tar.gz compression/decompression helpers.
- Create `internal/files/service/file_service.go`: business operations.
- Create `internal/files/service/file_service_test.go`: service tests with temporary directories.
- Create `internal/files/api/handler.go`: Gin route handlers.
- Modify `pkg/di/config.go`: add `FileManagerConfig`.
- Modify `config/config.development.yaml`, `config/config.test.yaml`, `config/config.production.yaml`, and `env.example`: add default `file_manager` config.
- Modify `pkg/di/wire.go`, `pkg/di/web.go`, and regenerate `pkg/di/wire_gen.go`: register service and handler.

Frontend files:

- Create `../VanPanelWebUI/apps/web-antd/src/api/core/files/files.ts`: API types and request functions.
- Create `../VanPanelWebUI/apps/web-antd/src/router/routes/modules/files.ts`: menu route.
- Create `../VanPanelWebUI/apps/web-antd/src/views/files/FileManager.vue`: workbench page.
- Create `../VanPanelWebUI/apps/web-antd/src/views/files/file-manager.css`: page styles.
- Create `../VanPanelWebUI/apps/web-antd/src/views/files/file-manager-utils.ts`: formatting and UI guard helpers.
- Create `../VanPanelWebUI/apps/web-antd/src/views/files/file-manager-utils.test.ts`: frontend utility tests.

## Task 1: Backend DTOs, Errors, And Configuration

**Files:**
- Create: `internal/files/model/types.go`
- Create: `internal/files/service/errors.go`
- Create: `internal/files/service/config.go`
- Modify: `pkg/di/config.go`
- Modify: `config/config.development.yaml`
- Modify: `config/config.test.yaml`
- Modify: `config/config.production.yaml`
- Modify: `env.example`
- Test: `go test ./internal/files/...`

- [ ] **Step 1: Create file-manager DTOs**

Create `internal/files/model/types.go`:

```go
package model

import "time"

const (
	TargetTypeLocal = "local"
	ArchiveZip      = "zip"
	ArchiveTarGz    = "tar.gz"
)

type RootInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type RootsResponse struct {
	Enabled       bool       `json:"enabled"`
	AllowFullDisk bool       `json:"allow_full_disk"`
	OS            string     `json:"os"`
	PathSeparator string     `json:"path_separator"`
	Roots         []RootInfo `json:"roots"`
}

type TargetRequest struct {
	TargetType string `json:"target_type" form:"target_type"`
	NodeID     int    `json:"node_id" form:"node_id"`
}

type ListRequest struct {
	TargetRequest
	Path       string `json:"path" form:"path" binding:"required"`
	Page       int    `json:"page" form:"page"`
	Size       int    `json:"size" form:"size"`
	Search     string `json:"search" form:"search"`
	ShowHidden bool   `json:"show_hidden" form:"show_hidden"`
	SortBy     string `json:"sort_by" form:"sort_by"`
	SortOrder  string `json:"sort_order" form:"sort_order"`
}

type TreeRequest struct {
	TargetRequest
	Path  string `json:"path" binding:"required"`
	Depth int    `json:"depth"`
}

type ContentRequest struct {
	TargetRequest
	Path string `json:"path" binding:"required"`
}

type SaveRequest struct {
	TargetRequest
	Path    string `json:"path" binding:"required"`
	Content string `json:"content"`
}

type CreateRequest struct {
	TargetRequest
	Path  string `json:"path" binding:"required"`
	IsDir bool   `json:"is_dir"`
}

type RenameRequest struct {
	TargetRequest
	Path    string `json:"path" binding:"required"`
	NewName string `json:"new_name" binding:"required"`
}

type DeleteRequest struct {
	TargetRequest
	Path string `json:"path" binding:"required"`
}

type MoveRequest struct {
	TargetRequest
	Paths      []string `json:"paths" binding:"required"`
	TargetPath string   `json:"target_path" binding:"required"`
	Operation  string   `json:"operation" binding:"required"` // copy or move
	Overwrite  bool     `json:"overwrite"`
}

type ChmodRequest struct {
	TargetRequest
	Path string `json:"path" binding:"required"`
	Mode uint32 `json:"mode" binding:"required"`
}

type ChownRequest struct {
	TargetRequest
	Path  string `json:"path" binding:"required"`
	User  string `json:"user" binding:"required"`
	Group string `json:"group" binding:"required"`
}

type CompressRequest struct {
	TargetRequest
	Paths      []string `json:"paths" binding:"required"`
	TargetPath string   `json:"target_path" binding:"required"`
	Name       string   `json:"name" binding:"required"`
	Type       string   `json:"type" binding:"required"`
	Overwrite  bool     `json:"overwrite"`
}

type DecompressRequest struct {
	TargetRequest
	Path       string `json:"path" binding:"required"`
	TargetPath string `json:"target_path" binding:"required"`
	Type       string `json:"type" binding:"required"`
	Overwrite  bool   `json:"overwrite"`
}

type FileInfo struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Extension  string    `json:"extension"`
	Size       int64     `json:"size"`
	IsDir      bool      `json:"is_dir"`
	IsSymlink  bool      `json:"is_symlink"`
	IsHidden   bool      `json:"is_hidden"`
	Mode       string    `json:"mode"`
	User       string    `json:"user"`
	Group      string    `json:"group"`
	ModTime    time.Time `json:"mod_time"`
	MimeType   string    `json:"mime_type"`
	Editable   bool      `json:"editable"`
	Previewable bool     `json:"previewable"`
	Content    string    `json:"content,omitempty"`
}

type ListResponse struct {
	Path  string     `json:"path"`
	Items []FileInfo `json:"items"`
	Total int        `json:"total"`
}

type TreeNode struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	IsDir    bool       `json:"is_dir"`
	Children []TreeNode `json:"children,omitempty"`
}
```

- [ ] **Step 2: Add typed file errors**

Create `internal/files/service/errors.go`:

```go
package service

import "errors"

var (
	ErrFilePathDenied           = errors.New("file path denied")
	ErrFileNotFound             = errors.New("file not found")
	ErrFileAlreadyExists        = errors.New("file already exists")
	ErrFileTooLarge             = errors.New("file too large")
	ErrFileBinaryUnsupported    = errors.New("binary file unsupported")
	ErrFileOperationUnsupported = errors.New("file operation unsupported")
	ErrFileInvalidName          = errors.New("invalid file name")
	ErrFileArchiveUnsupported   = errors.New("archive type unsupported")
)
```

- [ ] **Step 3: Extend backend config types**

Modify `pkg/di/config.go`:

```go
type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Log          LogConfig          `mapstructure:"log"`
	JWT          JWTConfig          `mapstructure:"jwt"`
	Redis        RedisConfig        `mapstructure:"redis"`
	Database     DatabaseConfig     `mapstructure:"database"`
	MySQL        MySQLConfig        `mapstructure:"mysql"`
	Tree         TreeConfig         `mapstructure:"tree"`
	K8s          K8sConfig          `mapstructure:"k8s"`
	Prometheus   PrometheusConfig   `mapstructure:"prometheus"`
	Mock         MockConfig         `mapstructure:"mock"`
	Notification NotificationConfig `mapstructure:"notification"`
	Webhook      WebhookConfig      `mapstructure:"webhook"`
	FileManager  FileManagerConfig  `mapstructure:"file_manager"`
}

type FileManagerRootConfig struct {
	Name string `mapstructure:"name" .env:"FILE_MANAGER_ROOT_NAME" default:"VanPanel"`
	Path string `mapstructure:"path" .env:"FILE_MANAGER_ROOT_PATH" default:"."`
}

type FileManagerConfig struct {
	Enabled             bool                    `mapstructure:"enabled" .env:"FILE_MANAGER_ENABLED" default:"true"`
	AllowFullDisk       bool                    `mapstructure:"allow_full_disk" .env:"FILE_MANAGER_ALLOW_FULL_DISK" default:"false"`
	Roots               []FileManagerRootConfig `mapstructure:"roots"`
	MaxEditSizeMB       int                     `mapstructure:"max_edit_size_mb" .env:"FILE_MANAGER_MAX_EDIT_SIZE_MB" default:"5"`
	MaxPreviewSizeMB    int                     `mapstructure:"max_preview_size_mb" .env:"FILE_MANAGER_MAX_PREVIEW_SIZE_MB" default:"10"`
	AllowedArchiveTypes []string                `mapstructure:"allowed_archive_types"`
}
```

- [ ] **Step 4: Add file-manager YAML config**

Append this block to `config/config.development.yaml`, `config/config.test.yaml`, and `config/config.production.yaml`:

```yaml
file_manager:
  enabled: true
  allow_full_disk: false
  roots:
    - name: VanPanel
      path: .
    - name: Logs
      path: ./logs
    - name: Deploy
      path: ./deploy
  max_edit_size_mb: 5
  max_preview_size_mb: 10
  allowed_archive_types:
    - zip
    - tar.gz
```

Append these variables to `env.example`:

```dotenv
FILE_MANAGER_ENABLED=true
FILE_MANAGER_ALLOW_FULL_DISK=false
FILE_MANAGER_MAX_EDIT_SIZE_MB=5
FILE_MANAGER_MAX_PREVIEW_SIZE_MB=10
```

- [ ] **Step 5: Add normalized service config**

Create `internal/files/service/config.go`:

```go
package service

import (
	"os"
	"path/filepath"

	"github.com/GoSimplicity/AI-CloudOps/internal/files/model"
	"github.com/GoSimplicity/AI-CloudOps/pkg/di"
)

type ManagerConfig struct {
	Enabled             bool
	AllowFullDisk       bool
	Roots               []model.RootInfo
	MaxEditBytes        int64
	MaxPreviewBytes     int64
	AllowedArchiveTypes map[string]struct{}
}

func NewManagerConfigFromGlobal() ManagerConfig {
	cfg := di.GlobalConfig.FileManager
	roots := make([]model.RootInfo, 0, len(cfg.Roots))
	for _, root := range cfg.Roots {
		abs, err := filepath.Abs(root.Path)
		if err != nil {
			continue
		}
		roots = append(roots, model.RootInfo{Name: root.Name, Path: filepath.Clean(abs)})
	}
	if len(roots) == 0 {
		wd, _ := os.Getwd()
		roots = append(roots, model.RootInfo{Name: "VanPanel", Path: filepath.Clean(wd)})
	}
	allowed := map[string]struct{}{"zip": {}, "tar.gz": {}}
	if len(cfg.AllowedArchiveTypes) > 0 {
		allowed = map[string]struct{}{}
		for _, item := range cfg.AllowedArchiveTypes {
			allowed[item] = struct{}{}
		}
	}
	maxEditMB := cfg.MaxEditSizeMB
	if maxEditMB <= 0 {
		maxEditMB = 5
	}
	maxPreviewMB := cfg.MaxPreviewSizeMB
	if maxPreviewMB <= 0 {
		maxPreviewMB = 10
	}
	return ManagerConfig{
		Enabled:             cfg.Enabled,
		AllowFullDisk:       cfg.AllowFullDisk,
		Roots:               roots,
		MaxEditBytes:        int64(maxEditMB) * 1024 * 1024,
		MaxPreviewBytes:     int64(maxPreviewMB) * 1024 * 1024,
		AllowedArchiveTypes: allowed,
	}
}
```

- [ ] **Step 6: Run tests to verify package compiles**

Run:

```powershell
go test ./internal/files/...
```

Expected: package compiles or reports no test files.

- [ ] **Step 7: Commit**

```powershell
git add internal/files pkg/di/config.go config/config.development.yaml config/config.test.yaml config/config.production.yaml env.example
git commit -m "feat(files): add file manager types and config"
```

## Task 2: Backend Path Policy

**Files:**
- Create: `internal/files/service/path_policy.go`
- Create: `internal/files/service/path_policy_test.go`

- [ ] **Step 1: Write path-policy tests**

Create `internal/files/service/path_policy_test.go`:

```go
package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoSimplicity/AI-CloudOps/internal/files/model"
)

func TestPathPolicyAllowsRootChild(t *testing.T) {
	root := t.TempDir()
	policy := NewPathPolicy(ManagerConfig{Roots: []model.RootInfo{{Name: "tmp", Path: root}}})
	got, err := policy.Resolve(root)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != filepath.Clean(root) {
		t.Fatalf("Resolve() = %q, want %q", got, filepath.Clean(root))
	}
}

func TestPathPolicyRejectsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Dir(root)
	policy := NewPathPolicy(ManagerConfig{Roots: []model.RootInfo{{Name: "tmp", Path: root}}})
	if _, err := policy.Resolve(outside); err != ErrFilePathDenied {
		t.Fatalf("Resolve() error = %v, want ErrFilePathDenied", err)
	}
}

func TestPathPolicyRejectsEmptyPath(t *testing.T) {
	root := t.TempDir()
	policy := NewPathPolicy(ManagerConfig{Roots: []model.RootInfo{{Name: "tmp", Path: root}}})
	if _, err := policy.Resolve(""); err != ErrFilePathDenied {
		t.Fatalf("Resolve() error = %v, want ErrFilePathDenied", err)
	}
}

func TestPathPolicyRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	policy := NewPathPolicy(ManagerConfig{Roots: []model.RootInfo{{Name: "tmp", Path: root}}})
	if _, err := policy.Resolve(link); err != ErrFilePathDenied {
		t.Fatalf("Resolve() error = %v, want ErrFilePathDenied", err)
	}
}

func TestPathPolicyAllowsFullDisk(t *testing.T) {
	root := t.TempDir()
	policy := NewPathPolicy(ManagerConfig{AllowFullDisk: true})
	if _, err := policy.Resolve(root); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}
```

- [ ] **Step 2: Run failing tests**

Run:

```powershell
go test ./internal/files/service -run TestPathPolicy -v
```

Expected: FAIL because `NewPathPolicy` is undefined.

- [ ] **Step 3: Implement path policy**

Create `internal/files/service/path_policy.go`:

```go
package service

import (
	"os"
	"path/filepath"
	"strings"
)

type PathPolicy struct {
	config ManagerConfig
}

func NewPathPolicy(config ManagerConfig) *PathPolicy {
	return &PathPolicy{config: config}
}

func (p *PathPolicy) Resolve(input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", ErrFilePathDenied
	}
	clean := filepath.Clean(input)
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", err
	}
	resolved := abs
	if eval, evalErr := filepath.EvalSymlinks(abs); evalErr == nil {
		resolved = eval
	} else if !os.IsNotExist(evalErr) {
		return "", evalErr
	}
	resolved = filepath.Clean(resolved)
	if p.config.AllowFullDisk {
		return resolved, nil
	}
	for _, root := range p.config.Roots {
		rootPath := filepath.Clean(root.Path)
		if isWithinRoot(resolved, rootPath) {
			return resolved, nil
		}
	}
	return "", ErrFilePathDenied
}

func (p *PathPolicy) ResolveParentForCreate(input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", ErrFilePathDenied
	}
	parent := filepath.Dir(filepath.Clean(input))
	if _, err := p.Resolve(parent); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(filepath.Clean(input))
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func isWithinRoot(pathValue, root string) bool {
	if pathValue == root {
		return true
	}
	rel, err := filepath.Rel(root, pathValue)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
```

- [ ] **Step 4: Run path-policy tests**

Run:

```powershell
go test ./internal/files/service -run TestPathPolicy -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/files/service/path_policy.go internal/files/service/path_policy_test.go
git commit -m "feat(files): enforce safe root path policy"
```

## Task 3: Local Filesystem Adapter

**Files:**
- Create: `internal/files/fs/local.go`
- Create: `internal/files/fs/archive.go`
- Create: `internal/files/fs/local_test.go`

- [ ] **Step 1: Write local adapter tests**

Create `internal/files/fs/local_test.go`:

```go
package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalFSListAndReadText(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "hello.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	local := NewLocalFS()
	items, total, err := local.List(root, ListOptions{Page: 1, Size: 10, SortBy: "name", SortOrder: "asc"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].Name != "hello.txt" {
		t.Fatalf("List() = total %d items %#v", total, items)
	}
	content, err := local.ReadText(filePath, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if content != "hello" {
		t.Fatalf("ReadText() = %q", content)
	}
}

func TestLocalFSRejectsBinaryRead(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "bin.dat")
	if err := os.WriteFile(filePath, []byte{0, 1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}
	local := NewLocalFS()
	if _, err := local.ReadText(filePath, 1024); err == nil {
		t.Fatal("ReadText() expected binary error")
	}
}
```

- [ ] **Step 2: Run failing tests**

Run:

```powershell
go test ./internal/files/fs -run TestLocalFS -v
```

Expected: FAIL because `NewLocalFS` is undefined.

- [ ] **Step 3: Implement local adapter**

Create `internal/files/fs/local.go` with these exported members:

```go
package fs

import (
	"io"
	"mime"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	filemodel "github.com/GoSimplicity/AI-CloudOps/internal/files/model"
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
	f, err := os.OpenFile(pathValue, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
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

var ErrTooLarge = os.ErrInvalid
var ErrBinary = os.ErrInvalid

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
		Name:       info.Name(),
		Path:       filepath.Clean(pathValue),
		Extension:  filepath.Ext(info.Name()),
		Size:       info.Size(),
		IsDir:      info.IsDir(),
		IsSymlink:  mode&os.ModeSymlink != 0,
		IsHidden:   strings.HasPrefix(info.Name(), "."),
		Mode:       strconv.FormatUint(uint64(mode.Perm()), 8),
		ModTime:    info.ModTime(),
		MimeType:   mime.TypeByExtension(filepath.Ext(info.Name())),
		Editable:   !info.IsDir(),
		Previewable: !info.IsDir(),
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		item.User = strconv.FormatUint(uint64(stat.Uid), 10)
		item.Group = strconv.FormatUint(uint64(stat.Gid), 10)
		if u, err := user.LookupId(item.User); err == nil {
			item.User = u.Username
		}
		if g, err := user.LookupGroupId(item.Group); err == nil {
			item.Group = g.Name
		}
	}
	return item
}
```

- [ ] **Step 4: Add archive helpers**

Create `internal/files/fs/archive.go`:

```go
package fs

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func CompressZip(paths []string, dst string) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	defer zw.Close()
	for _, src := range paths {
		if err := filepath.Walk(src, func(pathValue string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			header.Name = filepath.ToSlash(filepath.Base(src))
			if pathValue != src {
				rel, _ := filepath.Rel(filepath.Dir(src), pathValue)
				header.Name = filepath.ToSlash(rel)
			}
			writer, err := zw.CreateHeader(header)
			if err != nil {
				return err
			}
			in, err := os.Open(pathValue)
			if err != nil {
				return err
			}
			defer in.Close()
			_, err = io.Copy(writer, in)
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

func CompressTarGz(paths []string, dst string) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	gw := gzip.NewWriter(out)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()
	for _, src := range paths {
		if err := filepath.Walk(src, func(pathValue string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			header.Name = filepath.ToSlash(filepath.Base(src))
			if pathValue != src {
				rel, _ := filepath.Rel(filepath.Dir(src), pathValue)
				header.Name = filepath.ToSlash(rel)
			}
			if err := tw.WriteHeader(header); err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			in, err := os.Open(pathValue)
			if err != nil {
				return err
			}
			defer in.Close()
			_, err = io.Copy(tw, in)
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

func safeJoin(baseDir, name string) (string, error) {
	clean := filepath.Clean(filepath.Join(baseDir, name))
	if clean != baseDir && !strings.HasPrefix(clean, baseDir+string(filepath.Separator)) {
		return "", os.ErrPermission
	}
	return clean, nil
}

func ExtractZip(src, dst string, overwrite bool) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		target, err := safeJoin(dst, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if _, err := os.Stat(target); err == nil && !overwrite {
			return os.ErrExist
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func ExtractTarGz(src, dst string, overwrite bool) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeJoin(dst, header.Name)
		if err != nil {
			return err
		}
		if header.FileInfo().IsDir() {
			if err := os.MkdirAll(target, header.FileInfo().Mode()); err != nil {
				return err
			}
			continue
		}
		if _, err := os.Stat(target); err == nil && !overwrite {
			return os.ErrExist
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, header.FileInfo().Mode())
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, tarReader)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
}
```

- [ ] **Step 5: Run adapter tests**

Run:

```powershell
go test ./internal/files/fs -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add internal/files/fs
git commit -m "feat(files): add local filesystem adapter"
```

## Task 4: Backend File Service

**Files:**
- Create: `internal/files/service/file_service.go`
- Create: `internal/files/service/file_service_test.go`

- [ ] **Step 1: Write service tests**

Create `internal/files/service/file_service_test.go`:

```go
package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	filemodel "github.com/GoSimplicity/AI-CloudOps/internal/files/model"
	"go.uber.org/zap"
)

func TestFileServiceListAndSave(t *testing.T) {
	root := t.TempDir()
	pathValue := filepath.Join(root, "a.txt")
	if err := os.WriteFile(pathValue, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := NewFileService(zap.NewNop(), ManagerConfig{
		Enabled: true,
		Roots: []filemodel.RootInfo{{Name: "tmp", Path: root}},
		MaxEditBytes: 1024,
		MaxPreviewBytes: 1024,
		AllowedArchiveTypes: map[string]struct{}{"zip": {}, "tar.gz": {}},
	})
	list, err := svc.List(context.Background(), filemodel.ListRequest{TargetRequest: filemodel.TargetRequest{TargetType: "local"}, Path: root, Page: 1, Size: 10})
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 {
		t.Fatalf("List total = %d, want 1", list.Total)
	}
	if err := svc.Save(context.Background(), filemodel.SaveRequest{TargetRequest: filemodel.TargetRequest{TargetType: "local"}, Path: pathValue, Content: "new"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(pathValue)
	if string(data) != "new" {
		t.Fatalf("file content = %q", data)
	}
}

func TestFileServiceRejectsNonLocalTarget(t *testing.T) {
	root := t.TempDir()
	svc := NewFileService(zap.NewNop(), ManagerConfig{Enabled: true, Roots: []filemodel.RootInfo{{Name: "tmp", Path: root}}})
	_, err := svc.List(context.Background(), filemodel.ListRequest{TargetRequest: filemodel.TargetRequest{TargetType: "ssh"}, Path: root})
	if err != ErrFileOperationUnsupported {
		t.Fatalf("List error = %v, want ErrFileOperationUnsupported", err)
	}
}
```

- [ ] **Step 2: Run failing service tests**

Run:

```powershell
go test ./internal/files/service -run TestFileService -v
```

Expected: FAIL because `NewFileService` is undefined.

- [ ] **Step 3: Implement service**

Create `internal/files/service/file_service.go`:

```go
package service

import (
	"context"
	"os"
	"path/filepath"
	"runtime"

	localfs "github.com/GoSimplicity/AI-CloudOps/internal/files/fs"
	filemodel "github.com/GoSimplicity/AI-CloudOps/internal/files/model"
	"go.uber.org/zap"
)

type FileService interface {
	Roots(ctx context.Context) (filemodel.RootsResponse, error)
	List(ctx context.Context, req filemodel.ListRequest) (filemodel.ListResponse, error)
	Content(ctx context.Context, req filemodel.ContentRequest) (filemodel.FileInfo, error)
	Save(ctx context.Context, req filemodel.SaveRequest) error
	Create(ctx context.Context, req filemodel.CreateRequest) error
	Rename(ctx context.Context, req filemodel.RenameRequest) error
	Delete(ctx context.Context, req filemodel.DeleteRequest) error
	Move(ctx context.Context, req filemodel.MoveRequest) error
}

type fileService struct {
	l      *zap.Logger
	config ManagerConfig
	policy *PathPolicy
	fs     *localfs.LocalFS
}

func NewFileService(l *zap.Logger, config ManagerConfig) FileService {
	return &fileService{
		l: l,
		config: config,
		policy: NewPathPolicy(config),
		fs: localfs.NewLocalFS(),
	}
}

func ProvideFileManagerConfig() ManagerConfig {
	return NewManagerConfigFromGlobal()
}

func (s *fileService) ensureLocal(target filemodel.TargetRequest) error {
	if target.TargetType == "" || target.TargetType == filemodel.TargetTypeLocal {
		return nil
	}
	return ErrFileOperationUnsupported
}

func (s *fileService) Roots(ctx context.Context) (filemodel.RootsResponse, error) {
	return filemodel.RootsResponse{
		Enabled: s.config.Enabled,
		AllowFullDisk: s.config.AllowFullDisk,
		OS: runtime.GOOS,
		PathSeparator: string(filepath.Separator),
		Roots: s.config.Roots,
	}, nil
}

func (s *fileService) List(ctx context.Context, req filemodel.ListRequest) (filemodel.ListResponse, error) {
	if err := s.ensureLocal(req.TargetRequest); err != nil {
		return filemodel.ListResponse{}, err
	}
	pathValue, err := s.policy.Resolve(req.Path)
	if err != nil {
		return filemodel.ListResponse{}, err
	}
	items, total, err := s.fs.List(pathValue, localfs.ListOptions{
		Page: req.Page, Size: req.Size, Search: req.Search, ShowHidden: req.ShowHidden, SortBy: req.SortBy, SortOrder: req.SortOrder,
	})
	if err != nil {
		return filemodel.ListResponse{}, err
	}
	return filemodel.ListResponse{Path: pathValue, Items: items, Total: total}, nil
}

func (s *fileService) Content(ctx context.Context, req filemodel.ContentRequest) (filemodel.FileInfo, error) {
	if err := s.ensureLocal(req.TargetRequest); err != nil {
		return filemodel.FileInfo{}, err
	}
	pathValue, err := s.policy.Resolve(req.Path)
	if err != nil {
		return filemodel.FileInfo{}, err
	}
	info, err := s.fs.Info(pathValue)
	if err != nil {
		if os.IsNotExist(err) {
			return filemodel.FileInfo{}, ErrFileNotFound
		}
		return filemodel.FileInfo{}, err
	}
	content, err := s.fs.ReadText(pathValue, s.config.MaxPreviewBytes)
	if err != nil {
		info.Editable = false
		info.Previewable = false
		return info, nil
	}
	info.Content = content
	return info, nil
}

func (s *fileService) Save(ctx context.Context, req filemodel.SaveRequest) error {
	if err := s.ensureLocal(req.TargetRequest); err != nil {
		return err
	}
	pathValue, err := s.policy.Resolve(req.Path)
	if err != nil {
		return err
	}
	if int64(len(req.Content)) > s.config.MaxEditBytes {
		return ErrFileTooLarge
	}
	return s.fs.WriteText(pathValue, req.Content)
}

func (s *fileService) Create(ctx context.Context, req filemodel.CreateRequest) error {
	if err := s.ensureLocal(req.TargetRequest); err != nil {
		return err
	}
	pathValue, err := s.policy.ResolveParentForCreate(req.Path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(pathValue); err == nil {
		return ErrFileAlreadyExists
	}
	return s.fs.Create(pathValue, req.IsDir)
}

func (s *fileService) Rename(ctx context.Context, req filemodel.RenameRequest) error {
	if err := s.ensureLocal(req.TargetRequest); err != nil {
		return err
	}
	oldPath, err := s.policy.Resolve(req.Path)
	if err != nil {
		return err
	}
	newPath := filepath.Join(filepath.Dir(oldPath), req.NewName)
	newPath, err = s.policy.ResolveParentForCreate(newPath)
	if err != nil {
		return err
	}
	return s.fs.Rename(oldPath, newPath)
}

func (s *fileService) Delete(ctx context.Context, req filemodel.DeleteRequest) error {
	if err := s.ensureLocal(req.TargetRequest); err != nil {
		return err
	}
	pathValue, err := s.policy.Resolve(req.Path)
	if err != nil {
		return err
	}
	return s.fs.Delete(pathValue)
}

func (s *fileService) Move(ctx context.Context, req filemodel.MoveRequest) error {
	if err := s.ensureLocal(req.TargetRequest); err != nil {
		return err
	}
	targetPath, err := s.policy.Resolve(req.TargetPath)
	if err != nil {
		return err
	}
	for _, item := range req.Paths {
		src, err := s.policy.Resolve(item)
		if err != nil {
			return err
		}
		dst := filepath.Join(targetPath, filepath.Base(src))
		if req.Operation == "copy" {
			if err := s.fs.Copy(src, dst, req.Overwrite); err != nil {
				return err
			}
			continue
		}
		if req.Operation == "move" {
			if err := s.fs.Rename(src, dst); err != nil {
				return err
			}
			continue
		}
		return ErrFileOperationUnsupported
	}
	return nil
}
```

- [ ] **Step 4: Run service tests**

Run:

```powershell
go test ./internal/files/service -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/files/service/file_service.go internal/files/service/file_service_test.go
git commit -m "feat(files): add file manager service"
```

## Task 5: Backend Handler And DI Wiring

**Files:**
- Create: `internal/files/api/handler.go`
- Modify: `pkg/di/wire.go`
- Modify: `pkg/di/web.go`
- Modify: `pkg/di/wire_gen.go`

- [ ] **Step 1: Create Gin handler**

Create `internal/files/api/handler.go`:

```go
package api

import (
	"net/http"
	"path/filepath"

	filemodel "github.com/GoSimplicity/AI-CloudOps/internal/files/model"
	"github.com/GoSimplicity/AI-CloudOps/internal/files/service"
	"github.com/GoSimplicity/AI-CloudOps/pkg/base"
	"github.com/gin-gonic/gin"
)

type FileHandler struct {
	svc service.FileService
}

func NewFileHandler(svc service.FileService) *FileHandler {
	return &FileHandler{svc: svc}
}

func (h *FileHandler) RegisterRouters(server *gin.Engine) {
	group := server.Group("/api/files")
	group.GET("/roots", h.Roots)
	group.POST("/list", h.List)
	group.POST("/content", h.Content)
	group.POST("/save", h.Save)
	group.POST("/create", h.Create)
	group.POST("/rename", h.Rename)
	group.POST("/delete", h.Delete)
	group.POST("/move", h.Move)
	group.GET("/download", h.Download)
}

func (h *FileHandler) Roots(ctx *gin.Context) {
	base.HandleRequest(ctx, nil, func() (interface{}, error) {
		return h.svc.Roots(ctx)
	})
}

func (h *FileHandler) List(ctx *gin.Context) {
	var req filemodel.ListRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return h.svc.List(ctx, req)
	})
}

func (h *FileHandler) Content(ctx *gin.Context) {
	var req filemodel.ContentRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return h.svc.Content(ctx, req)
	})
}

func (h *FileHandler) Save(ctx *gin.Context) {
	var req filemodel.SaveRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.Save(ctx, req)
	})
}

func (h *FileHandler) Create(ctx *gin.Context) {
	var req filemodel.CreateRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.Create(ctx, req)
	})
}

func (h *FileHandler) Rename(ctx *gin.Context) {
	var req filemodel.RenameRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.Rename(ctx, req)
	})
}

func (h *FileHandler) Delete(ctx *gin.Context) {
	var req filemodel.DeleteRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.Delete(ctx, req)
	})
}

func (h *FileHandler) Move(ctx *gin.Context) {
	var req filemodel.MoveRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.Move(ctx, req)
	})
}

func (h *FileHandler) Download(ctx *gin.Context) {
	pathValue := ctx.Query("path")
	if pathValue == "" {
		base.BadRequestError(ctx, "path is required")
		return
	}
	ctx.Header("Content-Disposition", "attachment; filename="+filepath.Base(pathValue))
	ctx.File(pathValue)
	ctx.Status(http.StatusOK)
}
```

- [ ] **Step 2: Wire the handler and service**

Modify `pkg/di/wire.go` imports to include:

```go
filesHandler "github.com/GoSimplicity/AI-CloudOps/internal/files/api"
filesService "github.com/GoSimplicity/AI-CloudOps/internal/files/service"
```

Add to `HandlerSet`:

```go
filesHandler.NewFileHandler,
```

Add to `ServiceSet`:

```go
filesService.NewFileService,
filesService.ProvideFileManagerConfig,
```

- [ ] **Step 3: Register routes in web.go**

Modify `pkg/di/web.go` imports to include:

```go
filesApi "github.com/GoSimplicity/AI-CloudOps/internal/files/api"
```

Add a parameter to `InitGinServer`:

```go
fileHdl *filesApi.FileHandler,
```

Call route registration before returning:

```go
fileHdl.RegisterRouters(server)
```

- [ ] **Step 4: Regenerate Wire output**

Run:

```powershell
go generate ./pkg/di
```

Expected: `pkg/di/wire_gen.go` updates and compiles.

- [ ] **Step 5: Run backend tests**

Run:

```powershell
go test ./internal/files/... ./pkg/di
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add internal/files/api pkg/di/wire.go pkg/di/web.go pkg/di/wire_gen.go
git commit -m "feat(files): expose file manager api"
```

## Task 6: Backend Upload, chmod/chown, Archive Completion

**Files:**
- Modify: `internal/files/model/types.go`
- Modify: `internal/files/fs/local.go`
- Modify: `internal/files/fs/archive.go`
- Modify: `internal/files/service/file_service.go`
- Modify: `internal/files/api/handler.go`
- Test: `internal/files/service/file_service_test.go`

- [ ] **Step 1: Extend service interface and handler routes**

Add these methods to `FileService` in `internal/files/service/file_service.go`:

```go
Chmod(ctx context.Context, req filemodel.ChmodRequest) error
Chown(ctx context.Context, req filemodel.ChownRequest) error
Compress(ctx context.Context, req filemodel.CompressRequest) error
Decompress(ctx context.Context, req filemodel.DecompressRequest) error
ResolveUploadPath(ctx context.Context, target filemodel.TargetRequest, dir string, filename string) (string, error)
```

Add routes in `internal/files/api/handler.go`:

```go
group.POST("/chmod", h.Chmod)
group.POST("/chown", h.Chown)
group.POST("/compress", h.Compress)
group.POST("/decompress", h.Decompress)
group.POST("/upload", h.Upload)
```

- [ ] **Step 2: Implement chmod/chown in local adapter**

Add to `internal/files/fs/local.go`:

```go
func (l *LocalFS) Chmod(pathValue string, mode uint32) error {
	return os.Chmod(pathValue, os.FileMode(mode))
}

func (l *LocalFS) Chown(pathValue string, uid int, gid int) error {
	return os.Chown(pathValue, uid, gid)
}
```

- [ ] **Step 3: Implement service operations**

Add to `internal/files/service/file_service.go`:

```go
func (s *fileService) Chmod(ctx context.Context, req filemodel.ChmodRequest) error {
	if runtime.GOOS == "windows" {
		return ErrFileOperationUnsupported
	}
	if err := s.ensureLocal(req.TargetRequest); err != nil {
		return err
	}
	pathValue, err := s.policy.Resolve(req.Path)
	if err != nil {
		return err
	}
	return s.fs.Chmod(pathValue, req.Mode)
}

func (s *fileService) Chown(ctx context.Context, req filemodel.ChownRequest) error {
	if runtime.GOOS == "windows" {
		return ErrFileOperationUnsupported
	}
	return ErrFileOperationUnsupported
}

func (s *fileService) Compress(ctx context.Context, req filemodel.CompressRequest) error {
	if err := s.ensureLocal(req.TargetRequest); err != nil {
		return err
	}
	if _, ok := s.config.AllowedArchiveTypes[req.Type]; !ok {
		return ErrFileArchiveUnsupported
	}
	targetDir, err := s.policy.Resolve(req.TargetPath)
	if err != nil {
		return err
	}
	dst := filepath.Join(targetDir, req.Name)
	if _, err := os.Stat(dst); err == nil && !req.Overwrite {
		return ErrFileAlreadyExists
	}
	paths := make([]string, 0, len(req.Paths))
	for _, item := range req.Paths {
		resolved, err := s.policy.Resolve(item)
		if err != nil {
			return err
		}
		paths = append(paths, resolved)
	}
	if req.Type == filemodel.ArchiveZip {
		return localfs.CompressZip(paths, dst)
	}
	if req.Type == filemodel.ArchiveTarGz {
		return localfs.CompressTarGz(paths, dst)
	}
	return ErrFileArchiveUnsupported
}

func (s *fileService) Decompress(ctx context.Context, req filemodel.DecompressRequest) error {
	return ErrFileOperationUnsupported
}
```

Add these imports to `internal/files/service/file_service.go` if they are not already present:

```go
import (
	"os/user"
	"strconv"
)
```

Replace `Chown` and `Decompress` with complete implementations:

```go
func (s *fileService) Chown(ctx context.Context, req filemodel.ChownRequest) error {
	if runtime.GOOS == "windows" {
		return ErrFileOperationUnsupported
	}
	if err := s.ensureLocal(req.TargetRequest); err != nil {
		return err
	}
	pathValue, err := s.policy.Resolve(req.Path)
	if err != nil {
		return err
	}
	u, err := user.Lookup(req.User)
	if err != nil {
		return err
	}
	g, err := user.LookupGroup(req.Group)
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return err
	}
	return s.fs.Chown(pathValue, uid, gid)
}

func (s *fileService) Decompress(ctx context.Context, req filemodel.DecompressRequest) error {
	if err := s.ensureLocal(req.TargetRequest); err != nil {
		return err
	}
	if _, ok := s.config.AllowedArchiveTypes[req.Type]; !ok {
		return ErrFileArchiveUnsupported
	}
	src, err := s.policy.Resolve(req.Path)
	if err != nil {
		return err
	}
	dst, err := s.policy.Resolve(req.TargetPath)
	if err != nil {
		return err
	}
	if req.Type == filemodel.ArchiveZip {
		return localfs.ExtractZip(src, dst, req.Overwrite)
	}
	if req.Type == filemodel.ArchiveTarGz {
		return localfs.ExtractTarGz(src, dst, req.Overwrite)
	}
	return ErrFileArchiveUnsupported
}

func (s *fileService) ResolveUploadPath(ctx context.Context, target filemodel.TargetRequest, dir string, filename string) (string, error) {
	if err := s.ensureLocal(target); err != nil {
		return "", err
	}
	if filename == "" || filepath.Base(filename) != filename {
		return "", ErrFileInvalidName
	}
	targetDir, err := s.policy.Resolve(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(targetDir, filename), nil
}
```

- [ ] **Step 4: Add handler methods**

Add to `internal/files/api/handler.go`:

```go
func (h *FileHandler) Chmod(ctx *gin.Context) {
	var req filemodel.ChmodRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.Chmod(ctx, req)
	})
}

func (h *FileHandler) Chown(ctx *gin.Context) {
	var req filemodel.ChownRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.Chown(ctx, req)
	})
}

func (h *FileHandler) Compress(ctx *gin.Context) {
	var req filemodel.CompressRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.Compress(ctx, req)
	})
}

func (h *FileHandler) Decompress(ctx *gin.Context) {
	var req filemodel.DecompressRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.Decompress(ctx, req)
	})
}

func (h *FileHandler) Upload(ctx *gin.Context) {
	targetPath := ctx.PostForm("path")
	if targetPath == "" {
		base.BadRequestError(ctx, "path is required")
		return
	}
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		base.BadRequestError(ctx, err.Error())
		return
	}
	dst, err := h.svc.ResolveUploadPath(ctx, filemodel.TargetRequest{TargetType: ctx.PostForm("target_type")}, targetPath, fileHeader.Filename)
	if err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	if err := ctx.SaveUploadedFile(fileHeader, dst); err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	base.Success(ctx)
}
```

- [ ] **Step 5: Run compile tests**

Run:

```powershell
go test ./internal/files/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add internal/files
git commit -m "feat(files): add archive and permission endpoints"
```

## Task 7: Frontend API Client And Route

**Files:**
- Create: `../VanPanelWebUI/apps/web-antd/src/api/core/files/files.ts`
- Create: `../VanPanelWebUI/apps/web-antd/src/router/routes/modules/files.ts`

- [ ] **Step 1: Add frontend API client**

Create `../VanPanelWebUI/apps/web-antd/src/api/core/files/files.ts`:

```ts
import { requestClient } from '#/api/request';

export interface FileRoot {
  name: string;
  path: string;
}

export interface FileRootsResponse {
  enabled: boolean;
  allow_full_disk: boolean;
  os: string;
  path_separator: string;
  roots: FileRoot[];
}

export interface FileTargetRequest {
  target_type?: 'local';
  node_id?: number;
}

export interface FileInfo {
  name: string;
  path: string;
  extension: string;
  size: number;
  is_dir: boolean;
  is_symlink: boolean;
  is_hidden: boolean;
  mode: string;
  user: string;
  group: string;
  mod_time: string;
  mime_type: string;
  editable: boolean;
  previewable: boolean;
  content?: string;
}

export interface FileListRequest extends FileTargetRequest {
  path: string;
  page: number;
  size: number;
  search?: string;
  show_hidden?: boolean;
  sort_by?: string;
  sort_order?: string;
}

export interface FileListResponse {
  path: string;
  items: FileInfo[];
  total: number;
}

export interface FileContentRequest extends FileTargetRequest {
  path: string;
}

export interface FileSaveRequest extends FileTargetRequest {
  path: string;
  content: string;
}

export function getFileRootsApi() {
  return requestClient.get<FileRootsResponse>('/files/roots');
}

export function listFilesApi(data: FileListRequest) {
  return requestClient.post<FileListResponse>('/files/list', data);
}

export function getFileContentApi(data: FileContentRequest) {
  return requestClient.post<FileInfo>('/files/content', data);
}

export function saveFileContentApi(data: FileSaveRequest) {
  return requestClient.post('/files/save', data);
}

export function createFileApi(data: FileTargetRequest & { path: string; is_dir: boolean }) {
  return requestClient.post('/files/create', data);
}

export function renameFileApi(data: FileTargetRequest & { path: string; new_name: string }) {
  return requestClient.post('/files/rename', data);
}

export function deleteFileApi(data: FileTargetRequest & { path: string }) {
  return requestClient.post('/files/delete', data);
}

export function moveFileApi(data: FileTargetRequest & { paths: string[]; target_path: string; operation: 'copy' | 'move'; overwrite: boolean }) {
  return requestClient.post('/files/move', data);
}

export function downloadFileUrl(path: string) {
  return `/api/files/download?path=${encodeURIComponent(path)}`;
}
```

- [ ] **Step 2: Add route module**

Create `../VanPanelWebUI/apps/web-antd/src/router/routes/modules/files.ts`:

```ts
import type { RouteRecordRaw } from 'vue-router';

import { BasicLayout } from '#/layouts';

const routes: RouteRecordRaw[] = [
  {
    component: BasicLayout,
    meta: {
      icon: 'lucide:folder-cog',
      order: 5,
      title: '文件管理',
    },
    name: 'FileOperations',
    path: '/files',
    children: [
      {
        name: 'FileManager',
        path: '/files/manager',
        component: () => import('#/views/files/FileManager.vue'),
        meta: {
          icon: 'lucide:folder-tree',
          title: '文件工作台',
        },
      },
    ],
  },
];

export default routes;
```

- [ ] **Step 3: Run frontend type check**

Run in `../VanPanelWebUI`:

```powershell
pnpm run check:type
```

Expected: FAIL because `FileManager.vue` does not exist yet.

- [ ] **Step 4: Commit after later task creates the page**

Do not commit this task alone if type check fails. Carry these files into Task 8.

## Task 8: Frontend Workbench Page

**Files:**
- Create: `../VanPanelWebUI/apps/web-antd/src/views/files/FileManager.vue`
- Create: `../VanPanelWebUI/apps/web-antd/src/views/files/file-manager.css`
- Create: `../VanPanelWebUI/apps/web-antd/src/views/files/file-manager-utils.ts`
- Create: `../VanPanelWebUI/apps/web-antd/src/views/files/file-manager-utils.test.ts`

- [ ] **Step 1: Add utility helpers and tests**

Create `../VanPanelWebUI/apps/web-antd/src/views/files/file-manager-utils.ts`:

```ts
import type { FileInfo } from '#/api/core/files/files';

export function formatFileSize(size: number): string {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  if (size < 1024 * 1024 * 1024) return `${(size / 1024 / 1024).toFixed(1)} MB`;
  return `${(size / 1024 / 1024 / 1024).toFixed(1)} GB`;
}

export function canEditFile(file: FileInfo | null): boolean {
  return Boolean(file && !file.is_dir && file.editable && file.previewable);
}

export function joinPath(base: string, name: string, separator = '/'): string {
  const trimmed = base.endsWith(separator) ? base.slice(0, -1) : base;
  return `${trimmed}${separator}${name}`;
}
```

Create `../VanPanelWebUI/apps/web-antd/src/views/files/file-manager-utils.test.ts`:

```ts
import { describe, expect, it } from 'vitest';

import { canEditFile, formatFileSize, joinPath } from './file-manager-utils';

describe('file-manager-utils', () => {
  it('formats file sizes', () => {
    expect(formatFileSize(12)).toBe('12 B');
    expect(formatFileSize(2048)).toBe('2.0 KB');
  });

  it('guards edit mode', () => {
    expect(canEditFile(null)).toBe(false);
    expect(canEditFile({ is_dir: true, editable: true, previewable: true } as any)).toBe(false);
    expect(canEditFile({ is_dir: false, editable: true, previewable: true } as any)).toBe(true);
  });

  it('joins paths without duplicate separators', () => {
    expect(joinPath('/tmp/', 'a.txt')).toBe('/tmp/a.txt');
  });
});
```

- [ ] **Step 2: Create minimal workbench page**

Create `../VanPanelWebUI/apps/web-antd/src/views/files/FileManager.vue`:

```vue
<template>
  <div class="file-manager-page">
    <div class="file-manager-header">
      <div>
        <h1>文件工作台</h1>
        <p>管理 VanPanelBackend 本机文件，后续可扩展远程节点。</p>
      </div>
      <a-space>
        <a-select v-model:value="targetType" style="width: 140px">
          <a-select-option value="local">本机</a-select-option>
        </a-select>
        <a-button :loading="loading" @click="loadCurrentPath">刷新</a-button>
      </a-space>
    </div>

    <div class="file-manager-layout">
      <aside class="file-manager-sidebar">
        <h3>安全根目录</h3>
        <a-list :data-source="roots" size="small">
          <template #renderItem="{ item }">
            <a-list-item class="file-root-item" @click="openPath(item.path)">
              <a-list-item-meta :title="item.name" :description="item.path" />
            </a-list-item>
          </template>
        </a-list>
      </aside>

      <main class="file-manager-main">
        <div class="file-manager-toolbar">
          <a-input-search v-model:value="search" placeholder="搜索文件名" allow-clear style="max-width: 320px" @search="loadCurrentPath" />
          <a-switch v-model:checked="showHidden" checked-children="隐藏文件" un-checked-children="隐藏文件" @change="loadCurrentPath" />
          <a-button type="primary" @click="openCreate(false)">新建文件</a-button>
          <a-button @click="openCreate(true)">新建目录</a-button>
          <a-button danger :disabled="!selectedRowKeys.length" @click="confirmDelete">删除</a-button>
        </div>

        <a-breadcrumb class="file-manager-breadcrumb">
          <a-breadcrumb-item>{{ currentPath }}</a-breadcrumb-item>
        </a-breadcrumb>

        <a-table
          :columns="columns"
          :data-source="files"
          :loading="loading"
          :pagination="pagination"
          :row-selection="{ selectedRowKeys, onChange: onSelectChange }"
          row-key="path"
          @change="onTableChange"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'name'">
              <a-button type="link" @click="record.is_dir ? openPath(record.path) : previewFile(record)">
                {{ record.name }}
              </a-button>
            </template>
            <template v-else-if="column.key === 'size'">
              {{ record.is_dir ? '-' : formatFileSize(record.size) }}
            </template>
            <template v-else-if="column.key === 'action'">
              <a-space>
                <a-button size="small" @click="previewFile(record)" :disabled="record.is_dir">预览</a-button>
                <a-button size="small" @click="downloadFile(record)" :disabled="record.is_dir">下载</a-button>
              </a-space>
            </template>
          </template>
        </a-table>
      </main>
    </div>

    <a-drawer v-model:open="drawerOpen" title="文件预览" width="720">
      <a-textarea v-model:value="editorContent" :rows="20" :disabled="!canEditFile(activeFile)" />
      <template #extra>
        <a-button type="primary" :disabled="!canEditFile(activeFile)" @click="saveActiveFile">保存</a-button>
      </template>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { message, Modal } from 'ant-design-vue';

import type { FileInfo, FileRoot } from '#/api/core/files/files';
import { createFileApi, deleteFileApi, downloadFileUrl, getFileContentApi, getFileRootsApi, listFilesApi, saveFileContentApi } from '#/api/core/files/files';

import { canEditFile, formatFileSize, joinPath } from './file-manager-utils';
import './file-manager.css';

const targetType = ref<'local'>('local');
const roots = ref<FileRoot[]>([]);
const files = ref<FileInfo[]>([]);
const currentPath = ref('');
const search = ref('');
const showHidden = ref(false);
const loading = ref(false);
const selectedRowKeys = ref<string[]>([]);
const page = ref(1);
const size = ref(50);
const total = ref(0);
const drawerOpen = ref(false);
const activeFile = ref<FileInfo | null>(null);
const editorContent = ref('');

const columns = [
  { title: '名称', dataIndex: 'name', key: 'name', sorter: true },
  { title: '大小', dataIndex: 'size', key: 'size', sorter: true },
  { title: '权限', dataIndex: 'mode', key: 'mode' },
  { title: '属主', dataIndex: 'user', key: 'user' },
  { title: '用户组', dataIndex: 'group', key: 'group' },
  { title: '修改时间', dataIndex: 'mod_time', key: 'mod_time', sorter: true },
  { title: '操作', key: 'action' },
];

const pagination = computed(() => ({
  current: page.value,
  pageSize: size.value,
  total: total.value,
  showSizeChanger: true,
}));

async function loadRoots() {
  const res = await getFileRootsApi();
  roots.value = res.roots;
  if (!currentPath.value && roots.value.length > 0) {
    currentPath.value = roots.value[0].path;
  }
}

async function loadCurrentPath() {
  if (!currentPath.value) return;
  loading.value = true;
  try {
    const res = await listFilesApi({
      target_type: targetType.value,
      path: currentPath.value,
      page: page.value,
      size: size.value,
      search: search.value,
      show_hidden: showHidden.value,
      sort_by: 'name',
      sort_order: 'asc',
    });
    files.value = res.items;
    total.value = res.total;
    currentPath.value = res.path;
  } finally {
    loading.value = false;
  }
}

function openPath(path: string) {
  currentPath.value = path;
  page.value = 1;
  void loadCurrentPath();
}

function onSelectChange(keys: string[]) {
  selectedRowKeys.value = keys;
}

function onTableChange(nextPagination: any) {
  page.value = nextPagination.current;
  size.value = nextPagination.pageSize;
  void loadCurrentPath();
}

async function previewFile(file: FileInfo) {
  activeFile.value = await getFileContentApi({ target_type: targetType.value, path: file.path });
  editorContent.value = activeFile.value.content || '';
  drawerOpen.value = true;
}

async function saveActiveFile() {
  if (!activeFile.value) return;
  await saveFileContentApi({ target_type: targetType.value, path: activeFile.value.path, content: editorContent.value });
  message.success('保存成功');
  await loadCurrentPath();
}

function downloadFile(file: FileInfo) {
  window.open(downloadFileUrl(file.path), '_blank');
}

function openCreate(isDir: boolean) {
  const name = window.prompt(isDir ? '请输入目录名' : '请输入文件名');
  if (!name) return;
  void createFileApi({ target_type: targetType.value, path: joinPath(currentPath.value, name), is_dir: isDir })
    .then(loadCurrentPath);
}

function confirmDelete() {
  Modal.confirm({
    title: '确认删除选中文件？',
    content: '第一版会直接删除，不进入回收站。',
    async onOk() {
      for (const path of selectedRowKeys.value) {
        await deleteFileApi({ target_type: targetType.value, path });
      }
      selectedRowKeys.value = [];
      await loadCurrentPath();
    },
  });
}

onMounted(async () => {
  await loadRoots();
  await loadCurrentPath();
});
</script>
```

- [ ] **Step 3: Add styles**

Create `../VanPanelWebUI/apps/web-antd/src/views/files/file-manager.css`:

```css
.file-manager-page {
  padding: 24px;
}

.file-manager-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}

.file-manager-header h1 {
  margin: 0;
  font-size: 24px;
}

.file-manager-header p {
  margin: 6px 0 0;
  color: #64748b;
}

.file-manager-layout {
  display: grid;
  grid-template-columns: 280px 1fr;
  gap: 16px;
}

.file-manager-sidebar,
.file-manager-main {
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  padding: 16px;
}

.file-root-item {
  cursor: pointer;
}

.file-root-item:hover {
  background: #f8fafc;
}

.file-manager-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
  margin-bottom: 12px;
}

.file-manager-breadcrumb {
  margin-bottom: 12px;
}

@media (max-width: 960px) {
  .file-manager-layout {
    grid-template-columns: 1fr;
  }
}
```

- [ ] **Step 4: Run frontend utility test and type check**

Run in `../VanPanelWebUI`:

```powershell
pnpm exec vitest run apps/web-antd/src/views/files/file-manager-utils.test.ts
pnpm run check:type
```

Expected: utility test PASS and type check PASS after any local Ant Design type fixes.

- [ ] **Step 5: Commit frontend API and page**

```powershell
git -C ../VanPanelWebUI add apps/web-antd/src/api/core/files/files.ts apps/web-antd/src/router/routes/modules/files.ts apps/web-antd/src/views/files
git -C ../VanPanelWebUI commit -m "feat(files): add file manager workbench"
```

## Task 9: Validation And Documentation

**Files:**
- Modify: `docs/superpowers/specs/2026-04-29-file-manager-design.md` if implementation decisions changed.
- Create: `docs/file-manager.md`

- [ ] **Step 1: Add user-facing backend notes**

Create `docs/file-manager.md`:

```markdown
# File Manager

The file manager exposes local-host file operations through `/api/files/*`.

Default safety mode restricts operations to configured `file_manager.roots`. Set `file_manager.allow_full_disk` to `true` only for trusted administrator deployments.

First release supports local target only:

```json
{ "target_type": "local", "node_id": 0 }
```

Remote service-tree nodes are reserved for a later SSH/SFTP adapter.
```

- [ ] **Step 2: Run backend verification**

Run in `VanPanelBackend`:

```powershell
go test ./internal/files/... ./pkg/di
go test ./...
```

Expected: all tests PASS. If unrelated existing tests fail, record the failing packages and exact error in the implementation summary.

- [ ] **Step 3: Run frontend verification**

Run in `VanPanelWebUI`:

```powershell
pnpm exec vitest run apps/web-antd/src/views/files/file-manager-utils.test.ts
pnpm run check:type
```

Expected: PASS.

- [ ] **Step 4: Manual smoke test**

Start backend:

```powershell
go run main.go
```

Start frontend in `../VanPanelWebUI`:

```powershell
pnpm run dev
```

Verify:

- `/files/manager` loads.
- Safe roots render.
- Directory list renders.
- Text file preview opens.
- Text file save persists changes.
- Create file and create directory work.
- Delete requires confirmation.
- Denied path returns an error.

- [ ] **Step 5: Commit docs and final verification notes**

```powershell
git add docs/file-manager.md
git commit -m "docs(files): document file manager safety model"
```

## Self-Review Checklist

- Spec coverage: local target, future remote target fields, safe roots, full-disk mode, list/tree/content/save/create/rename/delete/move/upload/download/chmod/chown/compress/decompress, frontend workbench, tests, and docs are covered by tasks.
- Completeness: all tasks provide exact files, commands, and concrete code for the planned first release.
- Type consistency: backend uses `target_type`, `node_id`, `path`, `target_path`, `sort_by`, `sort_order`; frontend uses the same JSON keys.
