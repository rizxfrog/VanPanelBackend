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
		Enabled:             true,
		Roots:               []filemodel.RootInfo{{Name: "tmp", Path: root}},
		MaxEditBytes:        1024,
		MaxPreviewBytes:     1024,
		AllowedArchiveTypes: map[string]struct{}{filemodel.ArchiveZip: {}, filemodel.ArchiveTarGz: {}},
	})

	list, err := svc.List(context.Background(), filemodel.ListRequest{
		TargetRequest: filemodel.TargetRequest{TargetType: filemodel.TargetTypeLocal},
		Path:          pathValue,
		Page:          1,
		Size:          10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 {
		t.Fatalf("List total = %d, want 1", list.Total)
	}

	if err := svc.Save(context.Background(), filemodel.SaveRequest{
		TargetRequest: filemodel.TargetRequest{TargetType: filemodel.TargetTypeLocal},
		Path:          pathValue,
		Content:       "new",
	}); err != nil {
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

	_, err := svc.List(context.Background(), filemodel.ListRequest{
		TargetRequest: filemodel.TargetRequest{TargetType: "ssh"},
		Path:          root,
	})
	if err != ErrFileOperationUnsupported {
		t.Fatalf("List error = %v, want ErrFileOperationUnsupported", err)
	}
}
