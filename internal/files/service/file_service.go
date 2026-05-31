package service

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"

	localfs "github.com/rizxfrog/VanPanelBackend/internal/files/fs"
	filemodel "github.com/rizxfrog/VanPanelBackend/internal/files/model"
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
	Chmod(ctx context.Context, req filemodel.ChmodRequest) error
	Chown(ctx context.Context, req filemodel.ChownRequest) error
	Compress(ctx context.Context, req filemodel.CompressRequest) error
	Decompress(ctx context.Context, req filemodel.DecompressRequest) error
	ResolveUploadPath(ctx context.Context, target filemodel.TargetRequest, dir string, filename string) (string, error)
}

type fileService struct {
	l      *zap.Logger
	config ManagerConfig
	policy *PathPolicy
	fs     *localfs.LocalFS
}

func NewFileService(l *zap.Logger, config ManagerConfig) FileService {
	return &fileService{
		l:      l,
		config: config,
		policy: NewPathPolicy(config),
		fs:     localfs.NewLocalFS(),
	}
}

func (s *fileService) ensureLocal(target filemodel.TargetRequest) error {
	if target.TargetType == "" || target.TargetType == filemodel.TargetTypeLocal {
		return nil
	}
	return ErrFileOperationUnsupported
}

func (s *fileService) Roots(ctx context.Context) (filemodel.RootsResponse, error) {
	return filemodel.RootsResponse{
		Enabled:       s.config.Enabled,
		AllowFullDisk: s.config.AllowFullDisk,
		OS:            runtime.GOOS,
		PathSeparator: string(filepath.Separator),
		Roots:         s.config.Roots,
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
	if info, err := os.Stat(pathValue); err == nil && !info.IsDir() {
		pathValue = filepath.Dir(pathValue)
	}
	items, total, err := s.fs.List(pathValue, localfs.ListOptions{
		Page:       req.Page,
		Size:       req.Size,
		Search:     req.Search,
		ShowHidden: req.ShowHidden,
		SortBy:     req.SortBy,
		SortOrder:  req.SortOrder,
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
		switch req.Operation {
		case "copy":
			if err := s.fs.Copy(src, dst, req.Overwrite); err != nil {
				return err
			}
		case "move":
			if err := s.fs.Rename(src, dst); err != nil {
				return err
			}
		default:
			return ErrFileOperationUnsupported
		}
	}
	return nil
}

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
	switch req.Type {
	case filemodel.ArchiveZip:
		return localfs.CompressZip(paths, dst)
	case filemodel.ArchiveTarGz:
		return localfs.CompressTarGz(paths, dst)
	default:
		return ErrFileArchiveUnsupported
	}
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
	switch req.Type {
	case filemodel.ArchiveZip:
		return localfs.ExtractZip(src, dst, req.Overwrite)
	case filemodel.ArchiveTarGz:
		return localfs.ExtractTarGz(src, dst, req.Overwrite)
	default:
		return ErrFileArchiveUnsupported
	}
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
