package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rizxfrog/VanPanelBackend/internal/files/model"
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
