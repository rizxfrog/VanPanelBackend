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

	allowed := map[string]struct{}{
		model.ArchiveZip:   {},
		model.ArchiveTarGz: {},
	}
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
