package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	filemodel "github.com/rizxfrog/VanPanelBackend/internal/files/model"
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

func TestFileServiceCompressAndDecompressZip(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "source.txt")
	if err := os.WriteFile(src, []byte("archive me"), 0o644); err != nil {
		t.Fatal(err)
	}
	extractDir := filepath.Join(root, "extract")
	if err := os.Mkdir(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}
	svc := NewFileService(zap.NewNop(), ManagerConfig{
		Enabled:             true,
		Roots:               []filemodel.RootInfo{{Name: "tmp", Path: root}},
		MaxEditBytes:        1024,
		MaxPreviewBytes:     1024,
		AllowedArchiveTypes: map[string]struct{}{filemodel.ArchiveZip: {}, filemodel.ArchiveTarGz: {}},
	})

	err := svc.Compress(context.Background(), filemodel.CompressRequest{
		TargetRequest: filemodel.TargetRequest{TargetType: filemodel.TargetTypeLocal},
		Paths:         []string{src},
		TargetPath:    root,
		Name:          "files.zip",
		Type:          filemodel.ArchiveZip,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = svc.Decompress(context.Background(), filemodel.DecompressRequest{
		TargetRequest: filemodel.TargetRequest{TargetType: filemodel.TargetTypeLocal},
		Path:          filepath.Join(root, "files.zip"),
		TargetPath:    extractDir,
		Type:          filemodel.ArchiveZip,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(extractDir, "source.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "archive me" {
		t.Fatalf("extracted content = %q", data)
	}
}

func TestFileServiceResolveUploadPathRejectsNestedFilename(t *testing.T) {
	root := t.TempDir()
	svc := NewFileService(zap.NewNop(), ManagerConfig{
		Enabled: true,
		Roots:   []filemodel.RootInfo{{Name: "tmp", Path: root}},
	})

	_, err := svc.ResolveUploadPath(context.Background(), filemodel.TargetRequest{TargetType: filemodel.TargetTypeLocal}, root, "../escape.txt")
	if err != ErrFileInvalidName {
		t.Fatalf("ResolveUploadPath() error = %v, want ErrFileInvalidName", err)
	}
}
