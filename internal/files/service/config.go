package service

import (
	"os"
	"path/filepath"

	"github.com/GoSimplicity/AI-CloudOps/internal/files/model"
)

type ManagerConfig struct {
	Enabled             bool
	AllowFullDisk       bool
	Roots               []model.RootInfo
	MaxEditBytes        int64
	MaxPreviewBytes     int64
	AllowedArchiveTypes map[string]struct{}
}

type RootConfig struct {
	Name string
	Path string
}

type RawManagerConfig struct {
	Enabled             bool
	AllowFullDisk       bool
	Roots               []RootConfig
	MaxEditSizeMB       int
	MaxPreviewSizeMB    int
	AllowedArchiveTypes []string
}

func NewManagerConfig(raw RawManagerConfig) ManagerConfig {
	roots := make([]model.RootInfo, 0, len(raw.Roots))
	for _, root := range raw.Roots {
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
	if len(raw.AllowedArchiveTypes) > 0 {
		allowed = map[string]struct{}{}
		for _, item := range raw.AllowedArchiveTypes {
			allowed[item] = struct{}{}
		}
	}

	maxEditMB := raw.MaxEditSizeMB
	if maxEditMB <= 0 {
		maxEditMB = 5
	}
	maxPreviewMB := raw.MaxPreviewSizeMB
	if maxPreviewMB <= 0 {
		maxPreviewMB = 10
	}

	return ManagerConfig{
		Enabled:             raw.Enabled,
		AllowFullDisk:       raw.AllowFullDisk,
		Roots:               roots,
		MaxEditBytes:        int64(maxEditMB) * 1024 * 1024,
		MaxPreviewBytes:     int64(maxPreviewMB) * 1024 * 1024,
		AllowedArchiveTypes: allowed,
	}
}
