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
		l:      l,
		config: config,
		policy: NewPathPolicy(config),
		fs:     localfs.NewLocalFS(),
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
