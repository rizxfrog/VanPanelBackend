package di

import filesService "github.com/GoSimplicity/AI-CloudOps/internal/files/service"

func ProvideFileManagerConfig() filesService.ManagerConfig {
	cfg := GlobalConfig.FileManager
	roots := make([]filesService.RootConfig, 0, len(cfg.Roots))
	for _, root := range cfg.Roots {
		roots = append(roots, filesService.RootConfig{Name: root.Name, Path: root.Path})
	}
	return filesService.NewManagerConfig(filesService.RawManagerConfig{
		Enabled:             cfg.Enabled,
		AllowFullDisk:       cfg.AllowFullDisk,
		Roots:               roots,
		MaxEditSizeMB:       cfg.MaxEditSizeMB,
		MaxPreviewSizeMB:    cfg.MaxPreviewSizeMB,
		AllowedArchiveTypes: cfg.AllowedArchiveTypes,
	})
}
